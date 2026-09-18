package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Auro-rium/skillmux/internal/app"
	"github.com/Auro-rium/skillmux/internal/core"
)

type viewKind int

const (
	viewFirstRun viewKind = iota
	viewMain
	viewDetails
	viewPalette
	viewAdd
	viewTargets
	viewSync
	viewSyncResult
	viewDoctor
	viewConflicts
	viewProfiles
	viewProfilePreview
	viewHarnesses
	viewDiff
	viewUpdate
	viewHelp
	viewConfirmDisable
	viewError
)

type profileSummary struct {
	Profile core.Profile
	Active  bool
}

type activityEvent struct {
	Kind string
	Text string
}

type profilePreview struct {
	From    string
	To      string
	Enable  []string
	Disable []string
}

type loadMsg struct {
	Skills   []core.Skill
	Scan     core.ScanReport
	Doctor   core.DoctorReport
	Plan     core.Plan
	Profile  string
	Profiles []profileSummary
	Err      error
}

type operationMsg struct {
	Kind    string
	Text    string
	Plan    core.Plan
	Updated []string
	Err     error
}

type diffMsg struct {
	Text string
	Err  error
}

type editorDoneMsg struct{ Err error }
type spinMsg time.Time

type Model struct {
	app   *app.App
	theme Theme
	sym   Symbols

	width  int
	height int

	view viewKind
	back viewKind

	loading   bool
	busy      bool
	busyLabel string
	spin      int

	firstRun bool

	skills   []core.Skill
	filtered []int
	selected int
	scan     core.ScanReport
	doctor   core.DoctorReport
	plan     core.Plan
	profile  string
	profiles []profileSummary

	searchMode  bool
	searchQuery string

	paletteQuery string
	paletteSel   int

	addSource      string
	addFocus       int
	addTargetIndex int
	addTargets     map[string]bool
	addScope       core.Scope

	targetIndex  int
	targetStaged map[string]bool

	conflictSel int
	doctorSel   int
	profileSel  int
	harnessSel  int

	profilePreview profilePreview

	diffTitle string
	diffText  string

	syncResult core.Plan

	errorText string

	events []activityEvent
}

func Run(a *app.App) error {
	m := NewModel(a)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func NewModel(a *app.App) Model {
	firstRun := false
	if _, err := os.Stat(firstRunMarker(a)); errors.Is(err, os.ErrNotExist) {
		firstRun = true
	}
	m := Model{
		app:          a,
		theme:        NewTheme(a.Config.TUI.Theme),
		sym:          symbolSet(),
		view:         viewMain,
		loading:      true,
		firstRun:     firstRun,
		addTargets:   map[string]bool{},
		targetStaged: map[string]bool{},
		addScope:     core.ScopeGlobal,
	}
	if firstRun {
		m.view = viewFirstRun
	}
	return m
}

func firstRunMarker(a *app.App) string {
	return filepath.Join(a.Store.Root, ".tui-initialized")
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadDataCmd(m.app), spinCmd())
}

func spinCmd() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(t time.Time) tea.Msg { return spinMsg(t) })
}

