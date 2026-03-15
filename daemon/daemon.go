package daemon

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/VolksRat71/awake/engine"
)

const pollInterval = 30 * time.Second

func pidPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config/awake/daemon.pid")
}

func logPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config/awake/daemon.log")
}

// IsRunning checks if the daemon process is alive.
func IsRunning() (bool, int) {
	data, err := os.ReadFile(pidPath())
	if err != nil {
		return false, 0
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false, 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		os.Remove(pidPath())
		return false, 0
	}
	return true, pid
}

// Run is the main daemon loop. It polls the state file and activates
// scheduled windows when their start time arrives. It also handles
// session-end cleanup and warning notifications.
func Run() {
	// Write PID file
	os.MkdirAll(filepath.Dir(pidPath()), 0755)
	os.WriteFile(pidPath(), []byte(strconv.Itoa(os.Getpid())), 0644)

	// Set up logging
	logFile, err := os.OpenFile(logPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}
	log.Println("awaked starting")

	// Startup notification
	cfg, err := engine.LoadConfig()
	if err == nil && cfg.Notifications.Enabled {
		engine.Notify("Awake", "Daemon started — keeping an eye on things")
	}

	// Clean shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Run once immediately, then on each tick
	poll()

	for {
		select {
		case <-ticker.C:
			poll()
		case s := <-sig:
			log.Printf("received %s, shutting down", s)
			os.Remove(pidPath())
			return
		}
	}
}

func poll() {
	cfg, err := engine.LoadConfig()
	if err != nil {
		log.Printf("config load error: %v", err)
		return
	}

	st, err := engine.LoadState()
	if err != nil {
		log.Printf("state load error: %v", err)
		return
	}

	// Check if a scheduled window should activate
	if st.Scheduled != nil {
		now := time.Now()
		if !st.Scheduled.StartsAt.After(now) {
			log.Printf("activating scheduled window: %s – %s",
				st.Scheduled.StartsAt.Format("15:04"),
				st.Scheduled.EndsAt.Format("15:04"))

			if err := engine.ActivateScheduled(cfg, st); err != nil {
				log.Printf("failed to activate scheduled window: %v", err)
			}
		}
	}

	// Auto-start workday session if enabled
	if cfg.AutoWorkday && !engine.IsActive(st) && st.Scheduled == nil {
		if endTime, err := engine.WorkdayEnd(cfg); err == nil {
			// We're on a workday and before EOD — start a session
			log.Printf("auto-starting workday session until %s", endTime.Format("15:04"))
			opts := engine.StartOpts{
				Until: endTime,
				Mode:  "workday",
				Label: "Workday",
			}
			if err := engine.StartSession(cfg, st, opts); err != nil {
				log.Printf("failed to auto-start workday: %v", err)
			}
		}
	}

	// Check if active session has ended (clean up stale state)
	if st.Active != nil {
		engine.IsActive(st) // side-effect: clears state if process is dead
	}
}

// Install creates or updates the launchd plist for the daemon.
func Install() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	plistDir := filepath.Join(home, "Library/LaunchAgents")
	os.MkdirAll(plistDir, 0755)
	plistPath := filepath.Join(plistDir, "com.awake.daemon.plist")

	logFile := logPath()

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.awake.daemon</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>daemon</string>
		<string>run</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>`, exe, logFile, logFile)

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	return nil
}

// PlistPath returns the path to the launchd plist.
func PlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library/LaunchAgents/com.awake.daemon.plist")
}

// Uninstall removes the launchd plist.
func Uninstall() error {
	return os.Remove(PlistPath())
}
