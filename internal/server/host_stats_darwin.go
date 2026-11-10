//go:build darwin

package server

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func probeHostStatic() hostStaticInfo {
	cpu := getCmdOutput("sysctl", "-n", "machdep.cpu.brand_string")
	if cpu == "" {
		cpu = "Apple Silicon"
	}
	model := getCmdOutput("sysctl", "-n", "hw.model")
	if model == "" {
		model = "Mac"
	} else if strings.HasPrefix(model, "Mac16,") {
		model = "Apple Mac mini (" + model + ")"
	}
	memBytesStr := getCmdOutput("sysctl", "-n", "hw.memsize")
	memStr := "16 GB Unified Memory"
	if bytesVal, err := strconv.ParseUint(memBytesStr, 10, 64); err == nil && bytesVal > 0 {
		memStr = strconv.FormatUint(bytesVal/(1024*1024*1024), 10) + " GB Unified Memory"
	}

	return hostStaticInfo{
		CPUModel:    cpu,
		BoardModel:  model,
		MemoryModel: memStr,
		DiskModel:   "Apple NVMe APFS SSD",
	}
}

var (
	cpuMu           sync.Mutex
	lastCPUTick     = time.Now()
	cumTotalTicks   uint64 = 100000000
	cumIdleTicks    uint64 = 85000000
	cachedIdleRatio float64 = 0.85
	lastTopSample   time.Time
)

func updateDarwinCPUUsage() {
	if time.Since(lastTopSample) < 2*time.Second {
		return
	}
	lastTopSample = time.Now()
	out := getCmdOutput("top", "-l", "1", "-n", "0")
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "CPU usage:") {
			// CPU usage: 5.82% user, 10.61% sys, 83.56% idle
			parts := strings.Split(line, ",")
			for _, p := range parts {
				if strings.Contains(p, "idle") {
					fields := strings.Fields(p)
					if len(fields) >= 2 {
						pctStr := strings.TrimSuffix(fields[0], "%")
						if val, err := strconv.ParseFloat(pctStr, 64); err == nil && val >= 0 && val <= 100 {
							cachedIdleRatio = val / 100.0
						}
					}
				}
			}
			break
		}
	}
}

func readHostCPUTimes() (hostCPUTimes, bool) {
	cpuMu.Lock()
	defer cpuMu.Unlock()

	updateDarwinCPUUsage()

	now := time.Now()
	elapsedUs := now.Sub(lastCPUTick).Microseconds()
	if elapsedUs > 0 {
		deltaTotal := uint64(elapsedUs)
		deltaIdle := uint64(float64(deltaTotal) * cachedIdleRatio)
		cumTotalTicks += deltaTotal
		cumIdleTicks += deltaIdle
		lastCPUTick = now
	}

	return hostCPUTimes{idle: cumIdleTicks, total: cumTotalTicks}, true
}

func readHostNetTotals() (uint64, uint64, bool) {
	out := getCmdOutput("netstat", "-ibn")
	var rx, tx uint64
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		f := strings.Fields(l)
		if len(f) >= 10 && (strings.HasPrefix(f[0], "en0") || strings.HasPrefix(f[0], "en1")) {
			r, _ := strconv.ParseUint(f[6], 10, 64)
			t, _ := strconv.ParseUint(f[9], 10, 64)
			rx += r
			tx += t
		}
	}
	if rx > 0 || tx > 0 {
		return rx, tx, true
	}
	return 0, 0, false
}

func readHostMemory() (float64, uint64, uint64) {
	memBytesStr := getCmdOutput("sysctl", "-n", "hw.memsize")
	total, err := strconv.ParseUint(memBytesStr, 10, 64)
	if err != nil || total == 0 {
		total = 16 * 1024 * 1024 * 1024
	}

	out := getCmdOutput("vm_stat")
	var freePages, inactivePages, specPages, purgeablePages uint64
	for _, l := range strings.Split(out, "\n") {
		parseVal := func(prefix string) uint64 {
			if strings.HasPrefix(l, prefix) {
				parts := strings.Fields(l)
				if len(parts) >= 3 {
					numStr := strings.TrimSuffix(parts[len(parts)-1], ".")
					v, _ := strconv.ParseUint(numStr, 10, 64)
					return v
				}
			}
			return 0
		}

		if v := parseVal("Pages free:"); v > 0 {
			freePages = v
		} else if v := parseVal("Pages inactive:"); v > 0 {
			inactivePages = v
		} else if v := parseVal("Pages speculative:"); v > 0 {
			specPages = v
		} else if v := parseVal("Pages purgeable:"); v > 0 {
			purgeablePages = v
		}
	}

	// macOS available memory = free + inactive + speculative + purgeable
	availPages := freePages + inactivePages + specPages + purgeablePages
	availBytes := availPages * 16384
	if availBytes > total {
		availBytes = total / 4
	}
	usedBytes := total - availBytes
	percent := float64(usedBytes) / float64(total) * 100
	return percent, usedBytes, total
}

func readHostDisk() (float64, uint64, uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0, 0, 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	percent := 0.0
	if total > 0 {
		percent = float64(used) / float64(total) * 100
	}
	return percent, used, total
}

func getCmdOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
