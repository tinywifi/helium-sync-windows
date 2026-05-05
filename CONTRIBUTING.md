# Contributing

## Scope

This is the Windows fork of [aadarwal/helium-sync](https://github.com/aadarwal/helium-sync). In scope:

- Bug fixes for Helium on Windows
- Improvements to the Bookmarks and SavedTabGroups sync targets
- New optional sync targets that conform to the `Target` protocol in `bin/targets/__init__.py`
- Tests, CI, docs, build tooling

Out of scope:

- macOS or Linux support (use the [original](https://github.com/aadarwal/helium-sync))
- Bidirectional automatic merge (push-overwrites-canonical is intentional)
- Browser-extension form factor
- History/cookies/passwords/extensions sync

## Dev setup

```
git clone https://github.com/tinywifi/helium-sync-windows
cd helium-sync-windows
python -m venv .venv
.venv\Scripts\pip install -r requirements.txt
bin\_go\build.ps1
.venv\Scripts\python -m unittest discover tests/
```

## Tests

Pure-Python tests run on every CI push and PR. Real-Helium tests (`TestRealHelium`, `TestApply`) auto-skip when no Helium profile is present. Override the path with `HELIUM_PROFILE=...` if your install is non-standard.

## Architecture

- `bin\helium-sync` — CLI entry point. Auto-relaunches under `.venv\Scripts\python.exe`.
- `bin\targets\__init__.py` — `Target` protocol. New sync targets conform to this and register in `ALL_TARGETS`.
- `bin\targets\bookmarks.py` — JSON file I/O.
- `bin\targets\saved_tab_groups.py` — protobuf decode/encode + LevelDB read/write via `bin\leveldb-writer.exe`.
- `bin\_go\leveldb_writer\main.go` — Go binary: `read` dumps DB to JSON, `write` applies a batch of ops.
- `proto\` — vendored Chromium proto schemas.

## PR conventions

- One concept per PR.
- All CI checks must be green before merge.
- Squash on merge.
