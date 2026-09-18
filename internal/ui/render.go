package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Auro-rium/skillmux/internal/core"
)

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "skillmux\n"
	}
	if m.height < 8 {
		return fitPlain("skillmux  terminal too small", m.width)
	}

	if m.view == viewFirstRun {
		return m.renderFirstRun()
	}

	header := m.renderHeader()
	bodyHeight := m.height - 3
	if bodyHeight < 4 {
		bodyHeight = 4
	}
	body := m.renderViewBody(bodyHeight)
	status := m.renderStatusLine()
	footer := m.renderFooter()

	return strings.Join([]string{header, body, status, footer}, "\n")
}

func (m Model) renderViewBody(height int) string {
	switch m.view {
	case viewMain:
		return m.renderMain(height)
	case viewDetails:
		return m.renderDetailsScreen(height)
	case viewPalette:
		return m.renderPalette(height)
	case viewAdd:
		return m.renderAdd(height)
	case viewTargets:
		return m.renderTargets(height)
	case viewSync:
		return m.renderSync(height)
	case viewSyncResult:
		return m.renderSyncResult(height)
	case viewDoctor:
		return m.renderDoctor(height)
	case viewConflicts:
		return m.renderConflicts(height)
	case viewProfiles:
		return m.renderProfiles(height)
	case viewProfilePreview:
		return m.renderProfilePreview(height)
	case viewHarnesses:
		return m.renderHarnesses(height)
	case viewDiff:
		return m.renderDiff(height)
	case viewUpdate:
		return m.renderUpdate(height)
	case viewHelp:
		return m.renderHelp(height)
	case viewConfirmDisable:
		return m.renderConfirmDisable(height)
	case viewError:
		return m.renderError(height)
	default:
		return m.renderMain(height)
	}
}

func (m Model) renderHeader() string {
	left := m.theme.Heading.Render(" skillmux")
	project := projectLabel(m.app.Root)
	profile := m.profile
	if profile == "" {
		profile = "default"
	}
	right := m.theme.Secondary.Render(profile + " " + m.sym.Dot)

	plainMiddle := project
	maxMiddle := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if maxMiddle < 0 {
		maxMiddle = 0
	}
	plainMiddle = truncateText(plainMiddle, maxMiddle)
	middle := m.theme.Muted.Render(plainMiddle)

	used := lipgloss.Width(left) + lipgloss.Width(middle) + lipgloss.Width(right)
	gap1 := 2
	gap2 := m.width - used - gap1
	if gap2 < 1 {
		gap2 = 1
	}
	line := left + strings.Repeat(" ", gap1) + middle + strings.Repeat(" ", gap2) + right
	return fitStyled(line, m.width)
}

