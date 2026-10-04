package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/config"
	"github.com/Ultra2000/netuipilot/internal/nm"
	"github.com/Ultra2000/netuipilot/internal/style"
)

type IfacePanel struct {
	client      *nm.Client
	cfg         config.Config
	devices     []nm.Device
	cursor      int
	showVirtual bool
	width       int
	height      int
	err         error
}

type devicesRefreshMsg struct {
	devices []nm.Device
	err     error
}

func NewIfacePanel(client *nm.Client, cfg config.Config) IfacePanel {
	return IfacePanel{
		client: client,
		cfg:    cfg,
	}
}

func (p IfacePanel) Init() tea.Cmd {
	return p.refresh
}

func (p IfacePanel) Update(msg tea.Msg) (IfacePanel, tea.Cmd) {
	switch msg := msg.(type) {
	case devicesRefreshMsg:
		p.err = msg.err
		if msg.err == nil {
			p.devices = msg.devices
		}
		return p, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if p.cursor < len(p.devices)-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		case "r":
			return p, p.refresh
		case "v":
			p.showVirtual = !p.showVirtual
			p.cursor = 0
			return p, p.refresh
		case "enter":
			if len(p.devices) > 0 && p.cursor < len(p.devices) {
				dev := p.devices[p.cursor]
				if dev.State == nm.DeviceStateDisconnected {
					return p, p.activateDevice(dev)
				}
			}
		case "d":
			if len(p.devices) > 0 && p.cursor < len(p.devices) {
				dev := p.devices[p.cursor]
				if dev.State == nm.DeviceStateActivated {
					return p, p.deactivateDevice(dev)
				}
			}
		}
	}
	return p, nil
}

