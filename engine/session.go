package engine

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// StartOpts configures a new awake session.
type StartOpts struct {
	Minutes int
	Until   time.Time
	Mode    string // "manual", "workday", "preset"
	Label   string
}

// StatusInfo is a snapshot of the current session for display.
type StatusInfo struct {
	Active        bool          `json:"active"`
	Mode          string        `json:"mode,omitempty"`
	Label         string        `json:"label,omitempty"`
	StartedAt     time.Time     `json:"started_at,omitempty"`
	EndsAt        time.Time     `json:"ends_at,omitempty"`
	TimeRemaining time.Duration `json:"time_remaining,omitempty"`
	PID           int           `json:"pid,omitempty"`
	Flags         string        `json:"flags,omitempty"`
	Command       string        `json:"command,omitempty"`
}

// StartSession launches a new caffeinate process. Errors if one is already active.
func StartSession(cfg *Config, st *State, opts StartOpts) error {
	if IsActive(st) {
		return fmt.Errorf("session already active (PID %d) — stop it first or use extend", st.Active.PID)
	}

	var endsAt time.Time
	var durationSec int
	now := time.Now()

	if !opts.Until.IsZero() {
		endsAt = opts.Until
		if endsAt.Before(now) {
			endsAt = endsAt.Add(24 * time.Hour)
		}
		durationSec = int(endsAt.Sub(now).Seconds())
	} else {
		durationSec = opts.Minutes * 60
		endsAt = now.Add(time.Duration(durationSec) * time.Second)
	}

	maxSec := cfg.MaxDurationH * 3600
	if durationSec > maxSec {
		return fmt.Errorf("duration exceeds maximum of %d hours", cfg.MaxDurationH)
	}
	if durationSec <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	flags := strings.Fields(cfg.Flags)
	flags = append(flags, "-t", fmt.Sprintf("%d", durationSec))

	cmd := exec.Command("/usr/bin/caffeinate", flags...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start caffeinate: %w", err)
	}

	cmdStr := fmt.Sprintf("/usr/bin/caffeinate %s", strings.Join(flags, " "))

	st.SetActive(&ActiveSession{
		PID:       cmd.Process.Pid,
		StartedAt: now,
		EndsAt:    endsAt,
		Mode:      opts.Mode,
		Label:     opts.Label,
		Flags:     cfg.Flags,
		Command:   cmdStr,
	})

	if err := st.Save(); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to save state: %w", err)
	}

	if cfg.Notifications.Enabled {
		dur := time.Duration(durationSec) * time.Second
		msg := fmt.Sprintf("Session started — %s", FormatDuration(dur))
		if opts.Label != "" {
			msg = fmt.Sprintf("[%s] %s", opts.Label, msg)
		}
		Notify("Awake", msg)
		startNotifyWatcher(endsAt, cfg.Notifications.WarnMinutes, opts.Label)
	}

	return nil
}

// StopSession terminates the active caffeinate process and records it in history.
func StopSession(cfg *Config, st *State) error {
	if st.Active == nil {
		return fmt.Errorf("no active session")
	}

	proc, err := os.FindProcess(st.Active.PID)
	if err == nil {
		proc.Signal(syscall.SIGTERM)
	}

	killNotifyWatcher()
	label := st.Active.Label
	st.ClearActive()

	if err := st.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	if cfg.Notifications.Enabled {
		msg := "Session stopped"
		if label != "" {
			msg = fmt.Sprintf("[%s] %s", label, msg)
		}
		Notify("Awake", msg)
	}

	return nil
}

// ExtendSession adds time to the current session by restarting caffeinate
// with an adjusted timeout.
func ExtendSession(cfg *Config, st *State, minutes int) error {
	if !IsActive(st) {
		return fmt.Errorf("no active session to extend")
	}

	newEndsAt := st.Active.EndsAt.Add(time.Duration(minutes) * time.Minute)
	newDurationSec := int(time.Until(newEndsAt).Seconds())

	if newDurationSec <= 0 {
		return fmt.Errorf("extended duration would already be expired")
	}

	proc, err := os.FindProcess(st.Active.PID)
	if err == nil {
		proc.Signal(syscall.SIGTERM)
	}
	killNotifyWatcher()

	flags := strings.Fields(cfg.Flags)
	flags = append(flags, "-t", fmt.Sprintf("%d", newDurationSec))

	cmd := exec.Command("/usr/bin/caffeinate", flags...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start caffeinate: %w", err)
	}

	cmdStr := fmt.Sprintf("/usr/bin/caffeinate %s", strings.Join(flags, " "))

	st.Active.PID = cmd.Process.Pid
	st.Active.EndsAt = newEndsAt
	st.Active.Command = cmdStr

	if err := st.Save(); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to save state: %w", err)
	}

	if cfg.Notifications.Enabled {
		remaining := time.Until(newEndsAt)
		msg := fmt.Sprintf("Extended by %dm — %s remaining", minutes, FormatDuration(remaining))
		if st.Active.Label != "" {
			msg = fmt.Sprintf("[%s] %s", st.Active.Label, msg)
		}
		Notify("Awake", msg)
		startNotifyWatcher(newEndsAt, cfg.Notifications.WarnMinutes, st.Active.Label)
	}

	return nil
}