func (m Model) renderMain(height int) string {
	if m.loading {
		return m.panel("Skills", []string{"", m.spinner()+" Loading environment state..."}, m.width, height)
	}

	if m.width < 96 {
		return m.panel("Skills", m.skillListLines(m.width-2, height-2), m.width, height)
	}

	if m.width < 140 {
		leftWidth := maxInt(30, m.width*38/100)
		rightWidth := m.width - leftWidth
		left := m.panel("Skills", m.skillListLines(leftWidth-2, height-2), leftWidth, height)
		right := m.panel("Details", m.detailLines(rightWidth-2, height-2), rightWidth, height)
		return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	leftWidth := maxInt(32, m.width*27/100)
	detailsWidth := maxInt(48, m.width*43/100)
	activityWidth := m.width - leftWidth - detailsWidth
	left := m.panel("Skills", m.skillListLines(leftWidth-2, height-2), leftWidth, height)
	details := m.panel("Details", m.detailLines(detailsWidth-2, height-2), detailsWidth, height)
	activity := m.panel("Activity", m.activityLines(activityWidth-2, height-2), activityWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, details, activity)
}

func (m Model) skillListLines(width, height int) []string {
	if len(m.skills) == 0 {
		lines := []string{
			"",
			m.theme.Heading.Render("No skills found."),
			"",
			"Skillmux can scan your existing agent environments.",
			"",
			m.theme.Key.Render("Enter") + " scan",
			m.theme.Key.Render("a") + "     add skill",
			m.theme.Key.Render("?") + "     help",
		}
		return clampLines(lines, height)
	}
	if len(m.filtered) == 0 {
		return clampLines([]string{"", "No skills match " + quoted(m.searchQuery) + ".", "", m.theme.Muted.Render("Esc clears the filter.")}, height)
	}

	start, end := listWindow(m.selected, len(m.filtered), height)
	lines := make([]string, 0, height)
	conflicts := m.conflictNames()

	for row := start; row < end; row++ {
		sk := m.skills[m.filtered[row]]
		marker, markerStyle := m.skillMarker(sk, conflicts[sk.Name])
		cursor := "  "
		nameStyle := m.theme.Primary
		if row == m.selected {
			cursor = m.sym.Cursor + " "
			nameStyle = m.theme.Selected
		}

		markerWidth := lipgloss.Width(marker)
		nameWidth := width - 2 - markerWidth - 1
		if nameWidth < 4 {
			nameWidth = 4
		}
		name := truncateText(sk.Name, nameWidth)
		if m.searchQuery != "" {
			name = highlightMatch(name, m.searchQuery, nameStyle.Render, m.theme.Emphasis.Copy().Underline(true).Render)
		} else {
			name = nameStyle.Render(name)
		}
		spaces := width - lipgloss.Width(cursor) - lipgloss.Width(name) - markerWidth
		if spaces < 1 {
			spaces = 1
		}
		lines = append(lines, cursor+name+strings.Repeat(" ", spaces)+markerStyle.Render(marker))
	}
	if start > 0 {
		lines[0] = m.theme.Muted.Render("↑ more") + strings.Repeat(" ", maxInt(0, width-len("↑ more")))
	}
	if end < len(m.filtered) && len(lines) > 0 {
		lines[len(lines)-1] = m.theme.Muted.Render(fmt.Sprintf("↓ %d more", len(m.filtered)-end))
	}
	return clampLines(lines, height)
}

func (m Model) detailLines(width, height int) []string {
	sk := m.selectedSkill()
	if sk == nil {
		return []string{m.theme.Muted.Render("Select a skill to inspect it.")}
	}
	lines := []string{m.theme.Heading.Render(truncateText(sk.Name, width))}
	if sk.Description != "" {
		lines = append(lines, "", truncateText(sk.Description, width))
	}

	conflict := m.conflictNames()[sk.Name]
	marker, style := m.skillMarker(*sk, conflict)
	label := stateLabel(*sk, conflict)
	lines = append(lines, "", m.theme.Muted.Render("Status"), style.Render(marker+" "+label))

	if sk.Source != nil {
		source := sk.Source.URL
		if source == "" {
			source = "github:" + sk.Source.Repository
			if sk.Source.Subdir != "" {
				source += "/" + sk.Source.Subdir
			}
		}
		lines = append(lines, "", alignedField("Source", truncateText(source, maxInt(10, width-11)), width, m.theme))
		version := sk.Source.Version
		if version == "" && sk.Source.Commit != "" {
			version = shortHash(sk.Source.Commit)
		}
		if version != "" {
			lines = append(lines, alignedField("Version", version, width, m.theme))
		}
	} else {
		lines = append(lines, "", alignedField("Source", "local", width, m.theme))
	}
	lines = append(lines, alignedField("Scope", string(sk.Scope), width, m.theme))

	lines = append(lines, "", m.theme.Muted.Render("Targets"))
	for _, h := range m.scan.Harnesses {
		enabled := sk.Enabled[h.Name]
		if enabled {
			lines = append(lines, m.theme.Success.Render(m.sym.Healthy)+" "+displayHarness(h.Name))
		} else {
			lines = append(lines, m.theme.Disabled.Render(m.sym.Disabled)+" "+displayHarness(h.Name))
		}
	}
	return clampLines(lines, height)
}

func (m Model) activityLines(width, height int) []string {
	if len(m.events) == 0 {
		return []string{m.theme.Muted.Render("No recent activity.")}
	}
	start := maxInt(0, len(m.events)-height)
	var lines []string
	for _, ev := range m.events[start:] {
		sym, style := m.eventVisual(ev.Kind)
		text := truncateText(ev.Text, maxInt(5, width-lipgloss.Width(sym)-1))
		lines = append(lines, style.Render(sym)+" "+text)
	}
	return lines
}

func (m Model) renderDetailsScreen(height int) string {
	return m.panel("Details", m.detailLines(m.width-2, height-2), m.width, height)
}

func (m Model) renderFirstRun() string {
	height := m.height
	width := m.width
	var lines []string
	lines = append(lines, m.theme.Heading.Render("skillmux"), "")
	if m.loading {
		lines = append(lines, m.spinner()+" Scanning agent environments...")
	} else {
		lines = append(lines, "Scanning agent environments...", "")
		for _, h := range m.scan.Harnesses {
			if h.Detected {
				lines = append(lines, m.theme.Success.Render(m.sym.Healthy)+" "+displayHarness(h.Name))
			} else {
				lines = append(lines, m.theme.Disabled.Render(m.sym.Disabled)+" "+displayHarness(h.Name))
			}
		}
		lines = append(lines, "",
			fmt.Sprintf("%d skills found", len(m.scan.Installations)),
			fmt.Sprintf("%d duplicates", m.scan.Duplicates),
			fmt.Sprintf("%d conflicts", len(m.scan.Conflicts)),
			"",
			m.theme.Key.Render("Enter")+" continue",
		)
	}
	bodyHeight := height - 1
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}
	return strings.Join(clampLines(lines, height), "\n")
}

