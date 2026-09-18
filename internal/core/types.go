package core

import "time"

type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
)

type Skill struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Path        string            `json:"path"`
	Hash        string            `json:"hash"`
	Source      *Source           `json:"source,omitempty"`
	Enabled     map[string]bool   `json:"enabled,omitempty"`
	Managed     bool              `json:"managed"`
	Modified    bool              `json:"modified"`
	Health      string            `json:"health,omitempty"`
	Scope       Scope             `json:"scope,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type Source struct {
	Type        string    `json:"type"`
	URL         string    `json:"url,omitempty"`
	Repository  string    `json:"repository,omitempty"`
	Subdir      string    `json:"subdirectory,omitempty"`
	Commit      string    `json:"commit,omitempty"`
	Version     string    `json:"version,omitempty"`
	InstalledAt time.Time `json:"installed_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
	ContentHash string    `json:"content_hash,omitempty"`
}

type HarnessInfo struct {
	Name      string          `json:"name"`
	Detected  bool            `json:"detected"`
	Locations []SkillLocation `json:"locations"`
}

type SkillLocation struct {
	Path  string `json:"path"`
	Scope Scope  `json:"scope"`
}

type Installation struct {
	SkillName string `json:"skill_name"`
	Harness   string `json:"harness"`
	Path      string `json:"path"`
	Hash      string `json:"hash,omitempty"`
	Broken    bool   `json:"broken"`
	Managed   bool   `json:"managed"`
}

type Conflict struct {
	Name    string         `json:"name"`
	Copies  []Installation `json:"copies"`
	Reason  string         `json:"reason"`
}

type ScanReport struct {
	Harnesses    []HarnessInfo  `json:"harnesses"`
	Installations []Installation `json:"installations"`
	UniqueSkills int            `json:"unique_skills"`
	Duplicates   int            `json:"duplicates"`
	Conflicts    []Conflict     `json:"conflicts"`
	Broken       int            `json:"broken"`
}

type State struct {
	Version       int                         `json:"version"`
	ActiveProfile string                      `json:"active_profile"`
	Enabled       map[string]map[string]bool `json:"enabled"`
	Sources       map[string]Source           `json:"sources"`
	Scopes        map[string]Scope            `json:"scopes"`
}

type Profile struct {
	Name      string            `json:"name"`
	Skills    []string          `json:"skills"`
	Harnesses map[string]bool   `json:"harnesses"`
	Scopes    map[string]string `json:"scopes,omitempty"`
}

type ChangeKind string

const (
	ChangeInstall ChangeKind = "install"
	ChangeUpdate  ChangeKind = "update"
	ChangeRemove  ChangeKind = "remove"
	ChangeRepair  ChangeKind = "repair"
)

type Change struct {
	Kind      ChangeKind `json:"kind"`
	Skill     string     `json:"skill"`
	Harness   string     `json:"harness"`
	Source    string     `json:"source,omitempty"`
	Target    string     `json:"target"`
	Reason    string     `json:"reason,omitempty"`
}

type Plan struct {
	Changes []Change `json:"changes"`
}

type DoctorFinding struct {
	Severity string `json:"severity"`
	Skill    string `json:"skill,omitempty"`
	Path     string `json:"path,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type DoctorReport struct {
	SkillsChecked int             `json:"skills_checked"`
	Findings      []DoctorFinding `json:"findings"`
}
