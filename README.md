# NetuiPilot

A modern, full-featured TUI network manager for Linux.

![License](https://img.shields.io/badge/license-MIT-blue)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/platform-Linux-FCC624?logo=linux&logoColor=black)

## Why NetuiPilot?

Existing tools only solve part of the problem:

| Tool | WiFi | Ethernet | VPN | DNS | Monitoring | Modern UI |
|------|------|----------|-----|-----|------------|-----------|
| `nmtui` | Yes | Yes | No | No | No | No |
| `nmtui-go` | Yes | No | No | No | No | Yes |
| `wtui` | Yes | Yes | No | No | No | Yes |
| **NetuiPilot** | Yes | Yes | Yes | Yes | Yes | Yes |

NetuiPilot is a **single dashboard** that replaces `nmtui` + `iftop` + `ss` + `resolvectl`.

## Features

- **WiFi management** — scan, connect with password prompt, disconnect
- **Interface control** — IP, gateway, subnet, MTU details at a glance
- **VPN support** — WireGuard & OpenVPN profiles, toggle on/off
- **DNS resolvers** — view DNS config per interface via resolvectl
- **Live monitoring** — real-time bandwidth sparklines, active connections
- **Notifications** — alerts when interfaces connect/disconnect/fail
- **Config file** — custom theme colors, hidden interfaces
- **Keyboard-driven** — vim-style navigation (`j`/`k`)
- **Zero dependencies** — single static binary, no Python, no Node
- **D-Bus native** — talks directly to NetworkManager, no `nmcli` parsing

## Installation

### From source

```bash
git clone https://github.com/Ultra2000/NetuiPilot.git
cd NetuiPilot
make install
```

### Go install

```bash
go install github.com/Ultra2000/netuipilot/cmd/netuipilot@latest
```

### Binary release

Download from [Releases](https://github.com/Ultra2000/NetuiPilot/releases):

```bash
curl -sL https://github.com/Ultra2000/netuipilot/releases/latest/download/netuipilot_linux_amd64.tar.gz | tar xz
sudo mv netuipilot /usr/local/bin/
```

## Usage

```bash
netuipilot
```

### Keybindings

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Switch panel |
| `1`-`5` | Jump to panel |
| `j` / `k` | Move down / up |
| `Enter` | Connect / toggle |
| `d` | Disconnect |
| `s` | Scan WiFi |
| `r` | Refresh |
| `Ctrl+T` | Show/hide WiFi password |
| `q` | Quit |

## Configuration

Config file: `~/.config/netuipilot/config.json`

```json
{
  "hidden_interfaces": ["lo", "docker0"],
  "refresh_interval_ms": 1000,
  "theme": {
    "primary": "#7F77DD",
    "secondary": "#1D9E75",
    "accent": "#D85A30",
    "success": "#639922",
    "warning": "#EF9F27",
    "danger": "#E24B4A"
  }
}
```

## Requirements

- Linux with NetworkManager
- Terminal with 256-color or truecolor support

## Architecture

```
netuipilot/
├── cmd/netuipilot/      # Entry point
├── internal/
│   ├── ui/              # Bubbletea models & views
│   │   ├── dashboard.go # Main dashboard with tabs
│   │   ├── wifi.go      # WiFi panel + password prompt
│   │   ├── iface.go     # Interface details panel
│   │   ├── vpn.go       # VPN toggle panel
│   │   ├── dns.go       # DNS resolvers panel
│   │   ├── monitor.go   # Bandwidth monitor + sparklines
│   │   └── notify.go    # Connection notifications
│   ├── nm/              # NetworkManager D-Bus client
│   ├── net/             # System network info (/proc, /sys)
│   ├── config/          # Config file loader
│   └── style/           # Lipgloss theme
├── .goreleaser.yml      # Release automation
├── Makefile
└── go.mod
```

## License

MIT
