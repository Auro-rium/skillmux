package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/harness"
	"github.com/Auro-rium/skillmux/internal/store"
)

type fakeHarness struct{ name, dir string }
func (f fakeHarness) Name() string { return f.name }
func (f fakeHarness) Detect() bool { return true }
func (f fakeHarness) SkillLocations(string) []core.SkillLocation {
	return []core.SkillLocation{{Path:f.dir,Scope:core.ScopeGlobal}}
}
func (f fakeHarness) SupportsSymlinks() bool { return true }

var _ harness.Adapter = fakeHarness{}

func TestDetectsConflict(t *testing.T) {
	root:=t.TempDir()
	s,err:=store.Open(filepath.Join(root,"state"))
	if err!=nil { t.Fatal(err) }
	a:=filepath.Join(root,"a"); b:=filepath.Join(root,"b")
	writeSkill(t,filepath.Join(a,"same"),"one")
	writeSkill(t,filepath.Join(b,"same"),"two")
	sc:=&Scanner{Store:s,Harnesses:[]harness.Adapter{fakeHarness{"a",a},fakeHarness{"b",b}}}
	r,err:=sc.Run()
	if err!=nil { t.Fatal(err) }
	if r.UniqueSkills!=1 || r.Duplicates!=1 || len(r.Conflicts)!=1 {
		t.Fatalf("unexpected report: %+v",r)
	}
}

func writeSkill(t *testing.T, dir, body string) {
	t.Helper()
	if err:=os.MkdirAll(dir,0o755); err!=nil { t.Fatal(err) }
	if err:=os.WriteFile(filepath.Join(dir,"SKILL.md"),[]byte(body),0o644); err!=nil { t.Fatal(err) }
}
