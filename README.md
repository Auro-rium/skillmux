# Skillmux

**One skill library. Every agent.**

Skillmux is a local-first CLI and terminal UI for managing reusable `SKILL.md` agent skills across Codex, Claude Code, Gemini CLI, Cursor, OpenCode, and other agent harnesses.

It keeps one canonical skill library, detects drift across harnesses, plans deterministic syncs, and refuses to silently overwrite differing unmanaged copies.

> Status: public V1 development. Releases are versioned, checksummed, cross-platform, and built automatically from `main`.

## Install

### macOS / Linux

The installer downloads the correct release binary and verifies it against the published SHA-256 checksums before installing:

```bash
curl -fsSL https://raw.githubusercontent.com/Auro-rium/skillmux/main/install.sh | sh
```

By default Skillmux is installed to `~/.local/bin/skillmux`. Override with `SKILLMUX_BIN_DIR` when needed.

Install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/Auro-rium/skillmux/main/install.sh | SKILLMUX_VERSION=v0.1.0 sh
```

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/Auro-rium/skillmux/main/install.ps1 | iex
```

The PowerShell installer also verifies the release SHA-256 checksum before installing.

### Homebrew

Skillmux publishes a Homebrew formula from this repository on every release:

```bash
brew tap Auro-rium/skillmux https://github.com/Auro-rium/skillmux
brew install Auro-rium/skillmux/skillmux
```

### Scoop

```powershell
scoop bucket add skillmux https://github.com/Auro-rium/skillmux
scoop install skillmux/skillmux
```

### WinGet

The repository carries versioned, checksum-pinned manifests ready for the Microsoft WinGet community repository:

```powershell
winget install --id Auro-rium.Skillmux -e
```

That command becomes available after the package is accepted into the public WinGet source. Until then, the submission-ready manifests live under `distribution/winget/manifests/`.

### Go

```bash
go install github.com/Auro-rium/skillmux/cmd/skillmux@latest
```

### Release binaries

Every release publishes native archives for:

- macOS ARM64
- macOS x86_64
- Linux ARM64
- Linux x86_64
- Windows x86_64
- SHA-256 checksums for every archive

GitHub Releases are the source of truth for binary distribution.

## Five-minute quickstart

See what already exists:

```bash
skillmux scan
```

Adopt identical unmanaged copies automatically:

```bash
skillmux import
```

If a skill differs across harnesses, choose the copy you actually want:

```bash
skillmux diff backend-review --target claude
skillmux import backend-review --from claude
```

Enable the canonical skill where you want it:

```bash
skillmux enable backend-review --target codex
skillmux enable backend-review --target claude
skillmux enable backend-review --target gemini
```

Preview the deterministic plan, then apply:

```bash
skillmux sync --dry-run
skillmux sync
```

Add a skill from disk or GitHub:

```bash
skillmux add ./my-skill --target codex,claude
skillmux add github:user/backend-review --target codex,claude,gemini
skillmux add github:user/repo@v2/path/to/skill
```

Run static diagnostics:

```bash
skillmux doctor
skillmux doctor --json
```

## Why Skillmux exists

Agent harnesses discover skills from different directories and scopes. Manually copying the same skill into several of them creates stale copies, conflicting versions, broken references, and behavior that differs by harness.

Skillmux sits underneath those harnesses. It is not another agent framework.

Canonical content lives in Skillmux storage. Harness directories receive symlinks where that is safe and supported, or regular copies where it is not. Copied state is hashed so drift can be detected.

## Supported harnesses

V1 ships adapters for Codex, Claude Code, Gemini CLI, Cursor, and OpenCode.

Harness detection is best-effort. A harness does not need to be installed for the rest of Skillmux to work.

## CLI

```text
skillmux
skillmux scan
skillmux status
skillmux list
skillmux get NAME
skillmux search QUERY
skillmux add SOURCE
skillmux import [NAME]
skillmux remove NAME
skillmux update [NAME]
skillmux sync
skillmux diff NAME
skillmux doctor
skillmux enable NAME --target HARNESS
skillmux disable NAME --target HARNESS
skillmux profile list
skillmux profile show NAME
skillmux profile create NAME
skillmux profile use NAME
skillmux profile delete NAME
skillmux eject [NAME]
skillmux completion bash|zsh|fish|powershell
skillmux version
```

Read-heavy commands support stable JSON where useful:

```bash
skillmux scan --json
skillmux status --json
skillmux list --json
skillmux get backend-review --json
skillmux doctor --json
skillmux sync --dry-run --json
```

## Canonical storage

On Unix-like systems:

```text
~/.skillmux/
├── skills/
├── profiles/
├── sources/
├── cache/
├── backups/
└── state.json
```

Set `SKILLMUX_HOME` to override the root. Tests use temporary roots and never touch the developer's real home directory.

