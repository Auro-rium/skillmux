package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/fsutil"
	"github.com/Auro-rium/skillmux/internal/harness"
	"github.com/Auro-rium/skillmux/internal/project"
	"github.com/Auro-rium/skillmux/internal/source"
)

func (a *App) ProjectSync(ctx context.Context, dryRun, force bool) (core.Plan, error) {
	manifest, err := project.LoadManifest(a.Root)
	if err != nil {
		return core.Plan{}, err
	}
	if manifest == nil {
		return a.Sync(dryRun, force)
	}

	if dryRun {
		base, err := a.Sync(true, force)
		if err != nil {
			return base, err
		}
		extra, err := a.projectPlan(manifest)
		if err != nil {
			return base, err
		}
		return mergePlans(base, extra), nil
	}

	if err := a.applyProjectManifest(ctx, manifest); err != nil {
		return core.Plan{}, err
	}
	plan, err := a.Syncer.Plan()
	if err != nil {
		return plan, err
	}
	if err := a.Syncer.Apply(plan, force); err != nil {
		return plan, err
	}
	skills, err := a.List()
	if err != nil {
		return plan, err
	}
	if err := project.WriteLock(a.Root, project.LockFrom(manifest, skills)); err != nil {
		return plan, err
	}
	return plan, nil
}

func (a *App) projectPlan(manifest *project.Manifest) (core.Plan, error) {
	skills, err := a.List()
	if err != nil {
		return core.Plan{}, err
	}
	lock, err := project.LoadLock(a.Root)
	if err != nil {
		return core.Plan{}, err
	}
	byName := map[string]core.Skill{}
	for _, sk := range skills {
		byName[sk.Name] = sk
	}

	adapters := map[string]harness.Adapter{}
	for _, h := range harness.Default() {
		adapters[h.Name()] = h
	}

	var plan core.Plan
	names := make([]string, 0, len(manifest.Skills))
	for name := range manifest.Skills {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		requested := manifest.Skills[name]
		sk, exists := byName[name]
		needsFetch := !exists
		reason := "manifest skill is not in the canonical store"
		if exists {
			if locked, ok := lock.ByName(name); ok && locked.Commit != "" && sk.Source != nil && sk.Source.Commit != locked.Commit {
				needsFetch = true
				reason = "canonical revision differs from project lockfile"
			}
		}
		if !needsFetch {
			continue
		}
		added := false
		for target, enabled := range manifest.Targets {
			if !enabled {
				continue
			}
			h, ok := adapters[target]
			if !ok {
				return plan, fmt.Errorf("manifest references unsupported harness %q", target)
			}
			dst := targetPath(h, a.Root, name, core.ScopeProject)
			kind := core.ChangeInstall
			if exists {
				kind = core.ChangeUpdate
			}
			plan.Changes = append(plan.Changes, core.Change{
				Kind: kind, Skill: name, Harness: target, Source: requested, Target: dst, Reason: reason,
			})
			added = true
		}
		if !added {
			kind := core.ChangeInstall
			if exists {
				kind = core.ChangeUpdate
			}
			plan.Changes = append(plan.Changes, core.Change{
				Kind: kind, Skill: name, Harness: "skillmux", Source: requested, Target: a.Store.SkillPath(name), Reason: reason,
			})
		}
	}
	return plan, nil
}

