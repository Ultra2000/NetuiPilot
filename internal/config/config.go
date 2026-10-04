package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Profile struct {
	Name      string   `json:"name"`
	WiFiSSID  string   `json:"wifi_ssid,omitempty"`
	DNSServers []string `json:"dns_servers,omitempty"`
	DNSIface  string   `json:"dns_iface,omitempty"`
	VPNName   string   `json:"vpn_name,omitempty"`
}

type Config struct {
	HiddenInterfaces []string  `json:"hidden_interfaces"`
	RefreshInterval  int       `json:"refresh_interval_ms"`
	Theme            Theme     `json:"theme"`
	Profiles         []Profile `json:"profiles,omitempty"`
}

type Theme struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	Accent    string `json:"accent"`
	Success   string `json:"success"`
	Warning   string `json:"warning"`
	Danger    string `json:"danger"`
}

func DefaultConfig() Config {
	return Config{
		HiddenInterfaces: []string{"lo"},
		RefreshInterval:  1000,
		Theme: Theme{
			Primary:   "#7F77DD",
			Secondary: "#1D9E75",
			Accent:    "#D85A30",
			Success:   "#639922",
			Warning:   "#EF9F27",
			Danger:    "#E24B4A",
		},
	}
}

func Load() Config {
	cfg := DefaultConfig()

	configDir, err := os.UserConfigDir()
	if err != nil {
		return cfg
	}

	path := filepath.Join(configDir, "netuipilot", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	_ = json.Unmarshal(data, &cfg)

	if len(cfg.HiddenInterfaces) == 0 {
		cfg.HiddenInterfaces = []string{"lo"}
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 1000
	}

	return cfg
}

func (c Config) IsHidden(iface string) bool {
	for _, h := range c.HiddenInterfaces {
		if h == iface {
			return true
		}
	}
	return false
}

func (c Config) Save() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(configDir, "netuipilot")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
}
