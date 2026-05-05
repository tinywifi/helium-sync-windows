# Contributing

`main` is the Go port. The pre-port Python implementation is preserved on the `original` branch.

## Setup

```powershell
go test ./...
.\scripts\build.ps1
bin\helium-sync.bat doctor
```

## Layout

- `cmd/helium-sync/`: CLI flag parsing and command dispatch.
- `internal/heliumsync/app.go`: application state and target registry helpers.
- `internal/heliumsync/config.go`: config, defaults, path resolution, timestamps.
- `internal/heliumsync/git.go`: git wrapper helpers.
- `internal/heliumsync/sync_workflow.go`: `push`, `pull`, and dry-run flows.
- `internal/heliumsync/status_diff.go`: `status`, `diff`, and bookmark diff formatting.
- `internal/heliumsync/export_import.go`: portable JSON export/import.
- `internal/heliumsync/maintenance.go`: `doctor`, `version`, `log`, `gc`, `init`, `adopt`, `restore`.
- `internal/heliumsync/setup_completion.go`: first-run setup and shell completion output.
- `internal/heliumsync/resolve.go`: Bubble Tea resolver UI.
- `internal/heliumsync/bookmarks.go`: bookmarks target.
- `internal/heliumsync/bookmark_tree.go`: bookmark tree walking and semantic comparison helpers.
- `internal/heliumsync/saved_tab_groups.go`: saved tab groups target.
- `internal/heliumsync/leveldb.go`: LevelDB copy/read support.
- `internal/heliumsync/protowire_saved_tab_groups.go`: protobuf wire encode/decode.
- `internal/heliumsync/validation.go`: target validation.
- `internal/heliumsync/json_helpers.go` and `fileutil.go`: small shared helpers.

## Testing

Run the Go tests before submitting changes:

```powershell
go test ./...
```

For local command testing:

```powershell
go run ./cmd/helium-sync --repo <repo> --profile <profile> status
```

Keep behavior changes covered by focused Go tests in `internal/heliumsync`.
