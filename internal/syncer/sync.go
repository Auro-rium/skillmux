package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/fsutil"
	"github.com/Auro-rium/skillmux/internal/harness"
	"github.com/Auro-rium/skillmux/internal/store"
)

type Engine struct {
	Store       *store.Store
	Harnesses   []harness.Adapter
	ProjectRoot string
	PreferLinks bool
}

func New(s *store.Store, projectRoot string) *Engine {
	return &Engine{Store:s, Harnesses:harness.Default(), ProjectRoot:projectRoot, PreferLinks:true}
}

func (e *Engine) Plan() (core.Plan, error) {
	skills, err := e.Store.ListSkills()
	if err != nil { return core.Plan{}, err }
	st, err := e.Store.LoadState()
	if err != nil { return core.Plan{}, err }
	adapters := map[string]harness.Adapter{}
	for _, h := range e.Harnesses { adapters[h.Name()] = h }
	var plan core.Plan
	for _, sk := range skills {
		for harnessName, enabled := range st.Enabled[sk.Name] {
			h, ok := adapters[harnessName]
			if !ok { continue }
			target, ok := preferredLocation(h.SkillLocations(e.ProjectRoot))
			if !ok { continue }
			dst := filepath.Join(target.Path, sk.Name)
			info, statErr := os.Lstat(dst)
			if enabled {
				if os.IsNotExist(statErr) {
					plan.Changes = append(plan.Changes, core.Change{Kind:core.ChangeInstall, Skill:sk.Name, Harness:harnessName, Source:sk.Path, Target:dst})
					continue
				}
				if statErr != nil { return plan, statErr }
				if info.Mode()&os.ModeSymlink != 0 {
					if resolved, err := filepath.EvalSymlinks(dst); err == nil {
						a, _ := filepath.Abs(resolved)
						b, _ := filepath.Abs(sk.Path)
						if a == b { continue }
					}
				}
				hash, err := fsutil.HashDir(dst)
				if err != nil {
					plan.Changes = append(plan.Changes, core.Change{Kind:core.ChangeRepair, Skill:sk.Name, Harness:harnessName, Source:sk.Path, Target:dst, Reason:"target is unreadable or a broken link"})
				} else if hash != sk.Hash {
					plan.Changes = append(plan.Changes, core.Change{Kind:core.ChangeUpdate, Skill:sk.Name, Harness:harnessName, Source:sk.Path, Target:dst, Reason:"target differs from canonical"})
				}
			} else if statErr == nil && managedTarget(dst, sk.Path) {
				plan.Changes = append(plan.Changes, core.Change{Kind:core.ChangeRemove, Skill:sk.Name, Harness:harnessName, Source:sk.Path, Target:dst, Reason:"disabled for target"})
			}
		}
	}
	sort.Slice(plan.Changes, func(i,j int) bool {
		if plan.Changes[i].Harness == plan.Changes[j].Harness { return plan.Changes[i].Skill < plan.Changes[j].Skill }
		return plan.Changes[i].Harness < plan.Changes[j].Harness
	})
	return plan, nil
}

func preferredLocation(locs []core.SkillLocation) (core.SkillLocation, bool) {
	for _, l := range locs { if l.Scope == core.ScopeGlobal { return l, true } }
	if len(locs) > 0 { return locs[0], true }
	return core.SkillLocation{}, false
}

func managedTarget(target, canonical string) bool {
	info, err := os.Lstat(target)
	if err != nil || info.Mode()&os.ModeSymlink == 0 { return false }
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil { return false }
	a, _ := filepath.Abs(resolved)
	b, _ := filepath.Abs(canonical)
	return a == b
}

func (e *Engine) Apply(plan core.Plan, force bool) error {
	if len(plan.Changes) == 0 { return nil }
	timestamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	backupRoot := filepath.Join(e.Store.BackupsDir(), timestamp)
	type applied struct {
		change core.Change
		backup string
		existed bool
	}
	var done []applied
	rollback := func(cause error) error {
		for i := len(done)-1; i >= 0; i-- {
			a := done[i]
			_ = os.RemoveAll(a.change.Target)
			if a.existed && a.backup != "" { _ = fsutil.CopyDir(a.backup, a.change.Target) }
		}
		return fmt.Errorf("%w; changes rolled back", cause)
	}
	for _, c := range plan.Changes {
		if (c.Kind == core.ChangeUpdate || c.Kind == core.ChangeRepair) && c.Reason != "" && !force {
			return rollback(fmt.Errorf("refusing to replace %s: %s; rerun with --force after reviewing the diff", c.Target, c.Reason))
		}
		a := applied{change:c}
		if _, err := os.Lstat(c.Target); err == nil {
			a.existed = true
			a.backup = filepath.Join(backupRoot, c.Harness, c.Skill)
			if err := fsutil.CopyDir(c.Target, a.backup); err != nil { return rollback(fmt.Errorf("backup %s: %w", c.Target, err)) }
		}
		switch c.Kind {
		case core.ChangeRemove:
			if err := os.RemoveAll(c.Target); err != nil { return rollback(err) }
		default:
			if err := os.MkdirAll(filepath.Dir(c.Target), 0o755); err != nil { return rollback(err) }
			_ = os.RemoveAll(c.Target)
			if e.PreferLinks && runtime.GOOS != "windows" {
				if err := os.Symlink(c.Source, c.Target); err != nil {
					if err := fsutil.CopyDir(c.Source, c.Target); err != nil { return rollback(err) }
				}
			} else if err := fsutil.CopyDir(c.Source, c.Target); err != nil { return rollback(err) }
		}
		done = append(done, a)
	}
	return nil
}

func (e *Engine) Eject(skill string) error {
	skills, err := e.Store.ListSkills()
	if err != nil { return err }
	found := false
	for _, sk := range skills {
		if skill != "" && sk.Name != skill { continue }
		found = true
		for _, h := range e.Harnesses {
			loc, ok := preferredLocation(h.SkillLocations(e.ProjectRoot))
			if !ok { continue }
			dst := filepath.Join(loc.Path, sk.Name)
			if !managedTarget(dst, sk.Path) { continue }
			tmp := dst + ".skillmux-eject"
			_ = os.RemoveAll(tmp)
			if err := fsutil.CopyDir(sk.Path, tmp); err != nil { return err }
			if err := os.Remove(dst); err != nil { return err }
			if err := os.Rename(tmp, dst); err != nil { return err }
		}
	}
	if skill != "" && !found { return errors.New("skill not found") }
	return nil
}
