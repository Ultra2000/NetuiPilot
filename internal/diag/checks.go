package diag

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type CheckResult struct {
	Name    string
	Status  Status
	Detail  string
	Latency time.Duration
}

type Status int

const (
	StatusPass Status = iota
	StatusWarn
	StatusFail
	StatusRunning
)

type DiagReport struct {
	Target  string
	Host    string
	Port    string
	Results []CheckResult
	Done    bool
}

func RunDiagnostics(target string) DiagReport {
	host, port := parseTarget(target)

	report := DiagReport{
		Target: target,
		Host:   host,
		Port:   port,
	}

	report.Results = append(report.Results, checkLocalNetwork())
	report.Results = append(report.Results, checkGateway())
	report.Results = append(report.Results, checkDNS(host))
	report.Results = append(report.Results, checkPing(host))
	report.Results = append(report.Results, checkTraceroute(host))

	if port != "" {
		report.Results = append(report.Results, checkPort(host, port))
		if port == "443" || port == "8443" {
			report.Results = append(report.Results, checkTLS(host, port))
		}
	}

	report.Results = append(report.Results, checkFirewall(port))
	report.Done = true

	return report
}

func parseTarget(target string) (string, string) {
	if h, p, err := net.SplitHostPort(target); err == nil {
		return h, p
	}
	return target, ""
}

func checkLocalNetwork() CheckResult {
	r := CheckResult{Name: "Local IP"}

	ifaces, err := net.Interfaces()
	if err != nil {
		r.Status = StatusFail
		r.Detail = err.Error()
		return r
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				r.Status = StatusPass
				r.Detail = fmt.Sprintf("%s on %s", ipnet.IP.String(), iface.Name)
				return r
			}
		}
	}

	r.Status = StatusFail
	r.Detail = "No IPv4 address found"
	return r
}

func checkGateway() CheckResult {
	r := CheckResult{Name: "Gateway"}

	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		r.Status = StatusFail
		r.Detail = "Cannot read routes"
		return r
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		r.Status = StatusFail
		r.Detail = "No default gateway"
		return r
	}

	fields := strings.Fields(line)
	if len(fields) >= 3 {
		gw := fields[2]
		start := time.Now()
		_, err := exec.Command("ping", "-c", "1", "-W", "2", gw).Output()
		r.Latency = time.Since(start)
		if err != nil {
			r.Status = StatusWarn
			r.Detail = fmt.Sprintf("%s (unreachable)", gw)
		} else {
			r.Status = StatusPass
			r.Detail = fmt.Sprintf("%s (%s)", gw, r.Latency.Round(time.Millisecond))
		}
	}

	return r
}

func checkDNS(host string) CheckResult {
	r := CheckResult{Name: "DNS Resolution"}

	if net.ParseIP(host) != nil {
		r.Status = StatusPass
		r.Detail = "Target is an IP address"
		return r
	}

	start := time.Now()
	addrs, err := net.LookupHost(host)
	r.Latency = time.Since(start)

	if err != nil {
		r.Status = StatusFail
		r.Detail = fmt.Sprintf("Cannot resolve %s", host)
		return r
	}

	r.Status = StatusPass
	r.Detail = fmt.Sprintf("%s -> %s (%s)", host, addrs[0], r.Latency.Round(time.Millisecond))
	return r
}

func checkPing(host string) CheckResult {
	r := CheckResult{Name: "Ping"}

	start := time.Now()
	out, err := exec.Command("ping", "-c", "3", "-W", "3", host).Output()
	r.Latency = time.Since(start)

	if err != nil {
		r.Status = StatusFail
		r.Detail = "Host unreachable"
		return r
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "rtt") || strings.Contains(line, "round-trip") {
			parts := strings.Split(line, "=")
			if len(parts) >= 2 {
				r.Detail = strings.TrimSpace(parts[1])
			}
			break
		}
		if strings.Contains(line, "packet loss") {
			r.Detail = strings.TrimSpace(line)
		}
	}

	if strings.Contains(r.Detail, "100%") {
		r.Status = StatusFail
	} else if strings.Contains(r.Detail, "0%") || strings.Contains(r.Detail, "0.0%") {
		r.Status = StatusPass
	} else {
		r.Status = StatusWarn
		r.Detail = "Partial packet loss: " + r.Detail
	}

	return r
}

