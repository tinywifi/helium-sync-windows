# helium-sync (Windows)

[![tests](https://github.com/tinywifi/helium-sync-windows/actions/workflows/test.yml/badge.svg)](https://github.com/tinywifi/helium-sync-windows/actions/workflows/test.yml)
[![license: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Windows fork of [aadarwal/helium-sync](https://github.com/aadarwal/helium-sync)**, which was macOS-only. This fork ports the tool to Windows.

Bidirectional sync of [Helium browser](https://helium.computer) bookmarks + saved tab groups across Windows machines, using your own private git repo as the transport. CLI, no extension.

```
helium-sync push     # done on this device  → git push
helium-sync pull     # starting on this one → git pull → write to live profile
```

Two devices, one user, sequential. Last push wins. Close Helium before pull; push is safe while Helium runs.

## install

### via Scoop (recommended)

```powershell
scoop bucket add tinywifi https://github.com/tinywifi/scoop-bucket
scoop install helium-sync
helium-sync setup
```

Scoop handles the venv and `requirements.txt` automatically. No Go or manual PATH setup needed.

### from source

```powershell
git clone https://github.com/tinywifi/helium-sync-windows
cd helium-sync-windows
python -m venv .venv
.venv\Scripts\pip install -r requirements.txt
bin\_go\build.ps1        # builds bin\leveldb-writer.exe — requires Go
bin\helium-sync.bat setup
```

### prerequisites (from source only)

- Python 3.11+
- [Go](https://go.dev/dl/) (only needed to build `leveldb-writer.exe`; pre-built `.exe` included in releases)
- Git

## commands

### core workflow

```
helium-sync setup     interactive first-time configuration
helium-sync push      snapshot live → git push     (Helium can stay open)
helium-sync pull      git pull → write to live     (close Helium first)
```

### inspect & debug

```
helium-sync status    diff live vs canonical state
helium-sync diff      human-readable diff of bookmarks (live ≠ canonical)
helium-sync doctor    check git, Python venv, profile, repo, remote, Scoop
helium-sync version   print version, git revision, Python runtime
helium-sync log -n 20 show recent sync commits (default: 10)
```

### portable backup

```
helium-sync export [--output path.json] [--target bookmarks]
helium-sync import file.json [--target bookmarks] [--allow-helium-running]
```

### maintenance

```
helium-sync init      lower-level: bootstrap on the source-of-truth device (--force, --target)
helium-sync adopt     lower-level: bootstrap on a new device receiving canonical
helium-sync gc        prune logs/ backups older than 30 days (--keep-days, --dry-run)
```

### safety & recovery

```
helium-sync push --dry-run    show what would change without committing or pushing
helium-sync pull --dry-run    show what would change without pulling or writing
helium-sync restore           restore profile from latest backup (prePull/preImport/preSync)
helium-sync resolve           interactively merge divergent bookmark states (--theirs file.json)
```

### shell completion

```
helium-sync completion --shell powershell    generate PowerShell tab completion
helium-sync completion --shell cmd           generate DOSKEY macros for cmd.exe
```

### shared flags

`--target <name>` — limit push, pull, status, diff, export, or import to one target
(e.g. `--target bookmarks`). Useful when only bookmarks changed.

`--strict` — used with `push`. Fails the push if any validation finds empty URLs,
invalid schemes, broken folder hierarchies, or tab group issues. Without it, push
warns but still proceeds.

`--allow-helium-running` — bypass the running-browser guard on import and pull.
**Dangerous** — can corrupt LevelDB. Only use for testing.

Discipline: pull at start of session, push at end. Backups under `logs\prePull.<ts>\` if you slip.

## architecture

Two targets, two formats, one transport.

**Bookmarks** — `Default\Bookmarks` is JSON. Read it, zero `checksum`, write back atomically.

**Saved tab groups** — protobuf entries keyed `saved_tab_group-dt-<UUID>` in a LevelDB at `Default\Sync Data\LevelDB\`. Both read and write go through a small Go binary (`bin\leveldb-writer.exe`, ~100 lines using `syndtr/goleveldb`). Because the Go binary opens LevelDB normally (acquiring the lock), Helium must be closed for both push and pull.

**Transport** — real git. `push` does `git add state\ && git commit && git push`. `pull` does `git pull --rebase`. State is plain JSON, so `git log` is your sync history and `git diff` is your bookmark diff. No three-way merge: "last push wins" is simpler than reasoning about divergent histories.

## what's not synced

History, cookies, passwords, extensions, settings, themes, live open tabs.

## differences from the macOS original

| | [aadarwal/helium-sync](https://github.com/aadarwal/helium-sync) | this fork |
|---|---|---|
| Platform | macOS (arm64 + Intel) | Windows |
| Profile path | `~/Library/Application Support/net.imput.helium` | `%LOCALAPPDATA%\imput\Helium\User Data` |
| Config path | `~/.config/helium-sync/config.toml` | `%APPDATA%\helium-sync\config.toml` |
| LevelDB reads | `leveldbutil dump` (no lock, push with Helium open) | temp-copy trick (no lock, push with Helium open) |
| Push while Helium open | yes | yes |
| Install | `brew install aadarwal/tap/helium-sync` | clone + venv + build.ps1 |

---

Fork of [aadarwal/helium-sync](https://github.com/aadarwal/helium-sync), MIT-licensed.