func (a *App) applyProjectManifest(ctx context.Context, manifest *project.Manifest) error {
	lock, err := project.LoadLock(a.Root)
	if err != nil {
		return err
	}
	skills, err := a.List()
	if err != nil {
		return err
	}
	byName := map[string]core.Skill{}
	for _, sk := range skills {
		byName[sk.Name] = sk
	}

	names := make([]string, 0, len(manifest.Skills))
	for name := range manifest.Skills {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		requested := manifest.Skills[name]
		resolvedRef := requested
		if locked, ok := lock.ByName(name); ok && locked.Commit != "" {
			resolvedRef = pinGitHubSource(requested, locked.Commit)
		}

		sk, exists := byName[name]
		needInstall := !exists
		if exists {
			if locked, ok := lock.ByName(name); ok && locked.Commit != "" {
				if sk.Source == nil || sk.Source.Commit != locked.Commit {
					if sk.Modified {
						return fmt.Errorf("%s has local modifications and cannot be restored to lockfile commit %s", name, locked.Commit)
					}
					needInstall = true
				}
			}
		}

		if needInstall {
			res, err := source.Resolve(ctx, resolvedRef, a.Store.CacheDir())
			if err != nil {
				return fmt.Errorf("resolve %s: %w", name, err)
			}
			err = func() error {
				if res.Cleanup != nil {
					defer res.Cleanup()
				}
				if exists {
					return a.replaceCanonical(name, res.Path, res.Source)
				}
				src := res.Source
				return a.Store.Adopt(name, res.Path, &src)
			}()
			if err != nil {
				return err
			}
		}

		if err := a.Store.SetScope(name, core.ScopeProject); err != nil {
			return err
		}

		for _, h := range harness.Default() {
			if enabled, ok := manifest.Targets[h.Name()]; ok {
				if err := a.Store.SetEnabled(name, h.Name(), enabled); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (a *App) replaceCanonical(name, srcPath string, src core.Source) error {
	dst := a.Store.SkillPath(name)
	backup := filepath.Join(a.Store.BackupsDir(), time.Now().UTC().Format("20060102T150405.000000000Z"), "project", name)
	if err := fsutil.CopyDir(dst, backup); err != nil {
		return fmt.Errorf("backup canonical %s: %w", name, err)
	}
	tmp := dst + ".skillmux-project"
	_ = os.RemoveAll(tmp)
	if err := fsutil.CopyDir(srcPath, tmp); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(tmp, "SKILL.md")); err != nil {
		_ = os.RemoveAll(tmp)
		return errors.New("replacement source does not contain SKILL.md")
	}
	if err := os.RemoveAll(dst); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = fsutil.CopyDir(backup, dst)
		return err
	}
	hash, err := fsutil.HashDir(dst)
	if err != nil {
		return err
	}
	state, err := a.Store.LoadState()
	if err != nil {
		return err
	}
	if old, ok := state.Sources[name]; ok && !old.InstalledAt.IsZero() {
		src.InstalledAt = old.InstalledAt
	}
	src.UpdatedAt = time.Now().UTC()
	src.ContentHash = hash
	state.Sources[name] = src
	return a.Store.SaveState(state)
}

func pinGitHubSource(raw, commit string) string {
	if commit == "" || !strings.HasPrefix(raw, "github:") {
		return raw
	}
	body := strings.TrimPrefix(raw, "github:")
	parts := strings.Split(body, "/")
	if len(parts) < 2 {
		return raw
	}
	repo := parts[1]
	if at := strings.Index(repo, "@"); at >= 0 {
		repo = repo[:at]
	}
	parts[1] = repo + "@" + commit
	return "github:" + strings.Join(parts, "/")
}

func targetPath(h harness.Adapter, projectRoot, skill string, scope core.Scope) string {
	locs := h.SkillLocations(projectRoot)
	for _, loc := range locs {
		if loc.Scope == scope {
			return filepath.Join(loc.Path, skill)
		}
	}
	return skill
}

func mergePlans(a, b core.Plan) core.Plan {
	seen := map[string]core.Change{}
	for _, c := range a.Changes {
		seen[c.Harness+"\x00"+c.Skill] = c
	}
	for _, c := range b.Changes {
		seen[c.Harness+"\x00"+c.Skill] = c
	}
	out := core.Plan{}
	for _, c := range seen {
		out.Changes = append(out.Changes, c)
	}
	sort.Slice(out.Changes, func(i, j int) bool {
		if out.Changes[i].Harness == out.Changes[j].Harness {
			return out.Changes[i].Skill < out.Changes[j].Skill
		}
		return out.Changes[i].Harness < out.Changes[j].Harness
	})
	return out
}
