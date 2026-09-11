package engine

import "testing"

// terminal-notifier parses its flags through the NSUserDefaults argument
// domain, where a value whose first character is [ ( { " or \ is read as a
// property-list literal, fails to parse, and silently yields an empty body.
// escapeNotifierArg must neutralize that leading character.
func TestEscapeNotifierArg(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"leading bracket", "[Workday] Session started", `\[Workday] Session started`},
		{"leading paren", "(build) done", `\(build) done`},
		{"leading brace", "{x} done", `\{x} done`},
		{"leading quote", `"quoted" thing`, `\"quoted" thing`},
		{"leading backslash", `\path`, `\\path`},
		{"leading letter", "Session started", "Session started"},
		{"leading digit", "10 minutes remaining", "10 minutes remaining"},
		{"bracket not first", "Session [Workday] started", "Session [Workday] started"},
		{"empty", "", ""},
		{"only a bracket", "[", `\[`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeNotifierArg(tt.in); got != tt.want {
				t.Errorf("escapeNotifierArg(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// The label must ride in -subtitle, never be concatenated into the message,
// so a label like "Workday" can never push a bracket to the front of the body.
func TestNotifierArgsKeepsLabelOutOfMessage(t *testing.T) {
	args := notifierArgs("Awake", "Workday", "Session started - 8h 0m")

	flag := func(name string) string {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == name {
				return args[i+1]
			}
		}
		return ""
	}

	if got := flag("-message"); got != "Session started - 8h 0m" {
		t.Errorf("-message = %q, want the bare message with no label", got)
	}
	if got := flag("-subtitle"); got != "Workday" {
		t.Errorf("-subtitle = %q, want %q", got, "Workday")
	}
	if got := flag("-title"); got != "Awake" {
		t.Errorf("-title = %q, want %q", got, "Awake")
	}
}

// A user-supplied label such as `awake 60 -l "[build]"` must be escaped too.
func TestNotifierArgsEscapesBracketLabel(t *testing.T) {
	args := notifierArgs("Awake", "[build]", "[Workday] queued")

	for i := 0; i < len(args)-1; i++ {
		switch args[i] {
		case "-subtitle":
			if args[i+1] != `\[build]` {
				t.Errorf("-subtitle = %q, want %q", args[i+1], `\[build]`)
			}
		case "-message":
			if args[i+1] != `\[Workday] queued` {
				t.Errorf("-message = %q, want %q", args[i+1], `\[Workday] queued`)
			}
		}
	}
}

// No label means no -subtitle flag at all.
func TestNotifierArgsOmitsEmptySubtitle(t *testing.T) {
	args := notifierArgs("Awake", "", "Daemon started")
	for _, a := range args {
		if a == "-subtitle" {
			t.Fatal("-subtitle flag present for an empty label; it should be omitted")
		}
	}
}