func (m Model) renderPalette(height int) string {
	cmds := m.paletteCommands()
	lines := []string{m.theme.Muted.Render("Type to filter commands"), ""}
	start, end := listWindow(m.paletteSel, len(cmds), maxInt(1, height-6))
	for i := start; i < end; i++ {
		prefix := "  "
		style := m.theme.Primary
		if i == m.paletteSel {
			prefix = m.sym.Cursor + " "
			style = m.theme.Selected
		}
		lines = append(lines, prefix+style.Render(cmds[i].Label))
	}
	if len(cmds) == 0 {
		lines = append(lines, m.theme.Muted.Render("No matching commands."))
	}
	lines = append(lines, "", "Command: "+m.paletteQuery+"█")
	return m.panel("Command", lines, m.width, height)
}

func (m Model) renderAdd(height int) string {
	lines := []string{
		m.theme.Heading.Render("Install skill"),
		"",
		m.theme.Muted.Render("Source"),
	}
	source := m.addSource
	if m.addFocus == 0 {
		source += "█"
	}
	if source == "" {
		source = m.theme.Muted.Render("github:owner/repo/path or local path")
	}
	lines = append(lines, source, "", m.theme.Muted.Render("Targets"))

	for i, h := range m.scan.Harnesses {
		cursor := "  "
		if m.addFocus == 1 && i == m.addTargetIndex {
			cursor = m.sym.Cursor + " "
		}
		mark := m.sym.Disabled
		style := m.theme.Disabled
		if m.addTargets[h.Name] {
			mark = m.sym.Healthy
			style = m.theme.Success
		}
		lines = append(lines, cursor+style.Render(mark)+" "+displayHarness(h.Name))
	}
	lines = append(lines, "", alignedField("Scope", string(m.addScope), m.width-4, m.theme))
	if m.app.Root == "" {
		lines = append(lines, m.theme.Muted.Render("Project scope unavailable outside a repository."))
	}
	return m.panel("Add", lines, m.width, height)
}

func (m Model) renderTargets(height int) string {
	sk := m.selectedSkill()
	title := "Targets"
	if sk != nil {
		title += ": " + sk.Name
	}
	var lines []string
	for i, h := range m.scan.Harnesses {
		cursor := "  "
		if i == m.targetIndex {
			cursor = m.sym.Cursor + " "
		}
		mark := m.sym.Disabled
		style := m.theme.Disabled
		if m.targetStaged[h.Name] {
			mark = m.sym.Healthy
			style = m.theme.Success
		}
		detected := ""
		if !h.Detected {
			detected = " " + m.theme.Muted.Render("(not configured)")
		}
		lines = append(lines, cursor+style.Render(mark)+" "+displayHarness(h.Name)+detected)
	}
	lines = append(lines, "", m.theme.Muted.Render("Space toggles. Enter saves desired exposure; sync applies it."))
	return m.panel(title, lines, m.width, height)
}

func (m Model) renderSync(height int) string {
	var lines []string
	if len(m.plan.Changes) == 0 {
		lines = append(lines, m.theme.Success.Render(m.sym.Healthy)+" Environment already synchronized.", "")
		for _, h := range m.scan.Harnesses {
			lines = append(lines, displayHarness(h.Name)+"  "+m.theme.Muted.Render("no changes"))
		}
		return m.panel("Sync Environment", lines, m.width, height)
	}

	grouped := map[string][]core.Change{}
	var order []string
	for _, change := range m.plan.Changes {
		if _, ok := grouped[change.Harness]; !ok {
			order = append(order, change.Harness)
		}
		grouped[change.Harness] = append(grouped[change.Harness], change)
	}
	for _, h := range order {
		lines = append(lines, m.theme.Heading.Render(displayHarness(h)))
		for _, change := range grouped[h] {
			symbol, style := m.changeVisual(change.Kind)
			text := "  " + style.Render(symbol) + " " + change.Skill
			if change.Reason != "" {
				text += "  " + m.theme.Muted.Render(truncateText(change.Reason, maxInt(8, m.width-10-len(change.Skill))))
			}
			lines = append(lines, text)
		}
		lines = append(lines, "")
	}
	adds, updates, removals := planCounts(m.plan)
	lines = append(lines, m.theme.Border.Render(strings.Repeat("─", maxInt(1, minInt(m.width-4, 48)))), "")
	lines = append(lines, fmt.Sprintf("%d additions   %d updates   %d removals", adds, updates, removals))
	return m.panel("Sync Environment", clampLines(lines, height-2), m.width, height)
}

