package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/VolksRat71/awake/engine"
)

type viewMode int

const (
	viewDashboard viewMode = iota
	viewPresets
	viewCustomDuration
	viewCustomUntil
	viewExtend
	viewHistory
	viewLabel
	viewSchedule
	viewOptions
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	cfg    *engine.Config
	state  *engine.State
	status engine.StatusInfo

	view   viewMode
	cursor int

	durationInput textinput.Model
	timeInput     textinput.Model
	labelInput    textinput.Model

	pendingOpts    *engine.StartOpts
	extendCursor   int
	historyOffset  int
	schedStartInput textinput.Model
	schedEndInput   textinput.Model
	schedFocusEnd   bool

	width  int
	height int

	confirmStop bool

	optionsCursor  int
	optionsEditing bool
	optionsInput   textinput.Model

	errMsg     string
	successMsg string
}

func newModel(cfg *engine.Config, state *engine.State) model {
	di := textinput.New()
	di.Placeholder = "minutes"
	di.CharLimit = 6
	di.Width = 20

	ti := textinput.New()
	ti.Placeholder = "HH:MM"
	ti.CharLimit = 5
	ti.Width = 20

	li := textinput.New()
	li.Placeholder = "optional label"
	li.CharLimit = 30
	li.Width = 30

	si := textinput.New()
	si.Placeholder = "HH:MM"
	si.CharLimit = 5
	si.Width = 10

	ei := textinput.New()
	ei.Placeholder = "HH:MM"
	ei.CharLimit = 5
	ei.Width = 10

	oi := textinput.New()
	oi.CharLimit = 20
	oi.Width = 20

	return model{
		cfg:           cfg,
		state:         state,
		status:        engine.GetStatus(cfg, state),
		view:            viewDashboard,
		durationInput:   di,
		timeInput:       ti,
		labelInput:      li,
		schedStartInput: si,
		schedEndInput:   ei,
		optionsInput:    oi,
	}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.status = engine.GetStatus(m.cfg, m.state)
		return m, tickCmd()

	case tea.KeyMsg:
		m.errMsg = ""
		m.successMsg = ""

		switch m.view {
		case viewDashboard:
			return m.updateDashboard(msg)
		case viewPresets:
			return m.updatePresets(msg)
		case viewCustomDuration:
			return m.updateCustomDuration(msg)
		case viewCustomUntil:
			return m.updateCustomUntil(msg)
		case viewExtend:
			return m.updateExtend(msg)
		case viewHistory:
			return m.updateHistory(msg)
		case viewLabel:
			return m.updateLabel(msg)
		case viewSchedule:
			return m.updateSchedule(msg)
		case viewOptions:
			return m.updateOptions(msg)
		}
	}

	return m, nil
}

// --- Dashboard ---

func (m model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// If waiting for stop confirmation, only x confirms — anything else cancels
	if m.confirmStop {
		m.confirmStop = false
		if key == "x" {
			if err := engine.StopSession(m.cfg, m.state); err != nil {
				m.errMsg = err.Error()
			} else {
				m.successMsg = "Session stopped"
			}
			m.status = engine.GetStatus(m.cfg, m.state)
		}
		return m, nil
	}

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "p":
		m.view = viewPresets
		m.cursor = 0
		return m, nil
	case "c":
		m.view = viewCustomDuration
		m.durationInput.Reset()
		m.durationInput.Focus()
		return m, textinput.Blink
	case "e":
		if m.status.Active {
			m.view = viewExtend
			m.extendCursor = 0
			return m, nil
		}
		m.errMsg = "No active session to extend"
		return m, nil
	case "x":
		if m.status.Active {
			m.confirmStop = true
			m.errMsg = "Press x again to stop session"
			return m, nil
		}
		m.errMsg = "No active session"
		return m, nil
	case "h":
		m.view = viewHistory
		m.historyOffset = 0
		return m, nil
	case "s":
		m.view = viewSchedule
		m.schedStartInput.Reset()
		m.schedEndInput.Reset()
		m.schedFocusEnd = false
		m.schedStartInput.Focus()
		return m, textinput.Blink
	case "o":
		m.view = viewOptions
		m.optionsCursor = 0
		m.optionsEditing = false
		return m, nil
	}
	return m, nil
}

