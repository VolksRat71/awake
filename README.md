<p align="center">
  <img src="assets/icon.png" width="400" />
</p>

# awake

A macOS utility that keeps your Mac from sleeping. Uses `caffeinate` under the hood with a fast CLI for muscle memory and a terminal UI for when you want a control panel.

## Quickstart

### Homebrew (recommended)

```bash
brew tap VolksRat71/awake https://github.com/VolksRat71/awake
brew install awake
awake install
```

Homebrew installs the binary. `awake install` sets up the daemon, notifications, and walks you through enabling permissions.

### GitHub Release

```bash
# Download the latest release (Apple Silicon)
gh release download --repo VolksRat71/awake --pattern "*arm64*tar.gz"

# For Intel Macs, use *amd64* instead

# Extract and install
tar xzf awake-*.tar.gz
sudo cp awake /usr/local/bin/
awake install
```

Prebuilt binaries for both Apple Silicon (arm64) and Intel (amd64) are available on the [releases page](https://github.com/VolksRat71/awake/releases).

### From source

Requires Go 1.22+.

```bash
git clone https://github.com/VolksRat71/awake.git
cd awake
go build -o awake .
sudo cp awake /usr/local/bin/
awake install
```

### What `awake install` does

When installing via GitHub release or from source, `awake install` handles the rest:

1. Creates a **launchd service** so the daemon starts on boot
2. Installs **terminal-notifier** via Homebrew (if not present) for rich notifications
3. Builds a custom **Awake.app** bundle with the awake icon for branded notifications

Homebrew runs this automatically via `post_install` — no extra step needed.

## CLI

```bash
# Quick start — stay awake for N minutes
awake 120
awake 60 -l "build running"

# Stay awake until a specific time
awake until 15:30
awake until 23:00 -l "deploy"

# Stay awake until end of workday
awake workday

# Schedule a future window
awake between 15:00 19:00
awake between 22:00 06:00 -l "overnight job"

# Extend the current session
awake extend 30

# Stop
awake stop

# Check status
awake status
awake status --json

# View or cancel a scheduled window
awake schedule
awake schedule --cancel

# Replace an active session instead of erroring
awake 240 -r
awake until 18:00 -r

# Open the TUI
awake
awake tui
```

## TUI

Run `awake` with no arguments to open the interactive terminal UI.

```
╭────────────────────────────────────────────────────────────╮
│ AWAKE                                                      │
│                                                            │
│   ● ACTIVE    1h 42m remaining                             │
│                                                            │
│   Mode      manual                                         │
│   Label     watching logs                                  │
│   Started   10:00 AM                                       │
│   Ends      12:00 PM                                       │
│   PID       12345                                          │
│   Flags     -dimsu                                         │
│                                                            │
│   Schedule  Mon–Fri 9:00 AM–5:00 PM                        │
│                                                            │
│   p presets  c custom   e extend   s schedule              │
│   h history  o options  x stop     q quit                  │
│                                                            │
│   /usr/bin/caffeinate -dimsu -t 7200                       │
╰────────────────────────────────────────────────────────────╯
```

**Hotkeys**

| Key | Action |
|-----|--------|
| `p` | Quick-start from presets (30m, 1h, 4h, until lunch, until EOD) |
| `c` | Custom duration or "until" time |
| `e` | Extend current session (+15m, +30m, +1h, +2h) |
| `s` | Schedule a future awake window |
| `h` | Session history |
| `o` | Options (time format, notifications, workday, flags) |
| `x` | Stop session (requires confirmation) |
| `q` | Quit TUI |

The border color reflects session state: **green** when active, **yellow** when ending soon, **neutral** when idle.

## Config

Stored at `~/.config/awake/config.json`. Editable directly or through the TUI options view (`o`).

```json
{
  "workday": {
    "start": "09:00",
    "end": "17:00",
    "days": [1, 2, 3, 4, 5]
  },
  "flags": "-dimsu",
  "time_format": "12h",
  "presets": [
    { "name": "30 minutes", "minutes": 30 },
    { "name": "1 hour", "minutes": 60 },
    { "name": "4 hours", "minutes": 240 },
    { "name": "Until lunch", "until": "12:00" },
    { "name": "Until end of workday", "until": "17:00" }
  ],
  "notifications": {
    "enabled": true,
    "warn_minutes": 10
  },
  "max_duration_hours": 24
}
```

| Field | Description |
|-------|-------------|
| `workday.start` / `workday.end` | Work hours in 24h format |
| `workday.days` | 1=Mon through 7=Sun |
| `flags` | Flags passed to `caffeinate` (`-d` display, `-i` idle, `-m` disk, `-s` system, `-u` user) |
| `time_format` | `"12h"` (default) or `"24h"` |
| `presets` | Named quick-start options with either `minutes` or `until` (HH:MM) |
| `notifications.enabled` | macOS notifications on start, warning, and end |
| `notifications.warn_minutes` | Minutes before session end to send warning |
| `max_duration_hours` | Safety cap on session length |

## State

Runtime state lives at `~/.config/awake/state.json`. Tracks the active session, scheduled windows, and session history (last 50). The CLI and TUI share this file so they always stay in sync.

## Daemon

`awake install` creates a launchd plist at `~/Library/LaunchAgents/com.awake.daemon.plist` and loads it. The daemon:

- Activates scheduled windows when their start time arrives
- Cleans up expired sessions
- Logs to `~/.config/awake/daemon.log`

```bash
awake daemon status    # Check if running
awake uninstall        # Remove the service
```

## Notifications

When enabled, awake sends macOS notifications with the app icon:

- **Session started** — with duration and label
- **Warning** — N minutes before session ends
- **Session ended** — when the timer expires or you stop it

A background watcher process handles the timing so notifications work even when the TUI isn't open. Uses standard macOS notification sounds and respects Focus/Do Not Disturb modes.

## How it works

`awake` is a control plane around macOS's built-in `/usr/bin/caffeinate`. Every session spawns a `caffeinate` process with a timeout flag (`-t`). Extending a session kills the old process and starts a new one with the adjusted duration. The daemon and CLI both read/write the same state file, so there's always one source of truth.

## Uninstall

```bash
awake uninstall                    # Remove the launchd service
sudo rm /usr/local/bin/awake       # Remove the binary
rm -rf ~/.config/awake             # Remove config, state, and Awake.app
```

## License

MIT
