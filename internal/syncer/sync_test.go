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