// --- Presets ---

func (m model) updatePresets(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewDashboard
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.cfg.Presets)-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		preset := m.cfg.Presets[m.cursor]
		opts := engine.StartOpts{
			Mode:  "preset",
			Label: preset.Name,
		}
		if preset.Minutes > 0 {
			opts.Minutes = preset.Minutes
		} else if preset.Until != "" {
			t, err := engine.ParseUntilTime(preset.Until)
			if err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			opts.Until = t
		}

		m.pendingOpts = &opts
		m.labelInput.Reset()
		m.labelInput.SetValue(preset.Name)
		m.labelInput.Focus()
		m.view = viewLabel
		return m, textinput.Blink
	}
	return m, nil
}

// --- Custom Duration ---

func (m model) updateCustomDuration(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewDashboard
		return m, nil
	case "tab":
		m.view = viewCustomUntil
		m.timeInput.Reset()
		m.timeInput.Focus()
		return m, textinput.Blink
	case "enter":
		val := m.durationInput.Value()
		var minutes int
		if _, err := fmt.Sscanf(val, "%d", &minutes); err != nil || minutes <= 0 {
			m.errMsg = "Enter a positive number of minutes"
			return m, nil
		}

		m.pendingOpts = &engine.StartOpts{
			Minutes: minutes,
			Mode:    "manual",
		}
		m.labelInput.Reset()
		m.labelInput.Focus()
		m.view = viewLabel
		return m, textinput.Blink
	}

	var cmd tea.Cmd
	m.durationInput, cmd = m.durationInput.Update(msg)
	return m, cmd
}

// --- Custom Until ---

func (m model) updateCustomUntil(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewDashboard
		return m, nil
	case "tab":
		m.view = viewCustomDuration
		m.durationInput.Reset()
		m.durationInput.Focus()
		return m, textinput.Blink
	case "enter":
		val := m.timeInput.Value()
		t, err := engine.ParseUntilTime(val)
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}

		m.pendingOpts = &engine.StartOpts{
			Until: t,
			Mode:  "manual",
		}
		m.labelInput.Reset()
		m.labelInput.Focus()
		m.view = viewLabel
		return m, textinput.Blink
	}

	var cmd tea.Cmd
	m.timeInput, cmd = m.timeInput.Update(msg)
	return m, cmd
}

// --- Extend ---

var extendOptions = []struct {
	label   string
	minutes int
}{
	{"+15 minutes", 15},
	{"+30 minutes", 30},
	{"+1 hour", 60},
	{"+2 hours", 120},
}

func (m model) updateExtend(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewDashboard
		return m, nil
	case "up", "k":
		if m.extendCursor > 0 {
			m.extendCursor--
		}
		return m, nil
	case "down", "j":
		if m.extendCursor < len(extendOptions)-1 {
			m.extendCursor++
		}
		return m, nil
	case "enter":
		minutes := extendOptions[m.extendCursor].minutes
		if err := engine.ExtendSession(m.cfg, m.state, minutes); err != nil {
			m.errMsg = err.Error()
		} else {
			m.successMsg = fmt.Sprintf("Extended by %dm", minutes)
		}
		m.status = engine.GetStatus(m.cfg, m.state)
		m.view = viewDashboard
		return m, nil
	}
	return m, nil
}

// --- History ---

func (m model) updateHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewDashboard
		return m, nil
	case "up", "k":
		if m.historyOffset > 0 {
			m.historyOffset--
		}
		return m, nil
	case "down", "j":
		maxOffset := len(m.state.History) - 10
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.historyOffset < maxOffset {
			m.historyOffset++
		}
		return m, nil
	}
	return m, nil
}

// --- Options ---

