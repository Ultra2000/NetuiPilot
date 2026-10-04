package ui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/config"
	"github.com/Ultra2000/netuipilot/internal/nm"
	"github.com/Ultra2000/netuipilot/internal/style"
)

type profileMode int

const (
	profileModeList profileMode = iota
	profileModeCreate
	profileModeCreateField
)

type profileField int

const (
	fieldName profileField = iota
	fieldWiFi
	fieldDNS
	fieldVPN
	fieldCount
)

var profileFieldNames = []string{"Name", "WiFi SSID", "DNS Servers", "VPN Connection"}

type ProfilesPanel struct {
	client      *nm.Client
	cfg         config.Config
	profiles    []config.Profile
	cursor      int
	mode        profileMode
	editField   profileField
	editInputs  [fieldCount]string
	width       int
	height      int
	err         error
	success     string
}

type profileActivateMsg struct {
	err error
}

func NewProfilesPanel(client *nm.Client, cfg config.Config) ProfilesPanel {
	return ProfilesPanel{
		client:   client,
		cfg:      cfg,
		profiles: cfg.Profiles,
	}
}

func (p ProfilesPanel) Init() tea.Cmd {
	return nil
}

func (p ProfilesPanel) Update(msg tea.Msg) (ProfilesPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case profileActivateMsg:
		if msg.err != nil {
			p.err = msg.err
			p.success = ""
		} else {
			p.err = nil
			p.success = "Profile activated"
		}
		return p, nil

	case tea.KeyMsg:
		if p.mode == profileModeCreate {
			return p.updateCreate(msg)
		}
		if p.mode == profileModeCreateField {
			return p.updateCreateField(msg)
		}
		return p.updateList(msg)
	}
	return p, nil
}

func (p ProfilesPanel) updateList(msg tea.KeyMsg) (ProfilesPanel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if p.cursor < len(p.profiles)-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "enter":
		if len(p.profiles) > 0 && p.cursor < len(p.profiles) {
			return p, p.activateProfile(p.profiles[p.cursor])
		}
	case "n":
		p.mode = profileModeCreate
		p.editField = fieldName
		p.editInputs = [fieldCount]string{}
		p.success = ""
		p.err = nil
	case "d":
		if len(p.profiles) > 0 && p.cursor < len(p.profiles) {
			p.profiles = append(p.profiles[:p.cursor], p.profiles[p.cursor+1:]...)
			p.cfg.Profiles = p.profiles
			_ = p.cfg.Save()
			if p.cursor >= len(p.profiles) && p.cursor > 0 {
				p.cursor--
			}
			p.success = "Profile deleted"
		}
	}
	return p, nil
}

func (p ProfilesPanel) updateCreate(msg tea.KeyMsg) (ProfilesPanel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		p.mode = profileModeList
	case "j", "down":
		if p.editField < fieldCount-1 {
			p.editField++
		}
	case "k", "up":
		if p.editField > 0 {
			p.editField--
		}
	case "enter":
		if p.editField == fieldName && p.editInputs[fieldName] == "" {
			return p, nil
		}
		p.mode = profileModeCreateField
	case "s":
		if p.editInputs[fieldName] != "" {
			profile := config.Profile{
				Name:     p.editInputs[fieldName],
				WiFiSSID: p.editInputs[fieldWiFi],
				VPNName:  p.editInputs[fieldVPN],
			}
			if p.editInputs[fieldDNS] != "" {
				profile.DNSServers = strings.Fields(strings.ReplaceAll(p.editInputs[fieldDNS], ",", " "))
			}
			p.profiles = append(p.profiles, profile)
			p.cfg.Profiles = p.profiles
			_ = p.cfg.Save()
			p.mode = profileModeList
			p.success = fmt.Sprintf("Profile '%s' created", profile.Name)
		}
	}
	return p, nil
}

func (p ProfilesPanel) updateCreateField(msg tea.KeyMsg) (ProfilesPanel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		p.mode = profileModeCreate
	case "enter":
		p.mode = profileModeCreate
		if p.editField < fieldCount-1 {
			p.editField++
		}
	case "backspace":
		if len(p.editInputs[p.editField]) > 0 {
			p.editInputs[p.editField] = p.editInputs[p.editField][:len(p.editInputs[p.editField])-1]
		}
	default:
		ch := msg.String()
		if len(ch) == 1 {
			p.editInputs[p.editField] += ch
		}
	}
	return p, nil
}

