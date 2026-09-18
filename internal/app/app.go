			}
			if len(copies) > 0 { chosen=copies[0] }
		}
		if chosen.Path == "" {
			result.Skipped = append(result.Skipped, n)
			continue
		}
		if err := a.Store.Adopt(n, chosen.Path, nil); err != nil {
			result.Skipped = append(result.Skipped, n)
			continue
		}
		if scope := a.installationScope(chosen); scope != "" {
			_ = a.Store.SetScope(n, scope)
		}
		for _, c := range copies { _ = a.Store.SetEnabled(n, c.Harness, true) }
		result.Imported = append(result.Imported, n)
	}

	if len(result.Imported) > 0 {
		imported := map[string]bool{}
		for _, n := range result.Imported { imported[n] = true }
		plan, planErr := a.Syncer.Plan()
		if planErr != nil { return result, planErr }
		filtered := core.Plan{}
		for _, change := range plan.Changes {
			if imported[change.Skill] {
				filtered.Changes = append(filtered.Changes, change)
			}
		}
		if err := a.Syncer.Apply(filtered, true); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (a *App) installationScope(inst core.Installation) core.Scope {
	for _, h := range harness.Default() {
		if h.Name() != inst.Harness { continue }
		for _, loc := range h.SkillLocations(a.Root) {
			if samePath(loc.Path, filepath.Dir(inst.Path)) {
				return loc.Scope
			}
		}