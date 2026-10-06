package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/config"
	"github.com/Ultra2000/netuipilot/internal/net"
	"github.com/Ultra2000/netuipilot/internal/nm"
	"github.com/Ultra2000/netuipilot/internal/style"
)

type Tab int

const (
	TabWiFi Tab = iota
	TabIfaces
	TabVPN
	TabDNS
	TabConns
	TabMonitor
	TabRescue
	TabProfiles
	TabLog
	tabCount = 9
)

var tabNames = []string{"WiFi", "Interfaces", "VPN", "DNS", "Conns", "Monitor", "Rescue", "Profiles", "Log"}

type Model struct {
	client    *nm.Client
	activeTab Tab
	wifi      WiFiPanel
	iface     IfacePanel
	vpn       VPNPanel
	dns       DNSPanel
	conns     ConnsPanel
	monitor   MonitorPanel
	rescue    RescuePanel
	profiles  ProfilesPanel
	logPanel  LogPanel
	notify    *NotifyManager
	cfg       config.Config
	version   string
	width     int
	height    int
	err       error
}

func NewModel(version string, cfg config.Config) Model {
	client, err := nm.NewClient()

	m := Model{
		client:    client,
		activeTab: TabWiFi,
		err:       err,
		notify:    NewNotifyManager(),
		version:   version,
		cfg:       cfg,
	}

	if client != nil {
		m.wifi = NewWiFiPanel(client)
		m.iface = NewIfacePanel(client, cfg)
		m.vpn = NewVPNPanel(client)
		m.profiles = NewProfilesPanel(client, cfg)
	}
	m.dns = NewDNSPanel()
	m.conns = NewConnsPanel()
	m.monitor = NewMonitorPanel()
	m.rescue = NewRescuePanel()
	m.logPanel = NewLogPanel(m.notify)

	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.monitor.Init(), m.dns.Init(), m.conns.Init()}
	if m.client != nil {
		cmds = append(cmds, m.wifi.Init(), m.iface.Init(), m.vpn.Init())
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.activeTab == TabWiFi && m.wifi.mode == wifiModePassword {
			if msg.String() != "q" && msg.String() != "ctrl+c" {
				var cmd tea.Cmd
				m.wifi, cmd = m.wifi.Update(msg)
				return m, cmd
			}
		}

		if m.activeTab == TabProfiles && (m.profiles.mode == profileModeCreateField) {
			if msg.String() != "q" && msg.String() != "ctrl+c" {
				var cmd tea.Cmd
				m.profiles, cmd = m.profiles.Update(msg)
				return m, cmd
			}
		}

		if m.activeTab == TabDNS && m.dns.mode == dnsModeChange {
			if msg.String() != "q" && msg.String() != "ctrl+c" {
				var cmd tea.Cmd
				m.dns, cmd = m.dns.Update(msg)
				return m, cmd
			}
		}

		if m.activeTab == TabRescue && m.rescue.mode == rescueModeInput {
			if msg.String() != "q" && msg.String() != "ctrl+c" {
				var cmd tea.Cmd
				m.rescue, cmd = m.rescue.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			if m.client != nil {
				m.client.Close()
			}
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % tabCount
			return m, nil
		case "shift+tab":
			m.activeTab = (m.activeTab + tabCount - 1) % tabCount
			return m, nil
		case "1":
			m.activeTab = TabWiFi
			return m, nil
		case "2":
			m.activeTab = TabIfaces
			return m, nil
		case "3":
			m.activeTab = TabVPN
			return m, nil
		case "4":
			m.activeTab = TabDNS
			return m, nil
		case "5":
			m.activeTab = TabConns
			return m, nil
		case "6":
			m.activeTab = TabMonitor
			return m, nil
		case "7":
			m.activeTab = TabRescue
			return m, nil
		case "8":
			m.activeTab = TabProfiles
			return m, nil
		case "9":
			m.activeTab = TabLog
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := msg.Height - 6
		m.wifi.SetSize(msg.Width-4, contentHeight)
		m.iface.SetSize(msg.Width-4, contentHeight)
		m.vpn.SetSize(msg.Width-4, contentHeight)
		m.dns.SetSize(msg.Width-4, contentHeight)
		m.conns.SetSize(msg.Width-4, contentHeight)
		m.monitor.SetSize(msg.Width-4, contentHeight)
		m.rescue.SetSize(msg.Width-4, contentHeight)
		m.profiles.SetSize(msg.Width-4, contentHeight)
		m.logPanel.SetSize(msg.Width-4, contentHeight)
		return m, nil

	case MonitorTickMsg, MonitorDataMsg:
		var monCmd tea.Cmd
		m.monitor, monCmd = m.monitor.Update(msg)
		return m, monCmd

	case devicesRefreshMsg:
		if msg.err == nil {
			m.notify.CheckDevices(msg.devices)
		}
		var cmd tea.Cmd
		m.iface, cmd = m.iface.Update(msg)
		return m, cmd
	}

	switch ev := msg.(type) {
	case dnsChangeMsg:
		if ev.err == nil {
			m.notify.AddEvent(NotifyInfo, "DNS", "DNS servers updated")
		} else {
			m.notify.AddEvent(NotifyError, "DNS", "DNS change failed")
		}
	case vpnToggleMsg:
		if ev.err == nil {
			m.notify.AddEvent(NotifyInfo, "VPN", "VPN connection toggled")
		}
	case profileActivateMsg:
		if ev.err == nil {
			m.notify.AddEvent(NotifyInfo, "Profile", "Profile activated")
		} else {
			m.notify.AddEvent(NotifyWarn, "Profile", "Profile activated with errors")
		}
	case connKillMsg:
		if ev.err == nil {
			m.notify.AddEvent(NotifyWarn, "Process", fmt.Sprintf("Killed %s (pid %d)", ev.process, ev.pid))
		}
	}

	var cmd tea.Cmd
	switch m.activeTab {
	case TabWiFi:
		m.wifi, cmd = m.wifi.Update(msg)
	case TabIfaces:
		m.iface, cmd = m.iface.Update(msg)
	case TabVPN:
		m.vpn, cmd = m.vpn.Update(msg)
	case TabDNS:
		m.dns, cmd = m.dns.Update(msg)
	case TabConns:
		m.conns, cmd = m.conns.Update(msg)
	case TabMonitor:
		m.monitor, cmd = m.monitor.Update(msg)
	case TabRescue:
		m.rescue, cmd = m.rescue.Update(msg)
	case TabProfiles:
		m.profiles, cmd = m.profiles.Update(msg)
	case TabLog:
		m.logPanel, cmd = m.logPanel.Update(msg)
	}

	return m, cmd
}

func (m Model) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(style.Primary).
		Render("NetuiPilot")

	version := lipgloss.NewStyle().
		Foreground(style.Muted).
		Render(" v" + m.version)

	notifyBar := m.notify.RenderBar()

	b.WriteString(title + version + notifyBar)
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	var tabs []string
	for i, name := range tabNames {
		if Tab(i) == m.activeTab {
			tabs = append(tabs, style.ActiveTabStyle.Render(name))
		} else {
			tabs = append(tabs, style.InactiveTabStyle.Render(name))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	b.WriteString(tabBar)
	b.WriteString("\n")

	separator := lipgloss.NewStyle().
		Foreground(style.Subtle).
		Render(strings.Repeat("─", max(m.width, 40)))
	b.WriteString(separator)
	b.WriteString("\n")

	if m.err != nil {
		errMsg := lipgloss.NewStyle().Foreground(style.Danger).Render(
			fmt.Sprintf("⚠ NetworkManager: %v", m.err),
		)
		b.WriteString(errMsg)
		b.WriteString("\n")
		b.WriteString(style.SubtitleStyle.Render("Monitoring-only mode (no D-Bus connection)"))
		b.WriteString("\n\n")
	}

	var content string
	switch m.activeTab {
	case TabWiFi:
		content = m.wifi.View()
	case TabIfaces:
		content = m.iface.View()
	case TabVPN:
		content = m.vpn.View()
	case TabDNS:
		content = m.dns.View()
	case TabConns:
		content = m.conns.View()
	case TabMonitor:
		content = m.monitor.View()
	case TabRescue:
		content = m.rescue.View()
	case TabProfiles:
		content = m.profiles.View()
	case TabLog:
		content = m.logPanel.View()
	}

	panelStyle := style.PanelStyle
	if m.width > 0 {
		panelStyle = panelStyle.Width(m.width - 2)
	}
	b.WriteString(panelStyle.Render(content))

	b.WriteString("\n")
	statusBar := fmt.Sprintf(" %s tab  %s-%s panels  %s quit",
		style.HelpKeyStyle.Render("Tab"),
		style.HelpKeyStyle.Render("1"),
		style.HelpKeyStyle.Render("9"),
		style.HelpKeyStyle.Render("q"),
	)
	b.WriteString(lipgloss.NewStyle().Foreground(style.Muted).Render(statusBar))

	return b.String()
}

func (m Model) renderStatusBar() string {
	sep := lipgloss.NewStyle().Foreground(style.Subtle).Render(" │ ")
	var parts []string

	activeIface := "none"
	if len(m.iface.devices) > 0 {
		for _, dev := range m.iface.devices {
			if dev.State == nm.DeviceStateActivated {
				activeIface = dev.Name
				break
			}
		}
	}
	if activeIface != "none" {
		parts = append(parts, lipgloss.NewStyle().Foreground(style.Success).Render("●")+" "+activeIface+" "+lipgloss.NewStyle().Foreground(style.Success).Render("↑"))
	} else {
		parts = append(parts, lipgloss.NewStyle().Foreground(style.Danger).Render("●")+" net "+lipgloss.NewStyle().Foreground(style.Danger).Render("↓"))
	}

	if len(m.dns.entries) > 0 {
		parts = append(parts, "DNS "+lipgloss.NewStyle().Foreground(style.Success).Render("✓"))
	} else {
		parts = append(parts, "DNS "+lipgloss.NewStyle().Foreground(style.Danger).Render("✗"))
	}

	hasVPN := false
	for _, entry := range m.vpn.entries {
		if entry.active {
			hasVPN = true
			break
		}
	}
	if hasVPN {
		parts = append(parts, "VPN "+lipgloss.NewStyle().Foreground(style.Success).Render("✓"))
	} else {
		parts = append(parts, "VPN "+lipgloss.NewStyle().Foreground(style.Muted).Render("✗"))
	}

	if len(m.monitor.stats) > 0 && len(m.monitor.trackers) > 0 {
		var totalRX, totalTX float64
		for _, s := range m.monitor.stats {
			tracker := m.monitor.trackers[s.Name]
			if tracker != nil && len(tracker.History) > 0 {
				last := tracker.History[len(tracker.History)-1]
				totalRX += last.RXBytesPS
				totalTX += last.TXBytesPS
			}
		}
		bw := fmt.Sprintf("%s %s  %s %s",
			lipgloss.NewStyle().Foreground(style.Secondary).Render("↓"),
			net.FormatBytesPerSec(totalRX),
			lipgloss.NewStyle().Foreground(style.Accent).Render("↑"),
			net.FormatBytesPerSec(totalTX),
		)
		parts = append(parts, bw)
	}

	return lipgloss.NewStyle().Foreground(style.Muted).Render(" ") + strings.Join(parts, sep)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
