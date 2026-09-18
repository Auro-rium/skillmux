package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdoptAndEnable(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(),"skillmux"))
	if err != nil { t.Fatal(err) }
	src := filepath.Join(t.TempDir(),"backend-review")
	if err := os.MkdirAll(src,0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(src,"SKILL.md"),[]byte("---\ndescription: Reviews backend code\n---\n# Backend\n"),0o644); err != nil { t.Fatal(err) }
	if err := s.Adopt("backend-review",src,nil); err != nil { t.Fatal(err) }
	if err := s.SetEnabled("backend-review","codex",true); err != nil { t.Fatal(err) }
	skills, err := s.ListSkills()
	if err != nil { t.Fatal(err) }
	if len(skills)!=1 { t.Fatalf("got %d skills",len(skills)) }
	if skills[0].Description!="Reviews backend code" { t.Fatalf("description=%q",skills[0].Description) }
	if !skills[0].Enabled["codex"] { t.Fatal("codex should be enabled") }
}
