"""Tests for CLI-level concerns. Right now: data-repo path resolution.

The CLI lives at bin/helium-sync (no .py extension), so we import it via
runpy / importlib so its module-level code (including the venv-relaunch
guard) doesn't fire. We import only the symbols we need.
"""

import importlib.machinery
import importlib.util
import json
import os
import subprocess
import sys
import tempfile
import unittest
from types import SimpleNamespace
from pathlib import Path
from unittest import mock


def _import_cli():
    """Import bin/helium-sync as a module. The script has no .py extension,
    so we need an explicit SourceFileLoader (importlib's auto-detection
    only handles .py / .pyc by default). The venv-relaunch guard at the
    top of the script short-circuits because we're already running under
    .venv during tests; module import is safe.
    """
    cli_path = Path(__file__).resolve().parent.parent / "bin" / "helium-sync"
    sys.path.insert(0, str(cli_path.parent))  # so `from targets import ...` resolves
    loader = importlib.machinery.SourceFileLoader("helium_sync_cli", str(cli_path))
    spec = importlib.util.spec_from_loader("helium_sync_cli", loader)
    mod = importlib.util.module_from_spec(spec)
    loader.exec_module(mod)
    return mod


class TestResolveRepo(unittest.TestCase):
    """_resolve_repo precedence: --repo > env > config > default."""

    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.tmp_path = Path(self.tmp.name)

    def test_cli_arg_wins(self):
        env_dir = self.tmp_path / "from_env"
        env_dir.mkdir()
        cli_dir = self.tmp_path / "from_cli"
        cli_dir.mkdir()
        cfg_file = self.tmp_path / "config.toml"
        cfg_file.write_text(f'repo = "{(self.tmp_path / "from_cfg").as_posix()}"\n')

        got = self.cli._resolve_repo(
            cli_arg=cli_dir,
            env={"HELIUM_SYNC_REPO": str(env_dir)},
            config_path=cfg_file,
        )
        self.assertEqual(got, cli_dir.resolve())

    def test_env_wins_over_config_and_default(self):
        env_dir = self.tmp_path / "from_env"
        env_dir.mkdir()
        cfg_file = self.tmp_path / "config.toml"
        cfg_file.write_text(f'repo = "{(self.tmp_path / "from_cfg").as_posix()}"\n')

        got = self.cli._resolve_repo(
            cli_arg=None,
            env={"HELIUM_SYNC_REPO": str(env_dir)},
            config_path=cfg_file,
        )
        self.assertEqual(got, env_dir.resolve())

    def test_config_wins_over_default(self):
        cfg_dir = self.tmp_path / "from_cfg"
        cfg_dir.mkdir()
        cfg_file = self.tmp_path / "config.toml"
        cfg_file.write_text(f'repo = "{cfg_dir.as_posix()}"\n')

        got = self.cli._resolve_repo(
            cli_arg=None,
            env={},
            config_path=cfg_file,
        )
        self.assertEqual(got, cfg_dir.resolve())

    def test_default_when_nothing_set(self):
        # No flag, empty env, missing config file → default fallback.
        cfg_file = self.tmp_path / "does_not_exist.toml"
        got = self.cli._resolve_repo(cli_arg=None, env={}, config_path=cfg_file)
        self.assertEqual(got, self.cli.DEFAULT_REPO.resolve())

    def test_expands_user_in_env(self):
        # ~ in env value should expand
        got = self.cli._resolve_repo(
            cli_arg=None,
            env={"HELIUM_SYNC_REPO": "~"},
            config_path=self.tmp_path / "missing.toml",
        )
        self.assertEqual(got, Path.home().resolve())

    def test_malformed_config_falls_through_to_default(self):
        # Garbage TOML → quietly fall through to default rather than crash.
        cfg_file = self.tmp_path / "config.toml"
        cfg_file.write_text("not = valid = toml")
        got = self.cli._resolve_repo(cli_arg=None, env={}, config_path=cfg_file)
        self.assertEqual(got, self.cli.DEFAULT_REPO.resolve())

    def test_empty_env_value_treated_as_unset(self):
        # HELIUM_SYNC_REPO="" should fall through, not resolve to "".
        cfg_file = self.tmp_path / "missing.toml"
        got = self.cli._resolve_repo(
            cli_arg=None,
            env={"HELIUM_SYNC_REPO": ""},
            config_path=cfg_file,
        )
        self.assertEqual(got, self.cli.DEFAULT_REPO.resolve())


class TestAsk(unittest.TestCase):
    """_ask fallback behavior — used by `helium-sync setup` so scripted runs
    (CI, --yes, piped input) fall through to defaults instead of blocking
    on an unanswerable prompt."""

    def test_returns_default_when_stdin_not_a_tty(self):
        cli = _import_cli()
        # In test runs, stdin is typically piped (not a TTY); _ask returns
        # the default without prompting.
        self.assertEqual(cli._ask("prompt: ", default="hello"), "hello")
        self.assertEqual(cli._ask("prompt: ", default=""), "")


