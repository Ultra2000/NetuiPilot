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
	entries     []net.SocketEntry
	filtered    []net.SocketEntry
	filter      connFilter
	cursor      int
	offset      int
	killConfirm bool
	status      string
	width       int
	height      int
	err         error
}

type connsRefreshMsg struct {
	entries []net.SocketEntry
	err     error
}

type connKillMsg struct {
	pid     int
	process string
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
		if c.killConfirm {
			switch msg.String() {
			case "y":
				c.killConfirm = false
				return c, c.killSelected()
			default:
				c.killConfirm = false
				c.status = ""
				return c, nil
			}
		}

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
			c.status = ""
			return c, c.refresh
		case "f":
			c.filter = (c.filter + 1) % 3
			c.cursor = 0
			c.offset = 0
			c.status = ""
			c.applyFilter()
		case "K":
			if c.cursor < len(c.filtered) {
				e := c.filtered[c.cursor]
				if e.PID > 0 {
					c.killConfirm = true
				} else {
					c.status = "No process owner found (try running as root)"
				}
			}
		}

	case connKillMsg:
		if msg.err != nil {
			c.status = fmt.Sprintf("Kill failed: %v", msg.err)
		} else {
			c.status = fmt.Sprintf("Sent SIGTERM to %s (pid %d)", msg.process, msg.pid)
		}
		return c, c.refresh
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
		fmt.Sprintf("  %-6s %-22s %-22s %-11s %s", "PROTO", "LOCAL", "REMOTE", "STATE", "PROCESS"),
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

		stateColor := style.Muted
		switch e.State {
		case "ESTABLISHED":
			stateColor = style.Success
		case "LISTEN":
			stateColor = style.Secondary
		case "TIME_WAIT", "CLOSE_WAIT":
			stateColor = style.Warning
		case "SYN_SENT", "SYN_RECV":
			stateColor = style.Accent
		}
		stateStyled := lipgloss.NewStyle().Foreground(stateColor).Width(11).Render(e.State)

		proc := ""
		if e.PID > 0 {
			proc = fmt.Sprintf("%s (%d)", e.Process, e.PID)
		}

		row := fmt.Sprintf("  %-6s %-22s %-22s %s %s",
			e.Proto,
			local,
			remote,
			stateStyled,
			proc,
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

	if c.killConfirm && c.cursor < len(c.filtered) {
		e := c.filtered[c.cursor]
		confirm := lipgloss.NewStyle().Foreground(style.Warning).Bold(true).Render(
			fmt.Sprintf("Kill %s (pid %d)? Press %s to confirm, any key to cancel",
				e.Process, e.PID, lipgloss.NewStyle().Foreground(style.Danger).Render("y")),
		)
		b.WriteString(confirm)
		b.WriteString("\n")
	} else if c.status != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(style.Secondary).Render("● " + c.status))
		b.WriteString("\n")
	}

	help := fmt.Sprintf("%s refresh  %s filter (%s)  %s kill proc  %s/%s navigate",
		style.HelpKeyStyle.Render("r"),
		style.HelpKeyStyle.Render("f"),
		connFilterNames[(c.filter+1)%3],
		style.HelpKeyStyle.Render("K"),
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

func (c ConnsPanel) killSelected() tea.Cmd {
	if c.cursor >= len(c.filtered) {
		return nil
	}
	e := c.filtered[c.cursor]
	return func() tea.Msg {
		err := net.KillProcess(e.PID)
		return connKillMsg{pid: e.PID, process: e.Process, err: err}
	}
}
