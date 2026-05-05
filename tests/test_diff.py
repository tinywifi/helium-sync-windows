"""Tests for the `helium-sync diff` command.

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


def url_node(name, url, **kw):
    n = {
        "type": "url", "name": name, "url": url,
        "date_added": "100", "date_last_used": "0",
        "guid": f"g-{name}", "id": "0",
    }
    n.update(kw)
    return n


def folder_node(name, children=None, **kw):
    n = {
        "type": "folder", "name": name, "children": children or [],
        "date_added": "100", "date_modified": "100", "date_last_used": "0",
        "guid": f"g-{name}", "id": "0",
    }
    n.update(kw)
    return n


def bookmark_tree(bookmark_bar=None, other=None, synced=None):
    return {
        "checksum": "abc123",
        "version": 1,
        "roots": {
            "bookmark_bar": folder_node("Bookmarks Bar", bookmark_bar or [], id="1"),
            "other":        folder_node("Other Bookmarks", other or [], id="2"),
            "synced":       folder_node("Mobile Bookmarks", synced or [], id="3"),
        },
    }


class TestBookmarkUrlMap(unittest.TestCase):
    def setUp(self):
        self.cli = _import_cli()

    def test_empty_tree(self):
        tree = bookmark_tree()
        result = self.cli._bookmark_url_map(tree)
        self.assertEqual(result, {})

    def test_single_url(self):
        tree = bookmark_tree(bookmark_bar=[url_node("Google", "https://google.com")])
        result = self.cli._bookmark_url_map(tree)
        self.assertIn(("bookmark_bar", "https://google.com"), result)
        self.assertEqual(result[("bookmark_bar", "https://google.com")], "Google")

    def test_nested_folder(self):
        tree = bookmark_tree(bookmark_bar=[
            folder_node("Dev", [url_node("GitHub", "https://github.com")]),
        ])
        result = self.cli._bookmark_url_map(tree)
        self.assertIn(("bookmark_bar/Dev", "https://github.com"), result)
        self.assertEqual(result[("bookmark_bar/Dev", "https://github.com")], "GitHub")


class TestPrintBookmarkDiff(unittest.TestCase):
    """Verify diff output contains correct +/- annotations."""

    def setUp(self):
        self.cli = _import_cli()

    def test_no_changes(self):
        """When live == canonical, _print_bookmark_diff prints nothing but 'in sync' is handled by cmd_diff."""
        tree = bookmark_tree(bookmark_bar=[url_node("A", "https://a")])
        # Capture output — should be empty since nothing differs
        import io
        captured = io.StringIO()
        orig_stdout = sys.stdout
        sys.stdout = captured
        try:
            self.cli._print_bookmark_diff(tree, tree)
        finally:
            sys.stdout = orig_stdout
        # No added/removed/changed means only "bookmarks:" header with no entries
        output = captured.getvalue()
        self.assertIn("bookmarks:", output)
        self.assertNotIn("+", output)
        self.assertNotIn("-", output)

    def test_added_bookmark(self):
        live = bookmark_tree(bookmark_bar=[
            url_node("A", "https://a"),
            url_node("B", "https://b"),
        ])
        canon = bookmark_tree(bookmark_bar=[url_node("A", "https://a")])

        import io
        captured = io.StringIO()
        orig_stdout = sys.stdout
        sys.stdout = captured
        try:
            self.cli._print_bookmark_diff(live, canon)
        finally:
            sys.stdout = orig_stdout
        output = captured.getvalue()
        self.assertIn("+", output)
        self.assertIn("B", output)
        self.assertIn("https://b", output)

    def test_removed_bookmark(self):
        live = bookmark_tree(bookmark_bar=[url_node("A", "https://a")])
        canon = bookmark_tree(bookmark_bar=[
            url_node("A", "https://a"),
            url_node("B", "https://b"),
        ])

        import io
        captured = io.StringIO()
        orig_stdout = sys.stdout
        sys.stdout = captured
        try:
            self.cli._print_bookmark_diff(live, canon)
        finally:
            sys.stdout = orig_stdout
        output = captured.getvalue()
        self.assertIn("-", output)
        self.assertIn("B", output)
        self.assertIn("https://b", output)

    def test_folder_structure_change(self):
        live = bookmark_tree(bookmark_bar=[
            folder_node("New", [url_node("A", "https://a")]),
        ])
        canon = bookmark_tree(bookmark_bar=[
            folder_node("Old", [url_node("A", "https://a")]),
        ])

        import io
        captured = io.StringIO()
        orig_stdout = sys.stdout
        sys.stdout = captured
        try:
            self.cli._print_bookmark_diff(live, canon)
        finally:
            sys.stdout = orig_stdout
        output = captured.getvalue()
        # Same URL in different folders shows as add in new + remove in old
        self.assertIn("+", output)
        self.assertIn("-", output)
        self.assertIn("New", output)
        self.assertIn("Old", output)


class TestDiffCommand(unittest.TestCase):
    """Integration tests for cmd_diff via the FakeTarget pattern."""

    def setUp(self):
        self.cli = _import_cli()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def _git(self, repo: Path, *args: str) -> None:
        subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True, text=True)

    def test_diff_shows_in_sync(self):
        """When live == canonical, diff reports 'in sync'."""
        from test_cli import FakeTarget
        repo = self.root / "repo"
        repo.mkdir()
        self._git(repo, "init", "-q", "-b", "main")

        profile = self.root / "profile"
        profile.mkdir()
        (profile / "Bookmarks").write_text(json.dumps({"roots": {"bookmark_bar": {"children": []}}}))

        state_dir = repo / "state"
        state_dir.mkdir()
        (state_dir / "fake_bookmarks.json").write_text(
            json.dumps({"roots": {"bookmark_bar": {"children": []}}})
        )

        old_targets = self.cli.ALL_TARGETS
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
                rc = self.cli.cmd_diff(SimpleNamespace(profile=profile, target=None))
            finally:
                sys.stdout = orig_stdout
            output = captured.getvalue()

            self.assertEqual(rc, 0)
            self.assertIn("in sync", output)
        finally:
            self.cli.ALL_TARGETS = old_targets

    def test_diff_shows_changes(self):
        """When live != canonical, diff shows + / - annotations."""
        from test_cli import FakeTarget
        repo = self.root / "repo"
        repo.mkdir()
        self._git(repo, "init", "-q", "-b", "main")

        profile = self.root / "profile"
        profile.mkdir()
        live_data = {"roots": {"bookmark_bar": {"children": [
            {"type": "url", "name": "New", "url": "https://new.com"}
        ]}}}
        (profile / "Bookmarks").write_text(json.dumps(live_data))

        state_dir = repo / "state"
        state_dir.mkdir()
        canon_data = {"roots": {"bookmark_bar": {"children": [
            {"type": "url", "name": "Old", "url": "https://old.com"}
        ]}}}
        (state_dir / "fake_bookmarks.json").write_text(json.dumps(canon_data))

        old_targets = self.cli.ALL_TARGETS
        old_targets_def = self.cli.ALL_TARGETS
        try:
            # Use Bookmarks target for real diff output
            from targets.bookmarks import Bookmarks
            self.cli.ALL_TARGETS = [Bookmarks()]
            self.cli.REPO_ROOT = repo
            self.cli.STATE_DIR = state_dir
            self.cli.LOGS_DIR = repo / "logs"

            (profile / "Default").mkdir()
            bm = profile / "Default" / "Bookmarks"
            bm.write_text(json.dumps(live_data))

            import io
            captured = io.StringIO()
            orig_stdout = sys.stdout
            sys.stdout = captured
            try:
                rc = self.cli.cmd_diff(SimpleNamespace(profile=profile, target=None))
            finally:
                sys.stdout = orig_stdout
            output = captured.getvalue()

            self.assertEqual(rc, 0)
            # Should show differences (not "in sync")
            self.assertNotIn("in sync", output)
            # Should have at least + or - markers
            self.assertTrue("+" in output or "-" in output, f"Expected +/- in output: {output}")
        finally:
            self.cli.ALL_TARGETS = old_targets_def

    def test_diff_no_canonical_state(self):
        """When no canonical state exists, diff reports it."""
        repo = self.root / "repo"
        repo.mkdir()
        self._git(repo, "init", "-q", "-b", "main")

        profile = self.root / "profile"
        (profile / "Default").mkdir(parents=True)
        (profile / "Default" / "Bookmarks").write_text(json.dumps({"roots": {"bookmark_bar": {"children": []}}}))

        from targets.bookmarks import Bookmarks
        old_targets = self.cli.ALL_TARGETS
        try:
            self.cli.ALL_TARGETS = [Bookmarks()]
            self.cli.REPO_ROOT = repo
            self.cli.STATE_DIR = repo / "state"
            self.cli.LOGS_DIR = repo / "logs"

            import io
            captured = io.StringIO()
            orig_stdout = sys.stdout
            sys.stdout = captured
            try:
                rc = self.cli.cmd_diff(SimpleNamespace(profile=profile, target=None))
            finally:
                sys.stdout = orig_stdout
            output = captured.getvalue()

            self.assertEqual(rc, 0)
            self.assertIn("no canonical state", output)
        finally:
            self.cli.ALL_TARGETS = old_targets


if __name__ == "__main__":
    unittest.main()