func (m Model) renderSyncResult(height int) string {
	lines := []string{m.theme.Success.Render(m.sym.Healthy) + " " + m.theme.Heading.Render("Environment synchronized"), ""}
	counts := m.harnessSkillCounts()
	for _, h := range m.scan.Harnesses {
		lines = append(lines, fmt.Sprintf("%-15s %d skills", displayHarness(h.Name), counts[h.Name]))
	}
	lines = append(lines, "")
	if len(m.scan.Conflicts) == 0 {
		lines = append(lines, "Completed with no conflicts.")
	} else {
		lines = append(lines, m.theme.Warning.Render(fmt.Sprintf("%s %d conflict(s) still require review.", m.sym.Warning, len(m.scan.Conflicts))))
	}
	lines = append(lines, "", m.theme.Key.Render("Enter")+" return")
	return m.panel("Sync Result", lines, m.width, height)
}

func (m Model) renderDoctor(height int) string {
	if len(m.doctor.Findings) == 0 {
		lines := []string{
			m.theme.Success.Render(m.sym.Healthy) + " " + fmt.Sprintf("%d skills checked", m.doctor.SkillsChecked),
			"",
			"No findings.",
		}
		return m.panel("Doctor", lines, m.width, height)
	}

	maxRows := maxInt(1, height-7)
	start, end := listWindow(m.doctorSel, len(m.doctor.Findings), maxRows)
	var lines []string
	for i := start; i < end; i++ {
		f := m.doctor.Findings[i]
		sym, style := m.findingVisual(f.Severity)
		cursor := "  "
		if i == m.doctorSel {
			cursor = m.sym.Cursor + " "
		}
		name := f.Skill
		if name == "" {
			name = filepath.Base(f.Path)
		}
		lines = append(lines, cursor+style.Render(sym)+" "+m.theme.Heading.Render(name))
		lines = append(lines, "    "+truncateText(f.Message, maxInt(8, m.width-8)))
		if f.Path != "" {
			lines = append(lines, "    "+m.theme.Muted.Render(truncateText(f.Path, maxInt(8, m.width-8))))
		}
	}
	warnings, errs := doctorCounts(m.doctor)
	lines = append(lines, "", m.theme.Border.Render(strings.Repeat("─", maxInt(1, minInt(m.width-4, 28)))), "")
	lines = append(lines, fmt.Sprintf("%d healthy   %d warnings   %d errors", maxInt(0, m.doctor.SkillsChecked-warnings-errs), warnings, errs))
	return m.panel("Doctor", lines, m.width, height)
}

func (m Model) renderConflicts(height int) string {
	if len(m.scan.Conflicts) == 0 {
		return m.panel("Conflicts", []string{m.theme.Success.Render(m.sym.Healthy) + " No conflicts."}, m.width, height)
	}
	c := m.scan.Conflicts[m.conflictSel]
	lines := []string{m.theme.Heading.Render("Conflict: " + c.Name), "", truncateText(c.Reason, maxInt(10, m.width-4)), ""}

	for i, cp := range c.Copies {
		stat, _ := os.Stat(cp.Path)
		modified := ""
		if stat != nil {
			modified = stat.ModTime().Format("02 Jan 15:04")
		}
		lines = append(lines,
			fmt.Sprintf("%d %s", i+1, m.theme.Heading.Render(displayHarness(cp.Harness))),
			"  "+m.theme.Muted.Render(strings.TrimSpace(modified+"  "+shortHash(cp.Hash))),
		)
	}
	lines = append(lines, "", m.theme.Border.Render(strings.Repeat("─", maxInt(1, minInt(m.width-4, 44)))), "")
	diff := strings.Split(conflictDiff(c), "\n")
	for _, line := range diff {
		if len(lines) >= height-5 {
			break
		}
		lines = append(lines, colorDiffLine(m.theme, truncateText(line, maxInt(8, m.width-4))))
	}
	return m.panel("Conflicts", lines, m.width, height)
}