type optionDef struct {
	key    string // config field name
	label  string // display label
	toggle bool   // true = toggle on enter, false = text edit
}

var optionDefs = []optionDef{
	{"time_format", "Time format", true},
	{"notifications", "Notifications", true},
	{"warn_minutes", "Warn before end", false},
	{"workday_start", "Workday start", false},
	{"workday_end", "Workday end", false},
	{"flags", "Caffeinate flags", false},
	{"max_duration", "Max duration (hours)", false},
}

func (m model) formatConfigTime(raw string) string {
	t, err := time.Parse("15:04", raw)
	if err != nil {
		return raw
	}
	return m.cfg.FormatTime(t)
}

func (m model) optionValue(idx int) string {
	switch optionDefs[idx].key {
	case "time_format":
		if m.cfg.TimeFormat == "24h" {
			return "24h"
		}
		return "12h"
	case "notifications":
		if m.cfg.Notifications.Enabled {
			return "enabled"
		}
		return "disabled"
	case "warn_minutes":
		return fmt.Sprintf("%d min", m.cfg.Notifications.WarnMinutes)
	case "workday_start":
		return m.formatConfigTime(m.cfg.Workday.Start)
	case "workday_end":
		return m.formatConfigTime(m.cfg.Workday.End)
	case "flags":
		return m.cfg.Flags
	case "max_duration":
		return fmt.Sprintf("%d", m.cfg.MaxDurationH)
	}
	return ""
}

func (m model) updateOptions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.optionsEditing {
		switch msg.String() {
		case "esc":
			m.optionsEditing = false
			return m, nil
		case "enter":
			val := m.optionsInput.Value()
			m.applyOption(m.optionsCursor, val)
			m.optionsEditing = false
			return m, nil
		}
		var cmd tea.Cmd
		m.optionsInput, cmd = m.optionsInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "esc":
		m.view = viewDashboard
		return m, nil
	case "up", "k":
		if m.optionsCursor > 0 {
			m.optionsCursor--
		}
		return m, nil
	case "down", "j":
		if m.optionsCursor < len(optionDefs)-1 {
			m.optionsCursor++
		}
		return m, nil
	case "enter":
		opt := optionDefs[m.optionsCursor]
		if opt.toggle {
			m.toggleOption(m.optionsCursor)
			return m, nil
		}
		// Start editing
		m.optionsEditing = true
		m.optionsInput.Reset()
		m.optionsInput.SetValue(m.optionRawValue(m.optionsCursor))
		m.optionsInput.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m model) toggleOption(idx int) {
	switch optionDefs[idx].key {
	case "time_format":
		if m.cfg.TimeFormat == "24h" {
			m.cfg.TimeFormat = "12h"
		} else {
			m.cfg.TimeFormat = "24h"
		}
	case "notifications":
		m.cfg.Notifications.Enabled = !m.cfg.Notifications.Enabled
	}
	m.cfg.Save()
}

func (m model) optionRawValue(idx int) string {
	switch optionDefs[idx].key {
	case "warn_minutes":
		return fmt.Sprintf("%d", m.cfg.Notifications.WarnMinutes)
	case "workday_start":
		return m.cfg.Workday.Start
	case "workday_end":
		return m.cfg.Workday.End
	case "flags":
		return m.cfg.Flags
	case "max_duration":
		return fmt.Sprintf("%d", m.cfg.MaxDurationH)
	}
	return ""
}

func (m model) applyOption(idx int, val string) {
	switch optionDefs[idx].key {
	case "warn_minutes":
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil && n > 0 {
			m.cfg.Notifications.WarnMinutes = n
		} else {
			m.errMsg = "Enter a positive number"
			return
		}
	case "workday_start":
		if _, err := time.Parse("15:04", val); err != nil {
			m.errMsg = "Use HH:MM format"
			return
		}
		m.cfg.Workday.Start = val
	case "workday_end":
		if _, err := time.Parse("15:04", val); err != nil {
			m.errMsg = "Use HH:MM format"
			return
		}
		m.cfg.Workday.End = val
	case "flags":
		m.cfg.Flags = val
	case "max_duration":
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil && n > 0 {
			m.cfg.MaxDurationH = n
		} else {
			m.errMsg = "Enter a positive number"
			return
		}
	}
	m.cfg.Save()
	m.successMsg = "Saved"
}

