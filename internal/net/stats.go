package net

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// InterfaceStats holds network statistics for a single interface.
type InterfaceStats struct {
	Name    string
	RXBytes uint64
	TXBytes uint64
	RXPkts  uint64
	TXPkts  uint64
	RXErrs  uint64
	TXErrs  uint64
}

// BandwidthSample holds a bandwidth measurement at a point in time.
type BandwidthSample struct {
	Time       time.Time
	RXBytesPS  float64 // bytes per second
	TXBytesPS  float64
}

// BandwidthTracker tracks bandwidth over time for an interface.
type BandwidthTracker struct {
	Interface string
	History   []BandwidthSample
	MaxLen    int
	lastStats *InterfaceStats
	lastTime  time.Time
}

// NewBandwidthTracker creates a new tracker for the given interface.
func NewBandwidthTracker(iface string, maxLen int) *BandwidthTracker {
	return &BandwidthTracker{
		Interface: iface,
		MaxLen:    maxLen,
		History:   make([]BandwidthSample, 0, maxLen),
	}
}

// Update reads current stats and computes bandwidth since last call.
func (bt *BandwidthTracker) Update() error {
	stats, err := GetInterfaceStats(bt.Interface)
	if err != nil {
		return err
	}

	now := time.Now()

	if bt.lastStats != nil {
		elapsed := now.Sub(bt.lastTime).Seconds()
		if elapsed > 0 {
			sample := BandwidthSample{
				Time:      now,
				RXBytesPS: float64(stats.RXBytes-bt.lastStats.RXBytes) / elapsed,
				TXBytesPS: float64(stats.TXBytes-bt.lastStats.TXBytes) / elapsed,
			}
			bt.History = append(bt.History, sample)
			if len(bt.History) > bt.MaxLen {
				bt.History = bt.History[1:]
			}
		}
	}

	bt.lastStats = stats
	bt.lastTime = now
	return nil
}

// GetInterfaceStats reads /sys/class/net/<iface>/statistics for a single interface.
func GetInterfaceStats(iface string) (*InterfaceStats, error) {
	basePath := fmt.Sprintf("/sys/class/net/%s/statistics", iface)

	stats := &InterfaceStats{Name: iface}
	var err error

	stats.RXBytes, err = readSysUint64(basePath + "/rx_bytes")
	if err != nil {
		return nil, err
	}
	stats.TXBytes, err = readSysUint64(basePath + "/tx_bytes")
	if err != nil {
		return nil, err
	}
	stats.RXPkts, err = readSysUint64(basePath + "/rx_packets")
	if err != nil {
		return nil, err
	}
	stats.TXPkts, err = readSysUint64(basePath + "/tx_packets")
	if err != nil {
		return nil, err
	}
	stats.RXErrs, _ = readSysUint64(basePath + "/rx_errors")
	stats.TXErrs, _ = readSysUint64(basePath + "/tx_errors")

	return stats, nil
}

// GetAllInterfaceStats reads /proc/net/dev for all interfaces.
func GetAllInterfaceStats() ([]InterfaceStats, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/net/dev: %w", err)
	}
	defer file.Close()

	var stats []InterfaceStats
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= 2 {
			continue // skip headers
		}

		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		rxPkts, _ := strconv.ParseUint(fields[1], 10, 64)
		rxErrs, _ := strconv.ParseUint(fields[2], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)
		txPkts, _ := strconv.ParseUint(fields[9], 10, 64)
		txErrs, _ := strconv.ParseUint(fields[10], 10, 64)

		stats = append(stats, InterfaceStats{
			Name:    name,
			RXBytes: rxBytes,
			TXBytes: txBytes,
			RXPkts:  rxPkts,
			TXPkts:  txPkts,
			RXErrs:  rxErrs,
			TXErrs:  txErrs,
		})
	}

	return stats, scanner.Err()
}

// GetActiveConnections reads /proc/net/tcp and /proc/net/tcp6 for active connections.
func GetActiveConnections() (int, error) {
	count := 0
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		file, err := os.Open(path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if lineNum <= 1 {
				continue
			}
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 4 {
				// State 01 = ESTABLISHED
				if fields[3] == "01" {
					count++
				}
			}
		}
		file.Close()
	}
	return count, nil
}

func readSysUint64(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

// FormatBytes formats bytes to a human-readable string.
func FormatBytes(bytes float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	unit := 0
	for bytes >= 1024 && unit < len(units)-1 {
		bytes /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f %s", bytes, units[unit])
	}
	return fmt.Sprintf("%.1f %s", bytes, units[unit])
}

// FormatBytesPerSec formats bytes/s to a human-readable throughput string.
func FormatBytesPerSec(bps float64) string {
	return FormatBytes(bps) + "/s"
}