func loadDataCmd(a *app.App) tea.Cmd {
	return func() tea.Msg {
		var out loadMsg
		var err error

		out.Skills, err = a.List()
		if err != nil {
			out.Err = err
			return out
		}
		out.Scan, err = a.Scan()
		if err != nil {
			out.Err = err
			return out
		}
		out.Doctor, err = a.Doctor()
		if err != nil {
			out.Err = err
			return out
		}
		out.Plan, err = a.ProjectSync(context.Background(), true, false)
		if err != nil {
			out.Err = err
			return out
		}
		state, err := a.Store.LoadState()
		if err != nil {
			out.Err = err
			return out
		}
		out.Profile = state.ActiveProfile

		names, err := a.Store.ListProfiles()
		if err != nil {
			out.Err = err
			return out
		}
		for _, name := range names {
			p, err := a.Store.LoadProfile(name)
			if err != nil {
				continue
			}
			out.Profiles = append(out.Profiles, profileSummary{Profile: p, Active: name == state.ActiveProfile})
		}
		return out
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case spinMsg:
		if m.loading || m.busy {
			m.spin = (m.spin + 1) % len(m.sym.Spinner)
			return m, spinCmd()
		}
		return m, nil
	case loadMsg:
		m.loading = false
		if msg.Err != nil {
			m.errorText = msg.Err.Error()
			m.back = viewMain
			m.view = viewError
			return m, nil
		}
		m.skills = msg.Skills
		m.scan = msg.Scan
		m.doctor = msg.Doctor
		m.plan = msg.Plan
		m.profile = msg.Profile
		m.profiles = msg.Profiles
		m.applyFilter()
		m.clampSelections()
		return m, nil
	case operationMsg:
		m.busy = false
		m.busyLabel = ""
		if msg.Err != nil {
			m.errorText = msg.Err.Error()
			m.back = m.view
			m.view = viewError
			return m, nil
		}
		switch msg.Kind {
		case "sync":
			m.syncResult = msg.Plan
			m.view = viewSyncResult
			m.pushEvent("success", msg.Text)
		case "add":
			m.view = viewMain
			m.searchQuery = ""
			m.pushEvent("success", msg.Text)
		case "import":
			m.view = viewMain
			m.pushEvent("success", msg.Text)
		case "targets":
			m.view = viewMain
			m.pushEvent("success", msg.Text)
		case "disable":
			m.view = viewMain
			m.pushEvent("success", msg.Text)
		case "profile":
			m.pushEvent("success", msg.Text)
			m.view = viewSync
		case "update":
			m.pushEvent("success", msg.Text)
			m.view = viewMain
		case "conflict":
			m.pushEvent("success", msg.Text)
			m.view = viewConflicts
		}
		return m, loadDataCmd(m.app)
	case diffMsg:
		m.busy = false
		m.busyLabel = ""
		if msg.Err != nil {
			m.errorText = msg.Err.Error()
			m.back = viewMain
			m.view = viewError
			return m, nil
		}
		m.diffText = msg.Text
		m.view = viewDiff
		return m, nil
	case editorDoneMsg:
		if msg.Err != nil {
			m.errorText = msg.Err.Error()
			m.back = viewDetails
			m.view = viewError
			return m, nil
		}
		m.pushEvent("info", "Editor closed; refreshed skill state")
		return m, loadDataCmd(m.app)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.searchMode {
		return m.handleSearchKey(msg)
	}

	if m.view == viewAdd && m.addFocus == 0 {
		return m.handleAddInput(msg)
	}

	if m.view == viewPalette {
		return m.handlePaletteKey(msg)
	}

	if key == "q" && m.view != viewAdd {
		return m, tea.Quit
	}

	if key == "?" && m.view != viewAdd {
		m.back = m.view
		m.view = viewHelp
		return m, nil
	}

	if (key == "ctrl+p" || key == ":") && m.view != viewAdd {
		m.back = m.view
		m.view = viewPalette
		m.paletteQuery = ""
		m.paletteSel = 0
		return m, nil
	}

	if key == "esc" {
		switch m.view {
		case viewFirstRun:
			return m, nil
		case viewMain:
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.applyFilter()
				return m, nil
			}
			return m, nil
		case viewError:
			if m.back == viewError {
				m.view = viewMain
			} else {
				m.view = m.back
			}
		default:
			m.view = viewMain
		}
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	switch m.view {
	case viewFirstRun:
		if key == "enter" {
			_ = os.WriteFile(firstRunMarker(m.app), []byte("1\n"), 0o600)
			m.firstRun = false
			m.view = viewMain
		}
	case viewMain:
		return m.handleMainKey(msg)
	case viewDetails:
		return m.handleDetailsKey(msg)
	case viewAdd:
		return m.handleAddTargetsKey(msg)
	case viewTargets:
		return m.handleTargetsKey(msg)
	case viewSync:
		return m.handleSyncKey(msg)
	case viewSyncResult:
		if key == "enter" || key == " " {
			m.view = viewMain
		}
	case viewDoctor:
		m.doctorSel = navigate(m.doctorSel, len(m.doctor.Findings), key)
	case viewConflicts:
		return m.handleConflictKey(msg)
	case viewProfiles:
		return m.handleProfilesKey(msg)
	case viewProfilePreview:
		if key == "enter" {
			m.busy = true
			m.busyLabel = "Switching profile"
			return m, profileUseCmd(m.app, m.profilePreview.To)
		}
	case viewHarnesses:
		m.harnessSel = navigate(m.harnessSel, len(m.scan.Harnesses), key)
	case viewDiff:
		if key == "e" {
			return m.openEditorForSelected()
		}
	case viewUpdate:
		if key == "enter" {
			if sk := m.selectedSkill(); sk != nil {
				m.busy = true
				m.busyLabel = "Updating " + sk.Name
				return m, updateCmd(m.app, sk.Name)
			}
		}
	case viewHelp:
		if key == "enter" {
			m.view = m.back
		}
	case viewConfirmDisable:
		if key == "y" || key == "enter" {
			if sk := m.selectedSkill(); sk != nil {
				m.busy = true
				m.busyLabel = "Disabling " + sk.Name
				return m, disableAllCmd(m.app, *sk)
			}
		}
		if key == "n" {
			m.view = viewMain
		}
	}
	return m, nil
}

func (m Model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "j", "down":
		m.selected = navigate(m.selected, len(m.filtered), "down")
	case "k", "up":
		m.selected = navigate(m.selected, len(m.filtered), "up")
	case "enter":
		if len(m.skills) == 0 {
			m.loading = true
			return m, tea.Batch(loadDataCmd(m.app), spinCmd())
		}
		if m.selectedSkill() != nil {
			m.view = viewDetails
		}
	case "/":
		m.searchMode = true
	case "a":
		m.prepareAdd()
		m.view = viewAdd
	case "s":
		m.view = viewSync
	case "d":
		m.view = viewDoctor
	case "p":
		m.view = viewProfiles
	case "h":
		m.view = viewHarnesses
	case "c":
		if len(m.scan.Conflicts) == 0 {
			m.pushEvent("info", "No conflicts")
		} else {
			m.conflictSel = 0
			m.view = viewConflicts
		}
	case " ":
		m.prepareTargets()
		m.view = viewTargets
	case "u":
		if m.selectedSkill() != nil {
			m.view = viewUpdate
		}
	case "v":
		return m.startDiff()
	case "e":
		return m.openEditorForSelected()
	case "x":
		if sk := m.selectedSkill(); sk != nil && enabledCount(*sk) > 0 {
			m.view = viewConfirmDisable
		}
	}
	return m, nil
}