class TestHeliumRunning(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()

    def test_detects_helium_running(self):
        result = SimpleNamespace(stdout="helium.exe                  1234 Console")
        with mock.patch.object(self.cli.subprocess, "run", return_value=result):
            self.assertTrue(self.cli.helium_running())

    def test_detects_no_matching_process(self):
        result = SimpleNamespace(stdout="INFO: No tasks are running which match the specified criteria.")
        with mock.patch.object(self.cli.subprocess, "run", return_value=result):
            self.assertFalse(self.cli.helium_running())

    def test_tasklist_failure_is_not_running(self):
        result = SimpleNamespace(stdout="")
        with mock.patch.object(self.cli.subprocess, "run", return_value=result):
            self.assertFalse(self.cli.helium_running())


class TestUpdateBanner(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()

    def test_version_tuple_parses_semver_prefix(self):
        self.assertEqual(self.cli._version_tuple("0.1.2"), (0, 1, 2))
        self.assertEqual(self.cli._version_tuple("v0.1.2"), (0, 1, 2))
        self.assertEqual(self.cli._version_tuple("0.1.2-3-gabc"), (0, 1, 2))
        self.assertIsNone(self.cli._version_tuple("unknown"))

    def test_update_banner_contains_command(self):
        banner = self.cli._update_banner("0.1.2", "0.1.3")
        self.assertIn("Update available: helium-sync 0.1.2 -> 0.1.3", banner)
        self.assertIn("scoop update && scoop update helium-sync", banner)

    def test_no_banner_when_latest_is_not_newer(self):
        with mock.patch.object(self.cli, "_app_version", return_value="0.1.3"), \
             mock.patch.object(self.cli, "_latest_release_version", return_value="0.1.3"), \
             mock.patch("builtins.print") as print_mock:
            self.cli.maybe_show_update_banner()
        print_mock.assert_not_called()


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

    def semantically_equal(self, live: dict, canonical: dict) -> bool:
        return live == canonical


class TestTempRepoIntegration(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def _git(self, repo: Path, *args: str) -> None:
        subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True, text=True)

    def _init_repo(self, repo: Path) -> None:
        repo.mkdir(parents=True)
        self._git(repo, "init", "-q", "-b", "main")
        self._git(repo, "config", "user.name", "helium-sync-test")
        self._git(repo, "config", "user.email", "helium-sync-test@example.invalid")

    def test_push_and_pull_with_local_git_remote(self):
        origin = self.root / "origin.git"
        subprocess.run(["git", "init", "--bare", str(origin)], check=True, capture_output=True, text=True)
        subprocess.run(["git", "-C", str(origin), "symbolic-ref", "HEAD", "refs/heads/main"], check=True)

        source_repo = self.root / "source-repo"
        self._init_repo(source_repo)
        self._git(source_repo, "remote", "add", "origin", str(origin))

        source_profile = self.root / "source-profile"
        source_profile.mkdir()
        (source_profile / "Bookmarks").write_text(json.dumps({"roots": {"bookmark_bar": {"children": []}}}))

        old_targets = self.cli.ALL_TARGETS
        try:
            self.cli.ALL_TARGETS = [FakeTarget()]
            self.cli.REPO_ROOT = source_repo
            self.cli.STATE_DIR = source_repo / "state"
            self.cli.LOGS_DIR = source_repo / "logs"
            push_rc = self.cli.cmd_push(SimpleNamespace(profile=source_profile))
            self.assertEqual(push_rc, 0)

            dest_repo = self.root / "dest-repo"
            subprocess.run(["git", "clone", "-q", "-b", "main", str(origin), str(dest_repo)], check=True)
            self._git(dest_repo, "config", "user.name", "helium-sync-test")
            self._git(dest_repo, "config", "user.email", "helium-sync-test@example.invalid")

            dest_profile = self.root / "dest-profile"
            dest_profile.mkdir()
            (dest_profile / "Bookmarks").write_text(json.dumps({"stale": True}))

            self.cli.REPO_ROOT = dest_repo
            self.cli.STATE_DIR = dest_repo / "state"
            self.cli.LOGS_DIR = dest_repo / "logs"
            pull_rc = self.cli.cmd_pull(SimpleNamespace(profile=dest_profile, allow_helium_running=True))
            self.assertEqual(pull_rc, 0)

            applied = json.loads((dest_profile / "Bookmarks").read_text())
            self.assertEqual(applied, {"roots": {"bookmark_bar": {"children": []}}})
            self.assertTrue(any((dest_repo / "logs").glob("prePull.*/Bookmarks")))
        finally:
            self.cli.ALL_TARGETS = old_targets


class TestDoctorCommand(unittest.TestCase):
    def test_doctor_passes_with_temp_profile_and_repo(self):
        tmp = tempfile.TemporaryDirectory(prefix="helium-sync-doctor.")
        self.addCleanup(tmp.cleanup)
        root = Path(tmp.name)
        profile = root / "profile"
        repo = root / "repo"
        (profile / "Default").mkdir(parents=True)
        repo.mkdir()
        subprocess.run(["git", "-C", str(repo), "init", "-q", "-b", "main"], check=True)

        tool = Path(__file__).resolve().parent.parent / "bin" / (
            "leveldb-writer.exe" if os.name == "nt" else "leveldb-writer"
        )
        if not tool.exists():
            self.skipTest("leveldb-writer not built")

        cli_path = Path(__file__).resolve().parent.parent / "bin" / "helium-sync"
        env = os.environ.copy()
        env["APPDATA"] = str(root / "appdata")
        result = subprocess.run(
            [
                sys.executable,
                str(cli_path),
                "--profile",
                str(profile),
                "--repo",
                str(repo),
                "doctor",
            ],
            capture_output=True,
            text=True,
            env=env,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("[ok] git", result.stdout)
        self.assertIn("[ok] profile", result.stdout)
        self.assertIn("[ok] repo git", result.stdout)


# --------------------------------------------------------------------------- #
# Validation tests
# --------------------------------------------------------------------------- #

def _bm_tree(bookmark_bar=None):
    return {
        "checksum": "",
        "version": 1,
        "roots": {
            "bookmark_bar": {
                "type": "folder", "name": "Bookmarks Bar",
                "children": bookmark_bar or [],
            },
            "other": {"type": "folder", "name": "Other", "children": []},
            "synced": {"type": "folder", "name": "Mobile", "children": []},
        },
    }


def _bm_url(name, url):
    return {"type": "url", "name": name, "url": url, "id": "0", "guid": "g"}


class TestValidateBookmarks(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()

    def test_valid_tree_passes(self):
        data = _bm_tree(bookmark_bar=[_bm_url("Test", "https://example.com")])
        issues = self.cli._validate_bookmarks(data)
        self.assertEqual(issues, [])

    def test_missing_roots_fails(self):
        issues = self.cli._validate_bookmarks({})
        self.assertTrue(any("roots" in i for i in issues))

    def test_empty_url_triggers_warning(self):
        data = _bm_tree(bookmark_bar=[{"type": "url", "name": "Bad", "url": ""}])
        issues = self.cli._validate_bookmarks(data)
        self.assertTrue(any("empty URL" in i for i in issues))

    def test_invalid_url_scheme_triggers_warning(self):
        data = _bm_tree(bookmark_bar=[_bm_url("FTP", "ftp://files.example.com")])
        issues = self.cli._validate_bookmarks(data)
        self.assertTrue(any("unusual URL scheme" in i for i in issues))

    def test_http_and_https_pass(self):
        data = _bm_tree(bookmark_bar=[
            _bm_url("HTTP", "http://example.com"),
            _bm_url("HTTPS", "https://example.com"),
        ])
        issues = self.cli._validate_bookmarks(data)
        self.assertFalse(any("scheme" in i for i in issues))

    def test_deeply_nested_folders_pass(self):
        deep = _bm_url("Deep", "https://deep.com")
        for i in range(10):
            deep = {"type": "folder", "name": f"L{i}", "children": [deep]}
        data = _bm_tree(bookmark_bar=[deep])
        issues = self.cli._validate_bookmarks(data)
        self.assertEqual(issues, [])

    def test_empty_tree_passes(self):
        data = _bm_tree()
        issues = self.cli._validate_bookmarks(data)
        self.assertEqual(issues, [])


class TestValidateTabGroups(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()

    def test_valid_data_passes(self):
        data = {
            "groups": {"g1": {"guid": "g1", "title": "Dev", "color": 1, "position": 0}},
            "tabs": {"t1": {"guid": "t1", "group_guid": "g1", "url": "https://a.com",
                            "title": "A", "position": 0}},
        }
        issues = self.cli._validate_tab_groups(data)
        self.assertEqual(issues, [])

    def test_missing_keys_fails(self):
        issues = self.cli._validate_tab_groups({})
        self.assertTrue(any("missing" in i for i in issues))

    def test_tab_references_nonexistent_group(self):
        data = {
            "groups": {},
            "tabs": {"t1": {"guid": "t1", "group_guid": "missing", "url": "https://a.com",
                            "title": "A", "position": 0}},
        }
        issues = self.cli._validate_tab_groups(data)
        self.assertTrue(any("non-existent group" in i for i in issues))

    def test_invalid_url_scheme_in_tab(self):
        data = {
            "groups": {"g1": {"guid": "g1", "title": "G", "color": 1, "position": 0}},
            "tabs": {"t1": {"guid": "t1", "group_guid": "g1", "url": "ftp://bad",
                            "title": "B", "position": 0}},
        }
        issues = self.cli._validate_tab_groups(data)
        self.assertTrue(any("unusual URL scheme" in i for i in issues))


if __name__ == "__main__":
    unittest.main()
