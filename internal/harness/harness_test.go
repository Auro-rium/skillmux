package harness

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Auro-rium/skillmux/internal/core"
)

func TestCodexUsesAgentsSkillDirectory(t *testing.T) {
	root := t.TempDir()
	var codex Adapter
	for _, h := range Default() {
		if h.Name() == "codex" {
			codex = h
			break
		}
	}
	if codex == nil {
		t.Fatal("codex adapter missing")
	}
	locs := codex.SkillLocations(root)
	wantProject := filepath.Join(root, ".agents", "skills")
	found := false
	for _, loc := range locs {
		if strings.Contains(filepath.ToSlash(loc.Path), ".codex/skills") {
			t.Fatalf("codex adapter must not expose deprecated .codex/skills path: %s", loc.Path)
		}
		if loc.Scope == core.ScopeProject && loc.Path == wantProject {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing Codex project skill path %s; got %+v", wantProject, locs)
	}
}