// --- Schedule ---

func (m model) updateSchedule(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewDashboard
		return m, nil
	case "tab":
		m.schedFocusEnd = !m.schedFocusEnd
		if m.schedFocusEnd {
			m.schedStartInput.Blur()
			m.schedEndInput.Focus()
		} else {
			m.schedEndInput.Blur()
			m.schedStartInput.Focus()
		}
		return m, textinput.Blink
	case "enter":
		startVal := m.schedStartInput.Value()
		endVal := m.schedEndInput.Value()

		startTime, err := engine.ParseUntilTime(startVal)
		if err != nil {
			m.errMsg = fmt.Sprintf("start: %s", err.Error())
			return m, nil
		}
		endTime, err := engine.ParseUntilTime(endVal)
		if err != nil {
			m.errMsg = fmt.Sprintf("end: %s", err.Error())
			return m, nil
		}
		if endTime.Before(startTime) {
			endTime = endTime.Add(24 * time.Hour)
		}

		label := fmt.Sprintf("%s–%s", startVal, endVal)
		if err := engine.ScheduleWindow(m.cfg, m.state, startTime, endTime, label); err != nil {
			m.errMsg = err.Error()
		} else {
			m.successMsg = fmt.Sprintf("Scheduled %s – %s", startVal, endVal)
		}
		m.status = engine.GetStatus(m.cfg, m.state)
		m.view = viewDashboard
		return m, nil
	case "d":
		// Cancel pending schedule
		if m.state.Scheduled != nil {
			if err := engine.CancelSchedule(m.state); err != nil {
				m.errMsg = err.Error()
			} else {
				m.successMsg = "Scheduled window cancelled"
			}
			m.view = viewDashboard
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.schedFocusEnd {
		m.schedEndInput, cmd = m.schedEndInput.Update(msg)
	} else {
		m.schedStartInput, cmd = m.schedStartInput.Update(msg)
	}
	return m, cmd
}

// --- Label ---

func (m model) updateLabel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewDashboard
		m.pendingOpts = nil
		return m, nil
	case "enter":
		if m.pendingOpts != nil {
			m.pendingOpts.Label = m.labelInput.Value()

			var err error
			if m.status.Active {
				err = engine.ForceReplace(m.cfg, m.state, *m.pendingOpts)
			} else {
				err = engine.StartSession(m.cfg, m.state, *m.pendingOpts)
			}
			if err != nil {
				m.errMsg = err.Error()
			} else {
				m.successMsg = "Session started"
			}
			m.status = engine.GetStatus(m.cfg, m.state)
			m.pendingOpts = nil
			m.view = viewDashboard
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.labelInput, cmd = m.labelInput.Update(msg)
	return m, cmd
}

// ============================================================
// Views
// ============================================================

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.view {
	case viewDashboard:
		return m.viewDashboard()
	case viewPresets:
		return m.viewPresets()
	case viewCustomDuration:
		return m.viewCustom(true)
	case viewCustomUntil:
		return m.viewCustom(false)
	case viewExtend:
		return m.viewExtend()
	case viewHistory:
		return m.viewHistory()
	case viewLabel:
		return m.viewLabel()
	case viewSchedule:
		return m.viewSchedule()
	case viewOptions:
		return m.viewOptions()
	}
	return ""
}

func (m model) boxWidth() int {
	w := m.width - 4
	if w > 60 {
		w = 60
	}
	return w
}

