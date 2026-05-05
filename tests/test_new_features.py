"""Tests for --dry-run (push/pull), restore, resolve, and completion commands.

Run from the repo root: python -m unittest discover tests/
"""

import importlib.machinery
import importlib.util
import io
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


def _import_cli():
    cli_path = Path(__file__).resolve().parent.parent / "bin" / "helium-sync"
    sys.path.insert(0, str(cli_path.parent))
    loader = importlib.machinery.SourceFileLoader("helium_sync_cli", str(cli_path))
    spec = importlib.util.spec_from_loader("helium_sync_cli", loader)
    mod = importlib.util.module_from_spec(spec)
    loader.exec_module(mod)
    return mod


class FakeTarget:
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


def _init_git_repo(repo: Path):
    subprocess.run(["git", "init", "-b", "main"], cwd=repo, capture_output=True, check=True)
    subprocess.run(["git", "config", "user.email", "test@test"], cwd=repo, capture_output=True, check=True)
    subprocess.run(["git", "config", "user.name", "Test"], cwd=repo, capture_output=True, check=True)


def _run_cli(cli_mod, *args, profile, repo):
    original_targets = cli_mod.ALL_TARGETS
    cli_mod.ALL_TARGETS = [FakeTarget()]
    cli_mod.REPO_ROOT = Path(repo)
    cli_mod.STATE_DIR = Path(repo) / "state"
    cli_mod.LOGS_DIR = Path(repo) / "logs"

    parser = cli_mod.argparse.ArgumentParser()
    parser.add_argument("--profile", type=Path, default=Path(profile))
    parser.add_argument("--repo", type=Path, default=Path(repo))
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_push = sub.add_parser("push")
    p_push.add_argument("--target", default=None)
    p_push.add_argument("--strict", action="store_true")
    p_push.add_argument("--dry-run", action="store_true")

    p_pull = sub.add_parser("pull")
    p_pull.add_argument("--allow-helium-running", action="store_true")
    p_pull.add_argument("--target", default=None)
    p_pull.add_argument("--dry-run", action="store_true")

    p_restore = sub.add_parser("restore")
    p_restore.add_argument("--allow-helium-running", action="store_true")

    p_completion = sub.add_parser("completion")
    p_completion.add_argument("--shell", required=True, choices=["powershell", "cmd"])

    p_resolve = sub.add_parser("resolve")
    p_resolve.add_argument("--theirs", default=None)

    p_log = sub.add_parser("log")
    p_log.add_argument("-n", type=int, default=10)

    parsed = parser.parse_args(list(args))
    handler = {
        "push": cli_mod.cmd_push,
        "pull": cli_mod.cmd_pull,
        "restore": cli_mod.cmd_restore,
        "completion": cli_mod.cmd_completion,
        "resolve": cli_mod.cmd_resolve,
        "log": cli_mod.cmd_log,
    }[parsed.cmd]
    try:
        return handler(parsed)
    finally:
        cli_mod.ALL_TARGETS = original_targets