func (m Model) renderProfiles(height int) string {
	if len(m.profiles) == 0 {
		lines := []string{"No saved profiles.", "", m.theme.Key.Render("c") + " create from current environment"}
		return m.panel("Profiles", lines, m.width, height)
	}
	var lines []string
	for i, item := range m.profiles {
		cursor := "  "
		style := m.theme.Primary
		if i == m.profileSel {
			cursor = m.sym.Cursor + " "
			style = m.theme.Selected
		}
		active := ""
		if item.Active {
			active = " " + m.theme.Success.Render(m.sym.Dot)
		}
		lines = append(lines, fmt.Sprintf("%s%s  %d skills%s", cursor, style.Render(item.Profile.Name), len(item.Profile.Skills), active))
	}
	return m.panel("Profiles", lines, m.width, height)
}

func (m Model) renderProfilePreview(height int) string {
	p := m.profilePreview
	lines := []string{m.theme.Heading.Render("Switch " + p.From + " → " + p.To), ""}
	if len(p.Enable) == 0 {
		lines = append(lines, m.theme.Muted.Render("Enable: none"))
	} else {
		lines = append(lines, m.theme.Muted.Render("Enable"))
		for _, name := range p.Enable {
			lines = append(lines, m.theme.Success.Render("+")+" "+name)
		}
	}
	lines = append(lines, "")
	if len(p.Disable) == 0 {
		lines = append(lines, m.theme.Muted.Render("Disable: none"))
	} else {
		lines = append(lines, m.theme.Muted.Render("Disable"))
		for _, name := range p.Disable {
			lines = append(lines, m.theme.Warning.Render("-")+" "+name)
		}
	}
	lines = append(lines, "", m.theme.Muted.Render("Enter updates desired state, then opens the sync preview."))
	return m.panel("Profile Preview", lines, m.width, height)
}

func (m Model) renderHarnesses(height int) string {
	counts := m.harnessSkillCounts()
	drift := m.harnessDriftCounts()
	var lines []string
	for i, h := range m.scan.Harnesses {
		cursor := "  "
		if i == m.harnessSel {
			cursor = m.sym.Cursor + " "
		}
		lines = append(lines, cursor+m.theme.Heading.Render(displayHarness(h.Name)))
		if h.Detected {
			lines = append(lines, "  "+m.theme.Success.Render(m.sym.Healthy)+" detected")
		} else {
			lines = append(lines, "  "+m.theme.Disabled.Render(m.sym.Disabled)+" not configured")
		}
		lines = append(lines, fmt.Sprintf("  %d active", counts[h.Name]))
		if drift[h.Name] > 0 {
			lines = append(lines, "  "+m.theme.Modified.Render(fmt.Sprintf("%d drift", drift[h.Name])))
		} else {
			lines = append(lines, "  "+m.theme.Muted.Render("0 drift"))
		}
		lines = append(lines, "")
	}
	return m.panel("Harnesses", lines, m.width, height)
}

func (m Model) renderDiff(height int) string {
	title := m.diffTitle
	if title == "" {
		title = "Diff"
	}
	if m.diffText == "" {
		return m.panel(title, []string{m.theme.Muted.Render("No diff loaded.")}, m.width, height)
	}
	all := strings.Split(strings.TrimSuffix(m.diffText, "\n"), "\n")
	offset := clampIndex(m.doctorSel, maxInt(1, len(all)))
	maxRows := maxInt(1, height-2)
	if offset > maxInt(0, len(all)-maxRows) {
		offset = maxInt(0, len(all)-maxRows)
	}
	end := minInt(len(all), offset+maxRows)
	var lines []string
	for _, line := range all[offset:end] {
		lines = append(lines, colorDiffLine(m.theme, truncateText(line, maxInt(8, m.width-2))))
	}
	return m.panel(title, lines, m.width, height)
}

func (m Model) renderUpdate(height int) string {
	sk := m.selectedSkill()
	if sk == nil {
		return m.panel("Update", []string{"No skill selected."}, m.width, height)
	}
	lines := []string{m.theme.Heading.Render(sk.Name), ""}
	if sk.Source == nil || sk.Source.Type != "git" {
		lines = append(lines, m.theme.Warning.Render(m.sym.Warning)+" This skill is not backed by a Git source.", "", "No update will be attempted.")
		return m.panel("Update", lines, m.width, height)
	}
	source := sk.Source.Repository
	if sk.Source.Subdir != "" {
		source += "/" + sk.Source.Subdir
	}
	lines = append(lines,
		alignedField("Source", source, m.width-4, m.theme),
		alignedField("Current", shortHash(sk.Source.Commit), m.width-4, m.theme),
		"",
	)
	if sk.Modified {
		lines = append(lines, m.theme.Warning.Render(m.sym.Warning)+" Local modifications detected.", "Skillmux will refuse to overwrite them.")
	} else {
		lines = append(lines, m.theme.Key.Render("Enter")+" check and apply update")
	}
	return m.panel("Update", lines, m.width, height)
}

