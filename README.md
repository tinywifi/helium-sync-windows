# helium-sync (Windows)

Bidirectional sync of Helium browser bookmarks and saved tab groups across Windows machines, using your own private git repo as the transport.

```powershell
helium-sync push     # snapshot live profile -> git repo -> origin
helium-sync pull     # git pull -> write canonical state to live profile
```

Two devices, one user, sequential. Last push wins. Close Helium before `pull`; `push` can read bookmarks while Helium is running.

## Install From Source

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
helium-sync resolve --target bookmarks
helium-sync gc --keep-days 30
```

Shared flags:

- `--profile <path>`: Helium profile directory.
- `--repo <path>`: private data repo path.
- `--target <bookmarks|saved_tab_groups>`: limit commands to one target.
- `--strict`: fail `push` on validation warnings.
- `--allow-helium-running`: bypass write guard for tests.

## Architecture

The project is now a Go CLI.

- `cmd/helium-sync`: command-line flag parsing and dispatch.
- `internal/heliumsync`: sync workflows, targets, import/export, restore, resolver TUI, and utilities.
- `internal/heliumsync/bookmarks.go`: Chromium bookmark JSON handling.
- `internal/heliumsync/saved_tab_groups.go`: saved tab group state handling.
- `internal/heliumsync/leveldb.go`: LevelDB read/write support via `syndtr/goleveldb`.
- `internal/heliumsync/protowire_saved_tab_groups.go`: direct protobuf wire encode/decode for Chromium saved tab group specifics.

Charm libraries are used throughout the Go port:

- Bubble Tea, Bubble Tree, and Bubbles for the interactive resolver UI.
- Huh for setup/adopt prompts.
- Lip Gloss for terminal styling.
- Log for structured internal logging.
- Harmonica for resolver animation state.

## Synced Data

Bookmarks are stored in `Default\Bookmarks` as JSON. The checksum is cleared during extraction so Chromium can recompute it per device.

Saved tab groups are protobuf values in `Default\Sync Data\LevelDB\` under keys like `saved_tab_group-dt-<UUID>`. The Go port reads LevelDB through a temporary copy for safe extraction and writes through LevelDB batches when applying canonical state.

## Not Synced

History, cookies, passwords, extensions, settings, themes, and live open tabs.
