package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/diag"
	"github.com/Ultra2000/netuipilot/internal/style"
)

type rescueMode int

const (
	rescueModeInput rescueMode = iota
	rescueModeRunning
	rescueModeResults
)

type RescuePanel struct {
	mode       rescueMode
	target     string
	report     diag.DiagReport
	exportPath string
	width      int
	height     int
	err        error
}

type diagCompleteMsg struct {
	report diag.DiagReport
}

func NewRescuePanel() RescuePanel {
	return RescuePanel{mode: rescueModeInput}
}

func (r RescuePanel) Init() tea.Cmd {
	return nil
}

func (r RescuePanel) Update(msg tea.Msg) (RescuePanel, tea.Cmd) {
	switch msg := msg.(type) {
	case diagCompleteMsg:
		r.mode = rescueModeResults
		r.report = msg.report
		return r, nil

	case tea.KeyMsg:
		switch r.mode {
		case rescueModeInput:
			return r.updateInput(msg)
		case rescueModeResults:
			return r.updateResults(msg)
		}
	}
	return r, nil
}

func (r RescuePanel) updateInput(msg tea.KeyMsg) (RescuePanel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if len(r.target) > 0 {
			r.mode = rescueModeRunning
			target := r.target
			return r, func() tea.Msg {
				report := diag.RunDiagnostics(target)
				return diagCompleteMsg{report: report}
			}
		}
	case "backspace":
		if len(r.target) > 0 {
			r.target = r.target[:len(r.target)-1]
		}
	default:
		ch := msg.String()
		if len(ch) == 1 {
			r.target += ch
		}
	}
	return r, nil
}

func (r RescuePanel) updateResults(msg tea.KeyMsg) (RescuePanel, tea.Cmd) {
	switch msg.String() {
	case "n":
		r.mode = rescueModeInput
		r.target = ""
		r.report = diag.DiagReport{}
		r.exportPath = ""
	case "r":
		if r.target != "" {
			r.mode = rescueModeRunning
			r.exportPath = ""
			target := r.target
			return r, func() tea.Msg {
				report := diag.RunDiagnostics(target)
				return diagCompleteMsg{report: report}
			}
		}
	case "e":
		path, err := r.exportReport()
		if err != nil {
			r.err = err
		} else {
			r.exportPath = path
		}
	}
	return r, nil
}

func (r RescuePanel) View() string {
	var b strings.Builder

	header := style.TitleStyle.Render("🩺 NetRescue")
	b.WriteString(header)
	b.WriteString("\n")
	desc := lipgloss.NewStyle().Foreground(style.Muted).Render("Diagnose connectivity issues — ping, DNS, TLS, firewall checks")
	b.WriteString(desc)
	b.WriteString("\n\n")

	switch r.mode {
	case rescueModeInput:
		b.WriteString(r.inputView())
	case rescueModeRunning:
		b.WriteString(r.runningView())
	case rescueModeResults:
		b.WriteString(r.resultsView())
	}

	return b.String()
}

func (r RescuePanel) inputView() string {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Foreground(style.Muted).Render("Target (host or host:port):"))
	b.WriteString("\n\n")

	prompt := lipgloss.NewStyle().Foreground(style.Secondary).Render("  > ")
	cursor := lipgloss.NewStyle().Foreground(style.Primary).Render("█")
	b.WriteString(prompt + r.target + cursor)
	b.WriteString("\n\n")

	examples := lipgloss.NewStyle().Foreground(style.Subtle).Render(
		"  Examples: google.com   192.168.1.1   myserver.com:443",
	)
	b.WriteString(examples)
	b.WriteString("\n\n")

	help := fmt.Sprintf("%s run diagnostics",
		style.HelpKeyStyle.Render("↵"),
	)
	b.WriteString(help)

	return b.String()
}

func (r RescuePanel) runningView() string {
	var b strings.Builder

	target := lipgloss.NewStyle().Bold(true).Foreground(style.Primary).Render(r.target)
	b.WriteString(fmt.Sprintf("Diagnosing %s ...\n\n", target))

	checks := []string{"Local IP", "Gateway", "DNS", "Ping", "Traceroute", "Port", "TLS", "Firewall"}
	spinner := lipgloss.NewStyle().Foreground(style.Warning).Render("⠋")
	for _, check := range checks {
		b.WriteString(fmt.Sprintf("  %s %s\n", spinner, check))
	}

	return b.String()
}

