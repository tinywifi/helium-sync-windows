"""Tests for the `helium-sync export` and `helium-sync import` commands.

Run from the repo root: python -m unittest discover tests/
"""

import importlib.machinery
import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace


def _import_cli():
    cli_path = Path(__file__).resolve().parent.parent / "bin" / "helium-sync"
    sys.path.insert(0, str(cli_path.parent))
    loader = importlib.machinery.SourceFileLoader("helium_sync_cli", str(cli_path))
    spec = importlib.util.spec_from_loader("helium_sync_cli", loader)
    mod = importlib.util.module_from_spec(spec)
    loader.exec_module(mod)
    return mod


class FakeTarget:
    """Minimal target that reads/writes a JSON file in the profile."""
    name = "fake_bookmarks"
    state_filename = "fake_bookmarks.json"

    def extract(self, profile_dir: Path) -> dict:
        return json.loads((profile_dir / "Bookmarks").read_text())

    def apply(self, profile_dir: Path, data: dict, backup_dir: Path) -> None:
        profile_dir.mkdir(parents=True, exist_ok=True)
        backup_dir.mkdir(parents=True, exist_ok=True)
        live = profile_dir / "Bookmarks"
        if live.exists():
            (backup_dir / "Bookmarks").write_text(live.read_text())
        live.write_text(json.dumps(data, sort_keys=True))

    def serialize(self, data: dict) -> str:
        return json.dumps(data, sort_keys=True, indent=2)

    def deserialize(self, text: str) -> dict:
        return json.loads(text)

    def semantically_equal(self, a: dict, b: dict) -> bool:
        return a == b


