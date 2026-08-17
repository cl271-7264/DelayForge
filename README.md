# ⚒ DelayForge

**Advanced Network Impairment Tool for Windows**

DelayForge is an open-source tool that simulates degraded network conditions on Windows — adding latency, jitter, packet loss, duplication, reordering, corruption, and bandwidth throttling to your network traffic. Built for developers, QA engineers, and network researchers who need to test how their applications perform under real-world network stress.

> **Inspired by [Clumsy](https://github.com/jagt/clumsy)** — reimagined with a modern WPF UI, single-file deployment, and extended capabilities.

![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)
![Platform: Windows 10/11](https://img.shields.io/badge/Platform-Windows%2010%2F11-lightgrey.svg)
![.NET 9](https://img.shields.io/badge/.NET-9-purple.svg)

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
- **IP Address / CIDR**: Target specific IPs or subnets (comma-separated)
- **Port**: Target specific ports (comma-separated)
- **Process Name**: Filter by application (e.g. `chrome.exe`)

Live WinDivert filter preview shown in the UI.

### 📊 Real-Time Statistics

- Packets processed / bytes transferred
- Per-effect counters (delayed, dropped, duplicated, reordered, tampered)
- Live queue depth monitoring

## Quick Start

### Download

1. Go to [Releases](../../releases) and download the latest `DelayForge.exe`
2. **Right-click → Run as Administrator** (required for kernel-level packet interception)
3. Adjust parameters and click **Start**

That's it — **single file, no installation, no runtime required**.

### Build from Source

Prerequisites:
- [.NET 9 SDK](https://dotnet.microsoft.com/download/dotnet/9.0)
- Windows 10/11

```bash
cd src/DelayForge
dotnet publish -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true
```

The output `DelayForge.exe` will be in the `publish/` directory.

## How It Works

DelayForge uses [WinDivert](https://github.com/basil00/WinDivert) (Windows Packet Divert) to intercept network packets at the kernel level. Matched packets are held in userspace queues and re-injected after the configured delay, or dropped/duplicated/corrupted according to your settings.

```
Application → Network Stack → WinDivert Driver → DelayForge Engine → Real Network
                    ↑                                    ↓
                    └──── Re-injected packets ───────────┘
```

## Usage Tips

- **Testing web apps**: Set latency to 200ms + 50ms jitter + 2% loss to simulate 3G
- **Testing game netcode**: Use 100ms latency + 20ms jitter + 0.5% loss
- **Stress testing**: Combine 500ms latency + 10% loss + 50kbps throttle
- **Debugging race conditions**: Use 10% reorder + 5% duplicate
- **Filter by process**: Enter `myapp.exe` in the Process Name field to only affect your app

## Technical Details

- **Driver**: WinDivert 2.2.2 (Microsoft WHQL signed)
- **Framework**: .NET 9 WPF (self-contained, single-file)
- **Architecture**: x64
- **Packet processing**: Multi-threaded with lock-free statistics, token bucket throttle, priority queue delay scheduling
- **No persistent installation**: Driver loads dynamically at runtime, unloaded on exit

## Contributing

Contributions are welcome! Please open an issue or PR.

## License

This project is licensed under the [GNU General Public License v3.0](LICENSE).

## Acknowledgments

- [Clumsy](https://github.com/jagt/clumsy) — the original Windows network impairment tool
- [WinDivert](https://github.com/basil00/WinDivert) — Windows packet interception driver
