package project

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/fsutil"
)

type Manifest struct {
	Version int             `json:"version"`
	Skills  map[string]string `json:"skills"`
	Targets map[string]bool `json:"targets"`
}

type Lock struct {
	Version int         `json:"version"`
	Skills  []LockSkill `json:"skills"`
}

type LockSkill struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	Requested string `json:"requested"`
	Resolved  string `json:"resolved,omitempty"`
	Commit    string `json:"commit"`
	Hash      string `json:"hash"`
}

func LoadManifest(root string) (*Manifest, error) {
	if root == "" { return nil, nil }
	path := filepath.Join(root, ".skillmux.toml")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) { return nil, nil }
	if err != nil { return nil, err }
	defer f.Close()

	m := &Manifest{Version:1, Skills:map[string]string{}, Targets:map[string]bool{}}
	section := ""
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(stripComment(sc.Text()))
		if line == "" { continue }
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1:len(line)-1])
			continue
		}
		key, value, ok := splitKV(line)
		if !ok { return nil, fmt.Errorf("%s:%d: expected key = value", path, lineNo) }
		switch section {
		case "":
			if key != "version" { continue }
			v, err := strconv.Atoi(value)
			if err != nil || v != 1 { return nil, fmt.Errorf("%s:%d: unsupported manifest version", path, lineNo) }
			m.Version = v
		case "skills":
			if !fsutil.SafeName(key) { return nil, fmt.Errorf("%s:%d: invalid skill name %q", path, lineNo, key) }
			s, err := unquote(value)
			if err != nil { return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err) }
			m.Skills[key] = s
		case "targets":
			b, err := strconv.ParseBool(value)
			if err != nil { return nil, fmt.Errorf("%s:%d: target must be boolean", path, lineNo) }
			m.Targets[key] = b
		}
	}
	if err := sc.Err(); err != nil { return nil, err }
	return m, nil
}

func LoadLock(root string) (*Lock, error) {
	if root == "" { return nil, nil }
	path := filepath.Join(root, ".skillmux.lock")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) { return nil, nil }
	if err != nil { return nil, err }
	defer f.Close()

	lock := &Lock{Version:1}
	var cur *LockSkill
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(stripComment(sc.Text()))
		if line == "" { continue }
		if line == "[[skill]]" {
			lock.Skills = append(lock.Skills, LockSkill{})
			cur = &lock.Skills[len(lock.Skills)-1]
			continue
		}
		key, value, ok := splitKV(line)
		if !ok { continue }
		if cur == nil {
			if key == "version" {
				v, _ := strconv.Atoi(value)
				lock.Version = v
			}
			continue
		}
		s, err := unquote(value)
		if err != nil { continue }
		switch key {
		case "name": cur.Name=s
		case "source": cur.Source=s
		case "requested": cur.Requested=s
		case "resolved": cur.Resolved=s
		case "commit": cur.Commit=s
		case "hash": cur.Hash=s
		}
	}
	return lock, sc.Err()
}

func WriteLock(root string, lock Lock) error {
	if root == "" { return errors.New("project root is unavailable") }
	sort.Slice(lock.Skills, func(i,j int) bool { return lock.Skills[i].Name < lock.Skills[j].Name })
	var b strings.Builder
	b.WriteString("version = 1\n\n")
	for _, s := range lock.Skills {
		b.WriteString("[[skill]]\n")
		fmt.Fprintf(&b, "name = %q\nsource = %q\nrequested = %q\n", s.Name, s.Source, s.Requested)
		if s.Resolved != "" { fmt.Fprintf(&b, "resolved = %q\n", s.Resolved) }
		fmt.Fprintf(&b, "commit = %q\nhash = %q\n\n", s.Commit, s.Hash)
	}
	return fsutil.AtomicWrite(filepath.Join(root, ".skillmux.lock"), []byte(b.String()), 0o644)
}

func (l *Lock) ByName(name string) (LockSkill, bool) {
	if l == nil { return LockSkill{}, false }
	for _, s := range l.Skills { if s.Name == name { return s, true } }
	return LockSkill{}, false
}

func LockFrom(manifest *Manifest, skills []core.Skill) Lock {
	lock := Lock{Version:1}
	if manifest == nil { return lock }
	byName:=map[string]core.Skill{}
	for _, sk:=range skills { byName[sk.Name]=sk }
	for name, requested:=range manifest.Skills {
		sk,ok:=byName[name]
		if !ok || sk.Source==nil || sk.Source.Type!="git" { continue }
		lock.Skills=append(lock.Skills,LockSkill{
			Name:name,
			Source:"github:"+sk.Source.Repository,
			Requested:requested,
			Resolved:sk.Source.Version,
			Commit:sk.Source.Commit,
			Hash:sk.Hash,
		})
	}
	return lock
}

func stripComment(s string) string {
	inQuote:=false
	for i,r:=range s {
		if r=='"' { inQuote=!inQuote }
		if r=='#' && !inQuote { return s[:i] }
	}
	return s
}

func splitKV(s string) (string,string,bool) {
	i:=strings.Index(s,"=")
	if i<0 { return "","",false }
	return strings.TrimSpace(s[:i]),strings.TrimSpace(s[i+1:]),true
}

func unquote(s string) (string,error) {
	s=strings.TrimSpace(s)
	if len(s)>=2 && s[0]=='"' && s[len(s)-1]=='"' {
		v,err:=strconv.Unquote(s)
		return v,err
	}
	return s,nil
}