func (m Model) handleDetailsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "j", "down":
		m.selected = navigate(m.selected, len(m.filtered), "down")
	case "k", "up":
		m.selected = navigate(m.selected, len(m.filtered), "up")
	case "/":
		m.searchMode = true
	case " ":
		m.prepareTargets()
		m.view = viewTargets
	case "v":
		return m.startDiff()
	case "e":
		return m.openEditorForSelected()
	case "u":
		m.view = viewUpdate
	case "x":
		if sk := m.selectedSkill(); sk != nil && enabledCount(*sk) > 0 {
			m.view = viewConfirmDisable
		}
	case "s":
		m.view = viewSync
	}
	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.searchMode = false
	case "enter":
		m.searchMode = false
	case "backspace", "ctrl+h":
		m.searchQuery = trimLastRune(m.searchQuery)
		m.applyFilter()
	case "ctrl+u":
		m.searchQuery = ""
		m.applyFilter()
	case "down", "ctrl+n":
		m.selected = navigate(m.selected, len(m.filtered), "down")
	case "up", "ctrl+p":
		m.selected = navigate(m.selected, len(m.filtered), "up")
	default:
		if len(msg.Runes) > 0 {
			m.searchQuery += string(msg.Runes)
			m.applyFilter()
		}
	}
	return m, nil
}

