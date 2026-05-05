"""Saved tab groups sync target.

Reads from and writes to Helium's Sync Data LevelDB via the bundled
`leveldb-writer` binary (bin/leveldb-writer[.exe]). The binary must be built
from bin/_go/ before use.

Because the Go reader uses LevelDB's standard open (which acquires the LOCK
file), Helium must be quit before extract() or apply() is called. The CLI
guards this at the command level.

Each entry under `saved_tab_group-dt-<UUID>` is a sync-engine local-model
envelope (LocalEntityWrapper) wrapping a SavedTabGroupSpecifics proto from
upstream Chromium.
"""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

_PROTO_DIR = Path(__file__).resolve().parent / "_proto"
sys.path.insert(0, str(_PROTO_DIR))
import local_entity_wrapper_pb2 as _wrapper_pb       # noqa: E402
import saved_tab_group_specifics_pb2 as _stg_pb      # noqa: E402


KEY_PREFIX = "saved_tab_group-dt-"


class SavedTabGroups:
    name = "saved_tab_groups"
    state_filename = "saved_tab_groups.json"

    def _leveldb_dir(self, profile_dir: Path) -> Path:
        return profile_dir / "Default" / "Sync Data" / "LevelDB"

    # ------------------------------------------------------------------ #
    # Extract — Helium must be quit
    # ------------------------------------------------------------------ #

    def extract(self, profile_dir: Path) -> dict:
        ldb_dir = self._leveldb_dir(profile_dir)
        if not ldb_dir.exists():
            raise FileNotFoundError(f"no LevelDB at {ldb_dir}")

        entries = _read_leveldb(ldb_dir)

        groups: dict[str, dict] = {}
        tabs: dict[str, dict] = {}

        for entry in entries:
            key = entry["key"]
            if not key.startswith(KEY_PREFIX):
                continue

            val = bytes.fromhex(entry["val_hex"])

            wrapper = _wrapper_pb.LocalEntityWrapper()
            wrapper.ParseFromString(val)
            spec = _stg_pb.SavedTabGroupSpecifics()
            spec.ParseFromString(wrapper.specifics)

            base = {
                "guid":          spec.guid,
                "creation_time": spec.creation_time_windows_epoch_micros,
                "update_time":   spec.update_time_windows_epoch_micros,
                "version":       spec.version,
            }
            entity = spec.WhichOneof("entity")
            if entity == "group":
                g = spec.group
                groups[spec.guid] = {
                    **base,
                    "title":    g.title,
                    "color":    int(g.color),
                    "position": g.position,
                }
            elif entity == "tab":
                t = spec.tab
                tabs[spec.guid] = {
                    **base,
                    "group_guid": t.group_guid,
                    "url":        t.url,
                    "title":      t.title,
                    "position":   t.position,
                }

        return {"groups": groups, "tabs": tabs}

    # ------------------------------------------------------------------ #
    # Apply — write merged state into the live LevelDB
    # ------------------------------------------------------------------ #

    def apply(self, profile_dir: Path, data: dict, backup_dir: Path) -> None:
        """Replace saved-tab-group entries in the live LevelDB with `data`.

        Preconditions:
          - Helium MUST not be running (it holds an exclusive lock on LOCK).
          - bin/leveldb-writer[.exe] must exist (built from bin/_go/).

        Steps:
          1. Snapshot the entire LevelDB directory to backup_dir/LevelDB/.
          2. Compute the diff: which keys to put, which to delete.
          3. Encode each group/tab as LocalEntityWrapper(SavedTabGroupSpecifics).
          4. Invoke the Go writer.
        """
        ldb_dir = self._leveldb_dir(profile_dir)
        if not ldb_dir.exists():
            raise FileNotFoundError(f"no LevelDB at {ldb_dir}")

        # 1. Backup
        backup_dir.mkdir(parents=True, exist_ok=True)
        backup_target = backup_dir / "LevelDB"
        if backup_target.exists():
            shutil.rmtree(backup_target)
        shutil.copytree(ldb_dir, backup_target)

        # 2. Diff
        existing = self.extract(profile_dir)
        existing_keys = (
            {f"{KEY_PREFIX}{guid}" for guid in existing.get("groups", {})}
            | {f"{KEY_PREFIX}{guid}" for guid in existing.get("tabs", {})}
        )
        target_keys = (
            {f"{KEY_PREFIX}{guid}" for guid in data.get("groups", {})}
            | {f"{KEY_PREFIX}{guid}" for guid in data.get("tabs", {})}
        )

        ops: list[dict] = []
        for guid, g in data.get("groups", {}).items():
            ops.append({
                "op": "put",
                "key": f"{KEY_PREFIX}{guid}",
                "val_hex": _encode_group(g).hex(),
            })
        for guid, t in data.get("tabs", {}).items():
            ops.append({
                "op": "put",
                "key": f"{KEY_PREFIX}{guid}",
                "val_hex": _encode_tab(t).hex(),
            })
        for key in existing_keys - target_keys:
            ops.append({"op": "delete", "key": key})

        # 3. Invoke Go writer
        tool = _go_tool()
        with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as f:
            json.dump(ops, f)
            ops_path = f.name
        try:
            r = subprocess.run(
                [str(tool), "write", "-db", str(ldb_dir), "-ops", ops_path],
                check=False, capture_output=True, text=True,
            )
            if r.returncode != 0:
                raise RuntimeError(
                    f"leveldb-writer write failed (exit {r.returncode}):\n"
                    f"stdout: {r.stdout}\nstderr: {r.stderr}"
                )
        finally:
            Path(ops_path).unlink(missing_ok=True)

    # ------------------------------------------------------------------ #
    # Serialization — deterministic JSON
    # ------------------------------------------------------------------ #

    def serialize(self, data: dict) -> str:
        groups = {k: data["groups"][k] for k in sorted(data.get("groups", {}))}
        tabs = {k: data["tabs"][k] for k in sorted(data.get("tabs", {}))}
        return json.dumps({"groups": groups, "tabs": tabs}, indent=2, sort_keys=True)

    def deserialize(self, text: str) -> dict:
        return json.loads(text)

    # ------------------------------------------------------------------ #
    # Semantic equality — used for status output, not branching logic
    # ------------------------------------------------------------------ #

    def semantically_equal(self, a: dict, b: dict) -> bool:
        ag, bg = a.get("groups", {}), b.get("groups", {})
        at, bt = a.get("tabs", {}), b.get("tabs", {})
        if set(ag) != set(bg) or set(at) != set(bt):
            return False
        for k, ag_k in ag.items():
            bg_k = bg[k]
            if (ag_k.get("title") != bg_k.get("title") or
                    ag_k.get("color") != bg_k.get("color") or
                    ag_k.get("position") != bg_k.get("position")):
                return False
        for k, at_k in at.items():
            bt_k = bt[k]
            if (at_k.get("group_guid") != bt_k.get("group_guid") or
                    at_k.get("url") != bt_k.get("url") or
                    at_k.get("title") != bt_k.get("title") or
                    at_k.get("position") != bt_k.get("position")):
                return False
        return True


