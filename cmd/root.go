package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

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
		fmt.Printf("Awake until %s (%s)\n", target.Format("15:04"), engine.FormatDuration(remaining))
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
		fmt.Printf("Awake until %s (%s)\n", endTime.Format("15:04"), engine.FormatDuration(remaining))
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
			minutes, status.EndsAt.Format("15:04"), engine.FormatDuration(status.TimeRemaining))
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
		fmt.Printf("  Started:   %s\n", status.StartedAt.Format("3:04 PM"))
		fmt.Printf("  Ends:      %s\n", status.EndsAt.Format("3:04 PM"))
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

	rootCmd.AddCommand(untilCmd)
	rootCmd.AddCommand(workdayCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(extendCmd)
	rootCmd.AddCommand(statusCmd)
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
	fmt.Printf("Awake for %dm (until %s)\n", minutes, endsAt.Format("15:04"))
	return nil
}

func runTUI() error {
	cfg, st, err := load()
	if err != nil {
		return err
	}
	return tui.Run(cfg, st)
}
