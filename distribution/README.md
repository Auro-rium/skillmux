# Formal distribution

Skillmux publishes immutable release archives and SHA-256 checksums from GitHub Releases.

Supported first-party distribution artifacts:

- Unix installer: `install.sh`
- Windows installer: `install.ps1`
- Homebrew formula: `Formula/skillmux.rb`
- Scoop manifest: `bucket/skillmux.json`
- WinGet submission manifests: `distribution/winget/manifests/`

The Homebrew and Scoop files in this repository are release manifests, not the official Homebrew core or Scoop Main registries. Those registries require their own review/submission process. Homebrew documents third-party taps and direct qualified installs; Scoop supports custom buckets. WinGet's community repository accepts versioned multi-file YAML manifests.

The release workflow regenerates package-manager metadata from the exact published archive checksums.

For WinGet, the versioned manifest directory is submission-ready for the Microsoft community repository. After acceptance there, the public install command becomes:

```powershell
winget install --id Auro-rium.Skillmux -e
```

For a private/first-party Scoop bucket:

```powershell
scoop bucket add skillmux https://github.com/Auro-rium/skillmux.git
scoop install skillmux
```

For Homebrew, a dedicated `homebrew-skillmux` tap repository is required before the short `brew install skillmux` form is possible. Until that tap exists, the checked-in formula remains the canonical packaging definition.

No installer executes a project script. Release archives contain only the native executable plus checksum/install metadata.
