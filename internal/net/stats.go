package net

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

// GetActiveConnections reads /proc/net/tcp and /proc/net/tcp6 for active connections count.
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
				if fields[3] == "01" {
					count++
				}
			}
		}
		file.Close()
	}
	return count, nil
}

// SocketEntry represents a single TCP/UDP connection.
type SocketEntry struct {
	Proto    string
	LocalIP  string
	LocalPort uint16
	RemoteIP string
	RemotePort uint16
	State    string
	Inode    string
	PID      int
	Process  string
}

var tcpStates = map[string]string{
	"01": "ESTABLISHED",
	"02": "SYN_SENT",
	"03": "SYN_RECV",
	"04": "FIN_WAIT1",
	"05": "FIN_WAIT2",
	"06": "TIME_WAIT",
	"07": "CLOSE",
	"08": "CLOSE_WAIT",
	"09": "LAST_ACK",
	"0A": "LISTEN",
	"0B": "CLOSING",
}

// GetDetailedConnections returns all TCP/UDP connections with details.
func GetDetailedConnections() ([]SocketEntry, error) {
	var entries []SocketEntry

	for _, info := range []struct {
		path  string
		proto string
		ipv6  bool
	}{
		{"/proc/net/tcp", "tcp", false},
		{"/proc/net/tcp6", "tcp6", true},
		{"/proc/net/udp", "udp", false},
		{"/proc/net/udp6", "udp6", true},
	} {
		parsed, err := parseNetFile(info.path, info.proto, info.ipv6)
		if err != nil {
			continue
		}
		entries = append(entries, parsed...)
	}

	resolveProcesses(entries)

	return entries, nil
}

// resolveProcesses maps socket inodes to owning PID/process by scanning /proc.
func resolveProcesses(entries []SocketEntry) {
	inodeToPID := buildInodeMap()
	if len(inodeToPID) == 0 {
		return
	}
	for i := range entries {
		if pi, ok := inodeToPID[entries[i].Inode]; ok {
			entries[i].PID = pi.pid
			entries[i].Process = pi.name
		}
	}
}

type procInfo struct {
	pid  int
	name string
}

// buildInodeMap scans /proc/<pid>/fd/* for socket:[inode] links.
func buildInodeMap() map[string]procInfo {
	result := make(map[string]procInfo)

	procDir, err := os.Open("/proc")
	if err != nil {
		return result
	}
	defer procDir.Close()

	names, err := procDir.Readdirnames(-1)
	if err != nil {
		return result
	}

	for _, name := range names {
		pid, err := strconv.Atoi(name)
		if err != nil {
			continue
		}

		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		procName := ""
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(link, "socket:[") {
				inode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
				if procName == "" {
					procName = readProcName(pid)
				}
				result[inode] = procInfo{pid: pid, name: procName}
			}
		}
	}

	return result
}

func readProcName(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// KillProcess sends SIGTERM to the given PID.
func KillProcess(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

func parseNetFile(path, proto string, ipv6 bool) ([]SocketEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []SocketEntry
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= 1 {
			continue
		}

		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}

		localIP, localPort := parseAddr(fields[1], ipv6)
		remoteIP, remotePort := parseAddr(fields[2], ipv6)

		state := fields[3]
		stateName := tcpStates[state]
		if stateName == "" {
			stateName = state
		}

		// Skip unconnected UDP
		if (proto == "udp" || proto == "udp6") && remotePort == 0 {
			stateName = "LISTEN"
		}

		inode := ""
		if len(fields) >= 10 {
			inode = fields[9]
		}

		entries = append(entries, SocketEntry{
			Proto:      proto,
			LocalIP:    localIP,
			LocalPort:  localPort,
			RemoteIP:   remoteIP,
			RemotePort: remotePort,
			State:      stateName,
			Inode:      inode,
		})
	}

	return entries, scanner.Err()
}

func parseAddr(s string, ipv6 bool) (string, uint16) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return s, 0
	}

	port, _ := strconv.ParseUint(parts[1], 16, 16)

	if ipv6 {
		return parseIPv6(parts[0]), uint16(port)
	}
	return parseIPv4(parts[0]), uint16(port)
}

func parseIPv4(hex string) string {
	if len(hex) != 8 {
		return hex
	}
	b := make([]byte, 4)
	for i := 0; i < 4; i++ {
		val, _ := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		b[i] = byte(val)
	}
	// /proc/net uses little-endian on little-endian systems
	return fmt.Sprintf("%d.%d.%d.%d", b[3], b[2], b[1], b[0])
}

func parseIPv6(hex string) string {
	if len(hex) != 32 {
		return hex
	}
	if hex == "00000000000000000000000000000000" {
		return "::"
	}
	if hex[:24] == "000000000000000000000000" {
		return parseIPv4(hex[24:])
	}
	// Simplified: show as IPv4-mapped if possible
	if hex[:20] == "0000000000000000FFFF" || hex[:20] == "0000000000000000ffff" {
		return parseIPv4(hex[24:])
	}
	return hex
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
