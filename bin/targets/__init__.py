"""Sync targets registry.

Each sync target (bookmarks, saved tab groups, etc.) lives in its own module
and conforms to the protocol below. The CLI iterates over `ALL_TARGETS` to
push or pull, treating each one uniformly.

A target is responsible for:
  - extract(profile_dir)  → read live state from Helium's profile directory
                            (must be safe while Helium is running; may use
                            a tmpdir snapshot for files Helium has locked)
  - apply(profile_dir, data, backup_dir)
                          → write data into Helium's profile, replacing live
                            state. Caller MUST ensure Helium is not running.
                            A timestamped backup of pre-existing state is
                            written to backup_dir first.
  - serialize(data)       → render data as canonical text for state/<file>;
                            output must be deterministic
  - deserialize(text)     → inverse of serialize
  - semantically_equal(a, b)
                          → True if a and b represent the same user-visible
                            state (used for status reporting)

Targets carry two attributes:
  - name           : short identifier, e.g. "bookmarks"
  - state_filename : filename within state/, e.g. "bookmarks.json"

## CLI Commands (bin/helium-sync)
All commands auto-relaunch under .venv/Scripts/python.exe if not already.

### Core
  setup          — interactive first-time configuration (writes %APPDATA%/helium-sync/config.toml)
  push           — snapshot live → git commit → git push (--target, --strict, --dry-run)
  pull           — git pull --rebase → write to live profile (--target, --dry-run, --allow-helium-running)

### Inspect & debug
  status         — diff live vs canonical state (--target)
  diff           — human-readable bookmark diff (live ≠ canonical) (--target)
  doctor         — check git, Python venv, profile, repo, remote, Scoop
  version        — print version, git revision, Python runtime
  log            — show recent sync commits (-n count)

### Portable backup
  export         — export canonical state to JSON (--output, --target)
  import         — import JSON into canonical state (--target, --allow-helium-running)

### Safety & recovery
  restore        — restore profile from latest logs/prePull or preImport or preSync backup
  resolve        — interactive TUI to merge divergent states (--target bookmarks|saved_tab_groups, --theirs)
                   ↑↓ navigate, Space toggle, Enter apply, Q cancel.
                   Handles local-only, remote-only, and name/title conflicts.

### Shell completion
  completion     — generate PowerShell or cmd.exe completion scripts (--shell powershell|cmd)

### Maintenance
  init           — lower-level: bootstrap on source-of-truth device (--force, --target)
  adopt          — lower-level: bootstrap on new device receiving canonical (--yes)
  gc             — prune logs/ backups older than 30 days (--keep-days, --dry-run)

## Flags
  --target <name>         limit operation to one target (e.g. bookmarks)
  --strict                fail push on validation errors (empty URLs, invalid schemes)
  --dry-run               show what would change without committing/pushing/writing
  --allow-helium-running  bypass the running-browser guard (DANGEROUS)
  --profile <path>        override auto-detected Helium profile path
  --repo <path>           override auto-detected sync repo path

## Tests
  Pure-Python tests in tests/ run on every CI push and PR.
  Real-Helium tests (TestRealHelium, TestApply) auto-skip without a Helium profile.
  Override with HELIUM_PROFILE=... for non-standard installs.
  Run: python -m unittest discover tests/
  New features must have corresponding test cases in tests/test_<feature>.py.

## Architecture
  bin/helium-sync          — CLI entry point (auto-relaunches under venv Python)
  bin/targets/             — sync target modules (bookmarks, saved_tab_groups)
  bin/_go/leveldb_writer/  — Go binary source for LevelDB read/write
  proto/                   — vendored Chromium proto schemas
  tests/                   — pure-Python unit tests
  .github/ISSUE_TEMPLATE/  — interactive YAML issue forms
  .github/workflows/       — CI (test.yml) + release (release.yml on tag push)

## Platform
  Windows only. Profile: %LOCALAPPDATA%/imput/Helium/User Data
  Config: %APPDATA%/helium-sync/config.toml
  Data repo uses real git. Code repo .gitignore excludes state/ and logs/.
"""

from __future__ import annotations

from pathlib import Path
from typing import Protocol, runtime_checkable

from .bookmarks import Bookmarks
from .saved_tab_groups import SavedTabGroups


@runtime_checkable
class Target(Protocol):
    name: str
    state_filename: str

    def extract(self, profile_dir: Path) -> dict: ...
    def apply(self, profile_dir: Path, data: dict, backup_dir: Path) -> None: ...
    def serialize(self, data: dict) -> str: ...
    def deserialize(self, text: str) -> dict: ...
    def semantically_equal(self, a: dict, b: dict) -> bool: ...


# Active targets. Order doesn't matter — push/pull iterates over all.
ALL_TARGETS: list[Target] = [
    Bookmarks(),
    SavedTabGroups(),
]
