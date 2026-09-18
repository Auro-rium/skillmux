package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/harness"
	"github.com/Auro-rium/skillmux/internal/scan"
	"github.com/Auro-rium/skillmux/internal/store"
	"github.com/Auro-rium/skillmux/internal/syncer"
)

type importHarness struct{ dir string }

func (h importHarness) Name() string { return "codex" }
func (h importHarness) Detect() bool { return true }
func (h importHarness) SkillLocations(string) []core.SkillLocation {
	return []core.SkillLocation{{Path: h.dir, Scope: core.ScopeGlobal}}
}
func (h importHarness) SupportsSymlinks() bool { return true }

func TestImportCanonicalizesExistingCopy(t *testing.T) {
	root := t.TempDir()
	s, err := store.Open(filepath.Join(root, "state"))
	if err != nil { t.Fatal(err) }

	harnessRoot := filepath.Join(root, "codex", "skills")
	if err := os.MkdirAll(filepath.Join(harnessRoot, "existing"), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(harnessRoot, "existing", "SKILL.md"), []byte("# Existing\n"), 0o644); err != nil { t.Fatal(err) }

	h := importHarness{dir: harnessRoot}
	a := &App{
		Store: s,
		Scanner: scan.New(s, ""),
		Syncer: syncer.New(s, ""),
		Root: "",
	}
	a.Scanner.Harnesses = []harness.Adapter{h}
	a.Syncer.Harnesses = []harness.Adapter{h}

	result, err := a.Import("", "")
	if err != nil { t.Fatal(err) }
	if len(result.Imported) != 1 || result.Imported[0] != "existing" {
		t.Fatalf("unexpected import result: %+v", result)
	}
	info, err := os.Lstat(filepath.Join(harnessRoot, "existing"))
	if err != nil { t.Fatal(err) }
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("imported target should be canonicalized to a symlink")
	}
}
