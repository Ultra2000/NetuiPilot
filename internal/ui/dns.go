package ui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/style"
)

type DNSEntry struct {
	Interface string
	Servers   []string
	Domain    string
}

type DNSPanel struct {
	entries []DNSEntry
	cursor  int
	width   int
	height  int
	err     error
}

type dnsRefreshMsg struct {
	entries []DNSEntry
	err     error
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

	case tea.KeyMsg:
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
			return d, d.refresh
		}
	}
	return d, nil
}

func (d DNSPanel) View() string {
	var b strings.Builder

	header := style.TitleStyle.Render("🌐 DNS Resolvers")
	b.WriteString(header)
	b.WriteString("\n\n")

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

	help := fmt.Sprintf("%s refresh  %s/%s navigate",
		style.HelpKeyStyle.Render("r"),
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
