# NetuiPilot

A modern, full-featured TUI network manager for Linux.

![License](https://img.shields.io/badge/license-MIT-blue)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/platform-Linux-FCC624?logo=linux&logoColor=black)

## Why NetuiPilot?

Existing tools only solve part of the problem:

| Tool | WiFi | Ethernet | VPN | DNS | Monitoring | Modern UI |
|------|------|----------|-----|-----|------------|-----------|
| `nmtui` | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `nmtui-go` | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| `wtui` | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
| **NetuiPilot** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

NetuiPilot is a **single dashboard** that replaces `nmtui` + `iftop` + `ss` + `resolvectl`.

## Features

- **Dashboard view** — all interfaces at a glance with live status
- **WiFi management** — scan, connect, disconnect, manage saved networks
- **Interface control** — Ethernet, WiFi, VPN, Bridge, all in one place
- **DNS configuration** — view and manage DNS resolvers per interface
- **Live monitoring** — real-time bandwidth (RX/TX), latency, active connections
- **Keyboard-driven** — vim-style navigation (`j`/`k`/`h`/`l`)
- **Zero dependencies** — single static binary, no Python, no Node
- **D-Bus native** — talks directly to NetworkManager, no `nmcli` parsing

## Installation

### From source

```bash
go install github.com/Ultra2000/netuipilot@latest
```

### Binary release

```bash
curl -sL https://github.com/Ultra2000/netuipilot/releases/latest/download/netuipilot-linux-amd64 -o netuipilot
chmod +x netuipilot
sudo mv netuipilot /usr/local/bin/
```

## Usage

```bash
netuipilot
```

### Keybindings

| Key | Action |
|-----|--------|
| `Tab` | Switch panel |
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Connect / expand |
| `d` | Disconnect |
| `s` | Scan WiFi |
| `m` | Toggle monitoring |
| `r` | Refresh |
| `q` / `Esc` | Quit |

## Requirements

- Linux with NetworkManager
- Terminal with 256-color or truecolor support

## Architecture

```
netuipilot/
├── cmd/netuipilot/      # Entry point
├── internal/
│   ├── ui/              # Bubbletea models & views
│   │   ├── dashboard.go # Main dashboard
│   │   ├── wifi.go      # WiFi panel
│   │   ├── iface.go     # Interface panel
│   │   └── monitor.go   # Bandwidth monitor
│   ├── nm/              # NetworkManager D-Bus client
│   ├── net/             # System network info (/proc, /sys)
│   └── style/           # Lipgloss styles
├── go.mod
└── README.md
```

## License

MIT
