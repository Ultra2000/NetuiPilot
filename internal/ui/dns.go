package ui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/style"
)

type dnsMode int

const (
	dnsModeView dnsMode = iota
	dnsModeChange
)

type DNSPreset struct {
	Name    string
	Servers []string
}

var dnsPresets = []DNSPreset{
	{"Cloudflare", []string{"1.1.1.1", "1.0.0.1"}},
	{"Google", []string{"8.8.8.8", "8.8.4.4"}},
	{"Quad9", []string{"9.9.9.9", "149.112.112.112"}},
	{"OpenDNS", []string{"208.67.222.222", "208.67.220.220"}},
	{"Custom", nil},
}

type DNSEntry struct {
	Interface string
	Servers   []string
	Domain    string
}

type DNSPanel struct {
	entries      []DNSEntry
	cursor       int
	mode         dnsMode
	presetCursor int
	customInput  string
	width        int
	height       int
	err          error
	success      string
}

type dnsRefreshMsg struct {
	entries []DNSEntry
	err     error
}

type dnsChangeMsg struct {
	err error
}

func NewDNSPanel() DNSPanel {
	return DNSPanel{}
}

func (d DNSPanel) Init() tea.Cmd {
	return d.refresh
}

func (d DNSPanel) Update(msg tea.Msg) (DNSPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case dnsRefreshMsg:
		d.err = msg.err
		if msg.err == nil {
			d.entries = msg.entries
		}
		return d, nil

	case dnsChangeMsg:
		if msg.err != nil {
			d.err = msg.err
			d.success = ""
		} else {
			d.err = nil
			d.success = "DNS updated successfully"
		}
		d.mode = dnsModeView
		d.customInput = ""
		return d, d.refresh

	case tea.KeyMsg:
		if d.mode == dnsModeChange {
			return d.updateChangeMode(msg)
		}
		return d.updateViewMode(msg)
	}
	return d, nil
}

func (d DNSPanel) updateViewMode(msg tea.KeyMsg) (DNSPanel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if d.cursor < len(d.entries)-1 {
			d.cursor++
		}
	case "k", "up":
		if d.cursor > 0 {
			d.cursor--
		}
	case "r":
		d.success = ""
		return d, d.refresh
	case "c":
		if len(d.entries) > 0 && d.cursor < len(d.entries) {
			entry := d.entries[d.cursor]
			if entry.Interface != "Global" && entry.Interface != "System" {
				d.mode = dnsModeChange
				d.presetCursor = 0
				d.customInput = ""
				d.success = ""
			}
		}
	case "x":
		if len(d.entries) > 0 && d.cursor < len(d.entries) {
			entry := d.entries[d.cursor]
			if entry.Interface != "Global" && entry.Interface != "System" {
				return d, d.resetDNS(entry.Interface)
			}
		}
	}
	return d, nil
}

func (d DNSPanel) updateChangeMode(msg tea.KeyMsg) (DNSPanel, tea.Cmd) {
	preset := dnsPresets[d.presetCursor]

	switch msg.String() {
	case "esc":
		d.mode = dnsModeView
		d.customInput = ""
	case "j", "down":
		if d.presetCursor < len(dnsPresets)-1 {
			d.presetCursor++
		}
	case "k", "up":
		if d.presetCursor > 0 {
			d.presetCursor--
		}
	case "enter":
		iface := d.entries[d.cursor].Interface
		if preset.Name == "Custom" {
			servers := strings.Fields(strings.ReplaceAll(d.customInput, ",", " "))
			if len(servers) > 0 {
				return d, d.changeDNS(iface, servers)
			}
		} else {
			return d, d.changeDNS(iface, preset.Servers)
		}
	case "backspace":
		if preset.Name == "Custom" && len(d.customInput) > 0 {
			d.customInput = d.customInput[:len(d.customInput)-1]
		}
	default:
		if preset.Name == "Custom" && len(msg.String()) == 1 {
			d.customInput += msg.String()
		}
	}
	return d, nil
}

func (d DNSPanel) View() string {
	var b strings.Builder

	header := style.TitleStyle.Render("🌐 DNS Resolvers")
	b.WriteString(header)
	b.WriteString("\n")
	desc := lipgloss.NewStyle().Foreground(style.Muted).Render("View, change or reset DNS servers per interface via resolvectl")
	b.WriteString(desc)
	b.WriteString("\n\n")

	if d.mode == dnsModeChange {
		return b.String() + d.changeView()
	}

	if d.success != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(style.Success).Render("● " + d.success))
		b.WriteString("\n\n")
	}

	if d.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(style.Danger).Render(fmt.Sprintf("Error: %v", d.err)))
		b.WriteString("\n\n")
	}

	if len(d.entries) == 0 {
		b.WriteString(style.SubtitleStyle.Render("No DNS configuration found. Press 'r' to refresh."))
		return b.String()
	}

	for i, entry := range d.entries {
		ifaceLabel := lipgloss.NewStyle().Bold(true).Foreground(style.Primary).Render(entry.Interface)
		if i == d.cursor {
			ifaceLabel = style.SelectedRowStyle.Render(entry.Interface)
		}
		b.WriteString(ifaceLabel)
		b.WriteString("\n")

		if entry.Domain != "" {
			domLabel := lipgloss.NewStyle().Foreground(style.Muted).Render("  Domain: ")
			b.WriteString(domLabel + entry.Domain + "\n")
		}

		for _, server := range entry.Servers {
			srvLabel := lipgloss.NewStyle().Foreground(style.Secondary).Render("  ▸ ")
			b.WriteString(srvLabel + server + "\n")
		}
		b.WriteString("\n")
	}

	help := fmt.Sprintf("%s refresh  %s change DNS  %s reset  %s/%s navigate",
		style.HelpKeyStyle.Render("r"),
		style.HelpKeyStyle.Render("c"),
		style.HelpKeyStyle.Render("x"),
		style.HelpKeyStyle.Render("j"),
		style.HelpKeyStyle.Render("k"),
	)
	b.WriteString(help)

	return b.String()
}