func (m model) viewDashboard() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("AWAKE") + "\n\n")

	if m.status.Active {
		remaining := m.status.TimeRemaining
		warnThreshold := time.Duration(m.cfg.Notifications.WarnMinutes) * time.Minute
		isWarning := remaining > 0 && remaining < warnThreshold

		dot := statusActiveStyle.Render("●")
		label := statusActiveStyle.Render("ACTIVE")
		if isWarning {
			dot = statusWarningStyle.Render("●")
			label = statusWarningStyle.Render("ENDING SOON")
		}

		countdown := countdownStyle.Render(engine.FormatDuration(remaining) + " remaining")
		b.WriteString(fmt.Sprintf("  %s %s    %s\n\n", dot, label, countdown))

		rows := []struct{ k, v string }{
			{"Mode    ", m.status.Mode},
			{"Label   ", m.status.Label},
			{"Started ", m.cfg.FormatTime(m.status.StartedAt)},
			{"Ends    ", m.cfg.FormatTime(m.status.EndsAt)},
			{"PID     ", fmt.Sprintf("%d", m.status.PID)},
			{"Flags   ", m.status.Flags},
		}
		for _, r := range rows {
			if r.v == "" {
				continue
			}
			b.WriteString(fmt.Sprintf("  %s  %s\n",
				labelStyle.Render(r.k), valueStyle.Render(r.v)))
		}
	} else {
		dot := statusIdleStyle.Render("○")
		text := statusIdleStyle.Render("IDLE — normal sleep policy")
		b.WriteString(fmt.Sprintf("  %s %s\n", dot, text))
	}

	b.WriteString("\n")
	days := formatDays(m.cfg.Workday.Days)
	sched := fmt.Sprintf("%s %s–%s", days, m.formatConfigTime(m.cfg.Workday.Start), m.formatConfigTime(m.cfg.Workday.End))
	b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Schedule"), valueStyle.Render(sched)))

	if m.state.Scheduled != nil {
		w := m.state.Scheduled
		nextStr := fmt.Sprintf("%s – %s", m.cfg.FormatTime(w.StartsAt), m.cfg.FormatTime(w.EndsAt))
		if w.Label != "" {
			nextStr = fmt.Sprintf("%s [%s]", nextStr, w.Label)
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Next    "), valueStyle.Render(nextStr)))
	}

	m.writeMessages(&b)

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s presets  %s custom   %s extend   %s schedule\n",
		hotkeyStyle.Render("p"), hotkeyStyle.Render("c"), hotkeyStyle.Render("e"), hotkeyStyle.Render("s")))
	b.WriteString(fmt.Sprintf("  %s history  %s options  %s stop     %s quit\n",
		hotkeyStyle.Render("h"), hotkeyStyle.Render("o"), hotkeyStyle.Render("x"), hotkeyStyle.Render("q")))

	b.WriteString("\n")
	if m.status.Active {
		b.WriteString(fmt.Sprintf("  %s\n", footerStyle.Render(m.status.Command)))
	} else {
		b.WriteString(fmt.Sprintf("  %s\n", footerStyle.Render("no active caffeinate process")))
	}

	return m.applyBorder(b.String())
}

func (m model) viewPresets() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("START PRESET") + "\n\n")

	for i, p := range m.cfg.Presets {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = "▸ "
			style = selectedStyle
		}

		desc := p.Name
		if p.Minutes > 0 {
			desc = fmt.Sprintf("%s (%dm)", p.Name, p.Minutes)
		} else if p.Until != "" {
			desc = fmt.Sprintf("%s (%s)", p.Name, p.Until)
		}
		b.WriteString(fmt.Sprintf("  %s%s\n", cursor, style.Render(desc)))
	}

	m.writeMessages(&b)

	b.WriteString(fmt.Sprintf("\n  %s select   %s start   %s back\n",
		hotkeyStyle.Render("↑↓"), hotkeyStyle.Render("enter"), hotkeyStyle.Render("esc")))

	return borderStyle.Width(m.boxWidth()).Render(b.String())
}

