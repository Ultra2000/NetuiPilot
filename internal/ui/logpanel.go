package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/style"
)

type LogPanel struct {
	notify *NotifyManager
	offset int
	width  int
	height int
}

func NewLogPanel(notify *NotifyManager) LogPanel {
	return LogPanel{notify: notify}
}

func (l LogPanel) Init() tea.Cmd {
	return nil
}

func (l LogPanel) Update(msg tea.Msg) (LogPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		entries := l.notify.Log()
		switch msg.String() {
		case "j", "down":
			if l.offset < len(entries)-1 {
				l.offset++
			}
		case "k", "up":
			if l.offset > 0 {
				l.offset--
			}
		case "g":
			l.offset = 0
		}
	}
	return l, nil
}

func (l LogPanel) View() string {
	var b strings.Builder

	header := style.TitleStyle.Render("📜 Event Log")
	b.WriteString(header)
	b.WriteString("\n")
	desc := lipgloss.NewStyle().Foreground(style.Muted).Render("Live timeline of network events — connections, DNS, VPN, failures")
	b.WriteString(desc)
	b.WriteString("\n\n")

	entries := l.notify.Log()

	if len(entries) == 0 {
		b.WriteString(style.SubtitleStyle.Render("No events yet. Network changes will appear here."))
		return b.String()
	}

	visible := l.height - 7
	if visible < 5 {
		visible = 15
	}

	end := l.offset + visible
	if end > len(entries) {
		end = len(entries)
	}

	for i := l.offset; i < end; i++ {
		e := entries[i]

		ts := lipgloss.NewStyle().Foreground(style.Subtle).Render("[" + e.Time.Format("15:04:05") + "]")

		var icon string
		var color lipgloss.Color
		switch e.Level {
		case NotifyInfo:
			icon = "●"
			color = style.Success
		case NotifyWarn:
			icon = "▲"
			color = style.Warning
		case NotifyError:
			icon = "✕"
			color = style.Danger
		}
		iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)

		cat := lipgloss.NewStyle().Foreground(style.Secondary).Width(12).Render(truncate(e.Category, 11))

		b.WriteString(fmt.Sprintf("%s %s %s %s\n", ts, iconStyled, cat, e.Message))
	}

	if len(entries) > visible {
		scroll := lipgloss.NewStyle().Foreground(style.Muted).Render(
			fmt.Sprintf("\n  [%d-%d of %d]", l.offset+1, end, len(entries)),
		)
		b.WriteString(scroll)
	}

	b.WriteString("\n")
	help := fmt.Sprintf("%s/%s scroll  %s top",
		style.HelpKeyStyle.Render("j"),
		style.HelpKeyStyle.Render("k"),
		style.HelpKeyStyle.Render("g"),
	)
	b.WriteString(help)

	return b.String()
}

func (l *LogPanel) SetSize(width, height int) {
	l.width = width
	l.height = height
}
