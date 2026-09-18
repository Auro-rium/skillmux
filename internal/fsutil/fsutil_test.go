package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashDirDeterministicAndSensitive(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root,"SKILL.md"), []byte("# Test\n"), 0o644); err != nil { t.Fatal(err) }
	a, err := HashDir(root)
	if err != nil { t.Fatal(err) }
	b, err := HashDir(root)
	if err != nil { t.Fatal(err) }
	if a != b { t.Fatalf("hash changed without content change: %s != %s",a,b) }
	if err := os.WriteFile(filepath.Join(root,"SKILL.md"), []byte("# Changed\n"), 0o644); err != nil { t.Fatal(err) }
	c, err := HashDir(root)
	if err != nil { t.Fatal(err) }
	if a == c { t.Fatal("hash did not change after content change") }
}

func TestSafeName(t *testing.T) {
	for _, ok := range []string{"backend-review","foo_bar","v1.2"} {
		if !SafeName(ok) { t.Fatalf("expected valid name: %s",ok) }
	}
	for _, bad := range []string{"","..","../x","a/b","a\\b","has space"} {
		if SafeName(bad) { t.Fatalf("expected invalid name: %s",bad) }
	}
}
