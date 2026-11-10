//go:build darwin

package modem

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"go.bug.st/serial"
)

type darwinDiscoverer struct {
	options DiscoveryOptions
}

func NewSystemDiscoverer() Discoverer {
	return NewSystemDiscovererWithOptions(DiscoveryOptions{})
}

func NewSystemDiscovererWithOptions(opts DiscoveryOptions) Discoverer {
	return darwinDiscoverer{options: opts}
}

func (d darwinDiscoverer) Discover(ctx context.Context) ([]Candidate, error) {
	ports, _ := serial.GetPortsList()

	// 检查虚拟 PTY 桥接串口
	if _, err := os.Stat("/tmp/ttyQuectelModem"); err == nil {
		ports = append([]string{"/tmp/ttyQuectelModem"}, ports...)
	}

	var candidates []Candidate
	seen := make(map[string]bool)

	for _, p := range ports {
		name := filepath.Base(p)
		if strings.Contains(name, "Bluetooth") || strings.Contains(name, "debug-console") {
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true

		candidates = append(candidates, Candidate{
			ID:           "darwin-" + sanitizeID(name),
			VendorID:     "2c7c",
			ProductID:    "6005",
			Manufacturer: "Quectel",
			Product:      "Quectel LTE Modem",
			USBPath:      p,
			ATPort: Port{
				Path:       p,
				StablePath: p,
				Name:       name,
				Role:       PortRoleAT,
			},
			Ports: []Port{
				{Path: p, StablePath: p, Name: name, Role: PortRoleAT},
			},
		})
	}
	return candidates, nil
}
