package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Auro-rium/skillmux/internal/config"
	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/doctor"
	"github.com/Auro-rium/skillmux/internal/fsutil"
	"github.com/Auro-rium/skillmux/internal/harness"
	"github.com/Auro-rium/skillmux/internal/scan"
	"github.com/Auro-rium/skillmux/internal/source"
	"github.com/Auro-rium/skillmux/internal/store"
	"github.com/Auro-rium/skillmux/internal/syncer"
)

type App struct {
	Store   *store.Store
	Scanner *scan.Scanner
	Syncer  *syncer.Engine
	Config  config.Config
	Root    string
}

type Status struct {
	Profile        string             `json:"profile"`
	Harnesses      []core.HarnessInfo `json:"harnesses"`
	Skills         int                `json:"skills"`
	Enabled        int                `json:"enabled"`
	Conflicts      int                `json:"conflicts"`
	DoctorWarnings int                `json:"doctor_warnings"`
	DoctorErrors   int                `json:"doctor_errors"`
	PendingChanges int                `json:"pending_changes"`
}

type ImportResult struct {
	Imported  []string `json:"imported"`
	Conflicts []string `json:"conflicts"`
	Skipped   []string `json:"skipped"`
}

func Open() (*App, error) {
	root, _ := FindProjectRoot()
	st, err := store.Open("")
	if err != nil { return nil, err }
	cfg, err := config.Load(filepath.Join(st.Root, "config.toml"))
	if err != nil { return nil, err }
	sc := scan.New(st, root)
	sy := syncer.New(st, root)
	sy.PreferLinks = cfg.Sync.PreferSymlinks
	return &App{Store:st, Scanner:sc, Syncer:sy, Config:cfg, Root:root}, nil
}

func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil { return "", err }
	for {
		for _, marker := range []string{".git", ".skillmux.toml"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil { return dir, nil }
		}
		parent := filepath.Dir(dir)
		if parent == dir { return "", nil }
		dir = parent
	}
}

func (a *App) Scan() (core.ScanReport, error) { return a.Scanner.Run() }
func (a *App) List() ([]core.Skill, error) { return a.Store.ListSkills() }

func (a *App) Get(name string) (core.Skill, error) {
	skills, err := a.List()
	if err != nil { return core.Skill{}, err }
	for _, sk := range skills {
		if sk.Name == name { return sk, nil }
	}
	return core.Skill{}, errors.New("skill not found: "+name)
}

func (a *App) Search(q string) ([]core.Skill, error) {
	skills, err := a.List()
	if err != nil { return nil, err }
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" { return skills, nil }
	var out []core.Skill
	for _, sk := range skills {
		hay := strings.ToLower(sk.Name+" "+sk.Description)
		if sk.Source != nil { hay += " "+strings.ToLower(sk.Source.URL+" "+sk.Source.Repository) }
		for h, enabled := range sk.Enabled {
			if enabled { hay += " "+strings.ToLower(h) }
		}
		if strings.Contains(hay, q) { out = append(out, sk) }
	}
	return out, nil
}

func (a *App) Status() (Status, error) {
	st, err := a.Store.LoadState()
	if err != nil { return Status{}, err }
	skills, err := a.List()
	if err != nil { return Status{}, err }
	scanReport, err := a.Scan()
	if err != nil { return Status{}, err }
	doc, err := doctor.New(a.Store, a.Scanner).Run()
	if err != nil { return Status{}, err }
	plan, err := a.Syncer.Plan()
	if err != nil { return Status{}, err }
	status := Status{Profile:st.ActiveProfile, Harnesses:scanReport.Harnesses, Skills:len(skills), Conflicts:len(scanReport.Conflicts), PendingChanges:len(plan.Changes)}
	for _, sk := range skills {
		active := false
		for _, enabled := range sk.Enabled { if enabled { active=true; break } }
		if active { status.Enabled++ }
	}
	for _, f := range doc.Findings {
		if f.Severity == "error" { status.DoctorErrors++ } else { status.DoctorWarnings++ }
	}
	return status, nil
}

func (a *App) Add(ctx context.Context, raw string, targets []string) (core.Skill, error) {
	res, err := source.Resolve(ctx, raw, a.Store.CacheDir())
	if err != nil { return core.Skill{}, err }
	if res.Cleanup != nil { defer res.Cleanup() }
	src := res.Source
	if err := a.Store.Adopt(res.Name, res.Path, &src); err != nil { return core.Skill{}, err }
	if len(targets) == 0 {
		for _, h := range harness.Default() {
			if h.Detect() { targets = append(targets, h.Name()) }
		}
	}
	for _, h := range unique(targets) {
		if err := validateHarness(h); err != nil { return core.Skill{}, err }
		if err := a.Store.SetEnabled(res.Name, h, true); err != nil { return core.Skill{}, err }
	}
	return a.Get(res.Name)
}

