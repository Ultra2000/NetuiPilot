package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/nm"
	"github.com/Ultra2000/netuipilot/internal/style"
)

type VPNEntry struct {
	conn   nm.Connection
	active bool
}

type VPNPanel struct {
	client  *nm.Client
	entries []VPNEntry
	cursor  int
	width   int
	height  int
	err     error
}

type vpnRefreshMsg struct {
	entries []VPNEntry
	err     error
}

type vpnToggleMsg struct {
	err error
}

func NewVPNPanel(client *nm.Client) VPNPanel {
	return VPNPanel{
		client: client,
	}
}

func (v VPNPanel) Init() tea.Cmd {
	return v.refresh
}

func (v VPNPanel) Update(msg tea.Msg) (VPNPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case vpnRefreshMsg:
		v.err = msg.err
		if msg.err == nil {
			v.entries = msg.entries
		}
		return v, nil

	case vpnToggleMsg:
		v.err = msg.err
		return v, v.refresh

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if v.cursor < len(v.entries)-1 {
				v.cursor++
			}
		case "k", "up":
			if v.cursor > 0 {
				v.cursor--
			}
		case "r":
			return v, v.refresh
		case "enter":
			if len(v.entries) > 0 && v.cursor < len(v.entries) {
				entry := v.entries[v.cursor]
				if entry.active {
					return v, v.deactivate(entry)
				}
				return v, v.activate(entry)
			}
		}
	}
	return v, nil
}

func (v VPNPanel) View() string {
	var b strings.Builder

	header := style.TitleStyle.Render("🔒 VPN")
	b.WriteString(header)
	b.WriteString("\n\n")

	if v.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(style.Danger).Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	if len(v.entries) == 0 {
		b.WriteString(style.SubtitleStyle.Render("No VPN profiles found."))
		b.WriteString("\n")
		b.WriteString(style.SubtitleStyle.Render("Add VPN connections via NetworkManager or 'nmcli'."))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("%s refresh", style.HelpKeyStyle.Render("r")))
		return b.String()
	}

	for i, entry := range v.entries {
		icon := style.StatusIcon(entry.active)
		statusText := "Disconnected"
		if entry.active {
			statusText = style.ConnectedStyle.Render("Connected")
		}

		vpnType := vpnTypeName(entry.conn.Type)

		row := fmt.Sprintf("%s  %-24s %-14s %s",
			icon,
			truncate(entry.conn.ID, 22),
			vpnType,
			statusText,
		)

		if i == v.cursor {
			row = style.SelectedRowStyle.Render(row)
		} else {
			row = style.RowStyle.Render(row)
		}

		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	help := fmt.Sprintf("%s toggle  %s refresh  %s/%s navigate",
		style.HelpKeyStyle.Render("↵"),
		style.HelpKeyStyle.Render("r"),
		style.HelpKeyStyle.Render("j"),
		style.HelpKeyStyle.Render("k"),
	)
	b.WriteString(help)

	return b.String()
}

func (v *VPNPanel) SetSize(width, height int) {
	v.width = width
	v.height = height
}

func (v VPNPanel) refresh() tea.Msg {
	if v.client == nil {
		return vpnRefreshMsg{err: fmt.Errorf("no NetworkManager connection")}
	}

	conns, err := v.client.GetSavedConnections()
	if err != nil {
		return vpnRefreshMsg{err: err}
	}

	activeConns, _ := v.client.GetActiveVPNConnections()
	activeSet := make(map[string]bool)
	for _, ac := range activeConns {
		activeSet[ac] = true
	}

	var entries []VPNEntry
	for _, conn := range conns {
		if isVPNType(conn.Type) {
			entries = append(entries, VPNEntry{
				conn:   conn,
				active: activeSet[conn.ID],
			})
		}
	}

	return vpnRefreshMsg{entries: entries}
}

func (v VPNPanel) activate(entry VPNEntry) func() tea.Msg {
	return func() tea.Msg {
		devices, err := v.client.GetDevices()
		if err != nil {
			return vpnToggleMsg{err: err}
		}
		var devicePath = devices[0].Path
		err = v.client.ActivateConnection(entry.conn.Path, devicePath)
		return vpnToggleMsg{err: err}
	}
}

func (v VPNPanel) deactivate(entry VPNEntry) func() tea.Msg {
	return func() tea.Msg {
		activeConns, err := v.client.GetActiveConnections()
		if err != nil {
			return vpnToggleMsg{err: err}
		}
		for _, ac := range activeConns {
			if ac.ID == entry.conn.ID {
				err := v.client.DeactivateConnection(ac.Path)
				return vpnToggleMsg{err: err}
			}
		}
		return vpnToggleMsg{err: fmt.Errorf("active connection not found")}
	}
}

func isVPNType(connType string) bool {
	switch connType {
	case "vpn", "wireguard":
		return true
	}
	return false
}

func vpnTypeName(connType string) string {
	switch connType {
	case "vpn":
		return "OpenVPN"
	case "wireguard":
		return "WireGuard"
	default:
		return connType
	}
}
