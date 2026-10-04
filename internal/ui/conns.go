package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/net"
	"github.com/Ultra2000/netuipilot/internal/style"
)

type connFilter int

const (
	connFilterAll connFilter = iota
	connFilterEstablished
	connFilterListening
)

var connFilterNames = []string{"All", "Established", "Listening"}

type ConnsPanel struct {
	entries  []net.SocketEntry
	filtered []net.SocketEntry
	filter   connFilter
	cursor   int
	offset   int
	width    int
	height   int
	err      error
}

type connsRefreshMsg struct {
	entries []net.SocketEntry
	err     error
}

func NewConnsPanel() ConnsPanel {
	return ConnsPanel{}
}

func (c ConnsPanel) Init() tea.Cmd {
	return c.refresh
}

func (c ConnsPanel) Update(msg tea.Msg) (ConnsPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case connsRefreshMsg:
		c.err = msg.err
		if msg.err == nil {
			c.entries = msg.entries
			c.applyFilter()
		}
		return c, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if c.cursor < len(c.filtered)-1 {
				c.cursor++
				c.adjustScroll()
			}
		case "k", "up":
			if c.cursor > 0 {
				c.cursor--
				c.adjustScroll()
			}
		case "r":
			return c, c.refresh
		case "f":
			c.filter = (c.filter + 1) % 3
			c.cursor = 0
			c.offset = 0
			c.applyFilter()
		}
	}
	return c, nil
}

func (c *ConnsPanel) applyFilter() {
	c.filtered = nil
	for _, e := range c.entries {
		switch c.filter {
		case connFilterEstablished:
			if e.State != "ESTABLISHED" {
				continue
			}
		case connFilterListening:
			if e.State != "LISTEN" {
				continue
			}
		}
		c.filtered = append(c.filtered, e)
	}
}

func (c *ConnsPanel) adjustScroll() {
	visible := c.visibleRows()
	if c.cursor >= c.offset+visible {
		c.offset = c.cursor - visible + 1
	}
	if c.cursor < c.offset {
		c.offset = c.cursor
	}
}

func (c ConnsPanel) visibleRows() int {
	rows := c.height - 8
	if rows < 5 {
		rows = 15
	}
	return rows
}

func (c ConnsPanel) View() string {
	var b strings.Builder

	header := style.TitleStyle.Render("🔗 Connections")
	b.WriteString(header)
	b.WriteString("\n")
	desc := lipgloss.NewStyle().Foreground(style.Muted).Render("Active TCP/UDP sockets — like ss/netstat in your terminal")
	b.WriteString(desc)
	b.WriteString("\n")

	filterLabel := lipgloss.NewStyle().Foreground(style.Muted).Render("Filter: ")
	filterValue := lipgloss.NewStyle().Foreground(style.Primary).Bold(true).Render(connFilterNames[c.filter])
	countStr := lipgloss.NewStyle().Foreground(style.Muted).Render(fmt.Sprintf(" (%d)", len(c.filtered)))
	b.WriteString(filterLabel + filterValue + countStr)
	b.WriteString("\n\n")

	if c.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(style.Danger).Render(fmt.Sprintf("Error: %v", c.err)))
		b.WriteString("\n\n")
	}

	if len(c.filtered) == 0 {
		b.WriteString(style.SubtitleStyle.Render("No connections. Press 'r' to refresh."))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("%s refresh  %s filter", style.HelpKeyStyle.Render("r"), style.HelpKeyStyle.Render("f")))
		return b.String()
	}

	headerRow := style.HeaderStyle.Render(
		fmt.Sprintf("  %-6s %-22s %-22s %-12s", "PROTO", "LOCAL", "REMOTE", "STATE"),
	)
	b.WriteString(headerRow)
	b.WriteString("\n")

	visible := c.visibleRows()
	end := c.offset + visible
	if end > len(c.filtered) {
		end = len(c.filtered)
	}

	for i := c.offset; i < end; i++ {
		e := c.filtered[i]

		local := fmt.Sprintf("%s:%d", truncate(e.LocalIP, 15), e.LocalPort)
		remote := fmt.Sprintf("%s:%d", truncate(e.RemoteIP, 15), e.RemotePort)
		if e.RemotePort == 0 {
			remote = "*"
		}

		stateStyled := e.State
		switch e.State {
		case "ESTABLISHED":
			stateStyled = lipgloss.NewStyle().Foreground(style.Success).Render(e.State)
		case "LISTEN":
			stateStyled = lipgloss.NewStyle().Foreground(style.Secondary).Render(e.State)
		case "TIME_WAIT", "CLOSE_WAIT":
			stateStyled = lipgloss.NewStyle().Foreground(style.Warning).Render(e.State)
		case "SYN_SENT", "SYN_RECV":
			stateStyled = lipgloss.NewStyle().Foreground(style.Accent).Render(e.State)
		}

		row := fmt.Sprintf("  %-6s %-22s %-22s %-12s",
			e.Proto,
			local,
			remote,
			stateStyled,
		)

		if i == c.cursor {
			row = style.SelectedRowStyle.Render(row)
		} else {
			row = style.RowStyle.Render(row)
		}

		b.WriteString(row)
		b.WriteString("\n")
	}

	if len(c.filtered) > visible {
		scroll := lipgloss.NewStyle().Foreground(style.Muted).Render(
			fmt.Sprintf("\n  [%d-%d of %d]", c.offset+1, end, len(c.filtered)),
		)
		b.WriteString(scroll)
	}

	b.WriteString("\n")
	help := fmt.Sprintf("%s refresh  %s filter (%s)  %s/%s navigate",
		style.HelpKeyStyle.Render("r"),
		style.HelpKeyStyle.Render("f"),
		connFilterNames[(c.filter+1)%3],
		style.HelpKeyStyle.Render("j"),
		style.HelpKeyStyle.Render("k"),
	)
	b.WriteString(help)

	return b.String()
}

func (c *ConnsPanel) SetSize(width, height int) {
	c.width = width
	c.height = height
}

func (c ConnsPanel) refresh() tea.Msg {
	entries, err := net.GetDetailedConnections()
	if err != nil {
		return connsRefreshMsg{err: err}
	}
	return connsRefreshMsg{entries: entries}
}
