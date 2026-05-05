package heliumsync

import "testing"

func groupState(groups map[string]any, tabs map[string]any) map[string]any {
	if groups == nil {
		groups = map[string]any{}
	}
	if tabs == nil {
		tabs = map[string]any{}
	}
	return map[string]any{"groups": groups, "tabs": tabs}
}

func TestTabGroupDivergencesOf(t *testing.T) {
	live := groupState(
		map[string]any{
			"local": map[string]any{"title": "Local", "color": float64(1)},
			"both":  map[string]any{"title": "Live", "color": float64(1)},
		},
		map[string]any{
			"tab-local": map[string]any{"url": "https://local", "title": "Local", "group_guid": "local"},
			"tab-both":  map[string]any{"url": "https://live", "title": "Live", "group_guid": "both"},
		},
	)
	canonical := groupState(
		map[string]any{
			"canon": map[string]any{"title": "Canonical", "color": float64(1)},
			"both":  map[string]any{"title": "Canonical", "color": float64(1)},
		},
		map[string]any{
			"tab-canon": map[string]any{"url": "https://canon", "title": "Canonical", "group_guid": "canon"},
			"tab-both":  map[string]any{"url": "https://canon", "title": "Canonical", "group_guid": "both"},
		},
	)
	divs := TabGroupDivergencesOf(live, canonical)
	if !divs.LiveOnlyGroups["local"] || !divs.CanonOnlyGroups["canon"] || !divs.ConflictGroups["both"] {
		t.Fatalf("bad group divergences: %#v", divs)
	}
	if !divs.LiveOnlyTabs["tab-local"] || !divs.CanonOnlyTabs["tab-canon"] || !divs.ConflictTabs["tab-both"] {
		t.Fatalf("bad tab divergences: %#v", divs)
	}
}

func TestMergeTabGroups(t *testing.T) {
	live := groupState(
		map[string]any{
			"local": map[string]any{"title": "Local"},
			"both":  map[string]any{"title": "Live"},
		},
		map[string]any{"tab-both": map[string]any{"title": "Live"}},
	)
	canonical := groupState(
		map[string]any{
			"canon": map[string]any{"title": "Canonical"},
			"both":  map[string]any{"title": "Canonical"},
		},
		map[string]any{"tab-both": map[string]any{"title": "Canonical"}},
	)
	merged := MergeTabGroups(live, canonical, []ResolveSelection{
		{Type: "group", GUID: "local", Source: "local", Keep: true},
		{Type: "group", GUID: "canon", Source: "canonical", Keep: true},
		{Type: "group", GUID: "both", Source: "conflict", KeepTheirs: true},
		{Type: "tab", GUID: "tab-both", Source: "conflict", Keep: true},
	})
	groups := mapValue(merged, "groups")
	tabs := mapValue(merged, "tabs")
	if str(asMap(groups["both"])["title"]) != "Canonical" {
		t.Fatalf("expected canonical conflict group: %#v", groups["both"])
	}
	if str(asMap(tabs["tab-both"])["title"]) != "Live" {
		t.Fatalf("expected live conflict tab: %#v", tabs["tab-both"])
	}
	if _, ok := groups["local"]; !ok {
		t.Fatalf("missing selected local group")
	}
	if _, ok := groups["canon"]; !ok {
		t.Fatalf("missing selected canonical group")
	}
}

func TestMergeTabGroupsFullSelectionMatrix(t *testing.T) {
	live := groupState(
		map[string]any{"g1": map[string]any{"title": "Local"}, "g-conflict": map[string]any{"title": "Local Conflict"}},
		map[string]any{"t1": map[string]any{"url": "https://local"}, "t-conflict": map[string]any{"url": "https://local"}},
	)
	canonical := groupState(
		map[string]any{"g2": map[string]any{"title": "Canonical"}, "g-conflict": map[string]any{"title": "Canonical Conflict"}},
		map[string]any{"t2": map[string]any{"url": "https://canonical"}, "t-conflict": map[string]any{"url": "https://canonical"}},
	)
	dropped := MergeTabGroups(live, canonical, []ResolveSelection{
		{Type: "group", GUID: "g1", Source: "local", Keep: false},
		{Type: "group", GUID: "g2", Source: "canonical", Keep: false},
		{Type: "group", GUID: "g-conflict", Source: "conflict", Keep: false, KeepTheirs: false},
		{Type: "tab", GUID: "t1", Source: "local", Keep: false},
		{Type: "tab", GUID: "t2", Source: "canonical", Keep: false},
		{Type: "tab", GUID: "t-conflict", Source: "conflict", Keep: false, KeepTheirs: false},
	})
	if len(mapValue(dropped, "groups")) != 0 || len(mapValue(dropped, "tabs")) != 0 {
		t.Fatalf("expected all divergent items dropped: %#v", dropped)
	}
	theirs := MergeTabGroups(live, canonical, []ResolveSelection{
		{Type: "group", GUID: "g-conflict", Source: "conflict", KeepTheirs: true},
		{Type: "tab", GUID: "t-conflict", Source: "conflict", KeepTheirs: true},
	})
	if str(asMap(mapValue(theirs, "groups")["g-conflict"])["title"]) != "Canonical Conflict" {
		t.Fatalf("expected canonical conflict group")
	}
	if str(asMap(mapValue(theirs, "tabs")["t-conflict"])["url"]) != "https://canonical" {
		t.Fatalf("expected canonical conflict tab")
	}
}

func TestTabGroupDivergenceIndividualFields(t *testing.T) {
	base := groupState(
		map[string]any{"g": map[string]any{"title": "A", "color": float64(1)}},
		map[string]any{"t": map[string]any{"url": "u", "title": "T", "group_guid": "g"}},
	)
	same := groupState(
		map[string]any{"g": map[string]any{"title": "A", "color": float64(1)}},
		map[string]any{"t": map[string]any{"url": "u", "title": "T", "group_guid": "g"}},
	)
	if divs := TabGroupDivergencesOf(base, same); len(divs.ConflictGroups) != 0 || len(divs.ConflictTabs) != 0 {
		t.Fatalf("same state should have no conflicts: %#v", divs)
	}
	color := groupState(
		map[string]any{"g": map[string]any{"title": "A", "color": float64(2)}},
		map[string]any{"t": map[string]any{"url": "u", "title": "T", "group_guid": "g"}},
	)
	if !TabGroupDivergencesOf(base, color).ConflictGroups["g"] {
		t.Fatalf("color change should conflict")
	}
	tabTitle := groupState(
		map[string]any{"g": map[string]any{"title": "A", "color": float64(1)}},
		map[string]any{"t": map[string]any{"url": "u", "title": "Other", "group_guid": "g"}},
	)
	if !TabGroupDivergencesOf(base, tabTitle).ConflictTabs["t"] {
		t.Fatalf("tab title change should conflict")
	}
	tabGroup := groupState(
		map[string]any{"g": map[string]any{"title": "A", "color": float64(1)}},
		map[string]any{"t": map[string]any{"url": "u", "title": "T", "group_guid": "other"}},
	)
	if !TabGroupDivergencesOf(base, tabGroup).ConflictTabs["t"] {
		t.Fatalf("tab group_guid change should conflict")
	}
}