// ForceReplace stops any active session and starts a new one.
func ForceReplace(cfg *Config, st *State, opts StartOpts) error {
	if IsActive(st) {
		proc, err := os.FindProcess(st.Active.PID)
		if err == nil {
			proc.Signal(syscall.SIGTERM)
		}
		killNotifyWatcher()
		st.ClearActive()
		st.Save()
	}
	return StartSession(cfg, st, opts)
}

// IsActive checks whether the recorded session is actually still running.
func IsActive(st *State) bool {
	if st.Active == nil {
		return false
	}

	proc, err := os.FindProcess(st.Active.PID)
	if err != nil {
		st.ClearActive()
		st.Save()
		return false
	}

	// Signal(0) tests process existence without actually signaling.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		st.ClearActive()
		st.Save()
		return false
	}

	if time.Now().After(st.Active.EndsAt) {
		st.ClearActive()
		st.Save()
		return false
	}

	return true
}

// GetStatus returns a display-ready snapshot of the current session.
func GetStatus(cfg *Config, st *State) StatusInfo {
	if !IsActive(st) {
		return StatusInfo{Active: false}
	}

	return StatusInfo{
		Active:        true,
		Mode:          st.Active.Mode,
		Label:         st.Active.Label,
		StartedAt:     st.Active.StartedAt,
		EndsAt:        st.Active.EndsAt,
		TimeRemaining: time.Until(st.Active.EndsAt),
		PID:           st.Active.PID,
		Flags:         st.Active.Flags,
		Command:       st.Active.Command,
	}
}

// ScheduleWindow sets up a future awake session. If the current session already
// covers the requested window, it returns an informational error. If the current
// session partially overlaps, the window is accepted for the uncovered portion.
func ScheduleWindow(cfg *Config, st *State, startsAt, endsAt time.Time, label string) error {
	now := time.Now()

	if endsAt.Before(startsAt) {
		return fmt.Errorf("end time must be after start time")
	}
	if endsAt.Before(now) {
		return fmt.Errorf("window has already passed")
	}

	// Check if active session already covers this window
	if IsActive(st) {
		if st.Active.EndsAt.After(endsAt) || st.Active.EndsAt.Equal(endsAt) {
			return fmt.Errorf("current session already covers this window (ends %s)",
				cfg.FormatTime(st.Active.EndsAt))
		}
	}

	// If window starts now or in the past, start it immediately
	if !startsAt.After(now) {
		if IsActive(st) {
			// Extend current session to cover the window
			extraMin := int(endsAt.Sub(st.Active.EndsAt).Minutes()) + 1
			return ExtendSession(cfg, st, extraMin)
		}
		return StartSession(cfg, st, StartOpts{
			Until: endsAt,
			Mode:  "scheduled",
			Label: label,
		})
	}

	// Future window — persist it for the daemon to activate
	st.Scheduled = &ScheduledWindow{
		StartsAt: startsAt,
		EndsAt:   endsAt,
		Label:    label,
	}

	if err := st.Save(); err != nil {
		return fmt.Errorf("failed to save scheduled window: %w", err)
	}

	if cfg.Notifications.Enabled {
		msg := fmt.Sprintf("Scheduled %s – %s", cfg.FormatTime(startsAt), cfg.FormatTime(endsAt))
		if label != "" {
			msg = fmt.Sprintf("[%s] %s", label, msg)
		}
		Notify("Awake", msg)
	}

	return nil
}

// CancelSchedule removes a pending scheduled window.
func CancelSchedule(st *State) error {
	if st.Scheduled == nil {
		return fmt.Errorf("no scheduled window")
	}
	st.Scheduled = nil
	return st.Save()
}

// ActivateScheduled is called by the daemon when a scheduled window's start
// time arrives. It starts the session and clears the schedule.
func ActivateScheduled(cfg *Config, st *State) error {
	if st.Scheduled == nil {
		return fmt.Errorf("no scheduled window to activate")
	}

	window := st.Scheduled
	st.Scheduled = nil
	st.Save()

	opts := StartOpts{
		Until: window.EndsAt,
		Mode:  "scheduled",
		Label: window.Label,
	}

	if IsActive(st) {
		return ForceReplace(cfg, st, opts)
	}
	return StartSession(cfg, st, opts)
}

// ParseUntilTime parses "HH:MM" into a time.Time for today, rolling to tomorrow if past.
func ParseUntilTime(timeStr string) (time.Time, error) {
	now := time.Now()
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time format %q — use HH:MM", timeStr)
	}

	target := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if target.Before(now) {
		target = target.Add(24 * time.Hour)
	}
	return target, nil
}

// WorkdayEnd returns the end-of-day time based on config, or error if not a workday.
func WorkdayEnd(cfg *Config) (time.Time, error) {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	isWorkday := false
	for _, d := range cfg.Workday.Days {
		if d == weekday {
			isWorkday = true
			break
		}
	}
	if !isWorkday {
		return time.Time{}, fmt.Errorf("today is not a workday")
	}

	endTime, err := time.Parse("15:04", cfg.Workday.End)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid workday end time: %w", err)
	}

	target := time.Date(now.Year(), now.Month(), now.Day(), endTime.Hour(), endTime.Minute(), 0, 0, now.Location())
	if target.Before(now) {
		return time.Time{}, fmt.Errorf("workday has already ended")
	}

	return target, nil
}

// FormatDuration renders a duration as a human-friendly string.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