class TestDryRunPush(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.profile = self.root / "profile"
        self.profile.mkdir()
        _init_git_repo(self.repo)
        (self.profile / "Bookmarks").write_text(
            json.dumps({"roots": {"bookmark_bar": {"type": "folder", "name": "Bar", "children": []}},
                        "checksum": "", "version": 1}))
        with patch("sys.stdout", new_callable=io.StringIO):
            _run_cli(self.cli, "push", profile=self.profile, repo=self.repo)

    def test_dry_run_push_no_changes(self):
        out = io.StringIO()
        with patch("sys.stdout", new=out):
            ret = _run_cli(self.cli, "push", "--dry-run", profile=self.profile, repo=self.repo)
        self.assertEqual(ret, 0)
        output = out.getvalue()
        self.assertIn("no changes to push", output)
        self.assertIn("dry run", output)

    def test_dry_run_does_not_commit(self):
        with patch("sys.stdout", new_callable=io.StringIO):
            _run_cli(self.cli, "push", "--dry-run", profile=self.profile, repo=self.repo)
        result = subprocess.run(
            ["git", "log", "--oneline"], cwd=self.repo, capture_output=True, text=True)
        lines = [l for l in result.stdout.strip().split("\n") if l]
        self.assertEqual(len(lines), 1)


class TestDryRunPull(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.profile = self.root / "profile"
        self.profile.mkdir()
        _init_git_repo(self.repo)
        (self.profile / "Bookmarks").write_text(
            json.dumps({"roots": {"bookmark_bar": {"type": "folder", "name": "Bar", "children": []}},
                        "checksum": "", "version": 1}))
        with patch("sys.stdout", new_callable=io.StringIO):
            _run_cli(self.cli, "push", profile=self.profile, repo=self.repo)

    def test_dry_run_pull_no_write(self):
        (self.profile / "Bookmarks").write_text(
            json.dumps({"roots": {"bookmark_bar": {"type": "folder", "name": "Changed", "children": []}},
                        "checksum": "", "version": 1}))
        before = (self.profile / "Bookmarks").read_text()
        out = io.StringIO()
        with patch("sys.stdout", new=out):
            _run_cli(self.cli, "pull", "--dry-run", profile=self.profile, repo=self.repo)
        after = (self.profile / "Bookmarks").read_text()
        self.assertEqual(before, after)
        self.assertIn("dry run", out.getvalue())


class TestRestore(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.profile = self.root / "profile"
        self.profile.mkdir()
        _init_git_repo(self.repo)

    def test_restore_no_backups(self):
        out = io.StringIO()
        with patch("sys.stdout", new=out):
            ret = _run_cli(self.cli, "restore", profile=self.profile, repo=self.repo)
        self.assertEqual(ret, 1)
        self.assertIn("no backups", out.getvalue())

    def test_restore_from_backup(self):
        logs = self.repo / "logs"
        backup = logs / "prePull.20260101-120000"
        backup.mkdir(parents=True)
        backup_data = {"roots": {}, "checksum": "", "version": 1}
        (backup / "Bookmarks").write_text(json.dumps(backup_data))
        out = io.StringIO()
        with patch("sys.stdout", new=out):
            ret = _run_cli(self.cli, "restore", profile=self.profile, repo=self.repo)
        self.assertEqual(ret, 0)
        self.assertIn("restored", out.getvalue())
        bm = self.profile / "Bookmarks"
        self.assertTrue(bm.exists())
        self.assertEqual(json.loads(bm.read_text()), backup_data)

    def test_restore_picks_latest(self):
        logs = self.repo / "logs"
        (logs / "prePull.20260101-120000").mkdir(parents=True)
        (logs / "prePull.20260101-120000" / "Bookmarks").write_text(
            json.dumps({"roots": {}, "checksum": "", "version": 1}))
        (logs / "prePull.20260601-120000").mkdir(parents=True)
        (logs / "prePull.20260601-120000" / "Bookmarks").write_text(
            json.dumps({"roots": {"latest": {}}, "checksum": "", "version": 1}))
        out = io.StringIO()
        with patch("sys.stdout", new=out):
            ret = _run_cli(self.cli, "restore", profile=self.profile, repo=self.repo)
        self.assertEqual(ret, 0)
        bm = self.profile / "Bookmarks"
        data = json.loads(bm.read_text())
        self.assertIn("latest", data.get("roots", {}))


class TestCompletion(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.profile = self.root / "profile"
        self.profile.mkdir()

    def test_completion_powershell(self):
        out = io.StringIO()
        with patch("sys.stdout", new=out):
            ret = _run_cli(self.cli, "completion", "--shell", "powershell",
                          profile=self.profile, repo=self.repo)
        self.assertEqual(ret, 0)
        output = out.getvalue()
        self.assertIn("Register-ArgumentCompleter", output)
        self.assertIn("push", output)
        self.assertIn("pull", output)

    def test_completion_cmd(self):
        out = io.StringIO()
        with patch("sys.stdout", new=out):
            ret = _run_cli(self.cli, "completion", "--shell", "cmd",
                          profile=self.profile, repo=self.repo)
        self.assertEqual(ret, 0)
        self.assertIn("doskey", out.getvalue())


class TestResolve(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.profile = self.root / "profile"
        self.profile.mkdir()
        _init_git_repo(self.repo)
        (self.profile / "Bookmarks").write_text(
            json.dumps({"roots": {"bookmark_bar": {"type": "folder", "name": "Bar", "children": []}},
                        "checksum": "", "version": 1}))

    def test_resolve_no_state(self):
        out = io.StringIO()
        with patch("sys.stdout", new=out):
            ret = _run_cli(self.cli, "resolve", profile=self.profile, repo=self.repo)
        self.assertIn(ret, [1, 2])

    def test_resolve_requires_tty(self):
        with patch("sys.stdout", new_callable=io.StringIO):
            _run_cli(self.cli, "push", profile=self.profile, repo=self.repo)
        theirs = self.repo / "theirs.json"
        theirs.write_text(json.dumps({
            "roots": {"bookmark_bar": {"type": "folder", "name": "Bar", "children": [
                {"type": "url", "url": "https://example.com", "name": "Example"}
            ]}},
            "checksum": "", "version": 1,
        }))
        out = io.StringIO()
        with patch("sys.stdout", new=out), patch("sys.stdin.isatty", return_value=False):
            ret = _run_cli(self.cli, "resolve", "--theirs", str(theirs),
                          profile=self.profile, repo=self.repo)
        self.assertEqual(ret, 1)
        self.assertIn("interactive terminal", out.getvalue())


if __name__ == "__main__":
    unittest.main()
