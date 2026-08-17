# ⚒ DelayForge

**Advanced Network Impairment Tool for Windows — Zero Dependencies**

DelayForge is a lightweight, single-binary tool that simulates degraded network conditions on Windows — adding latency, jitter, packet loss, duplication, reordering, corruption, and bandwidth throttling to your network traffic.

> **Inspired by [Clumsy](https://github.com/jagt/clumsy)** — reimagined with a web-based UI, Go performance, and extended capabilities.

![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)
![Platform: Windows](https://img.shields.io/badge/Platform-Windows-lightgrey.svg)
![Size: ~6MB](https://img.shields.io/badge/Size-~6MB-brightgreen.svg)

## Features

### 🎛 Seven Network Impairment Types

| Effect | Description |
|--------|-------------|
| **⏱ Latency** | Add fixed delay (0–3000ms) to all matching packets |
| **〰 Jitter** | Random ±variation on top of base latency |
| **✕ Packet Loss** | Drop packets at configurable probability (0–100%) |
| **⧉ Duplicate** | Send copies of packets (0–50%) |
| **⇅ Reorder** | Shuffle packet order (0–50%) |
| **⚡ Tamper** | Flip random bytes in packet payload to simulate corruption |
| **🔒 Throttle** | Bandwidth limiting via token bucket (0–10,000 kbps) |

All effects can be **combined simultaneously** and adjusted in real-time.

### 🔍 Fine-Grained Filtering

- **Direction**: Both / Outbound only / Inbound only
- **Protocol**: All / TCP / UDP / ICMP
- **IP Address / CIDR**: Target specific IPs or subnets
- **Port**: Target specific ports

Live WinDivert filter preview shown in the UI.

### 📊 Real-Time Statistics

- Packets processed / bytes transferred
- Per-effect counters (delayed, dropped, duplicated, reordered, tampered)
- Live queue depth monitoring

## Quick Start

### Download

1. Go to [Releases](../../releases) and download `DelayForge.exe`
2. **Right-click → Run as Administrator** (required for kernel-level packet interception)
3. Browser opens automatically at `http://127.0.0.1:8380`
4. Adjust parameters and click **Start**

**Single file, no installation, no runtime required.**

### Build from Source

Prerequisites: [Go 1.21+](https://go.dev/dl/)

```bash
git clone https://github.com/cl271-7264/DelayForge.git
cd DelayForge/cmd/delayforge
go build -ldflags="-s -w" -o DelayForge.exe .
```

You also need `WinDivert.dll` and `WinDivert64.sys` in the same directory as the exe (or in `webui/windivert/` for embedding).

## How It Works

DelayForge uses [WinDivert](https://github.com/basil00/WinDivert) to intercept packets at the kernel level. Matched packets are held in userspace queues and re-injected after the configured delay, or dropped/duplicated/corrupted according to your settings.

```
Application → Network Stack → WinDivert Driver → DelayForge Engine → Real Network
                    ↑                                    ↓
                    └──── Re-injected packets ───────────┘
```

## Usage Tips

- **Testing web apps**: Latency 200ms + Jitter 50ms + Loss 2% (simulate 3G)
- **Testing game netcode**: Latency 100ms + Jitter 20ms + Loss 0.5%
- **Stress testing**: Latency 500ms + Loss 10% + Throttle 50kbps
- **Debugging race conditions**: Reorder 10% + Duplicate 5%

## Technical Details

- **Language**: Go (static binary, no runtime dependencies)
- **Driver**: WinDivert 2.2.2 (Microsoft WHQL signed, embedded)
- **UI**: Web-based (served on localhost, opens in browser)
- **Size**: ~6.4MB (single file, self-contained)

## License

This project is licensed under the [GNU General Public License v3.0](LICENSE).

## Acknowledgments

- [Clumsy](https://github.com/jagt/clumsy) — the original Windows network impairment tool
- [WinDivert](https://github.com/basil00/WinDivert) — Windows packet interception driver