func (m Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.view = m.back
	case "backspace", "ctrl+h":
		m.paletteQuery = trimLastRune(m.paletteQuery)
		m.paletteSel = 0
	case "ctrl+u":
		m.paletteQuery = ""
		m.paletteSel = 0
	case "j", "down", "ctrl+n":
		m.paletteSel = navigate(m.paletteSel, len(m.paletteCommands()), "down")
	case "k", "up":
		m.paletteSel = navigate(m.paletteSel, len(m.paletteCommands()), "up")
	case "enter":
		cmds := m.paletteCommands()
		if len(cmds) > 0 {
			return m.executePalette(cmds[m.paletteSel].ID)
		}
	default:
		if len(msg.Runes) > 0 {
			m.paletteQuery += string(msg.Runes)
			m.paletteSel = 0
		}
	}
	return m, nil
}

type paletteCommand struct {
	ID    string
	Label string
}

func (m Model) paletteCommands() []paletteCommand {
	all := []paletteCommand{
		{ID: "sync", Label: "Sync environment"},
		{ID: "doctor", Label: "Run doctor"},
		{ID: "add", Label: "Add skill"},
		{ID: "profiles", Label: "Change profile"},
		{ID: "import", Label: "Import existing skills"},
		{ID: "conflicts", Label: "Show conflicts"},
		{ID: "harnesses", Label: "Harness overview"},
		{ID: "update", Label: "Update selected skill"},
		{ID: "help", Label: "Show shortcuts"},
	}
	q := strings.TrimSpace(m.paletteQuery)
	if q == "" {
		return all
	}
	var out []paletteCommand
	for _, item := range all {
		if _, ok := fuzzyMatch(item.Label, q); ok {
			out = append(out, item)
		}
	}
	return out
}

func (m Model) executePalette(id string) (tea.Model, tea.Cmd) {
	switch id {
	case "sync":
		m.view = viewSync
	case "doctor":
		m.view = viewDoctor
	case "add":
		m.prepareAdd()
		m.view = viewAdd
	case "profiles":
		m.view = viewProfiles
	case "import":
		m.busy = true
		m.busyLabel = "Importing existing skills"
		m.view = viewMain
		return m, importCmd(m.app)
	case "conflicts":
		if len(m.scan.Conflicts) == 0 {
			m.view = viewMain
			m.pushEvent("info", "No conflicts")
		} else {
			m.conflictSel = 0
			m.view = viewConflicts
		}
	case "harnesses":
		m.view = viewHarnesses
	case "update":
		if m.selectedSkill() == nil {
			m.view = viewMain
			m.pushEvent("warning", "No skill selected")
		} else {
			m.view = viewUpdate
		}
	case "help":
		m.back = viewMain
		m.view = viewHelp
	}
	return m, nil
}

func (m *Model) prepareAdd() {
	m.addSource = ""
	m.addFocus = 0
	m.addTargetIndex = 0
	m.addScope = core.ScopeGlobal
	m.addTargets = map[string]bool{}
	for _, h := range m.scan.Harnesses {
		if h.Detected {
			m.addTargets[h.Name] = true
		}
	}
}

func (m Model) handleAddInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.view = viewMain
	case "tab", "enter":
		if strings.TrimSpace(m.addSource) != "" {
			m.addFocus = 1
		}
	case "backspace", "ctrl+h":
		m.addSource = trimLastRune(m.addSource)
	case "ctrl+u":
		m.addSource = ""
	default:
		if len(msg.Runes) > 0 {
			m.addSource += string(msg.Runes)
		}
	}
	return m, nil
}

