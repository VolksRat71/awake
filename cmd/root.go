package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/VolksRat71/awake/daemon"
	"github.com/VolksRat71/awake/engine"
	"github.com/VolksRat71/awake/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "awake [minutes]",
	Short: "Keep your Mac awake",
	Long:  "A macOS utility to prevent sleep using caffeinate.\nRun without arguments to open the TUI.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runTUI()
		}

		minutes, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid duration %q — provide minutes as a number", args[0])
		}

		label, _ := cmd.Flags().GetString("label")
		replace, _ := cmd.Flags().GetBool("replace")
		return startManual(minutes, label, replace)
	},
}

var untilCmd = &cobra.Command{
	Use:   "until <HH:MM>",
	Short: "Stay awake until a specific time",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, err := engine.ParseUntilTime(args[0])
		if err != nil {
			return err
		}

		label, _ := cmd.Flags().GetString("label")
		replace, _ := cmd.Flags().GetBool("replace")
		cfg, st, err := load()
		if err != nil {
			return err
		}

		opts := engine.StartOpts{
			Until: target,
			Mode:  "manual",
			Label: label,
		}

		if replace {
			err = engine.ForceReplace(cfg, st, opts)
		} else {
			err = engine.StartSession(cfg, st, opts)
		}
		if err != nil {
			return err
		}

		remaining := time.Until(target)
		fmt.Printf("Awake until %s (%s)\n", cfg.FormatTime(target), engine.FormatDuration(remaining))
		return nil
	},
}

var workdayCmd = &cobra.Command{
	Use:   "workday",
	Short: "Stay awake until end of workday",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, st, err := load()
		if err != nil {
			return err
		}

		endTime, err := engine.WorkdayEnd(cfg)
		if err != nil {
			return err
		}

		label, _ := cmd.Flags().GetString("label")
		replace, _ := cmd.Flags().GetBool("replace")
		if label == "" {
			label = "Workday"
		}

		opts := engine.StartOpts{
			Until: endTime,
			Mode:  "workday",
			Label: label,
		}

		if replace {
			err = engine.ForceReplace(cfg, st, opts)
		} else {
			err = engine.StartSession(cfg, st, opts)
		}
		if err != nil {
			return err
		}

		remaining := time.Until(endTime)
		fmt.Printf("Awake until %s (%s)\n", cfg.FormatTime(endTime), engine.FormatDuration(remaining))
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current awake session",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, st, err := load()
		if err != nil {
			return err
		}
		if err := engine.StopSession(cfg, st); err != nil {
			return err
		}
		fmt.Println("Session stopped")
		return nil
	},
}

var extendCmd = &cobra.Command{
	Use:   "extend <minutes>",
	Short: "Extend the current session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		minutes, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid duration %q", args[0])
		}

		cfg, st, err := load()
		if err != nil {
			return err
		}

		if err := engine.ExtendSession(cfg, st, minutes); err != nil {
			return err
		}

		status := engine.GetStatus(cfg, st)
		fmt.Printf("Extended by %dm — ends at %s (%s remaining)\n",
			minutes, cfg.FormatTime(status.EndsAt), engine.FormatDuration(status.TimeRemaining))
		return nil
	},
}

var jsonFlag bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current session status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, st, err := load()
		if err != nil {
			return err
		}

		status := engine.GetStatus(cfg, st)

		if jsonFlag {
			data, err := json.MarshalIndent(status, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		if !status.Active {
			fmt.Println("No active session")
			return nil
		}

		fmt.Printf("● ACTIVE\n")
		fmt.Printf("  Mode:      %s\n", status.Mode)
		if status.Label != "" {
			fmt.Printf("  Label:     %s\n", status.Label)
		}
		fmt.Printf("  Started:   %s\n", cfg.FormatTime(status.StartedAt))
		fmt.Printf("  Ends:      %s\n", cfg.FormatTime(status.EndsAt))
		fmt.Printf("  Remaining: %s\n", engine.FormatDuration(status.TimeRemaining))
		fmt.Printf("  PID:       %d\n", status.PID)
		fmt.Printf("  Command:   %s\n", status.Command)
		return nil
	},
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the interactive terminal UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

var betweenCmd = &cobra.Command{
	Use:   "between <start HH:MM> <end HH:MM>",
	Short: "Schedule an awake window between two times",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime, err := engine.ParseUntilTime(args[0])
		if err != nil {
			return fmt.Errorf("start time: %w", err)
		}
		endTime, err := engine.ParseUntilTime(args[1])
		if err != nil {
			return fmt.Errorf("end time: %w", err)
		}

		// If end is before start, it means end is the next day
		if endTime.Before(startTime) {
			endTime = endTime.Add(24 * time.Hour)
		}

		label, _ := cmd.Flags().GetString("label")
		if label == "" {
			label = fmt.Sprintf("%s–%s", args[0], args[1])
		}

		cfg, st, err := load()
		if err != nil {
			return err
		}

		if err := engine.ScheduleWindow(cfg, st, startTime, endTime, label); err != nil {
			return err
		}

		if startTime.Before(time.Now()) || startTime.Equal(time.Now()) {
			fmt.Printf("Awake now until %s\n", cfg.FormatTime(endTime))
		} else {
			fmt.Printf("Scheduled: %s – %s\n", cfg.FormatTime(startTime), cfg.FormatTime(endTime))
		}
		return nil
	},
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the background daemon",
}

var daemonRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "Run the daemon (usually called by launchd)",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		daemon.Run()
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if the daemon is running",
	Run: func(cmd *cobra.Command, args []string) {
		running, pid := daemon.IsRunning()
		if running {
			fmt.Printf("Daemon running (PID %d)\n", pid)
		} else {
			fmt.Println("Daemon not running")
		}
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install awake as a system service (launchd)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := daemon.Install(); err != nil {
			return err
		}

		plistPath := daemon.PlistPath()
		fmt.Println("Installed launchd service")

		// Load immediately
		out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput()
		if err != nil {
			fmt.Printf("Plist written to %s\n", plistPath)
			fmt.Printf("Auto-load failed: %s\nRun manually: launchctl load %s\n", string(out), plistPath)
			return nil
		}

		fmt.Println("Daemon started")

		// Set up branded notifications
		if _, err := exec.LookPath("terminal-notifier"); err != nil {
			fmt.Println("\nInstalling terminal-notifier for notifications...")
			if out, err := exec.Command("brew", "install", "terminal-notifier").CombinedOutput(); err != nil {
				fmt.Printf("Could not install terminal-notifier: %s\n", string(out))
				fmt.Println("Notifications will work but without the awake icon.")
				fmt.Println("Install manually: brew install terminal-notifier")
				return nil
			}
			fmt.Println("terminal-notifier installed")
		}

		fmt.Println("Building Awake.app notification bundle...")
		if err := engine.InstallNotifierApp(); err != nil {
			fmt.Printf("Warning: %v\n", err)
			fmt.Println("Notifications will work but with the default terminal-notifier icon.")
		} else {
			fmt.Println("Awake.app created — notifications will show the awake icon")
		}

		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the awake system service",
	RunE: func(cmd *cobra.Command, args []string) error {
		plistPath := daemon.PlistPath()
		exec.Command("launchctl", "unload", plistPath).Run()

		if err := daemon.Uninstall(); err != nil {
			return err
		}

		fmt.Println("Service removed")
		return nil
	},
}

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Show or cancel the pending scheduled window",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, st, err := load()
		if err != nil {
			return err
		}

		cancel, _ := cmd.Flags().GetBool("cancel")
		if cancel {
			if err := engine.CancelSchedule(st); err != nil {
				return err
			}
			fmt.Println("Scheduled window cancelled")
			return nil
		}

		if st.Scheduled == nil {
			fmt.Println("No scheduled window")
			return nil
		}

		cfg, _, _ := load()
		w := st.Scheduled
		lbl := w.Label
		if lbl != "" {
			lbl = fmt.Sprintf(" [%s]", lbl)
		}
		fmt.Printf("Scheduled: %s – %s%s\n",
			cfg.FormatTime(w.StartsAt), cfg.FormatTime(w.EndsAt), lbl)
		return nil
	},
}

var notifyWatchCmd = &cobra.Command{
	Use:    "_notify-watch",
	Hidden: true,
	Args:   cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		engine.RunNotifyWatcher(args[0], args[1], args[2])
	},
}

func init() {
	rootCmd.PersistentFlags().StringP("label", "l", "", "Session label")
	rootCmd.PersistentFlags().BoolP("replace", "r", false, "Replace active session instead of erroring")
	statusCmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")
	scheduleCmd.Flags().Bool("cancel", false, "Cancel the pending scheduled window")

	daemonCmd.AddCommand(daemonRunCmd)
	daemonCmd.AddCommand(daemonStatusCmd)

	rootCmd.AddCommand(untilCmd)
	rootCmd.AddCommand(workdayCmd)
	rootCmd.AddCommand(betweenCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(extendCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(scheduleCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(notifyWatchCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func load() (*engine.Config, *engine.State, error) {
	cfg, err := engine.LoadConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	st, err := engine.LoadState()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load state: %w", err)
	}
	return cfg, st, nil
}

func startManual(minutes int, label string, replace bool) error {
	cfg, st, err := load()
	if err != nil {
		return err
	}

	opts := engine.StartOpts{
		Minutes: minutes,
		Mode:    "manual",
		Label:   label,
	}

	if replace {
		err = engine.ForceReplace(cfg, st, opts)
	} else {
		err = engine.StartSession(cfg, st, opts)
	}
	if err != nil {
		return err
	}

	endsAt := time.Now().Add(time.Duration(minutes) * time.Minute)
	fmt.Printf("Awake for %dm (until %s)\n", minutes, cfg.FormatTime(endsAt))
	return nil
}

func runTUI() error {
	cfg, st, err := load()
	if err != nil {
		return err
	}
	return tui.Run(cfg, st)
}