func (m Model) renderHelp(height int) string {
	lines := []string{
		m.theme.Heading.Render("Navigation"),
		"j / ↓          next",
		"k / ↑          previous",
		"Enter          select",
		"Esc            back",
		"",
		m.theme.Heading.Render("Skills"),
		"a              add",
		"space          enable / disable targets",
		"e              edit",
		"v              diff",
		"u              update",
		"x              disable exposures",
		"",
		m.theme.Heading.Render("Environment"),
		"s              sync",
		"d              doctor",
		"p              profiles",
		"h              harnesses",
		"c              conflicts",
		"",
		m.theme.Heading.Render("Global"),
		"/              search",
		"Ctrl+P / :     commands",
		"?              help",
		"q              quit",
	}
	return m.panel("Skillmux shortcuts", clampLines(lines, height-2), m.width, height)
}

func (m Model) renderConfirmDisable(height int) string {
	sk := m.selectedSkill()
	if sk == nil {
		return m.panel("Disable", []string{"No skill selected."}, m.width, height)
	}
	lines := []string{m.theme.Heading.Render("Disable " + sk.Name + " from all targets?"), "", "This updates desired exposure for:"}
	var targets []string
	for h, enabled := range sk.Enabled {
		if enabled {
			targets = append(targets, displayHarness(h))
		}
	}
	sort.Strings(targets)
	for _, h := range targets {
		lines = append(lines, "  "+h)
	}
	lines = append(lines, "", "The canonical Skillmux copy will remain.", "A sync is still required to modify harness directories.", "", m.theme.Key.Render("y")+" confirm    "+m.theme.Key.Render("n")+" cancel")
	return m.panel("Confirm", lines, m.width, height)
}

func (m Model) renderError(height int) string {
	lines := []string{m.theme.Error.Render(m.sym.Error) + " " + m.theme.Heading.Render("Operation failed"), "", truncateText(m.errorText, maxInt(10, m.width-4)), "", m.theme.Muted.Render("No success state was recorded."), "", m.theme.Key.Render("Esc")+" back"}
	return m.panel("Error", lines, m.width, height)
}

func (m Model) renderStatusLine() string {
	if m.busy {
		return fitStyled(" "+m.spinner()+" "+m.busyLabel, m.width)
	}
	if len(m.events) == 0 {
		return strings.Repeat(" ", maxInt(0, m.width))
	}
	ev := m.events[len(m.events)-1]
	sym, style := m.eventVisual(ev.Kind)
	line := " " + style.Render(sym) + " " + truncateText(ev.Text, maxInt(0, m.width-lipgloss.Width(sym)-4))
	return fitStyled(line, m.width)
}

func (m Model) renderFooter() string {
	if m.searchMode {
		q := truncateText(m.searchQuery, maxInt(0, m.width-10))
		return fitStyled(" Search: "+q+"█", m.width)
	}
	var items [][2]string
	switch m.view {
	case viewMain:
		items = [][2]string{{"/", "search"}, {"a", "add"}, {"s", "sync"}, {"d", "doctor"}, {"p", "profile"}, {"?", "help"}, {"q", "quit"}}
	case viewDetails:
		items = [][2]string{{"e", "edit"}, {"space", "targets"}, {"v", "diff"}, {"u", "update"}, {"x", "disable"}, {"Esc", "back"}}
	case viewAdd:
		if m.addFocus == 0 {
			items = [][2]string{{"Tab", "targets"}, {"Esc", "cancel"}}
		} else {
			items = [][2]string{{"space", "toggle"}, {"g", "scope"}, {"Enter", "install"}, {"Esc", "cancel"}}
		}
	case viewTargets:
		items = [][2]string{{"space", "toggle"}, {"Enter", "save"}, {"Esc", "cancel"}}
	case viewSync:
		items = [][2]string{{"Enter", "apply"}, {"d", "dry-run"}, {"Esc", "cancel"}}
	case viewDoctor:
		items = [][2]string{{"j/k", "navigate"}, {"Esc", "back"}}
	case viewConflicts:
		items = [][2]string{{"1-9", "use copy"}, {"m", "manual diff"}, {"Esc", "back"}}
	case viewProfiles:
		items = [][2]string{{"Enter", "preview"}, {"c", "snapshot"}, {"Esc", "back"}}
	case viewProfilePreview:
		items = [][2]string{{"Enter", "apply"}, {"Esc", "cancel"}}
	case viewHarnesses:
		items = [][2]string{{"j/k", "navigate"}, {"Esc", "back"}}
	case viewDiff:
		items = [][2]string{{"j/k", "scroll"}, {"e", "edit"}, {"Esc", "back"}}
	case viewUpdate:
		items = [][2]string{{"Enter", "update"}, {"Esc", "cancel"}}
	case viewHelp, viewError, viewSyncResult:
		items = [][2]string{{"Enter", "back"}, {"Esc", "back"}}
	case viewConfirmDisable:
		items = [][2]string{{"y", "confirm"}, {"n", "cancel"}}
	case viewPalette:
		return fitStyled(" Command: "+truncateText(m.paletteQuery, maxInt(0, m.width-11))+"█", m.width)
	}
	return m.shortcuts(items)
}

