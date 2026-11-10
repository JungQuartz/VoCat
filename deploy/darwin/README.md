# VoCat macOS 平台部署与运行指南

本项目基于官方 MengMengCode/VoCat 扩展支持 macOS (Apple Silicon Darwin)。

## 适配特性
1. **硬件与监控指标采集 (`host_stats_darwin.go`)**：
   - 适配 Apple Silicon 芯片型号读取与主板型号识别；
   - 动态追踪内核 CPU 使用率；
   - 针对 macOS 内存机制优化可用内存计算（剔除 Inactive/Purgeable 页面）；
   - 正确统计 APFS 容器与磁盘利用率。
2. **设备发现与串口适配 (`default_discovery_darwin.go`)**：
   - 自动发现 `/tmp/ttyQuectelModem` 虚拟串口桥接设备。
3. **USB-PTY 免驱动端点桥接 (`usb_pty_bridge.py`)**：
   - macOS 默认由 AppleUserECM 占用 4G 模组以太网端点，本桥接工具直接通过 Bulk 端点读写 AT 指令并创建 PTY 虚拟串口。
4. **macOS 守护服务托管**：
   - `com.vocat.bridge.plist`：开机自启虚拟串口守护；
   - `com.vocat.service.plist`：开机自启 VoCat Web 服务（监听 127.0.0.1:7575）。

## 编译与安装
```bash
# 1. 编译前端（如需修改 UI）
cd web && npm install && npm run build

# 2. 编译原生后端
cd ..
go build -o ~/.vocat/bin/vocat ./cmd/vocat
```
