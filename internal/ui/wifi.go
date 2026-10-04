package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/nm"
	"github.com/Ultra2000/netuipilot/internal/style"
	"github.com/godbus/dbus/v5"
)

type WiFiPanel struct {
	client       *nm.Client
	accessPoints []nm.AccessPoint
	cursor       int
	width        int
	height       int
	scanning     bool
	err          error
}

type scanCompleteMsg struct {
	aps []nm.AccessPoint
	err error
}

type connectResultMsg struct {
	err error
}

func NewWiFiPanel(client *nm.Client) WiFiPanel {
	return WiFiPanel{
		client: client,
	}
}

func (w WiFiPanel) Init() tea.Cmd {
	return w.scan
}

func (w WiFiPanel) Update(msg tea.Msg) (WiFiPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case scanCompleteMsg:
		w.scanning = false
		w.err = msg.err
		if msg.err == nil {
			w.accessPoints = msg.aps
		}
		return w, nil

	case connectResultMsg:
		w.err = msg.err
		return w, w.scan

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if w.cursor < len(w.accessPoints)-1 {
				w.cursor++
			}
		case "k", "up":
			if w.cursor > 0 {
				w.cursor--
			}
		case "s":
			w.scanning = true
			return w, w.scan
		case "enter":
			if len(w.accessPoints) > 0 && w.cursor < len(w.accessPoints) {
				ap := w.accessPoints[w.cursor]
				if !ap.Active {
					return w, w.connectAP(ap)
				}
			}
		case "d":
			if len(w.accessPoints) > 0 && w.cursor < len(w.accessPoints) {
				ap := w.accessPoints[w.cursor]
				if ap.Active {
					return w, w.disconnectAP()
				}
			}
		}
	}
	return w, nil
}

func (w WiFiPanel) View() string {
	var b strings.Builder

	header := style.TitleStyle.Render("📡 WiFi Networks")
	b.WriteString(header)
	b.WriteString("\n\n")

	if w.scanning {
		b.WriteString(style.SubtitleStyle.Render("Scanning..."))
		b.WriteString("\n")
	}

	if w.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(style.Danger).Render(fmt.Sprintf("Error: %v", w.err)))
		b.WriteString("\n\n")
	}

	if len(w.accessPoints) == 0 && !w.scanning {
		b.WriteString(style.SubtitleStyle.Render("No networks found. Press 's' to scan."))
		return b.String()
	}

	headerRow := style.HeaderStyle.Render(
		fmt.Sprintf("  %-28s %-10s %-8s %-12s", "SSID", "SIGNAL", "FREQ", "SECURITY"),
	)
	b.WriteString(headerRow)
	b.WriteString("\n")

	for i, ap := range w.accessPoints {
		sigColor := style.SignalColor(int(ap.Strength))
		bars := style.SignalBars(int(ap.Strength))

		prefix := "  "
		if ap.Active {
			prefix = style.ConnectedStyle.Render("● ")
		}

		freq := fmt.Sprintf("%d MHz", ap.Freq)
		signal := fmt.Sprintf("%s %d%%", bars, ap.Strength)

		row := fmt.Sprintf("%s%-28s %-10s %-8s %-12s",
			prefix,
			truncate(ap.SSID, 26),
			lipgloss.NewStyle().Foreground(sigColor).Render(signal),
			freq,
			ap.Security,
		)

		if i == w.cursor {
			row = style.SelectedRowStyle.Render(row)
		} else {
			row = style.RowStyle.Render(row)
		}

		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	help := fmt.Sprintf("%s scan  %s connect  %s disconnect  %s/%s navigate",
		style.HelpKeyStyle.Render("s"),
		style.HelpKeyStyle.Render("↵"),
		style.HelpKeyStyle.Render("d"),
		style.HelpKeyStyle.Render("j"),
		style.HelpKeyStyle.Render("k"),
	)
	b.WriteString(help)

	return b.String()
}

func (w *WiFiPanel) SetSize(width, height int) {
	w.width = width
	w.height = height
}

func (w WiFiPanel) scan() tea.Msg {
	if w.client == nil {
		return scanCompleteMsg{err: fmt.Errorf("no NetworkManager connection")}
	}

	devices, err := w.client.GetDevices()
	if err != nil {
		return scanCompleteMsg{err: err}
	}

	for _, dev := range devices {
		if dev.Type == nm.DeviceTypeWiFi {
			_ = w.client.RequestScan(dev.Path)
			aps, err := w.client.GetAccessPoints(dev.Path)
			if err != nil {
				return scanCompleteMsg{err: err}
			}
			sort.Slice(aps, func(i, j int) bool {
				if aps[i].Active != aps[j].Active {
					return aps[i].Active
				}
				return aps[i].Strength > aps[j].Strength
			})
			return scanCompleteMsg{aps: aps}
		}
	}

	return scanCompleteMsg{err: fmt.Errorf("no WiFi device found")}
}

func (w WiFiPanel) connectAP(ap nm.AccessPoint) func() tea.Msg {
	return func() tea.Msg {
		devices, err := w.client.GetDevices()
		if err != nil {
			return connectResultMsg{err: err}
		}

		for _, dev := range devices {
			if dev.Type == nm.DeviceTypeWiFi {
				settings := makeWiFiSettings(ap)
				err := w.client.AddAndActivateConnection(settings, dev.Path, ap.Path)
				return connectResultMsg{err: err}
			}
		}
		return connectResultMsg{err: fmt.Errorf("no WiFi device")}
	}
}

func (w WiFiPanel) disconnectAP() func() tea.Msg {
	return func() tea.Msg {
		devices, err := w.client.GetDevices()
		if err != nil {
			return connectResultMsg{err: err}
		}

		for _, dev := range devices {
			if dev.Type == nm.DeviceTypeWiFi && dev.State == nm.DeviceStateActivated {
				activeConn, err := w.client.GetActiveConnectionForDevice(dev.Path)
				if err != nil {
					return connectResultMsg{err: err}
				}
				err = w.client.DeactivateConnection(activeConn)
				return connectResultMsg{err: err}
			}
		}
		return connectResultMsg{err: fmt.Errorf("no active WiFi connection")}
	}
}

func makeWiFiSettings(ap nm.AccessPoint) map[string]map[string]dbus.Variant {
	settings := map[string]map[string]dbus.Variant{
		"connection": {
			"type": dbus.MakeVariant("802-11-wireless"),
			"id":   dbus.MakeVariant(ap.SSID),
		},
		"802-11-wireless": {
			"ssid": dbus.MakeVariant([]byte(ap.SSID)),
			"mode": dbus.MakeVariant("infrastructure"),
		},
	}

	if ap.Security != "Open" {
		settings["802-11-wireless-security"] = map[string]dbus.Variant{
			"key-mgmt": dbus.MakeVariant("wpa-psk"),
		}
	}

	return settings
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}