func (m Model) shortcuts(items [][2]string) string {
	var b strings.Builder
	b.WriteString(" ")
	for i, item := range items {
		if i > 0 {
			b.WriteString("   ")
		}
		b.WriteString(m.theme.Key.Render(item[0]))
		b.WriteString(" ")
		b.WriteString(item[1])
		if lipgloss.Width(b.String()) > m.width-2 {
			break
		}
	}
	return fitStyled(b.String(), m.width)
}

func (m Model) panel(title string, lines []string, width, height int) string {
	if width < 4 {
		return ""
	}
	if height < 3 {
		height = 3
	}
	inner := width - 2
	topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical := "┌", "┐", "└", "┘", "─", "│"
	if !supportsUnicode() {
		topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical = "+", "+", "+", "+", "-", "|"
	}

	titleText := ""
	if title != "" {
		titleText = " " + title + " "
	}
	fill := inner - lipgloss.Width(titleText)
	if fill < 0 {
		titleText = " " + truncateText(title, maxInt(0, inner-2)) + " "
		fill = inner - lipgloss.Width(titleText)
	}
	top := m.theme.Border.Render(topLeft+horizontal) + m.theme.Heading.Render(titleText) + m.theme.Border.Render(strings.Repeat(horizontal, maxInt(0, fill-1))+topRight)

	bodyRows := height - 2
	out := make([]string, 0, height)
	out = append(out, fitStyled(top, width))
	for i := 0; i < bodyRows; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		line = fitStyled(line, inner)
		padding := inner - lipgloss.Width(line)
		if padding < 0 {
			padding = 0
		}
		out = append(out, m.theme.Border.Render(vertical)+line+strings.Repeat(" ", padding)+m.theme.Border.Render(vertical))
	}
	out = append(out, m.theme.Border.Render(bottomLeft+strings.Repeat(horizontal, inner)+bottomRight))
	return strings.Join(out, "\n")
}

func (m Model) spinner() string {
	if len(m.sym.Spinner) == 0 {
		return ""
	}
	return m.theme.Info.Render(m.sym.Spinner[m.spin%len(m.sym.Spinner)])
}

func (m Model) skillMarker(sk core.Skill, conflict bool) (string, lipgloss.Style) {
	if conflict {
		return m.sym.Warning, m.theme.Warning
	}
	switch strings.ToLower(sk.Health) {
	case "error", "broken":
		return m.sym.Error, m.theme.Error
	case "warning":
		return m.sym.Warning, m.theme.Warning
	case "modified":
		return m.sym.Modified, m.theme.Modified
	}
	if sk.Modified {
		return m.sym.Modified, m.theme.Modified
	}
	if enabledCount(sk) == 0 {
		return m.sym.Disabled, m.theme.Disabled
	}
	return m.sym.Healthy, m.theme.Success
}

func stateLabel(sk core.Skill, conflict bool) string {
	if conflict {
		return "Conflict"
	}
	if sk.Modified {
		return "Modified"
	}
	switch strings.ToLower(sk.Health) {
	case "error", "broken":
		return "Broken"
	case "warning":
		return "Warning"
	}
	if enabledCount(sk) == 0 {
		return "Disabled"
	}
	return "Healthy"
}

func (m Model) conflictNames() map[string]bool {
	out := map[string]bool{}
	for _, c := range m.scan.Conflicts {
		out[c.Name] = true
	}
	return out
}

func (m Model) eventVisual(kind string) (string, lipgloss.Style) {
	switch kind {
	case "success":
		return m.sym.Healthy, m.theme.Success
	case "warning":
		return m.sym.Warning, m.theme.Warning
	case "error":
		return m.sym.Error, m.theme.Error
	default:
		return "·", m.theme.Info
	}
}

func (m Model) findingVisual(severity string) (string, lipgloss.Style) {
	if strings.EqualFold(severity, "error") {
		return m.sym.Error, m.theme.Error
	}
	return m.sym.Warning, m.theme.Warning
}

