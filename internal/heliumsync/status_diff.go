package heliumsync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (a App) Status(target string) int {
	fmt.Println(uiBlock("status",
		renderKV("repo", a.RepoRoot, "245"),
		renderKV("profile", a.Profile, "245"),
		renderKV("host", Hostname(), "245"),
		renderKV("helium running", yesNo(HeliumRunning()), "245"),
	))
	fmt.Println()
	fmt.Println(uiSection("Recent commits"))
	out, _ := a.RunGit("log", "--oneline", "--decorate", "-5")
	if strings.TrimSpace(out) == "" {
		fmt.Println("  " + uiDim.Render("(none)"))
	} else {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			fmt.Println("  " + line)
		}
	}
	fmt.Println()
	fmt.Println(uiSection("Per target"))
	for _, t := range a.targetByName(target) {
		live, err := t.Extract(a.Profile)
		if err != nil {
			fmt.Println("  " + uiStatus("fail", t.Name(), "extract failed: "+err.Error()))
			continue
		}
		raw, err := os.ReadFile(filepath.Join(a.StateDir, t.StateFilename()))
		if err != nil {
			fmt.Printf("  %s\n", uiStatus("warn", t.Name(), "live="+t.Summary(live)+"; canonical=(absent -- run `helium-sync init`)"))
			continue
		}
		canonical, err := t.Deserialize(string(raw))
		if err != nil {
			fmt.Println("  " + uiStatus("fail", t.Name(), "canonical parse failed: "+err.Error()))
			continue
		}
		if t.SemanticallyEqual(live, canonical) {
			fmt.Printf("  %s\n", uiStatus("ok", t.Name(), t.Summary(live)+" (in sync)"))
		} else {
			fmt.Printf("  %s\n", uiStatus("warn", t.Name(), fmt.Sprintf("live=%s | canonical=%s | differs (push to propagate)", t.Summary(live), t.Summary(canonical))))
		}
	}
	return 0
}

func (a App) Diff(target string) int {
	fmt.Println(uiBlock("diff",
		renderKV("repo", a.RepoRoot, "245"),
		renderKV("profile", a.Profile, "245"),
		renderKV("host", Hostname(), "245"),
	))
	for _, t := range a.targetByName(target) {
		live, err := t.Extract(a.Profile)
		if err != nil {
			fmt.Printf("  %s\n", uiStatus("fail", t.Name(), "extract failed: "+err.Error()))
			continue
		}
		raw, err := os.ReadFile(filepath.Join(a.StateDir, t.StateFilename()))
		if err != nil {
			fmt.Printf("  %s\n", uiStatus("warn", t.Name(), "no canonical state (run `helium-sync init` first)"))
			continue
		}
		canonical, _ := t.Deserialize(string(raw))
		if t.SemanticallyEqual(live, canonical) {
			fmt.Printf("  %s\n\n", uiStatus("ok", t.Name(), "in sync - no changes"))
			continue
		}
		if t.Name() == "bookmarks" {
			PrintBookmarkDiff(asMap(live), asMap(canonical))
		} else {
			fmt.Printf("  %s\n    %s\n", uiStatus("warn", t.Name(), "differs from canonical"), uiDim.Render("use `helium-sync status` for details"))
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
	fmt.Println("  " + uiTitle.Render("bookmarks"))
	for _, key := range all {
		liveName, canonName := liveURLs[key], canonURLs[key]
		parts := strings.SplitN(key, "\x00", 2)
		path, url := parts[0], ""
		if len(parts) > 1 {
			url = parts[1]
		}
		if canonName != "" && liveName == "" {
			fmt.Printf("    %s [%s] %s (%s)\n", uiGood.Render("-"), path, canonName, url)
		} else if liveName != "" && canonName == "" {
			fmt.Printf("    %s [%s] %s (%s)\n", uiGood.Render("+"), path, liveName, url)
		} else if liveName != canonName {
			fmt.Printf("    %s [%s] %s -> %s (%s)\n", uiWarn.Render("~"), path, canonName, liveName, url)
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
