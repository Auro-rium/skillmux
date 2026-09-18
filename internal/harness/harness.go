package harness

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/Auro-rium/skillmux/internal/core"
)

type Adapter interface {
	Name() string
	Detect() bool
	SkillLocations(projectRoot string) []core.SkillLocation
	SupportsSymlinks() bool
}

type adapter struct {
	name       string
	markers    []string
	globalDirs []string
	projectDirs []string
}

func (a adapter) Name() string { return a.name }

func (a adapter) Detect() bool {
	for _, p := range a.markers {
		if _, err := os.Stat(expand(p)); err == nil {
			return true
		}
	}
	for _, p := range a.globalDirs {
		if _, err := os.Stat(expand(p)); err == nil {
			return true
		}
	}
	return false
}

func (a adapter) SkillLocations(projectRoot string) []core.SkillLocation {
	var out []core.SkillLocation
	for _, p := range a.globalDirs {
		out = append(out, core.SkillLocation{Path: expand(p), Scope: core.ScopeGlobal})
	}
	if projectRoot != "" {
		for _, p := range a.projectDirs {
			out = append(out, core.SkillLocation{Path: filepath.Join(projectRoot, filepath.FromSlash(p)), Scope: core.ScopeProject})
		}
	}
	return out
}

func (a adapter) SupportsSymlinks() bool {
	return runtime.GOOS != "windows"
}

func Default() []Adapter {
	return []Adapter{
		adapter{name:"codex", markers:[]string{"~/.codex"}, globalDirs:[]string{"~/.agents/skills", "~/.codex/skills"}, projectDirs:[]string{".agents/skills", ".codex/skills"}},
		adapter{name:"claude", markers:[]string{"~/.claude"}, globalDirs:[]string{"~/.claude/skills"}, projectDirs:[]string{".claude/skills"}},
		adapter{name:"gemini", markers:[]string{"~/.gemini"}, globalDirs:[]string{"~/.gemini/skills"}, projectDirs:[]string{".gemini/skills"}},
		adapter{name:"cursor", markers:[]string{"~/.cursor"}, globalDirs:[]string{"~/.cursor/skills"}, projectDirs:[]string{".cursor/skills"}},
		adapter{name:"opencode", markers:[]string{"~/.config/opencode"}, globalDirs:[]string{"~/.config/opencode/skills"}, projectDirs:[]string{".opencode/skills"}},
	}
}

func expand(p string) string {
	if len(p) >= 2 && p[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, filepath.FromSlash(p[2:]))
		}
	}
	return filepath.Clean(p)
}