# ---------------------------------------------------------------------------- #
# Helpers
# ---------------------------------------------------------------------------- #

def _go_tool() -> Path:
    name = "leveldb-writer.exe" if sys.platform == "win32" else "leveldb-writer"
    tool = Path(__file__).resolve().parent.parent / name
    if not tool.exists():
        if sys.platform == "win32":
            hint = r"Build it: run bin\_go\build.ps1 in PowerShell"
        else:
            hint = "Build it: bin/_go/build.sh"
        raise FileNotFoundError(f"leveldb-writer not found at {tool}. {hint}")
    return tool


def _read_leveldb(ldb_dir: Path) -> list[dict]:
    tool = _go_tool()
    r = subprocess.run(
        [str(tool), "read", "-db", str(ldb_dir)],
        capture_output=True, text=True,
    )
    if r.returncode != 0:
        err = (r.stderr or "").strip()
        if "lock" in err.lower():
            raise RuntimeError(
                "LevelDB is locked — Helium must be closed before syncing saved tab groups"
            )
        raise RuntimeError(f"leveldb-writer read failed: {err}")
    return json.loads(r.stdout)


def _encode_group(g: dict) -> bytes:
    spec = _stg_pb.SavedTabGroupSpecifics()
    spec.guid = g["guid"]
    spec.creation_time_windows_epoch_micros = int(g.get("creation_time", 0))
    spec.update_time_windows_epoch_micros = int(g.get("update_time", 0))
    if "version" in g:
        spec.version = int(g["version"])
    spec.group.title = g.get("title", "")
    spec.group.color = int(g.get("color", 0))
    spec.group.position = int(g.get("position", 0))

    wrapper = _wrapper_pb.LocalEntityWrapper()
    wrapper.marker = 1
    wrapper.specifics = spec.SerializeToString()
    return wrapper.SerializeToString()


def _encode_tab(t: dict) -> bytes:
    spec = _stg_pb.SavedTabGroupSpecifics()
    spec.guid = t["guid"]
    spec.creation_time_windows_epoch_micros = int(t.get("creation_time", 0))
    spec.update_time_windows_epoch_micros = int(t.get("update_time", 0))
    if "version" in t:
        spec.version = int(t["version"])
    spec.tab.group_guid = t.get("group_guid", "")
    spec.tab.url = t.get("url", "")
    spec.tab.title = t.get("title", "")
    spec.tab.position = int(t.get("position", 0))

    wrapper = _wrapper_pb.LocalEntityWrapper()
    wrapper.marker = 1
    wrapper.specifics = spec.SerializeToString()
    return wrapper.SerializeToString()
