package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Ultra2000/netuipilot/internal/diag"
)

// runCLI handles non-TUI subcommands. Returns true if a subcommand was handled
// (the caller should then exit with the returned code), false to launch the TUI.
func runCLI(args []string) (handled bool, code int) {
	if len(args) == 0 {
		return false, 0
	}

	switch args[0] {
	case "--version", "version":
		fmt.Printf("netuipilot %s (%s)\n", version, commit)
		return true, 0

	case "--help", "-h", "help":
		printUsage()
		return true, 0

	case "check":
		return cmdCheck(args[1:])

	case "dns":
		return cmdDNS(args[1:])

	case "wifi":
		return cmdWiFi(args[1:])

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		return true, 2
	}
}

func printUsage() {
	fmt.Print(`netuipilot - TUI network manager for Linux

Usage:
  netuipilot                        Launch the interactive TUI
  netuipilot check <host[:port]>    Run network diagnostics, exit 0 if healthy
  netuipilot dns set <iface> <servers...>   Set DNS servers for an interface
  netuipilot dns reset <iface>      Revert DNS for an interface
  netuipilot wifi connect <ssid> [password]   Connect to a WiFi network
  netuipilot --version              Print version
  netuipilot --help                 Show this help

Examples:
  netuipilot check google.com
  netuipilot check myserver.com:443
  netuipilot dns set eth0 1.1.1.1 1.0.0.1
  netuipilot wifi connect MyNetwork hunter2
`)
}

func cmdCheck(args []string) (bool, int) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: netuipilot check <host[:port]>")
		return true, 2
	}

	report := diag.RunDiagnostics(args[0])

	failCount := 0
	for _, r := range report.Results {
		var tag string
		switch r.Status {
		case diag.StatusPass:
			tag = "PASS"
		case diag.StatusWarn:
			tag = "WARN"
		case diag.StatusFail:
			tag = "FAIL"
			failCount++
		default:
			tag = "----"
		}
		fmt.Printf("[%s] %-18s %s\n", tag, r.Name, r.Detail)
	}

	if failCount > 0 {
		fmt.Printf("\n%d check(s) failed\n", failCount)
		return true, 1
	}
	fmt.Println("\nAll checks passed")
	return true, 0
}

func cmdDNS(args []string) (bool, int) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: netuipilot dns set <iface> <servers...> | dns reset <iface>")
		return true, 2
	}

	switch args[0] {
	case "set":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: netuipilot dns set <iface> <servers...>")
			return true, 2
		}
		iface := args[1]
		servers := args[2:]
		cmdArgs := append([]string{"dns", iface}, servers...)
		if err := exec.Command("resolvectl", cmdArgs...).Run(); err != nil {
			fallback := append([]string{"--interface", iface, "--set-dns"}, servers...)
			if err2 := exec.Command("systemd-resolve", fallback...).Run(); err2 != nil {
				fmt.Fprintf(os.Stderr, "failed to set DNS: %v\n", err)
				return true, 1
			}
		}
		fmt.Printf("DNS for %s set to %s\n", iface, strings.Join(servers, ", "))
		return true, 0

	case "reset":
		iface := args[1]
		if err := exec.Command("resolvectl", "revert", iface).Run(); err != nil {
			if err2 := exec.Command("systemd-resolve", "--interface", iface, "--revert").Run(); err2 != nil {
				fmt.Fprintf(os.Stderr, "failed to reset DNS: %v\n", err)
				return true, 1
			}
		}
		fmt.Printf("DNS for %s reverted\n", iface)
		return true, 0

	default:
		fmt.Fprintf(os.Stderr, "unknown dns subcommand: %s\n", args[0])
		return true, 2
	}
}

func cmdWiFi(args []string) (bool, int) {
	if len(args) < 2 || args[0] != "connect" {
		fmt.Fprintln(os.Stderr, "usage: netuipilot wifi connect <ssid> [password]")
		return true, 2
	}

	ssid := args[1]
	nmArgs := []string{"device", "wifi", "connect", ssid}
	if len(args) >= 3 {
		nmArgs = append(nmArgs, "password", args[2])
	}

	out, err := exec.Command("nmcli", nmArgs...).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %v\n%s\n", err, strings.TrimSpace(string(out)))
		return true, 1
	}
	fmt.Printf("Connected to %s\n", ssid)
	return true, 0
}