func (a *App) Enable(name, target string, enabled bool) error {
	if err := validateHarness(target); err != nil { return err }
	return a.Store.SetEnabled(name, target, enabled)
}

func validateHarness(name string) error {
	for _, h := range harness.Default() {
		if h.Name() == name { return nil }
	}
	return errors.New("unknown harness: "+name)
}

func (a *App) Import(name, fromHarness string) (ImportResult, error) {
	report, err := a.Scan()
	if err != nil { return ImportResult{}, err }
	groups := map[string][]core.Installation{}
	for _, inst := range report.Installations {
		if inst.Managed { continue }
		groups[inst.SkillName] = append(groups[inst.SkillName], inst)
	}
	result := ImportResult{}
	names := make([]string, 0, len(groups))
	for n := range groups {
		if name == "" || n == name { names = append(names, n) }
	}
	sort.Strings(names)
	if name != "" && len(names) == 0 { return result, errors.New("unmanaged skill not found: "+name) }
	for _, n := range names {
		copies := groups[n]
		chosen := core.Installation{}
		if fromHarness != "" {
			for _, c := range copies { if c.Harness == fromHarness { chosen=c; break } }
			if chosen.Path == "" { return result, fmt.Errorf("%s not found in harness %s", n, fromHarness) }
		} else {
			hash := ""
			conflict := false
			for _, c := range copies {
				if c.Broken { continue }
				if hash == "" { hash=c.Hash } else if c.Hash != hash { conflict=true }
			}
			if conflict {
				result.Conflicts = append(result.Conflicts, n)
				continue
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
	}
	return ""
}

func samePath(aPath, bPath string) bool {
	aAbs, errA := filepath.Abs(aPath)
	bAbs, errB := filepath.Abs(bPath)
	if errA != nil || errB != nil {
		return false
	}
	return filepath.Clean(aAbs) == filepath.Clean(bAbs)
}

func (a *App) Sync(dryRun, force bool) (core.Plan, error) {
	plan, err := a.Syncer.Plan()
	if err != nil { return plan, err }
	if dryRun { return plan, nil }
	if err := a.Syncer.Apply(plan, force); err != nil { return plan, err }
	return plan, nil
}

func (a *App) Doctor() (core.DoctorReport, error) {
	return doctor.New(a.Store, a.Scanner).Run()
}

func (a *App) Diff(name, target string) (string, error) {
	sk, err := a.Get(name)
	if err != nil { return "", err }
	right := ""
	label := ""
	if target != "" {
		for _, h := range harness.Default() {
			if h.Name() != target { continue }
			locs := h.SkillLocations(a.Root)
			for _, loc := range locs {
				p := filepath.Join(loc.Path, name, "SKILL.md")
				if _, err := os.Stat(p); err == nil { right=p; label=target; break }
			}
		}
	} else {
		report, _ := a.Scan()
		for _, inst := range report.Installations {
			if inst.SkillName == name && !inst.Managed {
				p := filepath.Join(inst.Path, "SKILL.md")
				if _, err := os.Stat(p); err == nil { right=p; label=inst.Harness; break }
			}
		}
	}
	if right == "" { return "", errors.New("no comparison copy found; pass --target") }
	left := filepath.Join(sk.Path, "SKILL.md")
	aData, err := os.ReadFile(left)
	if err != nil { return "", err }
	bData, err := os.ReadFile(right)
	if err != nil { return "", err }
	return lineDiff("canonical", string(aData), label, string(bData)), nil
}

func lineDiff(aLabel, aText, bLabel, bText string) string {
	if aText == bText { return "No differences.\n" }
	aLines := strings.Split(aText, "\n")
	bLines := strings.Split(bText, "\n")
	var b bytes.Buffer
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", aLabel, bLabel)
	max := len(aLines)
	if len(bLines) > max { max=len(bLines) }
	for i:=0; i<max; i++ {
		if i<len(aLines) && i<len(bLines) && aLines[i]==bLines[i] { continue }
		if i<len(aLines) { fmt.Fprintf(&b, "-%04d %s\n", i+1, aLines[i]) }
		if i<len(bLines) { fmt.Fprintf(&b, "+%04d %s\n", i+1, bLines[i]) }
	}
	return b.String()
}

func (a *App) Remove(name, target string, canonical bool) error {
	if canonical {
		if err := a.Syncer.Eject(name); err != nil { return err }
		return a.Store.RemoveCanonical(name)
	}
	if target == "" { return errors.New("conservative remove requires --target, or --canonical to delete the canonical copy") }
	if err := a.Enable(name, target, false); err != nil { return err }
	plan, err := a.Syncer.Plan()
	if err != nil { return err }
	return a.Syncer.Apply(plan, false)
}

func (a *App) Eject(name string) error { return a.Syncer.Eject(name) }

func (a *App) Update(ctx context.Context, only string) ([]string, error) {
	skills, err := a.List()
	if err != nil { return nil, err }
	var updated []string
	for _, sk := range skills {
		if only != "" && sk.Name != only { continue }
		if sk.Source == nil || sk.Source.Type != "git" { continue }
		if sk.Modified { return updated, fmt.Errorf("%s has local modifications; refusing to update", sk.Name) }
		raw := "github:"+sk.Source.Repository
		if sk.Source.Version != "" {
			raw += "@" + sk.Source.Version
		}
		if sk.Source.Subdir != "" { raw += "/"+sk.Source.Subdir }
		res, err := source.Resolve(ctx, raw, a.Store.CacheDir())
		if err != nil { return updated, err }
		func() {
			if res.Cleanup != nil { defer res.Cleanup() }
			if res.Source.Commit == sk.Source.Commit { return }
			backup := filepath.Join(a.Store.BackupsDir(), time.Now().UTC().Format("20060102T150405.000000000Z"), "update", sk.Name)
			if err = fsutil.CopyDir(sk.Path, backup); err != nil { return }
			tmp := sk.Path+".skillmux-update"
			_ = os.RemoveAll(tmp)
			if err = fsutil.CopyDir(res.Path, tmp); err != nil { return }
			if err = os.RemoveAll(sk.Path); err != nil { return }
			if err = os.Rename(tmp, sk.Path); err != nil { return }
			hash, hErr := fsutil.HashDir(sk.Path)
			if hErr != nil { err=hErr; return }
			state, sErr := a.Store.LoadState()
			if sErr != nil { err=sErr; return }
			src := res.Source
			src.InstalledAt = sk.Source.InstalledAt
			src.UpdatedAt = time.Now().UTC()
			src.ContentHash = hash
			state.Sources[sk.Name] = src
			err = a.Store.SaveState(state)
			if err == nil { updated=append(updated, sk.Name) }
		}()
		if err != nil { return updated, err }
	}
	if only != "" && len(updated)==0 {
		sk, getErr := a.Get(only)
		if getErr != nil { return updated, getErr }
		if sk.Source == nil || sk.Source.Type != "git" { return updated, errors.New("skill is not backed by a GitHub source") }
	}
	return updated, nil
}

func (a *App) ProfileCreate(name string) (core.Profile, error) {
	skills, err := a.List()
	if err != nil { return core.Profile{}, err }
	p := core.Profile{Name:name, Harnesses:map[string]bool{}}
	seen := map[string]bool{}
	for _, sk := range skills {
		active := false
		for h, enabled := range sk.Enabled {
			if enabled { active=true; p.Harnesses[h]=true; seen[h]=true }
		}
		if active { p.Skills=append(p.Skills, sk.Name) }
	}
	sort.Strings(p.Skills)
	if err := a.Store.SaveProfile(p); err != nil { return core.Profile{}, err }
	return p, nil
}

func (a *App) ProfileUse(name string) (core.Profile, error) {
	p, err := a.Store.LoadProfile(name)
	if err != nil { return core.Profile{}, err }
	st, err := a.Store.LoadState()
	if err != nil { return core.Profile{}, err }
	for skill := range st.Enabled {
		for h := range st.Enabled[skill] { st.Enabled[skill][h]=false }
	}
	for _, skill := range p.Skills {
		if st.Enabled[skill] == nil { st.Enabled[skill]=map[string]bool{} }
		for h, enabled := range p.Harnesses { if enabled { st.Enabled[skill][h]=true } }
	}
	st.ActiveProfile=name
	if err := a.Store.SaveState(st); err != nil { return core.Profile{}, err }
	return p, nil
}

func (a *App) ProfileDelete(name string) error {
	st, err := a.Store.LoadState()
	if err != nil { return err }
	if st.ActiveProfile == name { return errors.New("cannot delete the active profile") }
	return os.Remove(filepath.Join(a.Store.ProfilesDir(), name+".json"))
}

func EncodeJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil { return nil, err }
	return b.Bytes(), nil
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		v=strings.TrimSpace(strings.ToLower(v))
		if v!="" && !seen[v] { seen[v]=true; out=append(out,v) }
	}
	sort.Strings(out)
	return out
}

func Confirm(r *bufio.Reader, prompt string) bool {
	fmt.Print(prompt)
	line, _ := r.ReadString('\n')
	line=strings.ToLower(strings.TrimSpace(line))
	return line=="" || line=="y" || line=="yes"
}