func checkTraceroute(host string) CheckResult {
	r := CheckResult{Name: "Route (tracepath)"}

	out, err := exec.Command("tracepath", "-m", "15", "-n", host).Output()
	if err != nil {
		out2, err2 := exec.Command("traceroute", "-m", "15", "-n", "-w", "2", host).Output()
		if err2 != nil {
			r.Status = StatusWarn
			r.Detail = "tracepath/traceroute not available"
			return r
		}
		out = out2
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	hops := 0
	lastHop := ""
	hasLoss := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "* * *") || strings.Contains(trimmed, "no reply") {
			hasLoss = true
		}
		hops++
		lastHop = trimmed
	}

	if hasLoss {
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("%d hops, some timeouts", hops)
	} else {
		r.Status = StatusPass
		r.Detail = fmt.Sprintf("%d hops", hops)
	}
	_ = lastHop

	return r
}

func checkPort(host, port string) CheckResult {
	r := CheckResult{Name: fmt.Sprintf("Port %s", port)}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 5*time.Second)
	r.Latency = time.Since(start)

	if err != nil {
		r.Status = StatusFail
		r.Detail = fmt.Sprintf("Connection refused/timeout (%s)", r.Latency.Round(time.Millisecond))
		return r
	}
	conn.Close()

	r.Status = StatusPass
	r.Detail = fmt.Sprintf("Open (%s)", r.Latency.Round(time.Millisecond))
	return r
}

func checkTLS(host, port string) CheckResult {
	r := CheckResult{Name: "TLS Handshake"}

	out, err := exec.Command("timeout", "5", "openssl", "s_client",
		"-connect", host+":"+port, "-servername", host, "-brief").CombinedOutput()

	if err != nil {
		r.Status = StatusFail
		r.Detail = "TLS handshake failed"
		return r
	}

	output := string(out)
	if strings.Contains(output, "Protocol version") || strings.Contains(output, "CONNECTION ESTABLISHED") {
		r.Status = StatusPass
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "Protocol") {
				r.Detail = strings.TrimSpace(line)
				break
			}
		}
		if r.Detail == "" {
			r.Detail = "TLS connection established"
		}
	} else if strings.Contains(output, "verify error") {
		r.Status = StatusWarn
		r.Detail = "Certificate verification failed"
	} else {
		r.Status = StatusFail
		r.Detail = "TLS handshake failed"
	}

	return r
}

func checkFirewall(port string) CheckResult {
	r := CheckResult{Name: "Local Firewall"}

	out, err := exec.Command("iptables", "-L", "OUTPUT", "-n", "--line-numbers").Output()
	if err != nil {
		out, err = exec.Command("nft", "list", "chain", "inet", "filter", "output").Output()
		if err != nil {
			r.Status = StatusPass
			r.Detail = "No firewall rules detected"
			return r
		}
	}

	output := string(out)
	if port != "" {
		portNum, _ := strconv.Atoi(port)
		if portNum > 0 && (strings.Contains(output, "DROP") || strings.Contains(output, "REJECT")) {
			if strings.Contains(output, port) {
				r.Status = StatusWarn
				r.Detail = fmt.Sprintf("Possible DROP/REJECT rule on port %s", port)
				return r
			}
		}
	}

	if strings.Contains(output, "DROP") || strings.Contains(output, "REJECT") {
		r.Status = StatusWarn
		r.Detail = "Firewall has DROP/REJECT rules (check manually)"
	} else {
		r.Status = StatusPass
		r.Detail = "No blocking rules detected"
	}

	return r
}
