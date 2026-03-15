package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const configDir = ".config/awake"
const configFile = "config.json"

// WorkdayConfig defines the automatic work-hours schedule.
type WorkdayConfig struct {
	Start string `json:"start"` // "09:00"
	End   string `json:"end"`   // "17:00"
	Days  []int  `json:"days"`  // 1=Monday ... 7=Sunday
}

// Preset is a named quick-start option.
type Preset struct {
	Name    string `json:"name"`
	Minutes int    `json:"minutes,omitempty"`
	Until   string `json:"until,omitempty"` // "HH:MM"
}

// NotificationConfig controls macOS notification behavior.
type NotificationConfig struct {
	Enabled     bool `json:"enabled"`
	WarnMinutes int  `json:"warn_minutes"`
}

// Config holds all persistent user preferences.
type Config struct {
	Workday       WorkdayConfig      `json:"workday"`
	Flags         string             `json:"flags"`
	Presets       []Preset           `json:"presets"`
	Notifications NotificationConfig `json:"notifications"`
	MaxDurationH  int                `json:"max_duration_hours"`
}

// DefaultConfig returns sensible defaults for a new installation.
func DefaultConfig() *Config {
	return &Config{
		Workday: WorkdayConfig{
			Start: "09:00",
			End:   "17:00",
			Days:  []int{1, 2, 3, 4, 5},
		},
		Flags: "-dimsu",
		Presets: []Preset{
			{Name: "30 minutes", Minutes: 30},
			{Name: "1 hour", Minutes: 60},
			{Name: "4 hours", Minutes: 240},
			{Name: "Until lunch", Until: "12:00"},
			{Name: "Until end of workday", Until: "17:00"},
		},
		Notifications: NotificationConfig{
			Enabled:     true,
			WarnMinutes: 10,
		},
		MaxDurationH: 24,
	}
}

// ConfigPath returns the absolute path to the config file.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, configFile), nil
}

// LoadConfig reads config from disk, creating defaults if none exists.
func LoadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if err := cfg.Save(); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes the config to disk atomically.
func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