func (r RescuePanel) resultsView() string {
	var b strings.Builder

	target := lipgloss.NewStyle().Bold(true).Foreground(style.Primary).Render(r.report.Target)
	b.WriteString(fmt.Sprintf("Results for %s\n\n", target))

	passCount := 0
	failCount := 0
	warnCount := 0

	for _, result := range r.report.Results {
		icon, color := statusIcon(result.Status)
		switch result.Status {
		case diag.StatusPass:
			passCount++
		case diag.StatusFail:
			failCount++
		case diag.StatusWarn:
			warnCount++
		}

		name := lipgloss.NewStyle().Width(18).Render(result.Name)
		statusStr := lipgloss.NewStyle().Foreground(color).Render(icon)
		detail := lipgloss.NewStyle().Foreground(style.Muted).Render(result.Detail)

		b.WriteString(fmt.Sprintf("  %s %s %s\n", statusStr, name, detail))
	}

	b.WriteString("\n")

	summary := lipgloss.NewStyle().Bold(true)
	if failCount > 0 {
		b.WriteString(summary.Foreground(style.Danger).Render(
			fmt.Sprintf("  ✕ %d failed", failCount),
		))
		b.WriteString("  ")
	}
	if warnCount > 0 {
		b.WriteString(summary.Foreground(style.Warning).Render(
			fmt.Sprintf("  ▲ %d warnings", warnCount),
		))
		b.WriteString("  ")
	}
	b.WriteString(summary.Foreground(style.Success).Render(
		fmt.Sprintf("  ● %d passed", passCount),
	))
	b.WriteString("\n")

	if failCount > 0 {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(style.Danger).Render("  Diagnosis: "))
		b.WriteString(r.diagnose())
		b.WriteString("\n")
	}

	if r.exportPath != "" {
		b.WriteString("\n")
		exported := lipgloss.NewStyle().Foreground(style.Success).Render("● Exported to: " + r.exportPath)
		b.WriteString(exported)
	}

	b.WriteString("\n")
	help := fmt.Sprintf("%s new target  %s re-run  %s export report",
		style.HelpKeyStyle.Render("n"),
		style.HelpKeyStyle.Render("r"),
		style.HelpKeyStyle.Render("e"),
	)
	b.WriteString(help)

	return b.String()
}

func (r RescuePanel) diagnose() string {
	for _, result := range r.report.Results {
		if result.Status == diag.StatusFail {
			switch result.Name {
			case "Local IP":
				return "No network interface has an IP. Check cable/WiFi connection."
			case "Gateway":
				return "No default gateway. DHCP may have failed or the network is misconfigured."
			case "DNS Resolution":
				return fmt.Sprintf("Cannot resolve %s. DNS server unreachable or hostname invalid.", r.report.Host)
			case "Ping":
				return fmt.Sprintf("%s is unreachable. Host may be down or ICMP is blocked.", r.report.Host)
			default:
				if strings.HasPrefix(result.Name, "Port") {
					return fmt.Sprintf("Port %s is closed/filtered. Service may be down or firewall is blocking.", r.report.Port)
				}
				if result.Name == "TLS Handshake" {
					return "TLS handshake failed. Certificate or protocol issue."
				}
			}
		}
	}
	return "Check the warnings above for details."
}

func (r *RescuePanel) SetSize(width, height int) {
	r.width = width
	r.height = height
}

func (r RescuePanel) exportReport() (string, error) {
	var b strings.Builder

	now := time.Now()
	b.WriteString(fmt.Sprintf("# NetuiPilot Diagnostic Report\n\n"))
	b.WriteString(fmt.Sprintf("**Target:** %s\n", r.report.Target))
	b.WriteString(fmt.Sprintf("**Date:** %s\n\n", now.Format("2006-01-02 15:04:05")))
	b.WriteString("## Results\n\n")
	b.WriteString("| Check | Status | Detail |\n")
	b.WriteString("|-------|--------|--------|\n")

	passCount, failCount, warnCount := 0, 0, 0
	for _, result := range r.report.Results {
		icon := "?"
		switch result.Status {
		case diag.StatusPass:
			icon = "PASS"
			passCount++
		case diag.StatusWarn:
			icon = "WARN"
			warnCount++
		case diag.StatusFail:
			icon = "FAIL"
			failCount++
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", result.Name, icon, result.Detail))
	}

	b.WriteString(fmt.Sprintf("\n## Summary\n\n"))
	b.WriteString(fmt.Sprintf("- **Passed:** %d\n", passCount))
	b.WriteString(fmt.Sprintf("- **Warnings:** %d\n", warnCount))
	b.WriteString(fmt.Sprintf("- **Failed:** %d\n", failCount))

	if failCount > 0 {
		b.WriteString(fmt.Sprintf("\n## Diagnosis\n\n%s\n", r.diagnose()))
	}

	b.WriteString(fmt.Sprintf("\n---\n*Generated by NetuiPilot*\n"))

	filename := fmt.Sprintf("netuipilot-diag-%s.md", now.Format("2006-01-02-15h04"))

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	path := filepath.Join(home, filename)

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func statusIcon(s diag.Status) (string, lipgloss.Color) {
	switch s {
	case diag.StatusPass:
		return "●", style.Success
	case diag.StatusWarn:
		return "▲", style.Warning
	case diag.StatusFail:
		return "✕", style.Danger
	default:
		return "○", style.Muted
	}
}
