package source

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Auro-rium/skillmux/internal/core"
	"github.com/Auro-rium/skillmux/internal/fsutil"
)

type Resolved struct {
	Name    string
	Path    string
	Source  core.Source
	Cleanup func()
}

func Resolve(ctx context.Context, raw string, cacheRoot string) (Resolved, error) {
	if raw == "" {
		return Resolved{}, errors.New("source is required")
	}
	if strings.HasPrefix(raw, "github:") || strings.HasPrefix(raw, "https://github.com/") || strings.HasPrefix(raw, "http://github.com/") {
		return resolveGitHub(ctx, raw, cacheRoot)
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return Resolved{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Resolved{}, err
	}
	if !info.IsDir() {
		return Resolved{}, errors.New("local source must be a directory")
	}
	if _, err := os.Stat(filepath.Join(abs, "SKILL.md")); err != nil {
		return Resolved{}, errors.New("local source does not contain SKILL.md")
	}
	return Resolved{
		Name: filepath.Base(abs),
		Path: abs,
		Source: core.Source{Type: "local", URL: abs, InstalledAt: time.Now().UTC()},
	}, nil
}

func resolveGitHub(ctx context.Context, raw, cacheRoot string) (Resolved, error) {
	repo, ref, subdir, cloneURL, err := parseGitHub(raw)
	if err != nil {
		return Resolved{}, err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return Resolved{}, errors.New("git is required for GitHub sources in V1")
	}
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return Resolved{}, err
	}
	tmp, err := os.MkdirTemp(cacheRoot, "git-*")
	if err != nil {
		return Resolved{}, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	cloneArgs := []string{"clone", "--depth", "1"}
	if ref != "" {
		cloneArgs = append(cloneArgs, "--no-checkout")
	}
	cloneArgs = append(cloneArgs, "--", cloneURL, tmp)
	cmd := exec.CommandContext(ctx, "git", cloneArgs...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		cleanup()
		return Resolved{}, fmt.Errorf("git clone failed: %s", strings.TrimSpace(string(out)))
	}

	if ref != "" {
		fetch := exec.CommandContext(ctx, "git", "-C", tmp, "fetch", "--depth", "1", "origin", ref)
		fetch.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		out, err = fetch.CombinedOutput()
		if err != nil {
			cleanup()
			return Resolved{}, fmt.Errorf("could not resolve Git ref %q: %s", ref, strings.TrimSpace(string(out)))
		}
		checkout := exec.CommandContext(ctx, "git", "-C", tmp, "checkout", "--detach", "FETCH_HEAD")
		if out, err = checkout.CombinedOutput(); err != nil {
			cleanup()
			return Resolved{}, fmt.Errorf("could not checkout Git ref %q: %s", ref, strings.TrimSpace(string(out)))
		}
	}

	commitOut, err := exec.CommandContext(ctx, "git", "-C", tmp, "rev-parse", "HEAD").Output()
	if err != nil {
		cleanup()
		return Resolved{}, fmt.Errorf("could not determine resolved commit: %w", err)
	}

	root := tmp
	if subdir != "" {
		clean := filepath.Clean(filepath.FromSlash(subdir))
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			cleanup()
			return Resolved{}, errors.New("invalid GitHub subdirectory")
		}
		root = filepath.Join(tmp, clean)
	}
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err != nil {
		cleanup()
		return Resolved{}, errors.New("GitHub source path does not contain SKILL.md")
	}

	hash, err := fsutil.HashDir(root)
	if err != nil {
		cleanup()
		return Resolved{}, err
	}
	now := time.Now().UTC()
	return Resolved{
		Name: filepath.Base(root),
		Path: root,
		Source: core.Source{
			Type:        "git",
			URL:         cloneURL,
			Repository:  repo,
			Subdir:      subdir,
			Commit:      strings.TrimSpace(string(commitOut)),
			Version:     ref,
			InstalledAt: now,
			UpdatedAt:   now,
			ContentHash: hash,
		},
		Cleanup: cleanup,
	}, nil
}

func parseGitHub(raw string) (repo, ref, subdir, cloneURL string, err error) {
	var path string
	if strings.HasPrefix(raw, "github:") {
		path = strings.TrimPrefix(raw, "github:")
	} else {
		u, e := url.Parse(raw)
		if e != nil {
			err = e
			return
		}
		if u.Host != "github.com" {
			err = errors.New("only github.com sources are supported")
			return
		}
		path = strings.TrimPrefix(u.Path, "/")
	}
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		err = errors.New("GitHub source must be owner/repo[@ref][/path]")
		return
	}
	owner := parts[0]
	repoSpec := strings.TrimSuffix(parts[1], ".git")
	if at := strings.Index(repoSpec, "@"); at >= 0 {
		ref = repoSpec[at+1:]
		repoSpec = repoSpec[:at]
		if ref == "" {
			err = errors.New("Git ref after @ cannot be empty")
			return
		}
	}
	if repoSpec == "" {
		err = errors.New("GitHub repository name cannot be empty")
		return
	}
	repo = owner + "/" + repoSpec
	if len(parts) > 2 {
		subdir = strings.Join(parts[2:], "/")
	}
	cloneURL = "https://github.com/" + repo + ".git"
	return
}