class TestExport(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def test_export_produces_valid_json(self):
        """Export creates a valid JSON file with correct structure."""
        repo = self.root / "repo"
        repo.mkdir()
        state_dir = repo / "state"
        state_dir.mkdir()

        test_data = {"roots": {"bookmark_bar": {"children": [
            {"type": "url", "name": "Test", "url": "https://test.com"}
        ]}}}
        (state_dir / "fake_bookmarks.json").write_text(json.dumps(test_data))

        old_targets = self.cli.ALL_TARGETS
        old_repo = self.cli.REPO_ROOT
        old_state = self.cli.STATE_DIR
        old_logs = self.cli.LOGS_DIR
        try:
            self.cli.ALL_TARGETS = [FakeTarget()]
            self.cli.REPO_ROOT = repo
            self.cli.STATE_DIR = state_dir
            self.cli.LOGS_DIR = repo / "logs"

            output_file = self.root / "export.json"
            rc = self.cli.cmd_export(SimpleNamespace(
                output=str(output_file), target=None
            ))

            self.assertEqual(rc, 0)
            self.assertTrue(output_file.exists())

            payload = json.loads(output_file.read_text())
            self.assertIn("exported_at", payload)
            self.assertIn("targets", payload)
            self.assertIn("fake_bookmarks", payload["targets"])
            self.assertEqual(payload["targets"]["fake_bookmarks"]["data"], test_data)
        finally:
            self.cli.ALL_TARGETS = old_targets
            self.cli.REPO_ROOT = old_repo
            self.cli.STATE_DIR = old_state
            self.cli.LOGS_DIR = old_logs

    def test_export_default_output_path(self):
        """Export without --output writes to repo with timestamped filename."""
        repo = self.root / "repo"
        repo.mkdir()
        state_dir = repo / "state"
        state_dir.mkdir()
        (state_dir / "fake_bookmarks.json").write_text(json.dumps({"roots": {}}))

        old_targets = self.cli.ALL_TARGETS
        old_repo = self.cli.REPO_ROOT
        old_state = self.cli.STATE_DIR
        old_logs = self.cli.LOGS_DIR
        try:
            self.cli.ALL_TARGETS = [FakeTarget()]
            self.cli.REPO_ROOT = repo
            self.cli.STATE_DIR = state_dir
            self.cli.LOGS_DIR = repo / "logs"

            import io
            captured = io.StringIO()
            orig_stdout = sys.stdout
            sys.stdout = captured
            try:
                rc = self.cli.cmd_export(SimpleNamespace(output=None, target=None))
            finally:
                sys.stdout = orig_stdout
            output = captured.getvalue()

            self.assertEqual(rc, 0)
            self.assertIn("helium-sync-export-", output)
        finally:
            self.cli.ALL_TARGETS = old_targets
            self.cli.REPO_ROOT = old_repo
            self.cli.STATE_DIR = old_state
            self.cli.LOGS_DIR = old_logs

    def test_export_no_canonical_state(self):
        """Export with no state files returns error."""
        repo = self.root / "repo"
        repo.mkdir()
        state_dir = repo / "state"
        state_dir.mkdir()

        old_targets = self.cli.ALL_TARGETS
        old_repo = self.cli.REPO_ROOT
        old_state = self.cli.STATE_DIR
        old_logs = self.cli.LOGS_DIR
        try:
            self.cli.ALL_TARGETS = [FakeTarget()]
            self.cli.REPO_ROOT = repo
            self.cli.STATE_DIR = state_dir
            self.cli.LOGS_DIR = repo / "logs"

            rc = self.cli.cmd_export(SimpleNamespace(output=None, target=None))
            self.assertNotEqual(rc, 0)
        finally:
            self.cli.ALL_TARGETS = old_targets
            self.cli.REPO_ROOT = old_repo
            self.cli.STATE_DIR = old_state
            self.cli.LOGS_DIR = old_logs

    def test_export_target_filter(self):
        """Export with --target only exports matching target."""
        repo = self.root / "repo"
        repo.mkdir()
        state_dir = repo / "state"
        state_dir.mkdir()
        (state_dir / "fake_bookmarks.json").write_text(json.dumps({"roots": {}}))

        old_targets = self.cli.ALL_TARGETS
        old_repo = self.cli.REPO_ROOT
        old_state = self.cli.STATE_DIR
        old_logs = self.cli.LOGS_DIR
        try:
            self.cli.ALL_TARGETS = [FakeTarget()]
            self.cli.REPO_ROOT = repo
            self.cli.STATE_DIR = state_dir
            self.cli.LOGS_DIR = repo / "logs"

            output_file = self.root / "export.json"
            rc = self.cli.cmd_export(SimpleNamespace(
                output=str(output_file), target="nonexistent"
            ))
            self.assertNotEqual(rc, 0)
        finally:
            self.cli.ALL_TARGETS = old_targets
            self.cli.REPO_ROOT = old_repo
            self.cli.STATE_DIR = old_state
            self.cli.LOGS_DIR = old_logs


class TestImport(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def _make_export_file(self, data, path=None):
        if path is None:
            path = self.root / "import.json"
        payload = {
            "exported_at": "2026-01-01T00:00:00+00:00",
            "exported_from": "test-host",
            "targets": {"fake_bookmarks": {"format_version": 1, "data": data}},
        }
        path.write_text(json.dumps(payload))
        return path

    def test_roundtrip_export_import(self):
        """Export → import → profile matches original."""
        repo = self.root / "repo"
        repo.mkdir()
        subprocess.run(["git", "-C", str(repo), "init", "-q", "-b", "main"], check=True)
        state_dir = repo / "state"
        state_dir.mkdir()

        original_data = {"roots": {"bookmark_bar": {"children": [
            {"type": "url", "name": "Test", "url": "https://test.com"}
        ]}}}
        (state_dir / "fake_bookmarks.json").write_text(json.dumps(original_data))

        profile = self.root / "profile"
        profile.mkdir()
        (profile / "Bookmarks").write_text(json.dumps({"roots": {}}))

        old_targets = self.cli.ALL_TARGETS
        old_repo = self.cli.REPO_ROOT
        old_state = self.cli.STATE_DIR
        old_logs = self.cli.LOGS_DIR
        try:
            self.cli.ALL_TARGETS = [FakeTarget()]
            self.cli.REPO_ROOT = repo
            self.cli.STATE_DIR = state_dir
            self.cli.LOGS_DIR = repo / "logs"

            export_file = self.root / "export.json"
            rc = self.cli.cmd_export(SimpleNamespace(output=str(export_file), target=None))
            self.assertEqual(rc, 0)

            rc = self.cli.cmd_import(SimpleNamespace(
                import_file=str(export_file),
                allow_helium_running=True,
                target=None,
                profile=profile,
            ))
            self.assertEqual(rc, 0)

            applied = json.loads((profile / "Bookmarks").read_text())
            self.assertEqual(applied, original_data)
        finally:
            self.cli.ALL_TARGETS = old_targets
            self.cli.REPO_ROOT = old_repo
            self.cli.STATE_DIR = old_state
            self.cli.LOGS_DIR = old_logs

    def test_import_refuses_when_helium_running(self):
        """Import returns error when Helium is running and --allow-helium-running is not set."""
        export_file = self._make_export_file({"roots": {}})

        profile = self.root / "profile"
        profile.mkdir()

        with unittest.mock.patch.object(self.cli, "helium_running", return_value=True):
            rc = self.cli.cmd_import(SimpleNamespace(
                import_file=str(export_file),
                allow_helium_running=False,
                target=None,
                profile=profile,
            ))
            self.assertEqual(rc, 4)

    def test_import_with_allow_helium_running(self):
        """Import proceeds when --allow-helium-running is set."""
        repo = self.root / "repo"
        repo.mkdir()
        subprocess.run(["git", "-C", str(repo), "init", "-q", "-b", "main"], check=True)
        state_dir = repo / "state"
        state_dir.mkdir()
        logs_dir = repo / "logs"
        logs_dir.mkdir()

        export_file = self._make_export_file({"roots": {"bookmark_bar": {"children": []}}})
        profile = self.root / "profile"
        profile.mkdir()
        (profile / "Bookmarks").write_text(json.dumps({"stale": True}))

        old_targets = self.cli.ALL_TARGETS
        old_repo = self.cli.REPO_ROOT
        old_state = self.cli.STATE_DIR
        old_logs = self.cli.LOGS_DIR
        try:
            self.cli.ALL_TARGETS = [FakeTarget()]
            self.cli.REPO_ROOT = repo
            self.cli.STATE_DIR = state_dir
            self.cli.LOGS_DIR = logs_dir

            with unittest.mock.patch.object(self.cli, "helium_running", return_value=True):
                rc = self.cli.cmd_import(SimpleNamespace(
                    import_file=str(export_file),
                    allow_helium_running=True,
                    target=None,
                    profile=profile,
                ))
            self.assertEqual(rc, 0)
        finally:
            self.cli.ALL_TARGETS = old_targets
            self.cli.REPO_ROOT = old_repo
            self.cli.STATE_DIR = old_state
            self.cli.LOGS_DIR = old_logs

    def test_import_corrupted_file(self):
        """Import of corrupted JSON returns clean error."""
        corrupted = self.root / "corrupted.json"
        corrupted.write_text("{not valid json!!!")

        profile = self.root / "profile"
        profile.mkdir()

        rc = self.cli.cmd_import(SimpleNamespace(
            import_file=str(corrupted),
            allow_helium_running=True,
            target=None,
            profile=profile,
        ))
        self.assertEqual(rc, 1)

    def test_import_invalid_format(self):
        """Import of file without 'targets' key returns error."""
        bad_format = self.root / "bad.json"
        bad_format.write_text(json.dumps({"hello": "world"}))

        rc = self.cli.cmd_import(SimpleNamespace(
            import_file=str(bad_format),
            allow_helium_running=True,
            target=None,
            profile=self.root / "profile",
        ))
        self.assertEqual(rc, 1)

    def test_import_missing_file(self):
        """Import of non-existent file returns error."""
        rc = self.cli.cmd_import(SimpleNamespace(
            import_file=str(self.root / "does_not_exist.json"),
            allow_helium_running=True,
            target=None,
            profile=self.root / "profile",
        ))
        self.assertEqual(rc, 1)

    def test_import_creates_backup(self):
        """Import backs up pre-existing profile state."""
        repo = self.root / "repo"
        repo.mkdir()
        subprocess.run(["git", "-C", str(repo), "init", "-q", "-b", "main"], check=True)
        state_dir = repo / "state"
        state_dir.mkdir()
        logs_dir = repo / "logs"
        logs_dir.mkdir()

        export_file = self._make_export_file({"roots": {"bookmark_bar": {"children": []}}})
        profile = self.root / "profile"
        profile.mkdir()
        (profile / "Bookmarks").write_text(json.dumps({"old": "data"}))

        old_targets = self.cli.ALL_TARGETS
        old_repo = self.cli.REPO_ROOT
        old_state = self.cli.STATE_DIR
        old_logs = self.cli.LOGS_DIR
        try:
            self.cli.ALL_TARGETS = [FakeTarget()]
            self.cli.REPO_ROOT = repo
            self.cli.STATE_DIR = state_dir
            self.cli.LOGS_DIR = logs_dir

            rc = self.cli.cmd_import(SimpleNamespace(
                import_file=str(export_file),
                allow_helium_running=True,
                target=None,
                profile=profile,
            ))
            self.assertEqual(rc, 0)

            # Check backup was created
            backups = list(logs_dir.glob("preImport.*"))
            self.assertTrue(len(backups) > 0)
            backup_files = list(backups[0].glob("*"))
            self.assertTrue(len(backup_files) > 0)
        finally:
            self.cli.ALL_TARGETS = old_targets
            self.cli.REPO_ROOT = old_repo
            self.cli.STATE_DIR = old_state
            self.cli.LOGS_DIR = old_logs


class TestImportExportIntegration(unittest.TestCase):
    """End-to-end integration: export from one repo, import into another profile."""

    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def _git(self, repo: Path, *args: str) -> None:
        subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True, text=True)

    def test_export_from_repo_import_to_different_profile(self):
        """Export canonical state from one repo, import into a separate profile."""
        repo = self.root / "repo"
        repo.mkdir()
        self._git(repo, "init", "-q", "-b", "main")
        state_dir = repo / "state"
        state_dir.mkdir()
        logs_dir = repo / "logs"
        logs_dir.mkdir()

        original = {"roots": {"bookmark_bar": {"children": [
            {"type": "url", "name": "Example", "url": "https://example.com"}
        ]}}}
        (state_dir / "fake_bookmarks.json").write_text(json.dumps(original))

        export_file = self.root / "export.json"

        old_targets = self.cli.ALL_TARGETS
        old_repo = self.cli.REPO_ROOT
        old_state = self.cli.STATE_DIR
        old_logs = self.cli.LOGS_DIR
        try:
            self.cli.ALL_TARGETS = [FakeTarget()]
            self.cli.REPO_ROOT = repo
            self.cli.STATE_DIR = state_dir
            self.cli.LOGS_DIR = logs_dir

            # Export
            rc = self.cli.cmd_export(SimpleNamespace(output=str(export_file), target=None))
            self.assertEqual(rc, 0)

            # Import into a fresh profile
            profile = self.root / "fresh-profile"
            profile.mkdir()
            rc = self.cli.cmd_import(SimpleNamespace(
                import_file=str(export_file),
                allow_helium_running=True,
                target=None,
                profile=profile,
            ))
            self.assertEqual(rc, 0)

            # Verify profile matches original
            applied = json.loads((profile / "Bookmarks").read_text())
            self.assertEqual(applied, original)
        finally:
            self.cli.ALL_TARGETS = old_targets
            self.cli.REPO_ROOT = old_repo
            self.cli.STATE_DIR = old_state
            self.cli.LOGS_DIR = old_logs


if __name__ == "__main__":
    unittest.main()
