package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const stateFile = "state.json"
const maxHistory = 50

// HistoryEntry records a completed awake session.
type HistoryEntry struct {
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Duration  int       `json:"duration_minutes"`
	Mode      string    `json:"mode"`
	Label     string    `json:"label"`
}

// ActiveSession tracks a running caffeinate process.
type ActiveSession struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	EndsAt    time.Time `json:"ends_at"`
	Mode      string    `json:"mode"`
	Label     string    `json:"label"`
	Flags     string    `json:"flags"`
	Command   string    `json:"command"`
}

// State holds runtime session data persisted to disk.
type State struct {
	Active  *ActiveSession `json:"active,omitempty"`
	History []HistoryEntry `json:"history"`
}

func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, stateFile), nil
}

// LoadState reads state from disk, returning empty state if none exists.
func LoadState() (*State, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, err
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// Save writes state to disk atomically.
func (s *State) Save() error {
	path, err := statePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// SetActive marks a session as the current active session.
func (s *State) SetActive(sess *ActiveSession) {
	s.Active = sess
}

// ClearActive moves the current session into history and clears it.
func (s *State) ClearActive() {
	if s.Active != nil {
		entry := HistoryEntry{
			StartedAt: s.Active.StartedAt,
			EndedAt:   time.Now(),
			Duration:  int(time.Since(s.Active.StartedAt).Minutes()),
			Mode:      s.Active.Mode,
			Label:     s.Active.Label,
		}
		s.History = append([]HistoryEntry{entry}, s.History...)
		if len(s.History) > maxHistory {
			s.History = s.History[:maxHistory]
		}
		s.Active = nil
	}
}