func (m model) viewCustom(isDuration bool) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("CUSTOM SESSION") + "\n\n")

	durMark := selectedStyle.Render("▸")
	untilMark := labelStyle.Render(" ")
	if !isDuration {
		durMark = labelStyle.Render(" ")
		untilMark = selectedStyle.Render("▸")
	}

	b.WriteString(fmt.Sprintf("  %s For duration    %s Until time\n\n", durMark, untilMark))

	if isDuration {
		b.WriteString(fmt.Sprintf("  Duration: %s minutes\n", m.durationInput.View()))
	} else {
		b.WriteString(fmt.Sprintf("  Until:    %s\n", m.timeInput.View()))
	}

	m.writeMessages(&b)

	b.WriteString(fmt.Sprintf("\n  %s switch   %s start   %s back\n",
		hotkeyStyle.Render("tab"), hotkeyStyle.Render("enter"), hotkeyStyle.Render("esc")))

	return borderStyle.Width(m.boxWidth()).Render(b.String())
}

func (m model) viewExtend() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("EXTEND SESSION") + "\n\n")

	for i, opt := range extendOptions {
		cursor := "  "
		style := normalStyle
		if i == m.extendCursor {
			cursor = "▸ "
			style = selectedStyle
		}
		b.WriteString(fmt.Sprintf("  %s%s\n", cursor, style.Render(opt.label)))
	}

	m.writeMessages(&b)

	b.WriteString(fmt.Sprintf("\n  %s select   %s extend   %s back\n",
		hotkeyStyle.Render("↑↓"), hotkeyStyle.Render("enter"), hotkeyStyle.Render("esc")))

	return borderStyle.Width(m.boxWidth()).Render(b.String())
}

func (m model) viewHistory() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("HISTORY") + "\n\n")

	if len(m.state.History) == 0 {
		b.WriteString("  No recent sessions\n")
	} else {
		end := m.historyOffset + 10
		if end > len(m.state.History) {
			end = len(m.state.History)
		}
		for i := m.historyOffset; i < end; i++ {
			h := m.state.History[i]
			lbl := h.Label
			if lbl == "" {
				lbl = h.Mode
			}
			b.WriteString(fmt.Sprintf("  %s  %-16s %4dm  %s\n",
				labelStyle.Render(h.StartedAt.Format("Jan 02")+" "+m.cfg.FormatTime(h.StartedAt)),
				valueStyle.Render(lbl),
				h.Duration,
				labelStyle.Render("→ "+m.cfg.FormatTime(h.EndedAt)),
			))
		}

		if len(m.state.History) > 10 {
			b.WriteString(fmt.Sprintf("\n  %s\n",
				labelStyle.Render(fmt.Sprintf("  showing %d–%d of %d",
					m.historyOffset+1, end, len(m.state.History)))))
		}
	}

	b.WriteString(fmt.Sprintf("\n  %s scroll   %s back\n",
		hotkeyStyle.Render("↑↓"), hotkeyStyle.Render("esc")))

	return borderStyle.Width(m.boxWidth()).Render(b.String())
}

func (m model) viewLabel() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("SESSION LABEL") + "\n\n")

	b.WriteString(fmt.Sprintf("  Label: %s\n", m.labelInput.View()))
	b.WriteString(fmt.Sprintf("\n  %s\n", labelStyle.Render("Press enter to start, or leave empty")))

	b.WriteString(fmt.Sprintf("\n  %s start   %s cancel\n",
		hotkeyStyle.Render("enter"), hotkeyStyle.Render("esc")))

	return borderStyle.Width(m.boxWidth()).Render(b.String())
}