func (p ProfilesPanel) View() string {
	var b strings.Builder

	header := style.TitleStyle.Render("⚡ Profiles")
	b.WriteString(header)
	b.WriteString("\n")
	desc := lipgloss.NewStyle().Foreground(style.Muted).Render("Switch WiFi + DNS + VPN in one click — network presets for any location")
	b.WriteString(desc)
	b.WriteString("\n\n")

	if p.mode == profileModeCreate || p.mode == profileModeCreateField {
		return b.String() + p.createView()
	}

	if p.success != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(style.Success).Render("● " + p.success))
		b.WriteString("\n\n")
	}
	if p.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(style.Danger).Render(fmt.Sprintf("Error: %v", p.err)))
		b.WriteString("\n\n")
	}

	if len(p.profiles) == 0 {
		b.WriteString(style.SubtitleStyle.Render("No profiles yet. Press 'n' to create one."))
		b.WriteString("\n\n")
		help := fmt.Sprintf("%s new profile", style.HelpKeyStyle.Render("n"))
		b.WriteString(help)
		return b.String()
	}

	for i, prof := range p.profiles {
		icon := lipgloss.NewStyle().Foreground(style.Primary).Render("◆")

		var parts []string
		if prof.WiFiSSID != "" {
			parts = append(parts, "WiFi: "+prof.WiFiSSID)
		}
		if len(prof.DNSServers) > 0 {
			parts = append(parts, "DNS: "+strings.Join(prof.DNSServers, ","))
		}
		if prof.VPNName != "" {
			parts = append(parts, "VPN: "+prof.VPNName)
		}

		detail := ""
		if len(parts) > 0 {
			detail = lipgloss.NewStyle().Foreground(style.Muted).Render(" (" + strings.Join(parts, " + ") + ")")
		}

		name := prof.Name
		row := fmt.Sprintf("%s  %s%s", icon, name, detail)

		if i == p.cursor {
			row = style.SelectedRowStyle.Render(row)
		} else {
			row = style.RowStyle.Render(row)
		}

		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	help := fmt.Sprintf("%s activate  %s new  %s delete  %s/%s navigate",
		style.HelpKeyStyle.Render("↵"),
		style.HelpKeyStyle.Render("n"),
		style.HelpKeyStyle.Render("d"),
		style.HelpKeyStyle.Render("j"),
		style.HelpKeyStyle.Render("k"),
	)
	b.WriteString(help)

	return b.String()
}

func (p ProfilesPanel) createView() string {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(style.Primary).Render("New Profile"))
	b.WriteString("\n\n")

	cursor := lipgloss.NewStyle().Foreground(style.Primary).Render("█")

	for i := profileField(0); i < fieldCount; i++ {
		marker := "  "
		if i == p.editField {
			marker = lipgloss.NewStyle().Foreground(style.Primary).Render("▸ ")
		}

		label := lipgloss.NewStyle().Width(18).Render(profileFieldNames[i] + ":")
		value := p.editInputs[i]

		if p.mode == profileModeCreateField && i == p.editField {
			value = value + cursor
		}

		if i == fieldName && p.editInputs[fieldName] == "" {
			value = lipgloss.NewStyle().Foreground(style.Subtle).Render("(required)")
		}

		b.WriteString(marker + label + " " + value + "\n")
	}

	b.WriteString("\n")

	if p.mode == profileModeCreateField {
		help := fmt.Sprintf("%s confirm  %s cancel",
			style.HelpKeyStyle.Render("↵"),
			style.HelpKeyStyle.Render("Esc"),
		)
		b.WriteString(help)
	} else {
		help := fmt.Sprintf("%s edit field  %s save  %s cancel  %s/%s navigate",
			style.HelpKeyStyle.Render("↵"),
			style.HelpKeyStyle.Render("s"),
			style.HelpKeyStyle.Render("Esc"),
			style.HelpKeyStyle.Render("j"),
			style.HelpKeyStyle.Render("k"),
		)
		b.WriteString(help)
	}

	return b.String()
}

func (p *ProfilesPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p ProfilesPanel) activateProfile(prof config.Profile) func() tea.Msg {
	return func() tea.Msg {
		var errors []string

		if prof.WiFiSSID != "" && p.client != nil {
			devices, err := p.client.GetDevices()
			if err == nil {
				for _, dev := range devices {
					if dev.Type == nm.DeviceTypeWiFi {
						conns, _ := p.client.GetSavedConnections()
						for _, conn := range conns {
							if conn.ID == prof.WiFiSSID {
								_ = p.client.ActivateConnection(conn.Path, dev.Path)
								break
							}
						}
						break
					}
				}
			} else {
				errors = append(errors, fmt.Sprintf("WiFi: %v", err))
			}
		}

		if len(prof.DNSServers) > 0 {
			iface := prof.DNSIface
			if iface == "" {
				if p.client != nil {
					devices, err := p.client.GetDevices()
					if err == nil {
						for _, dev := range devices {
							if dev.State == nm.DeviceStateActivated && dev.Name != "lo" {
								iface = dev.Name
								break
							}
						}
					}
				}
			}
			if iface != "" {
				args := append([]string{"dns", iface}, prof.DNSServers...)
				err := exec.Command("resolvectl", args...).Run()
				if err != nil {
					args = append([]string{"--interface", iface, "--set-dns"}, prof.DNSServers...)
					err = exec.Command("systemd-resolve", args...).Run()
					if err != nil {
						errors = append(errors, fmt.Sprintf("DNS: %v", err))
					}
				}
			}
		}

		if prof.VPNName != "" && p.client != nil {
			conns, err := p.client.GetSavedConnections()
			if err == nil {
				for _, conn := range conns {
					if conn.ID == prof.VPNName && isVPNType(conn.Type) {
						devices, _ := p.client.GetDevices()
						if len(devices) > 0 {
							_ = p.client.ActivateConnection(conn.Path, devices[0].Path)
						}
						break
					}
				}
			} else {
				errors = append(errors, fmt.Sprintf("VPN: %v", err))
			}
		}

		if len(errors) > 0 {
			return profileActivateMsg{err: fmt.Errorf("%s", strings.Join(errors, "; "))}
		}
		return profileActivateMsg{}
	}
}