func (m Model) handleAddTargetsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	harnesses := m.scan.Harnesses
	switch key {
	case "tab":
		m.addFocus = 0
	case "j", "down":
		m.addTargetIndex = navigate(m.addTargetIndex, len(harnesses), "down")
	case "k", "up":
		m.addTargetIndex = navigate(m.addTargetIndex, len(harnesses), "up")
	case " ":
		if len(harnesses) > 0 {
			name := harnesses[m.addTargetIndex].Name
			m.addTargets[name] = !m.addTargets[name]
		}
	case "g":
		if m.addScope == core.ScopeGlobal {
			if m.app.Root != "" {
				m.addScope = core.ScopeProject
			}
		} else {
			m.addScope = core.ScopeGlobal
		}
	case "enter":
		if strings.TrimSpace(m.addSource) == "" {
			m.addFocus = 0
			return m, nil
		}
		var targets []string
		for name, enabled := range m.addTargets {
			if enabled {
				targets = append(targets, name)
			}
		}
		sort.Strings(targets)
		m.busy = true
		m.busyLabel = "Installing " + m.addSource
		return m, addCmd(m.app, m.addSource, targets, m.addScope)
	}
	return m, nil
}

func (m *Model) prepareTargets() {
	m.targetIndex = 0
	m.targetStaged = map[string]bool{}
	if sk := m.selectedSkill(); sk != nil {
		for _, h := range m.scan.Harnesses {
			m.targetStaged[h.Name] = sk.Enabled[h.Name]
		}
	}
}

func (m Model) handleTargetsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	harnesses := m.scan.Harnesses
	switch key {
	case "j", "down":
		m.targetIndex = navigate(m.targetIndex, len(harnesses), "down")
	case "k", "up":
		m.targetIndex = navigate(m.targetIndex, len(harnesses), "up")
	case " ":
		if len(harnesses) > 0 {
			name := harnesses[m.targetIndex].Name
			m.targetStaged[name] = !m.targetStaged[name]
		}
	case "enter":
		if sk := m.selectedSkill(); sk != nil {
			staged := make(map[string]bool, len(m.targetStaged))
			for k, v := range m.targetStaged {
				staged[k] = v
			}
			m.busy = true
			m.busyLabel = "Updating targets"
			return m, targetCmd(m.app, sk.Name, staged)
		}
	}
	return m, nil
}

func (m Model) handleSyncKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if len(m.plan.Changes) == 0 {
			m.syncResult = m.plan
			m.view = viewSyncResult
			return m, nil
		}
		m.busy = true
		m.busyLabel = "Synchronizing environment"
		return m, syncCmd(m.app)
	case "d":
		m.pushEvent("info", "Dry-run only; no changes applied")
	}
	return m, nil
}

func (m Model) handleConflictKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "j", "down":
		m.conflictSel = navigate(m.conflictSel, len(m.scan.Conflicts), "down")
	case "k", "up":
		m.conflictSel = navigate(m.conflictSel, len(m.scan.Conflicts), "up")
	case "m":
		if len(m.scan.Conflicts) > 0 {
			c := m.scan.Conflicts[m.conflictSel]
			m.diffTitle = "Conflict: " + c.Name
			m.diffText = conflictDiff(c)
			m.back = viewConflicts
			m.view = viewDiff
		}
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' && len(m.scan.Conflicts) > 0 {
			c := m.scan.Conflicts[m.conflictSel]
			idx := int(key[0] - '1')
			if idx < len(c.Copies) {
				cp := c.Copies[idx]
				m.busy = true
				m.busyLabel = "Resolving " + c.Name
				return m, conflictCmd(m.app, c.Name, cp.Harness)
			}
		}
	}
	return m, nil
}

func (m Model) handleProfilesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "j", "down":
		m.profileSel = navigate(m.profileSel, len(m.profiles), "down")
	case "k", "up":
		m.profileSel = navigate(m.profileSel, len(m.profiles), "up")
	case "enter":
		if len(m.profiles) == 0 {
			return m, nil
		}
		p := m.profiles[m.profileSel]
		if p.Active {
			m.pushEvent("info", p.Profile.Name+" is already active")
			return m, nil
		}
		m.profilePreview = m.buildProfilePreview(p.Profile)
		m.view = viewProfilePreview
	case "c":
		m.busy = true
		m.busyLabel = "Creating profile"
		return m, profileCreateCmd(m.app)
	}
	return m, nil
}

