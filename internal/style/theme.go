package style

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	Primary   = lipgloss.Color("#7F77DD")
	Secondary = lipgloss.Color("#1D9E75")
	Accent    = lipgloss.Color("#D85A30")
	Warning   = lipgloss.Color("#EF9F27")
	Danger    = lipgloss.Color("#E24B4A")
	Success   = lipgloss.Color("#639922")
	Muted     = lipgloss.Color("#888780")
	Subtle    = lipgloss.Color("#5F5E5A")

	// Signal strength colors
	SignalExcellent = lipgloss.Color("#639922")
	SignalGood      = lipgloss.Color("#1D9E75")
	SignalFair      = lipgloss.Color("#EF9F27")
	SignalWeak      = lipgloss.Color("#E24B4A")

	// Base styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)

	// Panel styles
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Subtle).
			Padding(0, 1)

	ActivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Primary).
				Padding(0, 1)

	// Status indicators
	ConnectedStyle = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	DisconnectedStyle = lipgloss.NewStyle().
				Foreground(Danger)

	// Tab styles
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(Primary).
			Padding(0, 2)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(Muted).
				Padding(0, 2)

	// Table styles
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(Subtle).
			Padding(0, 1)

	RowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	SelectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#3C3489")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1)

	// Sparkline / bandwidth
	SparkStyle = lipgloss.NewStyle().
			Foreground(Secondary)

	// Help bar
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(Muted)
)

// SignalColor returns a color based on signal strength percentage.
func SignalColor(strength int) lipgloss.Color {
	switch {
	case strength >= 75:
		return SignalExcellent
	case strength >= 50:
		return SignalGood
	case strength >= 25:
		return SignalFair
	default:
		return SignalWeak
	}
}

// SignalBars returns a visual signal bar indicator.
func SignalBars(strength int) string {
	switch {
	case strength >= 75:
		return "▂▄▆█"
	case strength >= 50:
		return "▂▄▆_"
	case strength >= 25:
		return "▂▄__"
	default:
		return "▂___"
	}
}

// StatusIcon returns a connection status icon.
func StatusIcon(connected bool) string {
	if connected {
		return ConnectedStyle.Render("●")
	}
	return DisconnectedStyle.Render("○")
}
