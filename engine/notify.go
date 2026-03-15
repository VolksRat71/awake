package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// Notify sends a macOS notification via osascript.
func Notify(title, message string) {
	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	exec.Command("osascript", "-e", script).Run()
}

func watcherPidPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir, "watcher.pid")
}

// startNotifyWatcher spawns a background process that sends notifications
// at warn time and session end.
func startNotifyWatcher(endsAt time.Time, warnMinutes int, label string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	cmd := exec.Command(exe, "_notify-watch",
		endsAt.Format(time.RFC3339),
		strconv.Itoa(warnMinutes),
		label,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.Start()

	if cmd.Process != nil {
		os.WriteFile(watcherPidPath(), []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
	}
}

// killNotifyWatcher terminates a previously spawned watcher process.
func killNotifyWatcher() {
	data, err := os.ReadFile(watcherPidPath())
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	proc.Signal(syscall.SIGTERM)
	os.Remove(watcherPidPath())
}

// RunNotifyWatcher is the entry point for the hidden _notify-watch subprocess.
// It sleeps until the warning threshold, sends a warning notification,
// then sleeps until session end and sends a final notification.
func RunNotifyWatcher(endsAtStr string, warnMinutesStr string, label string) {
	endsAt, err := time.Parse(time.RFC3339, endsAtStr)
	if err != nil {
		return
	}
	warnMinutes, err := strconv.Atoi(warnMinutesStr)
	if err != nil {
		warnMinutes = 10
	}

	now := time.Now()
	warnAt := endsAt.Add(-time.Duration(warnMinutes) * time.Minute)

	if warnAt.After(now) {
		time.Sleep(time.Until(warnAt))
		remaining := time.Until(endsAt)
		msg := fmt.Sprintf("%d minutes remaining", int(remaining.Minutes()))
		if label != "" {
			msg = fmt.Sprintf("[%s] %s", label, msg)
		}
		Notify("Awake", msg)
	}

	if endsAt.After(time.Now()) {
		time.Sleep(time.Until(endsAt))
	}

	msg := "Session ended"
	if label != "" {
		msg = fmt.Sprintf("[%s] %s", label, msg)
	}
	Notify("Awake", msg)

	// Clean up state
	st, err := LoadState()
	if err == nil && st.Active != nil {
		st.ClearActive()
		st.Save()
	}

	os.Remove(watcherPidPath())
}
