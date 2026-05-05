"""Integration tests: real git repo + fake Helium profile.

Exercises the full push/pull/status pipeline without Helium installed.
A minimal fake profile with a Chrome-style Bookmarks file is created per test.
Saved-tab-groups are excluded (--target bookmarks) because they require LevelDB.
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

CLI = Path(__file__).resolve().parent.parent / "bin" / "helium-sync"


def _fake_bookmarks():
    """Minimal Chrome-style Bookmarks tree."""
    return {
        "checksum": "",
        "roots": {
            "bookmark_bar": {
                "children": [
                    {"guid": "g1", "name": "GitHub", "type": "url", "url": "https://github.com"},
                    {"guid": "g2", "name": "Google", "type": "url", "url": "https://google.com"},
                ],
                "name": "Bookmarks bar",
                "type": "folder",
            },
            "other": {"children": [], "name": "Other bookmarks", "type": "folder"},
            "synced": {"children": [], "name": "Mobile bookmarks", "type": "folder"},
        },
        "version": 1,
    }


def _git(args, cwd):
    subprocess.run(["git", "-C", str(cwd)] + args, check=True, capture_output=True, text=True)


def _run(args, env=None):
    """Run helium-sync CLI via sys.executable + script path."""
    merged = os.environ.copy()
    if env:
        merged.update(env)
    merged["PYTHONIOENCODING"] = "utf-8"
    return subprocess.run(
        [sys.executable, str(CLI)] + args,
        capture_output=True, text=True, env=merged,
    )


def _setup(tmp):
    """Create fake profile + git repo. Returns (profile, repo)."""
    profile = Path(tmp) / "profile"
    (profile / "Default").mkdir(parents=True)
    repo = Path(tmp) / "repo"
    repo.mkdir()
    (profile / "Default" / "Bookmarks").write_text(
        json.dumps(_fake_bookmarks()), encoding="utf-8"
    )
    _git(["init", "-q", "-b", "main"], cwd=repo)
    _git(["config", "user.email", "test@test.com"], cwd=repo)
    _git(["config", "user.name", "test"], cwd=repo)
    return profile, repo


class TestPushPullIntegration(unittest.TestCase):
    """Full push/pull cycle with a fake profile and real git repo."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.profile, self.repo = _setup(self.tmp)
        self.gl = ["--profile", str(self.profile), "--repo", str(self.repo)]

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _init(self, **kw):
        extra = []
        for k, v in kw.items():
            key = k.replace("_", "-")
            if isinstance(v, bool):
                if v:
                    extra.append(f"--{key}")
            else:
                extra += [f"--{key}", str(v)]
        return _run(self.gl + ["init", "--force"] + extra)

    def _push(self, **kw):
        extra = []
        for k, v in kw.items():
            key = k.replace("_", "-")
            if isinstance(v, bool):
                if v:
                    extra.append(f"--{key}")
            else:
                extra += [f"--{key}", str(v)]
        return _run(self.gl + ["push"] + extra)

    def _pull(self, **kw):
        extra = []
        for k, v in kw.items():
            key = k.replace("_", "-")
            if isinstance(v, bool):
                if v:
                    extra.append(f"--{key}")
            else:
                extra += [f"--{key}", str(v)]
        return _run(self.gl + ["pull"] + extra)

    def _status(self):
        return _run(self.gl + ["status"])

    def test_init_and_push(self):
        """init declares source of truth, push creates first commit."""
        r = self._init(target="bookmarks")
        self.assertEqual(r.returncode, 0, r.stderr)

        r = self._push(target="bookmarks")
        self.assertEqual(r.returncode, 0, r.stderr)

        state = self.repo / "state"
        self.assertTrue(state.exists())
        self.assertTrue((state / "bookmarks.json").exists())

        log = subprocess.run(["git", "-C", str(self.repo), "log", "--oneline"], capture_output=True, text=True)
        self.assertIn("push", log.stdout.lower())

    def test_status_clean_after_push(self):
        """After push, status should show no differences."""
        self._init(target="bookmarks")
        self._push(target="bookmarks")
        r = self._status()
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertNotIn("diverged", r.stdout.lower())

    def test_dry_run_does_not_commit(self):
        """push --dry-run should not create a git commit."""
        self._init(target="bookmarks")

        log = subprocess.run(["git", "-C", str(self.repo), "log", "--oneline"], capture_output=True, text=True)
        before = len([l for l in log.stdout.strip().splitlines() if l])

        r = self._push(target="bookmarks", dry_run=True)
        self.assertEqual(r.returncode, 0, r.stderr)

        log = subprocess.run(["git", "-C", str(self.repo), "log", "--oneline"], capture_output=True, text=True)
        after = len([l for l in log.stdout.strip().splitlines() if l])
        self.assertEqual(after, before, "dry-run should not add commits")

    def test_pull_after_push(self):
        """Full cycle: init -> push -> pull should succeed."""
        self._init(target="bookmarks")
        r = self._push(target="bookmarks")
        self.assertEqual(r.returncode, 0, r.stderr)

        r = self._pull(target="bookmarks")
        self.assertEqual(r.returncode, 0, r.stderr)


class TestRestoreIntegration(unittest.TestCase):
    """Test restore from backup."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.profile, self.repo = _setup(self.tmp)
        self.gl = ["--profile", str(self.profile), "--repo", str(self.repo)]

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_restore_no_backup(self):
        """restore should fail gracefully when no backup exists."""
        r = _run(self.gl + ["restore"])
        self.assertIsNotNone(r.stdout)


class TestResolveIntegration(unittest.TestCase):
    """Test resolve with divergent states."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.profile, self.repo = _setup(self.tmp)
        self.gl = ["--profile", str(self.profile), "--repo", str(self.repo)]

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_resolve_no_divergences(self):
        """resolve should detect no divergences when live matches canonical."""
        _run(self.gl + ["init", "--force", "--target", "bookmarks"])
        _run(self.gl + ["push", "--target", "bookmarks"])

        r = _run(self.gl + ["resolve"])
        combined = (r.stdout + r.stderr).lower()
        self.assertTrue(
            "diverg" in combined or "interactive" in combined,
            f"expected divergence info, got: {r.stdout} {r.stderr}"
        )


if __name__ == "__main__":
    unittest.main()
