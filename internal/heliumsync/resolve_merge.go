package heliumsync

type TabGroupDivergences struct {
	LiveOnlyGroups  map[string]bool
	CanonOnlyGroups map[string]bool
	ConflictGroups  map[string]bool
	LiveOnlyTabs    map[string]bool
	CanonOnlyTabs   map[string]bool
	ConflictTabs    map[string]bool
}

type ResolveSelection struct {
	Type       string
	GUID       string
	Source     string
	Keep       bool
	KeepTheirs bool
}

func TabGroupDivergencesOf(live, canonical map[string]any) TabGroupDivergences {
	lg, lt := mapValue(live, "groups"), mapValue(live, "tabs")
	cg, ct := mapValue(canonical, "groups"), mapValue(canonical, "tabs")
	divs := TabGroupDivergences{
		LiveOnlyGroups:  map[string]bool{},
		CanonOnlyGroups: map[string]bool{},
		ConflictGroups:  map[string]bool{},
		LiveOnlyTabs:    map[string]bool{},
		CanonOnlyTabs:   map[string]bool{},
		ConflictTabs:    map[string]bool{},
	}
	for guid := range lg {
		if _, ok := cg[guid]; !ok {
			divs.LiveOnlyGroups[guid] = true
		}
	}
	for guid := range cg {
		if _, ok := lg[guid]; !ok {
			divs.CanonOnlyGroups[guid] = true
		}
	}
	for guid, raw := range lg {
		craw, ok := cg[guid]
		if !ok {
			continue
		}
		l, c := asMap(raw), asMap(craw)
		if str(l["title"]) != str(c["title"]) || int64Value(l["color"]) != int64Value(c["color"]) {
			divs.ConflictGroups[guid] = true
		}
	}
	for guid := range lt {
		if _, ok := ct[guid]; !ok {
			divs.LiveOnlyTabs[guid] = true
		}
	}
	for guid := range ct {
		if _, ok := lt[guid]; !ok {
			divs.CanonOnlyTabs[guid] = true
		}
	}
	for guid, raw := range lt {
		craw, ok := ct[guid]
		if !ok {
			continue
		}
		l, c := asMap(raw), asMap(craw)
		if str(l["url"]) != str(c["url"]) ||
			str(l["title"]) != str(c["title"]) ||
			str(l["group_guid"]) != str(c["group_guid"]) {
			divs.ConflictTabs[guid] = true
		}
	}
	return divs
}

func MergeTabGroups(live, canonical map[string]any, selections []ResolveSelection) map[string]any {
	lg, lt := mapValue(live, "groups"), mapValue(live, "tabs")
	cg, ct := mapValue(canonical, "groups"), mapValue(canonical, "tabs")
	divGroups, divTabs := map[string]bool{}, map[string]bool{}
	for _, sel := range selections {
		if sel.Type == "group" {
			divGroups[sel.GUID] = true
		} else if sel.Type == "tab" {
			divTabs[sel.GUID] = true
		}
	}
	mergedGroups := map[string]any{}
	mergedTabs := map[string]any{}
	for guid, value := range lg {
		if !divGroups[guid] {
			mergedGroups[guid] = value
		}
	}
	for guid, value := range cg {
		if !divGroups[guid] {
			if _, ok := mergedGroups[guid]; !ok {
				mergedGroups[guid] = value
			}
		}
	}
	for guid, value := range lt {
		if !divTabs[guid] {
			mergedTabs[guid] = value
		}
	}
	for guid, value := range ct {
		if !divTabs[guid] {
			if _, ok := mergedTabs[guid]; !ok {
				mergedTabs[guid] = value
			}
		}
	}
	for _, sel := range selections {
		if sel.Type == "group" {
			if sel.Source == "conflict" {
				if sel.KeepTheirs {
					mergedGroups[sel.GUID] = cg[sel.GUID]
				} else if sel.Keep {
					mergedGroups[sel.GUID] = lg[sel.GUID]
				}
			} else if sel.Keep {
				if sel.Source == "local" {
					mergedGroups[sel.GUID] = lg[sel.GUID]
				} else {
					mergedGroups[sel.GUID] = cg[sel.GUID]
				}
			}
		}
		if sel.Type == "tab" {
			if sel.Source == "conflict" {
				if sel.KeepTheirs {
					mergedTabs[sel.GUID] = ct[sel.GUID]
				} else if sel.Keep {
					mergedTabs[sel.GUID] = lt[sel.GUID]
				}
			} else if sel.Keep {
				if sel.Source == "local" {
					mergedTabs[sel.GUID] = lt[sel.GUID]
				} else {
					mergedTabs[sel.GUID] = ct[sel.GUID]
				}
			}
		}
	}
	return map[string]any{"groups": mergedGroups, "tabs": mergedTabs}
}
