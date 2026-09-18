package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/fsutil"
)

// ResolveConflict promotes one discovered harness copy to the canonical Skillmux
// copy. It only accepts a path that is present in the current scan report and
// keeps a filesystem backup of any existing canonical copy before replacement.
func (a *App) ResolveConflict(name, harnessName string) error {
	report, err := a.Scan()
	if err != nil {
		return err
	}

	var conflict *core.Conflict
	for i := range report.Conflicts {
		if report.Conflicts[i].Name == name {
			conflict = &report.Conflicts[i]
			break
		}
	}
	if conflict == nil {
		return errors.New("conflict not found: " + name)
	}

	var chosen core.Installation
	for _, cp := range conflict.Copies {
		if cp.Harness == harnessName && !cp.Broken {
			chosen = cp
			break
		}
	}
	if chosen.Path == "" {
		return fmt.Errorf("no usable %s copy found for %s", harnessName, name)
	}
	if _, err := os.Stat(filepath.Join(chosen.Path, "SKILL.md")); err != nil {
		return fmt.Errorf("selected copy is missing SKILL.md: %w", err)
	}

	dst := a.Store.SkillPath(name)
	stage := dst + ".skillmux-resolve"
	_ = os.RemoveAll(stage)
	if err := fsutil.CopyDir(chosen.Path, stage); err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	backup := ""
	if _, err := os.Lstat(dst); err == nil {
		backup = filepath.Join(
			a.Store.BackupsDir(),
			time.Now().UTC().Format("20060102T150405.000000000Z"),
			"conflict",
			name,
		)
		if err := fsutil.CopyDir(dst, backup); err != nil {
			return err
		}
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}

	if err := os.Rename(stage, dst); err != nil {
		if backup != "" {
			_ = fsutil.CopyDir(backup, dst)
		}
		return err
	}

	hash, err := fsutil.HashDir(dst)
	if err != nil {
		if backup != "" {
			_ = os.RemoveAll(dst)
			_ = fsutil.CopyDir(backup, dst)
		}
		return err
	}

	state, err := a.Store.LoadState()
	if err != nil {
		return err
	}
	if state.Enabled[name] == nil {
		state.Enabled[name] = map[string]bool{}
	}
	for _, cp := range conflict.Copies {
		state.Enabled[name][cp.Harness] = true
	}
	delete(state.Sources, name)
	if state.Scopes[name] == "" {
		state.Scopes[name] = core.ScopeGlobal
	}
	_ = hash // Hash is recomputed by Store.ListSkills; state deliberately keeps no external provenance.

	if err := a.Store.SaveState(state); err != nil {
		if backup != "" {
			_ = os.RemoveAll(dst)
			_ = fsutil.CopyDir(backup, dst)
		}
		return err
	}
	return nil
}
