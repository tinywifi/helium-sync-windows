"""Tests for resolve helpers: Iterator type annotation, tab-group divergence
detection, and tab-group merge logic.

Run from the repo root: python -m unittest discover tests/
"""

import importlib.machinery
import importlib.util
import sys
import unittest
from pathlib import Path

_BIN = Path(__file__).resolve().parent.parent / "bin"
sys.path.insert(0, str(_BIN))

# Load bin/helium-sync (no .py extension) as a module.
# SourceFileLoader is required because spec_from_file_location can't infer the
# loader for an extension-less file.
# The auto-relaunch block only fires when sys.executable != .venv python, so it
# is a no-op when running inside the venv (the normal test environment).
_loader = importlib.machinery.SourceFileLoader("helium_sync", str(_BIN / "helium-sync"))
_spec   = importlib.util.spec_from_loader("helium_sync", _loader)
_hs     = importlib.util.module_from_spec(_spec)
_loader.exec_module(_hs)

_walk_all_nodes        = _hs._walk_all_nodes
_tab_group_divergences = _hs._tab_group_divergences
_merge_tab_groups      = _hs._merge_tab_groups
_merge_bookmark_tree   = _hs._merge_bookmark_tree


# --------------------------------------------------------------------------- #
# Fixture builders
# --------------------------------------------------------------------------- #

def make_group(guid, title="Group", color=0, position=0):
    return {"guid": guid, "title": title, "color": color, "position": position,
            "creation_time": 0, "update_time": 0, "version": 0}


def make_tab(guid, group_guid, url="https://example.com", title="Tab", position=0):
    return {"guid": guid, "group_guid": group_guid, "url": url, "title": title,
            "position": position, "creation_time": 0, "update_time": 0, "version": 0}


def stg(groups=None, tabs=None):
    return {"groups": groups or {}, "tabs": tabs or {}}


def bm_url_node(name, url):
    return {"type": "url", "name": name, "url": url}


def bm_folder(name, children=None):
    return {"type": "folder", "name": name, "children": children or []}


# --------------------------------------------------------------------------- #
# Iterator type annotation (regression: Iterator was not imported from typing)
# --------------------------------------------------------------------------- #

class TestWalkAllNodes(unittest.TestCase):
    """_walk_all_nodes uses -> Iterator[dict].  If Iterator isn't imported the
    annotation blows up under get_type_hints(); the test below also validates
    the generator yields correctly."""

    def test_yields_self_only_for_leaf(self):
        node = {"type": "url", "name": "A", "url": "https://a"}
        self.assertEqual(list(_walk_all_nodes(node)), [node])

    def test_yields_self_and_child(self):
        child = {"type": "url", "name": "C", "url": "https://c"}
        root = {"type": "folder", "name": "R", "children": [child]}
        result = list(_walk_all_nodes(root))
        self.assertIn(root,  result)
        self.assertIn(child, result)
        self.assertEqual(len(result), 2)

    def test_recursive_three_levels(self):
        grand = {"type": "url", "name": "G"}
        child = {"type": "folder", "name": "C", "children": [grand]}
        root  = {"type": "folder", "name": "R", "children": [child]}
        self.assertEqual(len(list(_walk_all_nodes(root))), 3)

    def test_empty_children(self):
        node = {"type": "folder", "name": "X", "children": []}
        self.assertEqual(list(_walk_all_nodes(node)), [node])

    def test_get_type_hints_does_not_raise(self):
        import typing
        hints = typing.get_type_hints(_walk_all_nodes)
        self.assertIn("return", hints)


# --------------------------------------------------------------------------- #
# _tab_group_divergences
# --------------------------------------------------------------------------- #

