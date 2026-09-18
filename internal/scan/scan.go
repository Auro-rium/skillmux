package scan

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/fsutil"
	"github.com/Auro-rium/skillmux/internal/harness"
	"github.com/Auro-rium/skillmux/internal/store"
)

type Scanner struct {
	Store    *store.Store
	Harnesses []harness.Adapter
	ProjectRoot string
}

func New(s *store.Store, projectRoot string) *Scanner {
	return &Scanner{Store:s, Harnesses:harness.Default(), ProjectRoot:projectRoot}
}

func (s *Scanner) Run() (core.ScanReport, error) {
	var report core.ScanReport
	byName := map[string][]core.Installation{}
	for _, h := range s.Harnesses {
		hi := core.HarnessInfo{Name:h.Name(), Detected:h.Detect(), Locations:h.SkillLocations(s.ProjectRoot)}
		report.Harnesses = append(report.Harnesses, hi)
		for _, loc := range hi.Locations {
			entries, err := os.ReadDir(loc.Path)
			if err != nil {
				if os.IsNotExist(err) || os.IsPermission(err) {
					continue
				}
				return report, err
			}
			for _, e := range entries {
				name := e.Name()
				path := filepath.Join(loc.Path, name)
				inst := core.Installation{SkillName:name, Harness:h.Name(), Path:path}
				info, err := os.Lstat(path)
				if err != nil {
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					if _, err := filepath.EvalSymlinks(path); err != nil {
						inst.Broken = true
						report.Broken++
					} else if target, err := filepath.EvalSymlinks(path); err == nil {
						canon, _ := filepath.Abs(s.Store.SkillPath(name))
						resolved, _ := filepath.Abs(target)
						inst.Managed = canon == resolved
					}
				}
				if !inst.Broken {
					if hash, err := fsutil.HashDir(path); err == nil {
						inst.Hash = hash
					} else {
						inst.Broken = true
						report.Broken++
					}
				}
				report.Installations = append(report.Installations, inst)
				byName[name] = append(byName[name], inst)
			}
		}
	}
	report.UniqueSkills = len(byName)
	for name, copies := range byName {
		if len(copies) < 2 {
			continue
		}
		report.Duplicates += len(copies)-1
		hash := ""
		conflict := false
		for _, c := range copies {
			if c.Broken {
				continue
			}
			if hash == "" {
				hash = c.Hash
			} else if c.Hash != hash {
				conflict = true
			}
		}
		if conflict {
			report.Conflicts = append(report.Conflicts, core.Conflict{Name:name, Copies:copies, Reason:"contents differ across harness locations"})
		}
	}
	sort.Slice(report.Installations, func(i,j int) bool {
		if report.Installations[i].SkillName == report.Installations[j].SkillName {
			return report.Installations[i].Harness < report.Installations[j].Harness
		}
		return report.Installations[i].SkillName < report.Installations[j].SkillName
	})
	sort.Slice(report.Conflicts, func(i,j int) bool { return report.Conflicts[i].Name < report.Conflicts[j].Name })
	return report, nil
}
