# helium-sync {{TAG}}

## Should you update?

Yes. This release adds support commands for debugging installs and fixes Scoop packaging behavior.

## Highlights

- Added `helium-sync doctor` to check git, Python venv, `leveldb-writer.exe`, profile path, repo config, remote setup, and Scoop install detection.
- Added `helium-sync version` to print the app version, git revision, Python runtime, and `leveldb-writer.exe` status.
- Fixed Scoop post-install venv creation to use the versioned install directory instead of the `current` symlink path.
- Added Scoop uninstall guidance so config and sync data are intentionally left in place.
- Added a doctor CLI fixture, Scoop autoupdate validation, curated release notes, and release dry-build coverage in CI.

## Install or update

```powershell
scoop bucket add tinywifi https://github.com/tinywifi/scoop-bucket
scoop update
scoop install helium-sync
```

For existing installs:

```powershell
scoop update helium-sync
```

## Changes

GitHub generated notes are appended below.
