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

func TestHashAndCopyIgnoreGitMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "objects"), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# Test\n"), 0o644); err != nil { t.Fatal(err) }
	a, err := HashDir(root)
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, ".git", "objects", "ignored"), []byte("noise"), 0o644); err != nil { t.Fatal(err) }
	b, err := HashDir(root)
	if err != nil { t.Fatal(err) }
	if a != b { t.Fatal("git metadata must not affect skill hash") }

	dst := filepath.Join(t.TempDir(), "copy")
	if err := CopyDir(root, dst); err != nil { t.Fatal(err) }
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git should not be copied, err=%v", err)
	}
}

func TestValidateTreeRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# Test\n"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("secret"), 0o644); err != nil { t.Fatal(err) }
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := ValidateTree(root); err == nil {
		t.Fatal("expected escaping symlink to be rejected")
	}
}

func TestValidateTreeAllowsInternalSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# Test\n"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("notes"), 0o644); err != nil { t.Fatal(err) }
	if err := os.Symlink("notes.md", filepath.Join(root, "linked-notes.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := ValidateTree(root); err != nil {
		t.Fatalf("internal symlink should be allowed: %v", err)
	}
}