func (m Model) buildProfilePreview(p core.Profile) profilePreview {
	current := map[string]bool{}
	for _, sk := range m.skills {
		if enabledCount(sk) > 0 {
			current[sk.Name] = true
		}
	}
	next := map[string]bool{}
	for _, name := range p.Skills {
		next[name] = true
	}
	out := profilePreview{From: m.profile, To: p.Name}
	for name := range next {
		if !current[name] {
			out.Enable = append(out.Enable, name)
		}
	}
	for name := range current {
		if !next[name] {
			out.Disable = append(out.Disable, name)
		}
	}
	sort.Strings(out.Enable)
	sort.Strings(out.Disable)
	return out
}

func (m Model) startDiff() (tea.Model, tea.Cmd) {
	sk := m.selectedSkill()
	if sk == nil {
		return m, nil
	}
	m.busy = true
	m.busyLabel = "Loading diff"
	m.diffTitle = "Diff: " + sk.Name
	return m, func() tea.Msg {
		text, err := m.app.Diff(sk.Name, "")
		return diffMsg{Text: text, Err: err}
	}
}

func (m Model) openEditorForSelected() (tea.Model, tea.Cmd) {
	sk := m.selectedSkill()
	if sk == nil {
		return m, nil
	}
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return m, nil
	}
	args := append(parts[1:], filepath.Join(sk.Path, "SKILL.md"))
	cmd := exec.Command(parts[0], args...)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editorDoneMsg{Err: err} })
}