func (m Model) changeVisual(kind core.ChangeKind) (string, lipgloss.Style) {
	switch kind {
	case core.ChangeInstall:
		return "+", m.theme.Success
	case core.ChangeRemove:
		return "-", m.theme.Warning
	case core.ChangeUpdate:
		return "~", m.theme.Modified
	case core.ChangeRepair:
		return "~", m.theme.Warning
	default:
		return "~", m.theme.Info
	}
}

func (m Model) harnessSkillCounts() map[string]int {
	out := map[string]int{}
	for _, sk := range m.skills {
		for h, enabled := range sk.Enabled {
			if enabled {
				out[h]++
			}
		}
	}
	return out
}

func (m Model) harnessDriftCounts() map[string]int {
	out := map[string]int{}
	canonical := map[string]string{}
	for _, sk := range m.skills {
		canonical[sk.Name] = sk.Hash
	}
	for _, inst := range m.scan.Installations {
		if inst.Broken {
			out[inst.Harness]++
			continue
		}
		if expected, ok := canonical[inst.SkillName]; ok && inst.Hash != "" && expected != inst.Hash {
			out[inst.Harness]++
		}
	}
	return out
}

func planCounts(p core.Plan) (adds, updates, removals int) {
	for _, c := range p.Changes {
		switch c.Kind {
		case core.ChangeInstall:
			adds++
		case core.ChangeRemove:
			removals++
		default:
			updates++
		}
	}
	return
}

func doctorCounts(r core.DoctorReport) (warnings, errs int) {
	for _, f := range r.Findings {
		if strings.EqualFold(f.Severity, "error") {
			errs++
		} else {
			warnings++
		}
	}
	return
}

func alignedField(label, value string, width int, theme Theme) string {
	labelWidth := 9
	value = truncateText(value, maxInt(4, width-labelWidth))
	return theme.Muted.Render(fmt.Sprintf("%-*s", labelWidth, label)) + value
}

func conflictDiff(c core.Conflict) string {
	if len(c.Copies) < 2 {
		return "Only one usable copy is available."
	}
	aPath := filepath.Join(c.Copies[0].Path, "SKILL.md")
	bPath := filepath.Join(c.Copies[1].Path, "SKILL.md")
	aData, aErr := os.ReadFile(aPath)
	bData, bErr := os.ReadFile(bPath)
	if aErr != nil || bErr != nil {
		return "Unable to read both SKILL.md copies."
	}
	a := strings.Split(string(aData), "\n")
	b := strings.Split(string(bData), "\n")
	if string(aData) == string(bData) {
		return "No textual differences."
	}
	var out []string
	out = append(out, "--- "+displayHarness(c.Copies[0].Harness), "+++ "+displayHarness(c.Copies[1].Harness))
	maxLines := maxInt(len(a), len(b))
	for i := 0; i < maxLines; i++ {
		if i < len(a) && i < len(b) && a[i] == b[i] {
			continue
		}
		if i < len(a) {
			out = append(out, "- "+a[i])
		}
		if i < len(b) {
			out = append(out, "+ "+b[i])
		}
		if len(out) >= 18 {
			out = append(out, "… diff truncated")
			break
		}
	}
	return strings.Join(out, "\n")
}

func colorDiffLine(theme Theme, line string) string {
	if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
		return theme.Heading.Render(line)
	}
	if strings.HasPrefix(line, "+") {
		return theme.Success.Render(line)
	}
	if strings.HasPrefix(line, "-") {
		return theme.Error.Render(line)
	}
	return line
}

func projectLabel(root string) string {
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}
	if root == "" {
		return ""
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		if rel, err := filepath.Rel(home, root); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return "~" + string(os.PathSeparator) + rel
		}
		if root == home {
			return "~"
		}
	}
	return root
}

func shortHash(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func quoted(s string) string {
	return "“" + s + "”"
}

func listWindow(selected, count, capacity int) (int, int) {
	if capacity <= 0 || count <= 0 {
		return 0, 0
	}
	if capacity >= count {
		return 0, count
	}
	start := selected - capacity/2
	if start < 0 {
		start = 0
	}
	if start+capacity > count {
		start = count - capacity
	}
	return start, minInt(count, start+capacity)
}

func clampLines(lines []string, height int) []string {
	if height < 0 {
		return nil
	}
	if len(lines) > height {
		return lines[:height]
	}
	return lines
}

func truncateText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return string(runes[:1])
	}
	if supportsUnicode() {
		return string(runes[:width-1]) + "…"
	}
	return string(runes[:width-1]) + "~"
}

func fitPlain(s string, width int) string {
	return truncateText(s, width)
}

func fitStyled(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w <= width {
		return s + strings.Repeat(" ", width-w)
	}
	// Styled strings are constructed from already bounded fragments throughout
	// the UI. This fallback avoids wrapping if a terminal reports an unusual width.
	return s
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
