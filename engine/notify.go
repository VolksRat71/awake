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

func iconPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir, "icon.png")
}

func awakeAppPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir, "Awake.app")
}

func awakeAppBinary() string {
	return filepath.Join(awakeAppPath(), "Contents", "MacOS", "terminal-notifier")
}

// EnsureIcon writes the embedded icon to the config directory if not present.
func EnsureIcon(data []byte) {
	path := iconPath()
	if _, err := os.Stat(path); err == nil {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, data, 0644)
}

// InstallNotifierApp creates a custom Awake.app based on terminal-notifier
// with our icon baked in so macOS notifications show the awake eye.
func InstallNotifierApp() error {
	tnPath, err := findTerminalNotifierApp()
	if err != nil {
		return fmt.Errorf("terminal-notifier.app not found: %w", err)
	}

	dest := awakeAppPath()

	// Remove old copy
	os.RemoveAll(dest)

	// Copy terminal-notifier.app → Awake.app
	if out, err := exec.Command("cp", "-R", tnPath, dest).CombinedOutput(); err != nil {
		return fmt.Errorf("copy failed: %s", string(out))
	}

	// Convert our PNG to icns
	icnsPath := filepath.Join(dest, "Contents", "Resources", "Awake.icns")
	if err := pngToIcns(iconPath(), icnsPath); err != nil {
		return fmt.Errorf("icon conversion failed: %w", err)
	}

	// Remove the old icon
	oldIcon := filepath.Join(dest, "Contents", "Resources", "Terminal.icns")
	os.Remove(oldIcon)

	// Update Info.plist
	plistPath := filepath.Join(dest, "Contents", "Info.plist")
	plistUpdates := map[string]string{
		"CFBundleName":       "Awake",
		"CFBundleIdentifier": "com.awake.notifier",
		"CFBundleIconFile":   "Awake",
	}
	for key, val := range plistUpdates {
		exec.Command("/usr/libexec/PlistBuddy", "-c",
			fmt.Sprintf("Set :%s %s", key, val), plistPath).Run()
	}

	// Touch the app so macOS picks up the new icon
	exec.Command("touch", dest).Run()

	return nil
}

// findTerminalNotifierApp locates the .app bundle.
func findTerminalNotifierApp() (string, error) {
	// Check homebrew cellar
	matches, _ := filepath.Glob("/opt/homebrew/Cellar/terminal-notifier/*/terminal-notifier.app")
	if len(matches) > 0 {
		return matches[len(matches)-1], nil
	}
	// Check /usr/local for Intel macs
	matches, _ = filepath.Glob("/usr/local/Cellar/terminal-notifier/*/terminal-notifier.app")
	if len(matches) > 0 {
		return matches[len(matches)-1], nil
	}
	return "", fmt.Errorf("not found in homebrew cellar")
}

// pngToIcns converts a PNG to icns via iconutil.
func pngToIcns(pngPath, icnsPath string) error {
	tmpDir, err := os.MkdirTemp("", "awake-icon")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	iconsetDir := filepath.Join(tmpDir, "Awake.iconset")
	os.MkdirAll(iconsetDir, 0755)

	// Generate all required sizes
	sizes := []struct {
		name string
		px   int
	}{
		{"icon_16x16.png", 16},
		{"icon_16x16@2x.png", 32},
		{"icon_32x32.png", 32},
		{"icon_32x32@2x.png", 64},
		{"icon_128x128.png", 128},
		{"icon_128x128@2x.png", 256},
		{"icon_256x256.png", 256},
		{"icon_256x256@2x.png", 512},
		{"icon_512x512.png", 512},
		{"icon_512x512@2x.png", 1024},
	}

	for _, s := range sizes {
		outPath := filepath.Join(iconsetDir, s.name)
		if out, err := exec.Command("sips", "-z",
			fmt.Sprintf("%d", s.px), fmt.Sprintf("%d", s.px),
			pngPath, "--out", outPath).CombinedOutput(); err != nil {
			return fmt.Errorf("sips resize to %d failed: %s", s.px, string(out))
		}
	}

	if out, err := exec.Command("iconutil", "-c", "icns",
		iconsetDir, "-o", icnsPath).CombinedOutput(); err != nil {
		return fmt.Errorf("iconutil failed: %s", string(out))
	}

	return nil
}

// Notify sends a macOS notification. Prefers the custom Awake.app for
// branded notifications, falls back to terminal-notifier, then osascript.
func Notify(title, message string) {
	// Try custom Awake.app first
	bin := awakeAppBinary()
	if _, err := os.Stat(bin); err == nil {
		exec.Command(bin,
			"-title", title,
			"-message", message,
			"-group", "com.awake",
			"-sound", "default",
		).Run()
		return
	}

	// Fall back to system terminal-notifier
	if tn, err := exec.LookPath("terminal-notifier"); err == nil {
		exec.Command(tn,
			"-title", title,
			"-message", message,
			"-group", "com.awake",
			"-sound", "default",
		).Run()
		return
	}

	// Last resort: osascript
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

	st, err := LoadState()
	if err == nil && st.Active != nil {
		st.ClearActive()
		st.Save()
	}

	os.Remove(watcherPidPath())
}
