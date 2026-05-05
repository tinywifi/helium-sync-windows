# helium-sync (Windows)

[![tests](https://github.com/tinywifi/helium-sync-windows/actions/workflows/test.yml/badge.svg)](https://github.com/tinywifi/helium-sync-windows/actions/workflows/test.yml)
[![license: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

This is a Windows fork of [`aadarwal/helium-sync`](https://github.com/aadarwal/helium-sync), which was originally macOS-only. This fork rewrites the tool in Go for Windows.

Bidirectional sync of Helium browser bookmarks and saved tab groups across Windows machines, using your own private git repo as the transport.

```powershell
helium-sync push     # snapshot live profile -> git repo -> origin
helium-sync pull     # git pull -> write canonical state to live profile
```

Two devices, one user, sequential. Last push wins. Close Helium before `pull`; `push` can read bookmarks while Helium is running.

## Install

### via Scoop

```powershell
scoop bucket add tinywifi https://github.com/tinywifi/scoop-bucket
scoop update
scoop install helium-sync
```

If you already have it installed:

```powershell
scoop update
scoop update helium-sync
```

### from source

```powershell
git clone https://github.com/tinywifi/helium-sync-windows
cd helium-sync-windows
go test ./...
.\scripts\build.ps1
bin\helium-sync.bat setup
```

Prerequisites:

- Go 1.23+
- Git

## Commands

Core workflow:

```powershell
helium-sync setup
helium-sync push
helium-sync pull
```

Inspect and debug:

```powershell
helium-sync status
helium-sync diff
helium-sync doctor
helium-sync version
helium-sync log -n 20
```

Portable backup:

```powershell
helium-sync export --output backup.json
helium-sync import backup.json --allow-helium-running
```

Maintenance and recovery:

```powershell
helium-sync init --force
helium-sync adopt --yes
helium-sync push --dry-run
helium-sync pull --dry-run
		helium-sync restore
		helium-sync gc --keep-days 30
```

Shared flags:

- `--profile <path>`: Helium profile directory.
- `--repo <path>`: private data repo path.
- `--target <bookmarks|saved_tab_groups>`: limit commands to one target.
- `--strict`: fail `push` on validation warnings.
- `--allow-helium-running`: bypass write guard for tests.

## Synced Data

Bookmarks are stored in `Default\Bookmarks` as JSON. The checksum is cleared during extraction so Chromium can recompute it per device.

Saved tab groups are protobuf values in `Default\Sync Data\LevelDB\` under keys like `saved_tab_group-dt-<UUID>`. The Go port reads LevelDB through a temporary copy for safe extraction and writes through LevelDB batches when applying canonical state.

## Not Synced

History, cookies, passwords, extensions, settings, themes, and live open tabs.
