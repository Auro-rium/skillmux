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
			scope := st.Scopes[sk.Name]
			if scope == "" { scope = core.ScopeGlobal }
			if scope == core.ScopeProject && e.ProjectRoot == "" {
				return plan, fmt.Errorf("%s is project-scoped but no project root was found", sk.Name)
			}
			target, ok := locationForScope(h.SkillLocations(e.ProjectRoot), scope)
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
			} else if statErr == nil && (managedTarget(dst, sk.Path) || ownedTarget(st, sk.Name, harnessName, dst)) {
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

func locationForScope(locs []core.SkillLocation, scope core.Scope) (core.SkillLocation, bool) {
	for _, l := range locs {
		if l.Scope == scope { return l, true }
	}
	return core.SkillLocation{}, false
}

func ownedTarget(st core.State, skill, harnessName, target string) bool {
	if st.ManagedTargets[skill] == nil { return false }
	recorded := st.ManagedTargets[skill][harnessName]
	if recorded == "" { return false }
	a, _ := filepath.Abs(recorded)
	b, _ := filepath.Abs(target)
	return a == b
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
		linkTarget string
		existed bool
	}
	var done []applied
	rollback := func(cause error) error {
		for i := len(done)-1; i >= 0; i-- {
			a := done[i]
			_ = os.RemoveAll(a.change.Target)
			if a.existed {
				if a.linkTarget != "" {
					_ = os.Symlink(a.linkTarget, a.change.Target)
				} else if a.backup != "" {
					_ = fsutil.CopyDir(a.backup, a.change.Target)
				}
			}
		}
		return fmt.Errorf("%w; changes rolled back", cause)
	}
	for _, c := range plan.Changes {
		if (c.Kind == core.ChangeUpdate || c.Kind == core.ChangeRepair) && c.Reason != "" && !force {
			return rollback(fmt.Errorf("refusing to replace %s: %s; rerun with --force after reviewing the diff", c.Target, c.Reason))
		}
		a := applied{change:c}
		if info, err := os.Lstat(c.Target); err == nil {
			a.existed = true
			if info.Mode()&os.ModeSymlink != 0 {
				link, readErr := os.Readlink(c.Target)
				if readErr != nil { return rollback(fmt.Errorf("read symlink %s: %w", c.Target, readErr)) }
				a.linkTarget = link
			} else {
				a.backup = filepath.Join(backupRoot, c.Harness, c.Skill)
				if err := fsutil.CopyDir(c.Target, a.backup); err != nil { return rollback(fmt.Errorf("backup %s: %w", c.Target, err)) }
			}
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
	st, err := e.Store.LoadState()
	if err != nil { return rollback(fmt.Errorf("load state after sync: %w", err)) }
	for _, a := range done {
		if st.ManagedTargets[a.change.Skill] == nil {
			st.ManagedTargets[a.change.Skill] = map[string]string{}
		}
		if a.change.Kind == core.ChangeRemove {
			delete(st.ManagedTargets[a.change.Skill], a.change.Harness)
			if len(st.ManagedTargets[a.change.Skill]) == 0 {
				delete(st.ManagedTargets, a.change.Skill)
			}
		} else {
			st.ManagedTargets[a.change.Skill][a.change.Harness] = filepath.Clean(a.change.Target)
		}
	}
	if err := e.Store.SaveState(st); err != nil {
		return rollback(fmt.Errorf("record managed targets: %w", err))
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
			scope := sk.Scope
			if scope == "" { scope = core.ScopeGlobal }
			loc, ok := locationForScope(h.SkillLocations(e.ProjectRoot), scope)
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
