package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/fsutil"
)

type Store struct {
	Root string
}

func DefaultRoot() (string, error) {
	if v := os.Getenv("SKILLMUX_HOME"); v != "" {
		return filepath.Clean(v), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		if app := os.Getenv("LOCALAPPDATA"); app != "" {
			return filepath.Join(app, "Skillmux"), nil
		}
	}
	return filepath.Join(home, ".skillmux"), nil
}

func Open(root string) (*Store, error) {
	if root == "" {
		var err error
		root, err = DefaultRoot()
		if err != nil {
			return nil, err
		}
	}
	s := &Store{Root: filepath.Clean(root)}
	for _, d := range []string{s.SkillsDir(), s.ProfilesDir(), s.BackupsDir(), s.SourcesDir(), s.CacheDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(s.statePath()); errors.Is(err, os.ErrNotExist) {
		st := defaultState()
		if err := s.SaveState(st); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) SkillsDir() string   { return filepath.Join(s.Root, "skills") }
func (s *Store) ProfilesDir() string { return filepath.Join(s.Root, "profiles") }
func (s *Store) BackupsDir() string  { return filepath.Join(s.Root, "backups") }
func (s *Store) SourcesDir() string  { return filepath.Join(s.Root, "sources") }
func (s *Store) CacheDir() string    { return filepath.Join(s.Root, "cache") }
func (s *Store) statePath() string   { return filepath.Join(s.Root, "state.json") }

func defaultState() core.State {
	return core.State{
		Version: 1,
		ActiveProfile: "default",
		Enabled: map[string]map[string]bool{},
		Sources: map[string]core.Source{},
	}
}

func (s *Store) LoadState() (core.State, error) {
	data, err := os.ReadFile(s.statePath())
	if err != nil {
		return core.State{}, err
	}
	var st core.State
	if err := json.Unmarshal(data, &st); err != nil {
		return core.State{}, err
	}
	if st.Enabled == nil {
		st.Enabled = map[string]map[string]bool{}
	}
	if st.Sources == nil {
		st.Sources = map[string]core.Source{}
	}
	if st.ActiveProfile == "" {
		st.ActiveProfile = "default"
	}
	return st, nil
}

func (s *Store) SaveState(st core.State) error {
	st.Version = 1
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '
')
	return fsutil.AtomicWrite(s.statePath(), data, 0o600)
}

func (s *Store) SkillPath(name string) string {
	return filepath.Join(s.SkillsDir(), name)
}

func (s *Store) ListSkills() ([]core.Skill, error) {
	entries, err := os.ReadDir(s.SkillsDir())
	if err != nil {
		return nil, err
	}
	st, err := s.LoadState()
	if err != nil {
		return nil, err
	}
	var out []core.Skill
	for _, e := range entries {
		if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			continue
		}
		name := e.Name()
		path := s.SkillPath(name)
		hash, err := fsutil.HashDir(path)
		if err != nil {
			continue
		}
		desc, _ := description(filepath.Join(path, "SKILL.md"))
		skill := core.Skill{Name:name, Description:desc, Path:path, Hash:hash, Managed:true, Enabled:map[string]bool{}}
		if src, ok := st.Sources[name]; ok {
			cp := src
			skill.Source = &cp
			skill.Modified = src.ContentHash != "" && src.ContentHash != hash
		}
		for h, v := range st.Enabled[name] {
			skill.Enabled[h] = v
		}
		if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err != nil {
			skill.Health = "error"
		} else if skill.Modified {
			skill.Health = "modified"
		} else {
			skill.Health = "healthy"
		}
		out = append(out, skill)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func description(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "
")
	inFront := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if i == 0 && t == "---" {
			inFront = true
			continue
		}
		if inFront && t == "---" {
			break
		}
		if inFront && strings.HasPrefix(strings.ToLower(t), "description:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "description:")), ""'"), nil
		}
		if !inFront && strings.HasPrefix(t, "# ") && i+1 < len(lines) {
			for _, next := range lines[i+1:] {
				next = strings.TrimSpace(next)
				if next != "" {
					return next, nil
				}
			}
		}
	}
	return "", nil
}

func (s *Store) Adopt(name, src string, source *core.Source) error {
	if !fsutil.SafeName(name) {
		return errors.New("invalid skill name")
	}
	if _, err := os.Stat(filepath.Join(src, "SKILL.md")); err != nil {
		return errors.New("source does not contain SKILL.md")
	}
	dst := s.SkillPath(name)
	if _, err := os.Lstat(dst); err == nil {
		oldHash, _ := fsutil.HashDir(dst)
		newHash, _ := fsutil.HashDir(src)
		if oldHash != newHash {
			return errors.New("canonical skill already exists with different contents")
		}
		return nil
	}
	if err := fsutil.CopyDir(src, dst); err != nil {
		return err
	}
	hash, err := fsutil.HashDir(dst)
	if err != nil {
		return err
	}
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	if source != nil {
		source.ContentHash = hash
		st.Sources[name] = *source
	}
	return s.SaveState(st)
}

func (s *Store) SetEnabled(skill, harness string, enabled bool) error {
	if !fsutil.SafeName(skill) {
		return errors.New("invalid skill name")
	}
	if _, err := os.Stat(s.SkillPath(skill)); err != nil {
		return errors.New("unknown canonical skill")
	}
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	if st.Enabled[skill] == nil {
		st.Enabled[skill] = map[string]bool{}
	}
	st.Enabled[skill][harness] = enabled
	return s.SaveState(st)
}

func (s *Store) RemoveCanonical(name string) error {
	if !fsutil.SafeName(name) {
		return errors.New("invalid skill name")
	}
	path := s.SkillPath(name)
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	st, err := s.LoadState()
	if err != nil {
		return err
	}
	delete(st.Enabled, name)
	delete(st.Sources, name)
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return s.SaveState(st)
}

func (s *Store) SaveProfile(p core.Profile) error {
	if !fsutil.SafeName(p.Name) {
		return errors.New("invalid profile name")
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.AtomicWrite(filepath.Join(s.ProfilesDir(), p.Name+".json"), append(data, '
'), 0o600)
}

func (s *Store) LoadProfile(name string) (core.Profile, error) {
	data, err := os.ReadFile(filepath.Join(s.ProfilesDir(), name+".json"))
	if err != nil {
		return core.Profile{}, err
	}
	var p core.Profile
	err = json.Unmarshal(data, &p)
	return p, err
}

func (s *Store) ListProfiles() ([]string, error) {
	ents, err := os.ReadDir(s.ProfilesDir())
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			out = append(out, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	sort.Strings(out)
	return out, nil
}
