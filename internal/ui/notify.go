package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/Ultra2000/netuipilot/internal/nm"
	"github.com/Ultra2000/netuipilot/internal/style"
)

type Notification struct {
	Message string
	Level   NotifyLevel
	Time    time.Time
}

type NotifyLevel int

const (
	NotifyInfo NotifyLevel = iota
	NotifyWarn
	NotifyError
)

type NotifyManager struct {
	notifications []Notification
	maxItems      int
	prevStates    map[string]nm.DeviceState
}

func NewNotifyManager() *NotifyManager {
	return &NotifyManager{
		maxItems:   10,
		prevStates: make(map[string]nm.DeviceState),
	}
}

func (n *NotifyManager) CheckDevices(devices []nm.Device) {
	for _, dev := range devices {
		if dev.Name == "lo" {
			continue
		}

		prev, known := n.prevStates[dev.Name]
		n.prevStates[dev.Name] = dev.State

		if !known {
			continue
		}

		if prev != dev.State {
			switch {
			case dev.State == nm.DeviceStateActivated && prev != nm.DeviceStateActivated:
				n.add(NotifyInfo, fmt.Sprintf("%s connected", dev.Name))
			case prev == nm.DeviceStateActivated && dev.State != nm.DeviceStateActivated:
				n.add(NotifyWarn, fmt.Sprintf("%s disconnected", dev.Name))
			case dev.State == nm.DeviceStateFailed:
				n.add(NotifyError, fmt.Sprintf("%s failed", dev.Name))
			}
		}
	}
}

func (n *NotifyManager) add(level NotifyLevel, msg string) {
	n.notifications = append(n.notifications, Notification{
		Message: msg,
		Level:   level,
		Time:    time.Now(),
	})
	if len(n.notifications) > n.maxItems {
		n.notifications = n.notifications[1:]
	}
}

func (n *NotifyManager) Recent(count int) []Notification {
	if len(n.notifications) == 0 {
		return nil
	}
	start := len(n.notifications) - count
	if start < 0 {
		start = 0
	}
	result := make([]Notification, len(n.notifications[start:]))
	copy(result, n.notifications[start:])
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func (n *NotifyManager) RenderBar() string {
	if len(n.notifications) == 0 {
		return ""
	}

	last := n.notifications[len(n.notifications)-1]

	age := time.Since(last.Time)
	if age > 30*time.Second {
		return ""
	}

	var icon string
	var fg lipgloss.Color

	switch last.Level {
	case NotifyInfo:
		icon = "●"
		fg = style.Success
	case NotifyWarn:
		icon = "▲"
		fg = style.Warning
	case NotifyError:
		icon = "✕"
		fg = style.Danger
	}

	return lipgloss.NewStyle().Foreground(fg).Render(
		fmt.Sprintf(" %s %s", icon, last.Message),
	)
}