class TestTabGroupDivergences(unittest.TestCase):

    def test_identical_states_no_divergences(self):
        g = {"g1": make_group("g1", "Work")}
        t = {"t1": make_tab("t1", "g1")}
        s = stg(g, t)
        d = _tab_group_divergences(s, s)
        for key in d.values():
            self.assertEqual(key, set(), msg=f"{key} expected empty")

    def test_live_only_group(self):
        d = _tab_group_divergences(stg({"g1": make_group("g1")}), stg())
        self.assertIn("g1", d["live_only_groups"])
        self.assertEqual(d["canon_only_groups"], set())

    def test_canon_only_group(self):
        d = _tab_group_divergences(stg(), stg({"g1": make_group("g1")}))
        self.assertIn("g1", d["canon_only_groups"])
        self.assertEqual(d["live_only_groups"], set())

    def test_conflict_group_different_title(self):
        live = stg({"g1": make_group("g1", title="Work")})
        can  = stg({"g1": make_group("g1", title="Research")})
        d = _tab_group_divergences(live, can)
        self.assertIn("g1", d["conflict_groups"])
        self.assertEqual(d["live_only_groups"],  set())
        self.assertEqual(d["canon_only_groups"], set())

    def test_conflict_group_different_color(self):
        live = stg({"g1": make_group("g1", color=1)})
        can  = stg({"g1": make_group("g1", color=2)})
        self.assertIn("g1", _tab_group_divergences(live, can)["conflict_groups"])

    def test_same_title_and_color_no_conflict(self):
        g = {"g1": make_group("g1", title="X", color=3)}
        self.assertEqual(_tab_group_divergences(stg(g), stg(g))["conflict_groups"], set())

    def test_live_only_tab(self):
        d = _tab_group_divergences(stg({}, {"t1": make_tab("t1", "g1")}), stg())
        self.assertIn("t1", d["live_only_tabs"])

    def test_canon_only_tab(self):
        d = _tab_group_divergences(stg(), stg({}, {"t1": make_tab("t1", "g1")}))
        self.assertIn("t1", d["canon_only_tabs"])

    def test_conflict_tab_different_url(self):
        live = stg({}, {"t1": make_tab("t1", "g1", url="https://a.com")})
        can  = stg({}, {"t1": make_tab("t1", "g1", url="https://b.com")})
        self.assertIn("t1", _tab_group_divergences(live, can)["conflict_tabs"])

    def test_conflict_tab_different_title(self):
        live = stg({}, {"t1": make_tab("t1", "g1", title="Old")})
        can  = stg({}, {"t1": make_tab("t1", "g1", title="New")})
        self.assertIn("t1", _tab_group_divergences(live, can)["conflict_tabs"])

    def test_conflict_tab_different_group_guid(self):
        live = stg({}, {"t1": make_tab("t1", "g1")})
        can  = stg({}, {"t1": make_tab("t1", "g2")})
        self.assertIn("t1", _tab_group_divergences(live, can)["conflict_tabs"])

    def test_same_tab_no_conflict(self):
        t = {"t1": make_tab("t1", "g1", url="https://x.com", title="X")}
        self.assertEqual(_tab_group_divergences(stg({}, t), stg({}, t))["conflict_tabs"], set())

    def test_multiple_divergence_types_independent(self):
        live = stg(
            {"g-only-live": make_group("g-only-live"), "g-conflict": make_group("g-conflict", title="A")},
            {"t-only-live": make_tab("t-only-live", "g-only-live")},
        )
        can = stg(
            {"g-only-can": make_group("g-only-can"), "g-conflict": make_group("g-conflict", title="B")},
            {"t-only-can": make_tab("t-only-can", "g-only-can")},
        )
        d = _tab_group_divergences(live, can)
        self.assertIn("g-only-live",  d["live_only_groups"])
        self.assertIn("g-only-can",   d["canon_only_groups"])
        self.assertIn("g-conflict",   d["conflict_groups"])
        self.assertIn("t-only-live",  d["live_only_tabs"])
        self.assertIn("t-only-can",   d["canon_only_tabs"])


# --------------------------------------------------------------------------- #
# _merge_tab_groups
# --------------------------------------------------------------------------- #

