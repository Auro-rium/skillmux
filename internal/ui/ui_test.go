package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/Auro-rium/skillmux/internal/app"
	"github.com/Auro-rium/skillmux/internal/core"
)

func fixtureModel(width, height int) Model {
	skill := core.Skill{
		Name:        "backend-review",
		Description: "Backend architecture review and API correctness.",
		Health:      "healthy",
		Scope:       core.ScopeProject,
		Enabled: map[string]bool{
			"codex":  true,
			"claude": true,
			"gemini": false,
			"cursor": true,
		},
	}
	m := Model{
		app:     &app.App{Root: "/tmp/acme-api"},
		theme:   NewTheme("mono"),
		sym:     Symbols{Healthy: "✓", Warning: "⚠", Error: "✗", Modified: "~", Update: "↓", Disabled: "○", Cursor: ">", Dot: "●", Spinner: []string{"-"}},
		width:   width,
		height:  height,
		view:    viewMain,
		skills:  []core.Skill{skill},
		filtered: []int{0},
		profile: "backend",
		scan: core.ScanReport{Harnesses: []core.HarnessInfo{
			{Name: "codex", Detected: true},
			{Name: "claude", Detected: true},
			{Name: "gemini", Detected: true},
			{Name: "cursor", Detected: true},
			{Name: "opencode", Detected: false},
		}},
	}
	return m
}

func TestFuzzyMatchSubsequenceAndMiss(t *testing.T) {
	res, ok := fuzzyMatch("postgres-debug", "pgdbg")
	if !ok || len(res.Positions) != 5 {
		t.Fatalf("expected fuzzy match, got ok=%v result=%+v", ok, res)
	}
	if _, ok := fuzzyMatch("backend-review", "zzzz"); ok {
		t.Fatal("unexpected fuzzy match")
	}
}

func TestResponsiveLayouts(t *testing.T) {
	small := fixtureModel(80, 24).View()
	if !strings.Contains(small, "Skills") || !strings.Contains(small, "backend-review") {
		t.Fatalf("small layout missing skill list:\n%s", small)
	}
	if strings.Contains(small, "┌─ Details") {
		t.Fatalf("80-column layout should collapse details panel:\n%s", small)
	}

	medium := fixtureModel(120, 30).View()
	if !strings.Contains(medium, "Details") {
		t.Fatalf("medium layout should show details:\n%s", medium)
	}

	large := fixtureModel(160, 50).View()
	if !strings.Contains(large, "Activity") {
		t.Fatalf("large layout should show activity:\n%s", large)
	}
}

func TestRenderNeverExceedsConfiguredWidth(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 30}, {160, 50}} {
		m := fixtureModel(size[0], size[1])
		for i, line := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(line); got > size[0] {
				t.Fatalf("%dx%d line %d width=%d exceeds terminal width:\n%s", size[0], size[1], i, got, line)
			}
		}
	}
}

func TestSemanticStateLabels(t *testing.T) {
	sk := core.Skill{Health: "healthy", Enabled: map[string]bool{"codex": true}}
	if got := stateLabel(sk, false); got != "Healthy" {
		t.Fatalf("got %q", got)
	}
	sk.Modified = true
	if got := stateLabel(sk, false); got != "Modified" {
		t.Fatalf("got %q", got)
	}
	if got := stateLabel(sk, true); got != "Conflict" {
		t.Fatalf("got %q", got)
	}
}
