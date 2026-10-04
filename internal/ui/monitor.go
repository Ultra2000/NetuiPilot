package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/net"
	"github.com/Ultra2000/netuipilot/internal/style"
)

const sparkChars = "▁▂▃▄▅▆▇█"

type MonitorPanel struct {
	trackers    map[string]*net.BandwidthTracker
	stats       []net.InterfaceStats
	activeConns int
	cursor      int
	width       int
	height      int
	err         error
}

type MonitorTickMsg time.Time

type MonitorDataMsg struct {
	stats       []net.InterfaceStats
	activeConns int
	err         error
}

func NewMonitorPanel() MonitorPanel {
	return MonitorPanel{
		trackers: make(map[string]*net.BandwidthTracker),
	}
}

func (m MonitorPanel) Init() tea.Cmd {
	return tea.Batch(m.fetchData, m.tickCmd())
}

func (m MonitorPanel) Update(msg tea.Msg) (MonitorPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case MonitorTickMsg:
		return m, tea.Batch(m.fetchData, m.tickCmd())

	case MonitorDataMsg:
		m.err = msg.err
		if msg.err == nil {
			m.stats = msg.stats
			m.activeConns = msg.activeConns
			for _, s := range msg.stats {
				tracker, ok := m.trackers[s.Name]
				if !ok {
					tracker = net.NewBandwidthTracker(s.Name, 60)
					m.trackers[s.Name] = tracker
				}
				_ = tracker.Update()
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.stats)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}

func (m MonitorPanel) View() string {
	var b strings.Builder

	header := style.TitleStyle.Render("📊 Bandwidth Monitor")
	b.WriteString(header)
	b.WriteString("\n")
	desc := lipgloss.NewStyle().Foreground(style.Muted).Render("Real-time bandwidth usage with sparkline graphs per interface")
	b.WriteString(desc)
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(style.DisconnectedStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	connStr := fmt.Sprintf("Active connections: %s",
		lipgloss.NewStyle().Foreground(style.Primary).Bold(true).Render(fmt.Sprintf("%d", m.activeConns)),
	)
	b.WriteString(connStr)
	b.WriteString("\n\n")

	if len(m.stats) == 0 {
		b.WriteString(style.SubtitleStyle.Render("No interfaces detected."))
		return b.String()
	}

	for i, s := range m.stats {
		selected := i == m.cursor

		ifaceStyle := lipgloss.NewStyle().Bold(true).Foreground(style.Primary)
		if selected {
			ifaceStyle = ifaceStyle.Background(lipgloss.Color("#3C3489"))
		}
		b.WriteString(ifaceStyle.Render(s.Name))
		b.WriteString("\n")

		rxTotal := net.FormatBytes(float64(s.RXBytes))
		txTotal := net.FormatBytes(float64(s.TXBytes))

		tracker := m.trackers[s.Name]
		rxRate := "0 B/s"
		txRate := "0 B/s"
		if tracker != nil && len(tracker.History) > 0 {
			last := tracker.History[len(tracker.History)-1]
			rxRate = net.FormatBytesPerSec(last.RXBytesPS)
			txRate = net.FormatBytesPerSec(last.TXBytesPS)
		}

		rxLabel := lipgloss.NewStyle().Foreground(style.Secondary).Render("↓ RX")
		txLabel := lipgloss.NewStyle().Foreground(style.Accent).Render("↑ TX")

		b.WriteString(fmt.Sprintf("  %s  %s (%s total)\n", rxLabel, rxRate, rxTotal))
		b.WriteString(fmt.Sprintf("  %s  %s (%s total)\n", txLabel, txRate, txTotal))

		if tracker != nil && len(tracker.History) > 1 {
			sparkRX := renderSparkline(tracker.History, true, m.width-6)
			sparkTX := renderSparkline(tracker.History, false, m.width-6)
			b.WriteString(fmt.Sprintf("  %s %s\n", rxLabel, style.SparkStyle.Render(sparkRX)))
			b.WriteString(fmt.Sprintf("  %s %s\n", txLabel, lipgloss.NewStyle().Foreground(style.Accent).Render(sparkTX)))
		}

		if s.RXErrs > 0 || s.TXErrs > 0 {
			errStr := lipgloss.NewStyle().Foreground(style.Warning).Render(
				fmt.Sprintf("  Errors: RX=%d TX=%d", s.RXErrs, s.TXErrs),
			)
			b.WriteString(errStr)
			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	help := fmt.Sprintf("%s/%s navigate interfaces",
		style.HelpKeyStyle.Render("j"),
		style.HelpKeyStyle.Render("k"),
	)
	b.WriteString(help)

	return b.String()
}

func (m *MonitorPanel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m MonitorPanel) fetchData() tea.Msg {
	stats, err := net.GetAllInterfaceStats()
	if err != nil {
		return MonitorDataMsg{err: err}
	}
	conns, _ := net.GetActiveConnections()
	return MonitorDataMsg{stats: stats, activeConns: conns}
}

func (m MonitorPanel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return MonitorTickMsg(t)
	})
}

func renderSparkline(history []net.BandwidthSample, rx bool, width int) string {
	if width <= 0 {
		width = 40
	}
	if width > 60 {
		width = 60
	}

	start := 0
	if len(history) > width {
		start = len(history) - width
	}
	samples := history[start:]

	var max float64
	for _, s := range samples {
		val := s.TXBytesPS
		if rx {
			val = s.RXBytesPS
		}
		if val > max {
			max = val
		}
	}

	if max == 0 {
		return strings.Repeat("▁", len(samples))
	}

	runes := []rune(sparkChars)
	var sb strings.Builder
	for _, s := range samples {
		val := s.TXBytesPS
		if rx {
			val = s.RXBytesPS
		}
		idx := int(val / max * float64(len(runes)-1))
		if idx >= len(runes) {
			idx = len(runes) - 1
		}
		sb.WriteRune(runes[idx])
	}
	return sb.String()
}
