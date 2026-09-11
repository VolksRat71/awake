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
	os.RemoveAll(dest)

	if out, err := exec.Command("cp", "-R", tnPath, dest).CombinedOutput(); err != nil {
		return fmt.Errorf("copy failed: %s", string(out))
	}

	icnsPath := filepath.Join(dest, "Contents", "Resources", "Awake.icns")
	if err := pngToIcns(iconPath(), icnsPath); err != nil {
		return fmt.Errorf("icon conversion failed: %w", err)
	}

	oldIcon := filepath.Join(dest, "Contents", "Resources", "Terminal.icns")
	os.Remove(oldIcon)

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

	exec.Command("touch", dest).Run()

	// Re-sign the app so macOS accepts notifications from it.
	// Without this, the modified bundle has a broken signature and
	// macOS Sequoia/Tahoe silently drops all notifications.
	if out, err := exec.Command("codesign", "--force", "--deep", "--sign", "-", dest).CombinedOutput(); err != nil {
		return fmt.Errorf("codesign failed: %s", string(out))
	}

	return nil
}

// escapeNotifierArg guards a value passed to terminal-notifier.
//
// terminal-notifier reads its flags through the NSUserDefaults argument
// domain, which parses values as property lists. A value whose first
// character is [ ( { " or \ is read as the start of a collection or quoted
// literal, fails to parse, and comes back nil — the notification still posts,
// but that field is silently empty. Escaping the first character with a
// backslash makes it parse as a plain string; the backslash is consumed and
// does not appear in the banner.
func escapeNotifierArg(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '[', '(', '{', '"', '\\':
		return "\\" + s
	}
	return s
}

// notifierArgs builds the terminal-notifier argument list for one banner.
// The label rides in -subtitle rather than being concatenated into the
// message, so a label can never push a bracket to the front of the body.
func notifierArgs(title, label, message string) []string {
	args := []string{
		"-title", escapeNotifierArg(title),
		"-message", escapeNotifierArg(message),
	}
	if label != "" {
		args = append(args, "-subtitle", escapeNotifierArg(label))
	}
	return append(args, "-group", "com.awake", "-sound", "default")
}

// osascriptFor builds the AppleScript fallback for one banner.
func osascriptFor(title, label, message string) string {
	if label != "" {
		return fmt.Sprintf(
			`display notification %q with title %q subtitle %q sound name "default"`,
			message, title, label)
	}
	return fmt.Sprintf(
		`display notification %q with title %q sound name "default"`,
		message, title)
}

// NotifySync sends a notification and waits for delivery. Used during install
// so the Awake.app gets registered with macOS notification center on first use.
func NotifySync(title, message string) {
	NotifySyncWithLabel(title, "", message)
}

// NotifySyncWithLabel sends a labelled notification and waits for delivery.
func NotifySyncWithLabel(title, label, message string) {
	appPath := awakeAppPath()
	if _, err := os.Stat(appPath); err == nil {
		exec.Command(awakeAppBinary(), notifierArgs(title, label, message)...).Run()
		return
	}

	exec.Command("osascript", "-e", osascriptFor(title, label, message)).Run()
}

func findTerminalNotifierApp() (string, error) {
	matches, _ := filepath.Glob("/opt/homebrew/Cellar/terminal-notifier/*/terminal-notifier.app")
	if len(matches) > 0 {
		return matches[len(matches)-1], nil
	}
	matches, _ = filepath.Glob("/usr/local/Cellar/terminal-notifier/*/terminal-notifier.app")
	if len(matches) > 0 {
		return matches[len(matches)-1], nil
	}
	return "", fmt.Errorf("not found in homebrew cellar")
}

func pngToIcns(pngPath, icnsPath string) error {
	tmpDir, err := os.MkdirTemp("", "awake-icon")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	iconsetDir := filepath.Join(tmpDir, "Awake.iconset")
	os.MkdirAll(iconsetDir, 0755)

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

// Notify sends a macOS notification. Uses the custom Awake.app for branded
// notifications when available, falls back to osascript.
// Runs in a goroutine so it never blocks the caller.
func Notify(title, message string) {
	NotifyWithLabel(title, "", message)
}

// NotifyWithLabel sends a macOS notification with the session label as the
// banner subtitle. Uses the custom Awake.app for branded notifications when
// available, falls back to osascript.
// Runs in a goroutine so it never blocks the caller.
func NotifyWithLabel(title, label, message string) {
	go func() {
		appPath := awakeAppPath()
		if _, err := os.Stat(appPath); err == nil {
			exec.Command(awakeAppBinary(), notifierArgs(title, label, message)...).Run()
			return
		}

		// Fall back to osascript
		exec.Command("osascript", "-e", osascriptFor(title, label, message)).Run()
	}()
}

func watcherPidPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir, "watcher.pid")
}

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
		NotifyWithLabel("Awake", label, msg)
	}

	if endsAt.After(time.Now()) {
		time.Sleep(time.Until(endsAt))
	}

	NotifyWithLabel("Awake", label, "Session ended")

	st, err := LoadState()
	if err == nil && st.Active != nil {
		st.ClearActive()
		st.Save()
	}

	os.Remove(watcherPidPath())
}