func (m model) viewOptions() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("OPTIONS") + "\n\n")

	for i, opt := range optionDefs {
		cursor := "  "
		style := normalStyle
		if i == m.optionsCursor {
			cursor = "▸ "
			style = selectedStyle
		}

		val := m.optionValue(i)

		if m.optionsEditing && i == m.optionsCursor {
			b.WriteString(fmt.Sprintf("  %s%-20s %s\n", cursor,
				style.Render(opt.label), m.optionsInput.View()))
		} else {
			hint := ""
			if opt.toggle {
				hint = labelStyle.Render("  ⏎ toggle")
			}
			b.WriteString(fmt.Sprintf("  %s%-20s %s%s\n", cursor,
				style.Render(opt.label), valueStyle.Render(val), hint))
		}
	}

	m.writeMessages(&b)

	b.WriteString("\n")
	if m.optionsEditing {
		b.WriteString(fmt.Sprintf("  %s save   %s cancel\n",
			hotkeyStyle.Render("enter"), hotkeyStyle.Render("esc")))
	} else {
		b.WriteString(fmt.Sprintf("  %s select   %s edit/toggle   %s back\n",
			hotkeyStyle.Render("↑↓"), hotkeyStyle.Render("enter"), hotkeyStyle.Render("esc")))
	}

	b.WriteString(fmt.Sprintf("\n  %s\n", footerStyle.Render("changes save to ~/.config/awake/config.json")))

	return borderStyle.Width(m.boxWidth()).Render(b.String())
}

func (m model) viewSchedule() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("SCHEDULE WINDOW") + "\n\n")

	if m.state.Scheduled != nil {
		w := m.state.Scheduled
		b.WriteString(fmt.Sprintf("  %s  %s – %s",
			labelStyle.Render("Pending"),
			valueStyle.Render(m.cfg.FormatTime(w.StartsAt)),
			valueStyle.Render(m.cfg.FormatTime(w.EndsAt))))
		if w.Label != "" {
			b.WriteString(fmt.Sprintf("  %s", labelStyle.Render("["+w.Label+"]")))
		}
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Press %s to cancel this window\n", hotkeyStyle.Render("d")))
		b.WriteString("\n")
	}

	startMark := selectedStyle.Render("▸")
	endMark := labelStyle.Render(" ")
	if m.schedFocusEnd {
		startMark = labelStyle.Render(" ")
		endMark = selectedStyle.Render("▸")
	}

	b.WriteString(fmt.Sprintf("  %s Start: %s\n", startMark, m.schedStartInput.View()))
	b.WriteString(fmt.Sprintf("  %s End:   %s\n", endMark, m.schedEndInput.View()))

	m.writeMessages(&b)

	b.WriteString(fmt.Sprintf("\n  %s switch   %s schedule   %s back\n",
		hotkeyStyle.Render("tab"), hotkeyStyle.Render("enter"), hotkeyStyle.Render("esc")))

	return borderStyle.Width(m.boxWidth()).Render(b.String())
}

// --- Helpers ---

func (m model) writeMessages(b *strings.Builder) {
	if m.errMsg != "" {
		b.WriteString(fmt.Sprintf("\n  %s\n",
			lipgloss.NewStyle().Foreground(red).Render(m.errMsg)))
	}
	if m.successMsg != "" {
		b.WriteString(fmt.Sprintf("\n  %s\n",
			lipgloss.NewStyle().Foreground(green).Render(m.successMsg)))
	}
}

func (m model) applyBorder(content string) string {
	w := m.boxWidth()
	if m.status.Active {
		remaining := m.status.TimeRemaining
		warnThreshold := time.Duration(m.cfg.Notifications.WarnMinutes) * time.Minute
		if remaining > 0 && remaining < warnThreshold {
			return warningBorderStyle.Width(w).Render(content)
		}
		return activeBorderStyle.Width(w).Render(content)
	}
	return borderStyle.Width(w).Render(content)
}

func formatDays(days []int) string {
	names := map[int]string{
		1: "Mon", 2: "Tue", 3: "Wed", 4: "Thu",
		5: "Fri", 6: "Sat", 7: "Sun",
	}

	if len(days) == 5 {
		weekdays := true
		for i, d := range days {
			if d != i+1 {
				weekdays = false
				break
			}
		}
		if weekdays {
			return "Mon–Fri"
		}
	}

	parts := make([]string, len(days))
	for i, d := range days {
		parts[i] = names[d]
	}
	return strings.Join(parts, ", ")
}

// Run starts the TUI application.
func Run(cfg *engine.Config, state *engine.State) error {
	p := tea.NewProgram(newModel(cfg, state), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