func (d DNSPanel) changeView() string {
	var b strings.Builder

	iface := d.entries[d.cursor].Interface
	ifaceStyled := lipgloss.NewStyle().Bold(true).Foreground(style.Primary).Render(iface)
	b.WriteString(fmt.Sprintf("Change DNS for %s\n\n", ifaceStyled))

	for i, preset := range dnsPresets {
		marker := "  "
		if i == d.presetCursor {
			marker = lipgloss.NewStyle().Foreground(style.Primary).Render("▸ ")
		}

		name := preset.Name
		if i == d.presetCursor {
			name = lipgloss.NewStyle().Bold(true).Foreground(style.Primary).Render(name)
		}

		servers := ""
		if preset.Servers != nil {
			servers = lipgloss.NewStyle().Foreground(style.Muted).Render(
				" (" + strings.Join(preset.Servers, ", ") + ")",
			)
		}

		b.WriteString(marker + name + servers + "\n")
	}

	if dnsPresets[d.presetCursor].Name == "Custom" {
		b.WriteString("\n")
		label := lipgloss.NewStyle().Foreground(style.Muted).Render("  Servers: ")
		cursor := lipgloss.NewStyle().Foreground(style.Primary).Render("█")
		b.WriteString(label + d.customInput + cursor)
		b.WriteString("\n")
		hint := lipgloss.NewStyle().Foreground(style.Subtle).Render("  (space or comma separated)")
		b.WriteString(hint)
	}

	b.WriteString("\n\n")
	help := fmt.Sprintf("%s apply  %s cancel  %s/%s select",
		style.HelpKeyStyle.Render("↵"),
		style.HelpKeyStyle.Render("Esc"),
		style.HelpKeyStyle.Render("j"),
		style.HelpKeyStyle.Render("k"),
	)
	b.WriteString(help)

	return b.String()
}

func (d *DNSPanel) SetSize(width, height int) {
	d.width = width
	d.height = height
}

func (d DNSPanel) refresh() tea.Msg {
	entries, err := parseDNSStatus()
	if err != nil {
		return dnsRefreshMsg{err: err}
	}
	return dnsRefreshMsg{entries: entries}
}

func (d DNSPanel) changeDNS(iface string, servers []string) func() tea.Msg {
	return func() tea.Msg {
		args := append([]string{"dns", iface}, servers...)
		err := exec.Command("resolvectl", args...).Run()
		if err != nil {
			// Fallback to systemd-resolve
			args = append([]string{"--interface", iface, "--set-dns"}, servers...)
			err = exec.Command("systemd-resolve", args...).Run()
		}
		return dnsChangeMsg{err: err}
	}
}

func (d DNSPanel) resetDNS(iface string) func() tea.Msg {
	return func() tea.Msg {
		err := exec.Command("resolvectl", "revert", iface).Run()
		if err != nil {
			err = exec.Command("systemd-resolve", "--interface", iface, "--revert").Run()
		}
		return dnsChangeMsg{err: err}
	}
}

func parseDNSStatus() ([]DNSEntry, error) {
	out, err := exec.Command("resolvectl", "status").Output()
	if err != nil {
		out, err = exec.Command("systemd-resolve", "--status").Output()
		if err != nil {
			return nil, fmt.Errorf("resolvectl not available: %w", err)
		}
	}

	var entries []DNSEntry
	var current *DNSEntry
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "Link ") {
			if current != nil && len(current.Servers) > 0 {
				entries = append(entries, *current)
			}
			parts := strings.SplitN(trimmed, "(", 2)
			iface := "unknown"
			if len(parts) == 2 {
				iface = strings.TrimRight(parts[1], ")")
			}
			current = &DNSEntry{Interface: iface}
		}

		if current == nil {
			if strings.HasPrefix(trimmed, "DNS Servers:") {
				servers := strings.TrimPrefix(trimmed, "DNS Servers:")
				server := strings.TrimSpace(servers)
				if server != "" {
					global := DNSEntry{Interface: "Global", Servers: []string{server}}
					entries = append(entries, global)
				}
			}
			continue
		}

		if strings.HasPrefix(trimmed, "DNS Servers:") {
			server := strings.TrimSpace(strings.TrimPrefix(trimmed, "DNS Servers:"))
			if server != "" {
				current.Servers = append(current.Servers, server)
			}
		} else if strings.HasPrefix(trimmed, "DNS Domain:") {
			current.Domain = strings.TrimSpace(strings.TrimPrefix(trimmed, "DNS Domain:"))
		} else if current != nil && len(current.Servers) > 0 && !strings.Contains(trimmed, ":") && trimmed != "" {
			current.Servers = append(current.Servers, trimmed)
		}
	}

	if current != nil && len(current.Servers) > 0 {
		entries = append(entries, *current)
	}

	if len(entries) == 0 {
		return parseDNSFromResolv()
	}

	return entries, nil
}

func parseDNSFromResolv() ([]DNSEntry, error) {
	out, err := exec.Command("cat", "/etc/resolv.conf").Output()
	if err != nil {
		return nil, fmt.Errorf("cannot read DNS config: %w", err)
	}

	var servers []string
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "nameserver") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				servers = append(servers, parts[1])
			}
		}
	}

	if len(servers) == 0 {
		return nil, nil
	}

	return []DNSEntry{{Interface: "System", Servers: servers}}, nil
}
