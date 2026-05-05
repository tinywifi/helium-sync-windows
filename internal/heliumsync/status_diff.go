package heliumsync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (a App) Status(target string) int {
	fmt.Printf("Repo:    %s\nProfile: %s\nHost:    %s\nHelium running here: %s\n\n", a.RepoRoot, a.Profile, Hostname(), yesNo(HeliumRunning()))
	fmt.Println("Recent commits:")
	out, _ := a.RunGit("log", "--oneline", "--decorate", "-5")
	if strings.TrimSpace(out) == "" {
		fmt.Println("  (none)")
	} else {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			fmt.Println("  " + line)
		}
	}
	fmt.Println("\nPer target:")
	for _, t := range a.targetByName(target) {
		live, err := t.Extract(a.Profile)
		if err != nil {
			fmt.Printf("  %-18s extract failed: %v\n", t.Name(), err)
			continue
		}
		raw, err := os.ReadFile(filepath.Join(a.StateDir, t.StateFilename()))
		if err != nil {
			fmt.Printf("  %-18s live=%s; canonical=(absent -- run `helium-sync init`)\n", t.Name(), t.Summary(live))
			continue
		}
		canonical, err := t.Deserialize(string(raw))
		if err != nil {
			fmt.Printf("  %-18s canonical parse failed: %v\n", t.Name(), err)
			continue
		}
		if t.SemanticallyEqual(live, canonical) {
			fmt.Printf("  %-18s [ok] %s  (in sync)\n", t.Name(), t.Summary(live))
		} else {
			fmt.Printf("  %-18s live=%s | canonical=%s | differs (push to propagate)\n", t.Name(), t.Summary(live), t.Summary(canonical))
		}
	}
	return 0
}

func (a App) Diff(target string) int {
	fmt.Printf("Repo:    %s\nProfile: %s\nHost:    %s\n\n", a.RepoRoot, a.Profile, Hostname())
	for _, t := range a.targetByName(target) {
		live, err := t.Extract(a.Profile)
		if err != nil {
			fmt.Printf("  %s: extract failed: %v\n", t.Name(), err)
			continue
		}
		raw, err := os.ReadFile(filepath.Join(a.StateDir, t.StateFilename()))
		if err != nil {
			fmt.Printf("  %s: no canonical state (run `helium-sync init` first)\n", t.Name())
			continue
		}
		canonical, _ := t.Deserialize(string(raw))
		if t.SemanticallyEqual(live, canonical) {
			fmt.Printf("  %s: in sync - no changes\n\n", t.Name())
			continue
		}
		if t.Name() == "bookmarks" {
			PrintBookmarkDiff(asMap(live), asMap(canonical))
		} else {
			fmt.Printf("  %s: differs from canonical\n    (use `helium-sync status` for details)\n", t.Name())
		}
		fmt.Println()
	}
	return 0
}

func PrintBookmarkDiff(live, canonical map[string]any) {
	liveURLs := bookmarkURLMap(live)
	canonURLs := bookmarkURLMap(canonical)
	keys := map[string]bool{}
	for k := range liveURLs {
		keys[k] = true
	}
	for k := range canonURLs {
		keys[k] = true
	}
	all := make([]string, 0, len(keys))
	for k := range keys {
		all = append(all, k)
	}
	sort.Strings(all)
	fmt.Println("  bookmarks:")
	for _, key := range all {
		liveName, canonName := liveURLs[key], canonURLs[key]
		parts := strings.SplitN(key, "\x00", 2)
		path, url := parts[0], ""
		if len(parts) > 1 {
			url = parts[1]
		}
		if canonName != "" && liveName == "" {
			fmt.Printf("    - [%s] %s (%s)\n", path, canonName, url)
		} else if liveName != "" && canonName == "" {
			fmt.Printf("    + [%s] %s (%s)\n", path, liveName, url)
		} else if liveName != canonName {
			fmt.Printf("    ~ [%s] %s -> %s (%s)\n", path, canonName, liveName, url)
		}
	}
}

func bookmarkURLMap(tree map[string]any) map[string]string {
	out := map[string]string{}
	var walk func(string, map[string]any)
	walk = func(path string, node map[string]any) {
		for _, raw := range sliceValue(node["children"]) {
			child := asMap(raw)
			if str(child["type"]) == "url" {
				out[path+"\x00"+str(child["url"])] = str(child["name"])
			} else if str(child["type"]) == "folder" {
				walk(path+"/"+str(child["name"]), child)
			}
		}
	}
	for _, rk := range []string{"bookmark_bar", "other", "synced"} {
		if root, ok := asMap(tree["roots"])[rk]; ok {
			walk(rk, asMap(root))
		}
	}
	return out
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
