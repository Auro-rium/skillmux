package syncer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/harness"
	"github.com/Auro-rium/skillmux/internal/store"
)

type fakeHarness struct{ dir string }
func (f fakeHarness) Name() string { return "codex" }
func (f fakeHarness) Detect() bool { return true }
func (f fakeHarness) SkillLocations(string) []core.SkillLocation {
	return []core.SkillLocation{{Path:f.dir,Scope:core.ScopeGlobal}}
}
func (f fakeHarness) SupportsSymlinks() bool { return true }

var _ harness.Adapter = fakeHarness{}

func TestSyncIsIdempotent(t *testing.T) {
	root:=t.TempDir()
	s,err:=store.Open(filepath.Join(root,"state"))
	if err!=nil { t.Fatal(err) }
	src:=filepath.Join(root,"src")
	if err:=os.MkdirAll(src,0o755); err!=nil { t.Fatal(err) }
	if err:=os.WriteFile(filepath.Join(src,"SKILL.md"),[]byte("# Test\n"),0o644); err!=nil { t.Fatal(err) }
	if err:=s.Adopt("test",src,nil); err!=nil { t.Fatal(err) }
	if err:=s.SetEnabled("test","codex",true); err!=nil { t.Fatal(err) }
	targetRoot:=filepath.Join(root,"codex")
	e:=New(s,"")
	e.Harnesses=[]harness.Adapter{fakeHarness{targetRoot}}
	first,err:=e.Plan()
	if err!=nil { t.Fatal(err) }
	if len(first.Changes)!=1 || first.Changes[0].Kind!=core.ChangeInstall { t.Fatalf("first plan=%+v",first) }
	if err:=e.Apply(first,false); err!=nil { t.Fatal(err) }
	second,err:=e.Plan()
	if err!=nil { t.Fatal(err) }
	if len(second.Changes)!=0 { t.Fatalf("second plan should be empty: %+v",second) }
}

type scopedHarness struct{ global, project string }
func (f scopedHarness) Name() string { return "claude" }
func (f scopedHarness) Detect() bool { return true }
func (f scopedHarness) SkillLocations(string) []core.SkillLocation {
	return []core.SkillLocation{
		{Path:f.global, Scope:core.ScopeGlobal},
		{Path:f.project, Scope:core.ScopeProject},
	}
}
func (f scopedHarness) SupportsSymlinks() bool { return true }

func TestProjectScopedSkillPlansProjectLocation(t *testing.T) {
	root := t.TempDir()
	s, err := store.Open(filepath.Join(root, "state"))
	if err != nil { t.Fatal(err) }
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Project\n"), 0o644); err != nil { t.Fatal(err) }
	if err := s.Adopt("project-skill", src, nil); err != nil { t.Fatal(err) }
	if err := s.SetScope("project-skill", core.ScopeProject); err != nil { t.Fatal(err) }
	if err := s.SetEnabled("project-skill", "claude", true); err != nil { t.Fatal(err) }

	global := filepath.Join(root, "global")
	projectDir := filepath.Join(root, "repo", ".claude", "skills")
	e := New(s, filepath.Join(root, "repo"))
	e.Harnesses = []harness.Adapter{scopedHarness{global:global, project:projectDir}}
	plan, err := e.Plan()
	if err != nil { t.Fatal(err) }
	if len(plan.Changes) != 1 {
		t.Fatalf("expected one change, got %+v", plan)
	}
	want := filepath.Join(projectDir, "project-skill")
	if plan.Changes[0].Target != want {
		t.Fatalf("target=%q want=%q", plan.Changes[0].Target, want)
	}
}