class TestMergeTabGroups(unittest.TestCase):

    def test_empty_inputs(self):
        self.assertEqual(_merge_tab_groups(stg(), stg(), []), {"groups": {}, "tabs": {}})

    def test_unchanged_groups_preserved(self):
        g = {"g1": make_group("g1")}
        merged = _merge_tab_groups(stg(g), stg(g), [])
        self.assertIn("g1", merged["groups"])

    def test_keep_local_group(self):
        live = stg({"g1": make_group("g1", "Work")})
        sel = [{"type": "group", "guid": "g1", "source": "local", "keep": True, "conflict": False}]
        self.assertIn("g1", _merge_tab_groups(live, stg(), sel)["groups"])

    def test_drop_local_group(self):
        live = stg({"g1": make_group("g1")})
        sel = [{"type": "group", "guid": "g1", "source": "local", "keep": False, "conflict": False}]
        self.assertNotIn("g1", _merge_tab_groups(live, stg(), sel)["groups"])

    def test_keep_canonical_group(self):
        can = stg({"g1": make_group("g1", "Research")})
        sel = [{"type": "group", "guid": "g1", "source": "canonical", "keep": True, "conflict": False}]
        merged = _merge_tab_groups(stg(), can, sel)
        self.assertEqual(merged["groups"]["g1"]["title"], "Research")

    def test_drop_canonical_group(self):
        can = stg({"g1": make_group("g1")})
        sel = [{"type": "group", "guid": "g1", "source": "canonical", "keep": False, "conflict": False}]
        self.assertNotIn("g1", _merge_tab_groups(stg(), can, sel)["groups"])

    def test_conflict_group_keep_local(self):
        live = stg({"g1": make_group("g1", "Work")})
        can  = stg({"g1": make_group("g1", "Research")})
        sel = [{"type": "group", "guid": "g1", "source": "conflict",
                "keep": True, "keep_theirs": False, "conflict": True}]
        self.assertEqual(_merge_tab_groups(live, can, sel)["groups"]["g1"]["title"], "Work")

    def test_conflict_group_keep_canonical(self):
        live = stg({"g1": make_group("g1", "Work")})
        can  = stg({"g1": make_group("g1", "Research")})
        sel = [{"type": "group", "guid": "g1", "source": "conflict",
                "keep": False, "keep_theirs": True, "conflict": True}]
        self.assertEqual(_merge_tab_groups(live, can, sel)["groups"]["g1"]["title"], "Research")

    def test_conflict_group_drop_both(self):
        live = stg({"g1": make_group("g1", "Work")})
        can  = stg({"g1": make_group("g1", "Research")})
        sel = [{"type": "group", "guid": "g1", "source": "conflict",
                "keep": False, "keep_theirs": False, "conflict": True}]
        self.assertNotIn("g1", _merge_tab_groups(live, can, sel)["groups"])

    def test_unchanged_group_and_divergent_coexist(self):
        g_common = make_group("g-common", "Shared")
        g_local  = make_group("g-local",  "Local Only")
        live = stg({"g-common": g_common, "g-local": g_local})
        can  = stg({"g-common": g_common})
        sel = [{"type": "group", "guid": "g-local", "source": "local", "keep": True, "conflict": False}]
        merged = _merge_tab_groups(live, can, sel)
        self.assertIn("g-common", merged["groups"])
        self.assertIn("g-local",  merged["groups"])

    def test_keep_local_tab(self):
        live = stg({}, {"t1": make_tab("t1", "g1")})
        sel = [{"type": "tab", "guid": "t1", "source": "local", "keep": True, "conflict": False}]
        self.assertIn("t1", _merge_tab_groups(live, stg(), sel)["tabs"])

    def test_drop_local_tab(self):
        live = stg({}, {"t1": make_tab("t1", "g1")})
        sel = [{"type": "tab", "guid": "t1", "source": "local", "keep": False, "conflict": False}]
        self.assertNotIn("t1", _merge_tab_groups(live, stg(), sel)["tabs"])

    def test_keep_canonical_tab(self):
        can = stg({}, {"t1": make_tab("t1", "g1", url="https://canon.com")})
        sel = [{"type": "tab", "guid": "t1", "source": "canonical", "keep": True, "conflict": False}]
        merged = _merge_tab_groups(stg(), can, sel)
        self.assertEqual(merged["tabs"]["t1"]["url"], "https://canon.com")

    def test_conflict_tab_keep_local(self):
        live = stg({}, {"t1": make_tab("t1", "g1", url="https://old.com")})
        can  = stg({}, {"t1": make_tab("t1", "g1", url="https://new.com")})
        sel = [{"type": "tab", "guid": "t1", "source": "conflict",
                "keep": True, "keep_theirs": False, "conflict": True}]
        self.assertEqual(_merge_tab_groups(live, can, sel)["tabs"]["t1"]["url"], "https://old.com")

    def test_conflict_tab_keep_canonical(self):
        live = stg({}, {"t1": make_tab("t1", "g1", url="https://old.com")})
        can  = stg({}, {"t1": make_tab("t1", "g1", url="https://new.com")})
        sel = [{"type": "tab", "guid": "t1", "source": "conflict",
                "keep": False, "keep_theirs": True, "conflict": True}]
        self.assertEqual(_merge_tab_groups(live, can, sel)["tabs"]["t1"]["url"], "https://new.com")

    def test_conflict_tab_drop_both(self):
        live = stg({}, {"t1": make_tab("t1", "g1", url="https://a.com")})
        can  = stg({}, {"t1": make_tab("t1", "g1", url="https://b.com")})
        sel = [{"type": "tab", "guid": "t1", "source": "conflict",
                "keep": False, "keep_theirs": False, "conflict": True}]
        self.assertNotIn("t1", _merge_tab_groups(live, can, sel)["tabs"])

    def test_mixed_group_and_tab_selections(self):
        live = stg({"g1": make_group("g1")}, {"t1": make_tab("t1", "g1")})
        can  = stg({"g2": make_group("g2")}, {"t2": make_tab("t2", "g2")})
        sel = [
            {"type": "group", "guid": "g1", "source": "local",     "keep": True,  "conflict": False},
            {"type": "group", "guid": "g2", "source": "canonical", "keep": False, "conflict": False},
            {"type": "tab",   "guid": "t1", "source": "local",     "keep": True,  "conflict": False},
            {"type": "tab",   "guid": "t2", "source": "canonical", "keep": True,  "conflict": False},
        ]
        merged = _merge_tab_groups(live, can, sel)
        self.assertIn("g1",  merged["groups"])
        self.assertNotIn("g2", merged["groups"])
        self.assertIn("t1",  merged["tabs"])
        self.assertIn("t2",  merged["tabs"])


if __name__ == "__main__":
    unittest.main()