func (p IfacePanel) View() string {
	var b strings.Builder

	header := style.TitleStyle.Render("🔌 Interfaces")
	b.WriteString(header)
	b.WriteString("\n")
	desc := lipgloss.NewStyle().Foreground(style.Muted).Render("Network interfaces — toggle virtual (docker, veth, bridge) with 'v'")
	b.WriteString(desc)
	b.WriteString("\n\n")

	if p.err != nil {
		b.WriteString(style.DisconnectedStyle.Render(fmt.Sprintf("Error: %v", p.err)))
		b.WriteString("\n\n")
	}

	if len(p.devices) == 0 {
		b.WriteString(style.SubtitleStyle.Render("No devices found. Press 'r' to refresh."))
		return b.String()
	}

	headerRow := style.HeaderStyle.Render(
		fmt.Sprintf("  %-16s %-12s %-18s %-18s", "INTERFACE", "TYPE", "STATE", "MAC"),
	)
	b.WriteString(headerRow)
	b.WriteString("\n")

	for i, dev := range p.devices {
		icon := style.StatusIcon(dev.State == nm.DeviceStateActivated)
		typeName := nm.DeviceTypeName(dev.Type)
		stateName := nm.DeviceStateName(dev.State)

		stateStyled := stateName
		if dev.State == nm.DeviceStateActivated {
			stateStyled = style.ConnectedStyle.Render(stateName)
		} else if dev.State == nm.DeviceStateFailed {
			stateStyled = style.DisconnectedStyle.Render(stateName)
		}

		row := fmt.Sprintf("%s %-16s %-12s %-18s %-18s",
			icon,
			dev.Name,
			typeName,
			stateStyled,
			dev.HWAddr,
		)

		if i == p.cursor {
			row = style.SelectedRowStyle.Render(row)
		} else {
			row = style.RowStyle.Render(row)
		}

		b.WriteString(row)
		b.WriteString("\n")
	}

	if p.cursor < len(p.devices) {
		dev := p.devices[p.cursor]
		b.WriteString("\n")
		detailTitle := lipgloss.NewStyle().Bold(true).Foreground(style.Primary).Render(dev.Name + " details")
		b.WriteString(detailTitle)
		b.WriteString("\n")

		detailLabel := lipgloss.NewStyle().Foreground(style.Muted)

		if dev.IP4Addr != "" {
			b.WriteString(fmt.Sprintf("  %s %s\n", detailLabel.Render("IP:"), dev.IP4Addr))
		}
		if dev.Subnet != "" {
			b.WriteString(fmt.Sprintf("  %s %s\n", detailLabel.Render("Subnet:"), dev.Subnet))
		}
		if dev.Gateway != "" {
			b.WriteString(fmt.Sprintf("  %s %s\n", detailLabel.Render("Gateway:"), dev.Gateway))
		}
		if dev.MTU > 0 {
			b.WriteString(fmt.Sprintf("  %s %d\n", detailLabel.Render("MTU:"), dev.MTU))
		}
		if dev.IP4Addr == "" && dev.State != nm.DeviceStateActivated {
			b.WriteString(style.SubtitleStyle.Render("  Not connected"))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	vLabel := "show virtual"
	if p.showVirtual {
		vLabel = "hide virtual"
	}
	help := fmt.Sprintf("%s refresh  %s connect  %s disconnect  %s %s  %s/%s navigate",
		style.HelpKeyStyle.Render("r"),
		style.HelpKeyStyle.Render("↵"),
		style.HelpKeyStyle.Render("d"),
		style.HelpKeyStyle.Render("v"),
		vLabel,
		style.HelpKeyStyle.Render("j"),
		style.HelpKeyStyle.Render("k"),
	)
	b.WriteString(help)

	return b.String()
}

func (p *IfacePanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p IfacePanel) refresh() tea.Msg {
	if p.client == nil {
		return devicesRefreshMsg{err: fmt.Errorf("no NetworkManager connection")}
	}

	devices, err := p.client.GetDevices()
	if err != nil {
		return devicesRefreshMsg{err: err}
	}
	var filtered []nm.Device
	for _, d := range devices {
		if p.cfg.IsHidden(d.Name) {
			continue
		}
		if !p.showVirtual && isVirtualIface(d.Name, d.Type) {
			continue
		}
		filtered = append(filtered, d)
	}
	return devicesRefreshMsg{devices: filtered}
}

func (p IfacePanel) activateDevice(dev nm.Device) func() tea.Msg {
	return func() tea.Msg {
		conns, err := p.client.GetSavedConnections()
		if err != nil {
			return devicesRefreshMsg{err: err}
		}

		typeName := ""
		switch dev.Type {
		case nm.DeviceTypeEthernet:
			typeName = "802-3-ethernet"
		case nm.DeviceTypeWiFi:
			typeName = "802-11-wireless"
		}

		for _, conn := range conns {
			if conn.Type == typeName {
				err := p.client.ActivateConnection(conn.Path, dev.Path)
				if err != nil {
					return devicesRefreshMsg{err: err}
				}
				devices, _ := p.client.GetDevices()
				return devicesRefreshMsg{devices: devices}
			}
		}

		return devicesRefreshMsg{err: fmt.Errorf("no saved connection for %s", dev.Name)}
	}
}

func (p IfacePanel) deactivateDevice(dev nm.Device) func() tea.Msg {
	return func() tea.Msg {
		activeConn, err := p.client.GetActiveConnectionForDevice(dev.Path)
		if err != nil {
			return devicesRefreshMsg{err: err}
		}
		err = p.client.DeactivateConnection(activeConn)
		if err != nil {
			return devicesRefreshMsg{err: err}
		}
		devices, _ := p.client.GetDevices()
		return devicesRefreshMsg{devices: devices}
	}
}

func isVirtualIface(name string, devType nm.DeviceType) bool {
	if devType == nm.DeviceTypeBridge {
		return true
	}
	virtualPrefixes := []string{"docker", "veth", "br-", "virbr", "vbox", "vmnet", "tun", "tap"}
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
