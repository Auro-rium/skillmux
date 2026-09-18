package config

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DefaultProfile string
	Sync struct {
		PreferSymlinks bool
		ConfirmChanges bool
	}
	Updates struct {
		CheckOnStart bool
	}
	TUI struct {
		ShowHidden bool
	}
}

func Default() Config {
	var c Config
	c.DefaultProfile = "default"
	c.Sync.PreferSymlinks = true
	c.Sync.ConfirmChanges = true
	return c
}

func Load(path string) (Config, error) {
	c := Default()
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	defer f.Close()

	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(stripComment(sc.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1:len(line)-1])
			continue
		}
		key, value, ok := split(line)
		if !ok {
			continue
		}
		switch section {
		case "":
			if key == "default_profile" {
				c.DefaultProfile = unquote(value)
			}
		case "sync":
			switch key {
			case "prefer_symlinks":
				if v, err := strconv.ParseBool(value); err == nil { c.Sync.PreferSymlinks = v }
			case "confirm_changes":
				if v, err := strconv.ParseBool(value); err == nil { c.Sync.ConfirmChanges = v }
			}
		case "updates":
			if key == "check_on_start" {
				if v, err := strconv.ParseBool(value); err == nil { c.Updates.CheckOnStart = v }
			}
		case "tui":
			if key == "show_hidden" {
				if v, err := strconv.ParseBool(value); err == nil { c.TUI.ShowHidden = v }
			}
		}
	}
	return c, sc.Err()
}

func split(s string) (string, string, bool) {
	i := strings.IndexByte(s, '=')
	if i < 0 { return "", "", false }
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if v, err := strconv.Unquote(s); err == nil { return v }
	}
	return s
}

func stripComment(s string) string {
	quoted := false
	for i, r := range s {
		if r == '"' { quoted = !quoted }
		if r == '#' && !quoted { return s[:i] }
	}
	return s
}