func (m *Model) applyFilter() {
	m.filtered = m.filtered[:0]
	for i, sk := range m.skills {
		if strings.TrimSpace(m.searchQuery) == "" {
			m.filtered = append(m.filtered, i)
			continue
		}
		hay := sk.Name + " " + sk.Description
		if sk.Source != nil {
			hay += " " + sk.Source.Repository + " " + sk.Source.URL
		}
		if _, ok := fuzzyMatch(hay, m.searchQuery); ok {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.selected >= len(m.filtered) {
		m.selected = maxInt(0, len(m.filtered)-1)
	}
}

func (m *Model) clampSelections() {
	m.selected = clampIndex(m.selected, len(m.filtered))
	m.conflictSel = clampIndex(m.conflictSel, len(m.scan.Conflicts))
	m.doctorSel = clampIndex(m.doctorSel, len(m.doctor.Findings))
	m.profileSel = clampIndex(m.profileSel, len(m.profiles))
	m.harnessSel = clampIndex(m.harnessSel, len(m.scan.Harnesses))
}

func (m Model) selectedSkill() *core.Skill {
	if len(m.filtered) == 0 || m.selected < 0 || m.selected >= len(m.filtered) {
		return nil
	}
	idx := m.filtered[m.selected]
	if idx < 0 || idx >= len(m.skills) {
		return nil
	}
	return &m.skills[idx]
}

func (m *Model) pushEvent(kind, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	m.events = append(m.events, activityEvent{Kind: kind, Text: text})
	if len(m.events) > 8 {
		m.events = append([]activityEvent(nil), m.events[len(m.events)-8:]...)
	}
}

func addCmd(a *app.App, source string, targets []string, scope core.Scope) tea.Cmd {
	return func() tea.Msg {
		sk, err := a.Add(context.Background(), source, targets)
		if err == nil && scope == core.ScopeProject {
			err = a.Store.SetScope(sk.Name, core.ScopeProject)
		}
		if err != nil {
			return operationMsg{Kind: "add", Err: err}
		}
		return operationMsg{Kind: "add", Text: sk.Name + " installed"}
	}
}

func targetCmd(a *app.App, skill string, staged map[string]bool) tea.Cmd {
	return func() tea.Msg {
		for harnessName, enabled := range staged {
			if err := a.Enable(skill, harnessName, enabled); err != nil {
				return operationMsg{Kind: "targets", Err: err}
			}
		}
		return operationMsg{Kind: "targets", Text: skill + " target selection updated; sync to apply"}
	}
}

func disableAllCmd(a *app.App, sk core.Skill) tea.Cmd {
	return func() tea.Msg {
		for harnessName, enabled := range sk.Enabled {
			if !enabled {
				continue
			}
			if err := a.Enable(sk.Name, harnessName, false); err != nil {
				return operationMsg{Kind: "disable", Err: err}
			}
		}
		return operationMsg{Kind: "disable", Text: sk.Name + " disabled for all targets; sync to apply"}
	}
}

func syncCmd(a *app.App) tea.Cmd {
	return func() tea.Msg {
		plan, err := a.ProjectSync(context.Background(), false, false)
		if err != nil {
			return operationMsg{Kind: "sync", Err: err}
		}
		return operationMsg{Kind: "sync", Text: "Environment synchronized", Plan: plan}
	}
}

func importCmd(a *app.App) tea.Cmd {
	return func() tea.Msg {
		result, err := a.Import("", "")
		if err != nil {
			return operationMsg{Kind: "import", Err: err}
		}
		text := fmt.Sprintf("Imported %d skill(s)", len(result.Imported))
		if len(result.Conflicts) > 0 {
			text += fmt.Sprintf("; %d conflict(s) require review", len(result.Conflicts))
		}
		return operationMsg{Kind: "import", Text: text}
	}
}

func updateCmd(a *app.App, name string) tea.Cmd {
	return func() tea.Msg {
		updated, err := a.Update(context.Background(), name)
		if err != nil {
			return operationMsg{Kind: "update", Err: err}
		}
		if len(updated) == 0 {
			return operationMsg{Kind: "update", Text: name + " is already current", Updated: updated}
		}
		return operationMsg{Kind: "update", Text: name + " updated", Updated: updated}
	}
}

func conflictCmd(a *app.App, name, harnessName string) tea.Cmd {
	return func() tea.Msg {
		err := a.ResolveConflict(name, harnessName)
		if err != nil {
			return operationMsg{Kind: "conflict", Err: err}
		}
		return operationMsg{Kind: "conflict", Text: name + " resolved using " + displayHarness(harnessName)}
	}
}

func profileUseCmd(a *app.App, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := a.ProfileUse(name)
		if err != nil {
			return operationMsg{Kind: "profile", Err: err}
		}
		return operationMsg{Kind: "profile", Text: "Profile " + name + " selected; review sync plan"}
	}
}

func profileCreateCmd(a *app.App) tea.Cmd {
	return func() tea.Msg {
		name := time.Now().Format("profile-20060102-150405")
		p, err := a.ProfileCreate(name)
		if err != nil {
			return operationMsg{Kind: "profile", Err: err}
		}
		return operationMsg{Kind: "profile", Text: fmt.Sprintf("Created %s with %d skill(s)", p.Name, len(p.Skills))}
	}
}

func navigate(current, count int, key string) int {
	if count <= 0 {
		return 0
	}
	switch key {
	case "j", "down", "ctrl+n":
		current++
	case "k", "up", "ctrl+p":
		current--
	}
	if current < 0 {
		current = count - 1
	}
	if current >= count {
		current = 0
	}
	return current
}

func clampIndex(i, count int) int {
	if count <= 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= count {
		return count - 1
	}
	return i
}

func trimLastRune(s string) string {
	if s == "" {
		return ""
	}
	_, size := utf8.DecodeLastRuneInString(s)
	if size <= 0 {
		return ""
	}
	return s[:len(s)-size]
}

func enabledCount(sk core.Skill) int {
	n := 0
	for _, enabled := range sk.Enabled {
		if enabled {
			n++
		}
	}
	return n
}

func displayHarness(name string) string {
	switch strings.ToLower(name) {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude Code"
	case "gemini":
		return "Gemini CLI"
	case "cursor":
		return "Cursor"
	case "opencode":
		return "OpenCode"
	default:
		if name == "" {
			return "Unknown"
		}
		return strings.ToUpper(name[:1]) + name[1:]
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