func TestCopiedManagedTargetCanBeSafelyDisabled(t *testing.T) {
	root := t.TempDir()
	s, err := store.Open(filepath.Join(root, "state"))
	if err != nil { t.Fatal(err) }
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Copied\n"), 0o644); err != nil { t.Fatal(err) }
	if err := s.Adopt("copied", src, nil); err != nil { t.Fatal(err) }
	if err := s.SetEnabled("copied", "codex", true); err != nil { t.Fatal(err) }

	targetRoot := filepath.Join(root, "codex")
	e := New(s, "")
	e.Harnesses = []harness.Adapter{fakeHarness{targetRoot}}
	e.PreferLinks = false

	plan, err := e.Plan()
	if err != nil { t.Fatal(err) }
	if err := e.Apply(plan, false); err != nil { t.Fatal(err) }

	target := filepath.Join(targetRoot, "copied")
	info, err := os.Lstat(target)
	if err != nil { t.Fatal(err) }
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("expected regular copied directory")
	}

	if err := s.SetEnabled("copied", "codex", false); err != nil { t.Fatal(err) }
	removePlan, err := e.Plan()
	if err != nil { t.Fatal(err) }
	if len(removePlan.Changes) != 1 || removePlan.Changes[0].Kind != core.ChangeRemove {
		t.Fatalf("expected managed copy removal, got %+v", removePlan)
	}
	if err := e.Apply(removePlan, false); err != nil { t.Fatal(err) }
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected copied target removed, err=%v", err)
	}
}

func TestIdenticalUnmanagedCopyIsAdoptedOnForcedSync(t *testing.T) {
	root := t.TempDir()
	s, err := store.Open(filepath.Join(root, "state"))
	if err != nil { t.Fatal(err) }
	src := filepath.Join(root, "src")
	targetRoot := filepath.Join(root, "codex")
	if err := os.MkdirAll(src, 0o755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(targetRoot, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Same\n"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(targetRoot, "imported", "SKILL.md"), []byte("# Same\n"), 0o644); err != nil {
		_ = os.MkdirAll(filepath.Join(targetRoot, "imported"), 0o755)
		if err := os.WriteFile(filepath.Join(targetRoot, "imported", "SKILL.md"), []byte("# Same\n"), 0o644); err != nil { t.Fatal(err) }
	}
	if err := s.Adopt("imported", src, nil); err != nil { t.Fatal(err) }
	if err := s.SetEnabled("imported", "codex", true); err != nil { t.Fatal(err) }
	e := New(s, "")
	e.Harnesses = []harness.Adapter{fakeHarness{targetRoot}}
	plan, err := e.Plan()
	if err != nil { t.Fatal(err) }
	if len(plan.Changes) != 1 || plan.Changes[0].Kind != core.ChangeRepair {
		t.Fatalf("expected unmanaged identical copy repair, got %+v", plan)
	}
	if err := e.Apply(plan, false); err == nil {
		t.Fatal("repair of an unmanaged target must require force")
	}
	if err := e.Apply(plan, true); err != nil { t.Fatal(err) }
	info, err := os.Lstat(filepath.Join(targetRoot, "imported"))
	if err != nil { t.Fatal(err) }
	if info.Mode()&os.ModeSymlink == 0 { t.Fatal("expected target to become a symlink") }
}

func TestEjectClearsManagementAndDesiredExposure(t *testing.T) {
	root := t.TempDir()
	s, err := store.Open(filepath.Join(root, "state"))
	if err != nil { t.Fatal(err) }
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Eject\n"), 0o644); err != nil { t.Fatal(err) }
	if err := s.Adopt("eject", src, nil); err != nil { t.Fatal(err) }
	if err := s.SetEnabled("eject", "codex", true); err != nil { t.Fatal(err) }
	targetRoot := filepath.Join(root, "codex")
	e := New(s, "")
	e.Harnesses = []harness.Adapter{fakeHarness{targetRoot}}
	plan, err := e.Plan()
	if err != nil { t.Fatal(err) }
	if err := e.Apply(plan, false); err != nil { t.Fatal(err) }
	if err := e.Eject("eject"); err != nil { t.Fatal(err) }
	st, err := s.LoadState()
	if err != nil { t.Fatal(err) }
	if st.Enabled["eject"]["codex"] {
		t.Fatal("eject must clear desired exposure")
	}
	if _, ok := st.ManagedTargets["eject"]; ok {
		t.Fatal("eject must clear managed target state")
	}
	post, err := e.Plan()
	if err != nil { t.Fatal(err) }
	if len(post.Changes) != 0 { t.Fatalf("ejected target must remain unmanaged: %+v", post) }
}