On Windows, Skillmux prefers `%LOCALAPPDATA%\Skillmux` when available.

Skill content stays as normal files. Skillmux management metadata is separate; it does not rewrite `SKILL.md` just to tag it.

## Project reproducibility

A repository may declare its intended environment in `.skillmux.toml`:

```toml
version = 1

[skills]
backend-review = "github:example/backend-review@v2"
postgres-debug = "github:example/postgres-debug@89ab21c"

[targets]
codex = true
claude = true
gemini = true
cursor = false
```

The lockfile format is `.skillmux.lock` and stores exact commits and hashes. Manifest parsing, pinned Git refs, and lockfile generation are implemented in the project layer; sync integration is being hardened before the first tagged release.

## Safety model

External skills are untrusted content. `skillmux add` never executes their scripts. `skillmux doctor` performs static inspection only.

Sync is deliberately conservative:

- scan is read-only
- differing unmanaged targets are surfaced as conflicts
- replacement requires explicit review and `--force`
- filesystem-changing sync creates backups
- failed sync rolls back applied changes
- `skillmux eject` converts managed links into regular directories
- canonical deletion is explicit with `skillmux remove NAME --canonical`

Doctor reports findings such as destructive command patterns, network-download commands, missing references, platform-specific paths, non-executable shell scripts, broken links, and diverged copies. A clean report is not a claim that a skill is safe.

## Profiles

Profiles snapshot active skills and harness targets:

```bash
skillmux profile create backend
skillmux profile use backend
skillmux sync
```

Disabling a skill changes exposure only; it does not delete canonical content.

## TUI

Running `skillmux` with no arguments opens the terminal interface. No setup command or account is required.

The TUI is intentionally terminal-native: monochrome-first, keyboard-driven, no Nerd Font dependency, no large background fills, and semantic color only for state.

### Layout

Skillmux adapts to terminal width instead of compressing panels until they become useless:

- around 80 columns: skill list only; press Enter for details
- medium terminals: skills + details
- wide terminals: skills + details + recent activity

The render path is tested at `80x24`, `120x30`, and `160x50`.

### Keyboard

```text
Navigation
j / ↓          next
k / ↑          previous
Enter          select
Esc            back
Tab            next panel
Shift+Tab      previous panel

Skills
a              add
space          edit target exposure
e              edit SKILL.md
v              diff
u              update
x              disable exposures
X              delete canonical skill (typed confirmation)

Environment
s              sync
d              doctor
p              profiles
h              harnesses
c              conflicts

Global
/              fuzzy search
Ctrl+P / :     command palette
?              help
q              quit
Ctrl+C         quit immediately
```

### Screens

The TUI includes first-run scan, main skills view, details, live fuzzy search, command palette, add/install, target selection, sync plan/result, conflict resolution, diff, doctor, profiles with switch preview, harness overview, update, empty state, help, destructive confirmation, and error state.

Sync, doctor, conflict, profile, update, and add operations all call the same application/service layer used by the CLI.

### Themes and terminal compatibility

Themes are semantic rather than hardcoded throughout the renderer.

```toml
[tui]
theme = "auto" # auto | dark | light | mono
```

`auto` is the default. Skillmux never assumes a black or white terminal background.

Set `NO_COLOR=1` to suppress TUI color. Set `SKILLMUX_ASCII=1` to force ASCII status markers.

When stdin or stdout is redirected, or when `TERM=dumb`, Skillmux automatically skips the full-screen TUI and emits plain CLI output instead. Subcommands are plain-text by default, so pipelines such as `skillmux list | grep backend` remain clean.

## Development

Clone the repository only when developing Skillmux itself:

```bash
git clone https://github.com/Auro-rium/skillmux.git
cd skillmux
go test ./...
go vet ./...
go build ./cmd/skillmux
```

CI runs on Linux, macOS, and Windows.

```text
cmd/skillmux       CLI entrypoint
internal/app       shared application/service layer
internal/store     canonical local state
internal/harness   harness adapters
internal/scan      discovery + conflict detection
internal/source    local/GitHub source resolution
internal/syncer    deterministic plan + transactional apply
internal/doctor    static diagnostics
internal/project   manifest + lockfile
internal/ui        terminal UI
internal/fsutil    filesystem primitives
```

## Adding a harness

Implement the small `harness.Adapter` interface: name, detection, skill locations, and symlink capability. Keep provider-specific paths inside the adapter rather than spreading them through the sync engine.

## Design constraints

Skillmux is local infrastructure. V1 does not need accounts, telemetry, cloud sync, a marketplace, billing, a daemon, an embedded LLM, or a proprietary skill format.

The boring parts are the product: reproducible state, useful diffs, deterministic plans, conservative mutations, actionable errors, and an escape hatch.
