package heliumsync

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureOutput(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	rc := fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return rc, buf.String()
}

func TestStatusAndDiffCommands(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	_ = os.MkdirAll(repo, 0755)
	initRepo(t, repo)
	profile := filepath.Join(tmp, "profile")
	seedProfile(t, profile, bookmarkTree([]any{urlNode("Live", "https://live")}))
	app := New(repo, profile)
	app.Targets = []Target{Bookmarks{}}
	if rc := app.Push("", false, false); rc != 0 {
		t.Fatalf("push rc=%d", rc)
	}

	seedProfile(t, profile, bookmarkTree([]any{urlNode("Changed", "https://changed")}))
	rc, out := captureOutput(t, func() int { return app.Status("") })
	if rc != 0 || !strings.Contains(out, "bookmarks") || !strings.Contains(out, "differs") {
		t.Fatalf("unexpected status rc=%d output=%s", rc, out)
	}
	rc, out = captureOutput(t, func() int { return app.Diff("") })
	if rc != 0 || !strings.Contains(out, "Changed") || !strings.Contains(out, "Live") || !strings.Contains(out, "bookmark_bar") {
		t.Fatalf("unexpected diff rc=%d output=%s", rc, out)
	}
}

func TestExportImportErrorsAndTargetFilter(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	_ = os.MkdirAll(filepath.Join(repo, "state"), 0755)
	app := New(repo, filepath.Join(tmp, "profile"))
	app.Targets = []Target{Bookmarks{}}

	if rc, _ := captureOutput(t, func() int { return app.Export("", "missing") }); rc != 1 {
		t.Fatalf("expected unknown target export failure, got %d", rc)
	}
	if rc, _ := captureOutput(t, func() int { return app.Import(filepath.Join(tmp, "nope.json"), "", true) }); rc != 1 {
		t.Fatalf("expected missing import failure, got %d", rc)
	}
	bad := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"hello":"world"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if rc, _ := captureOutput(t, func() int { return app.Import(bad, "", true) }); rc != 1 {
		t.Fatalf("expected invalid import failure, got %d", rc)
	}
	exportFile := filepath.Join(tmp, "export.json")
	payload := map[string]any{"targets": map[string]any{"bookmarks": map[string]any{"data": bookmarkTree(nil)}}}
	raw, _ := json.Marshal(payload)
	if err := os.WriteFile(exportFile, raw, 0644); err != nil {
		t.Fatal(err)
	}
	old := heliumRunningFunc
	heliumRunningFunc = func() bool { return true }
	defer func() { heliumRunningFunc = old }()
	if rc, _ := captureOutput(t, func() int { return app.Import(exportFile, "", false) }); rc != 4 {
		t.Fatalf("expected running-browser guard failure, got %d", rc)
	}
}

func TestRestoreNoBackups(t *testing.T) {
	app := New(filepath.Join(t.TempDir(), "repo"), filepath.Join(t.TempDir(), "profile"))
	if rc, out := captureOutput(t, func() int { return app.Restore(true) }); rc != 1 || !strings.Contains(out, "no logs") {
		t.Fatalf("unexpected restore no-backup rc=%d out=%s", rc, out)
	}
}

func TestRestorePicksLatestBackup(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	profile := filepath.Join(tmp, "profile")
	app := New(repo, profile)
	oldBackup := filepath.Join(repo, "logs", "prePull.20260101-120000")
	newBackup := filepath.Join(repo, "logs", "prePull.20260601-120000")
	_ = os.MkdirAll(oldBackup, 0755)
	_ = os.MkdirAll(newBackup, 0755)
	oldData := bookmarkTree([]any{urlNode("Old", "https://old")})
	newData := bookmarkTree([]any{urlNode("New", "https://new")})
	oldRaw, _ := json.Marshal(oldData)
	newRaw, _ := json.Marshal(newData)
	_ = os.WriteFile(filepath.Join(oldBackup, "Bookmarks"), oldRaw, 0644)
	_ = os.WriteFile(filepath.Join(newBackup, "Bookmarks"), newRaw, 0644)

	if rc, _ := captureOutput(t, func() int { return app.Restore(true) }); rc != 0 {
		t.Fatalf("restore rc=%d", rc)
	}
	gotRaw, err := os.ReadFile(filepath.Join(profile, "Bookmarks"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(gotRaw, &got); err != nil {
		t.Fatal(err)
	}
	if !(Bookmarks{}).SemanticallyEqual(newData, got) {
		t.Fatalf("restore did not pick latest backup")
	}
}

func TestCompletionAndInitGuards(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	_ = os.MkdirAll(filepath.Join(repo, "state"), 0755)
	app := New(repo, filepath.Join(tmp, "profile"))
	app.Targets = []Target{Bookmarks{}}
	stateFile := filepath.Join(repo, "state", "bookmarks.json")
	if err := os.WriteFile(stateFile, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if rc, out := captureOutput(t, func() int { return app.Init("", false, false, false) }); rc != 6 || !strings.Contains(out, "canonical state already exists") {
		t.Fatalf("unexpected init guard rc=%d out=%s", rc, out)
	}
	if rc, out := captureOutput(t, func() int { return app.Completion("powershell") }); rc != 0 || !strings.Contains(out, "Register-ArgumentCompleter") {
		t.Fatalf("unexpected powershell completion rc=%d out=%s", rc, out)
	}
	if rc, out := captureOutput(t, func() int { return app.Completion("cmd") }); rc != 0 || !strings.Contains(out, "doskey") {
		t.Fatalf("unexpected cmd completion rc=%d out=%s", rc, out)
	}
}

func TestVersionHelpersAndBanner(t *testing.T) {
	if tuple, ok := versionTuple("v0.1.2-3-gabc"); !ok || tuple != [3]int{0, 1, 2} {
		t.Fatalf("bad version tuple: %#v %v", tuple, ok)
	}
	if _, ok := versionTuple("unknown"); ok {
		t.Fatalf("unknown version should not parse")
	}
	banner := updateBanner("0.1.2", "0.1.3")
	if !strings.Contains(banner, "Update available: helium-sync 0.1.2 -> 0.1.3") ||
		!strings.Contains(banner, "scoop update && scoop update helium-sync") {
		t.Fatalf("bad banner: %s", banner)
	}
	if !isNewerVersion([3]int{0, 1, 3}, [3]int{0, 1, 2}) {
		t.Fatalf("expected newer version")
	}
}

func TestSetupAlreadyConfiguredShowsRepo(t *testing.T) {
	tmp := t.TempDir()
	appdata := filepath.Join(tmp, "appdata")
	t.Setenv("APPDATA", appdata)
	config := filepath.Join(appdata, "helium-sync", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(`repo = "C:/data/helium"`), 0644); err != nil {
		t.Fatal(err)
	}
	app := New(filepath.Join(tmp, "repo"), filepath.Join(tmp, "profile"))
	rc, out := captureOutput(t, func() int { return app.Setup(false, true) })
	if rc != 8 || !strings.Contains(out, "C:/data/helium") {
		t.Fatalf("unexpected setup configured rc=%d out=%s", rc, out)
	}
}



func TestBookmarkResolveRowsAndMerge(t *testing.T) {
	live := bookmarkTree([]any{
		urlNode("Live", "https://live"),
		urlNode("Shared Live", "https://shared"),
	})
	canonical := bookmarkTree([]any{
		urlNode("Canonical", "https://canonical"),
		urlNode("Shared Canon", "https://shared"),
	})
	rows := bookmarkResolveRows(live, canonical)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	var canonRow bookmarkResolveRow
	for _, row := range rows {
		if row.URL == "https://canonical" {
			canonRow = row
		}
	}
	if canonRow.Selected != "canonical" {
		t.Fatalf("canonical-only row should default to canonical: %#v", canonRow)
	}
	merged := mergeBookmarksFromRows(live, canonical, []bookmarkResolveRow{
		{Key: "bookmark_bar\x00https://live", Path: "bookmark_bar", URL: "https://live", LocalPresent: true, Selected: "local"},
		{Key: "bookmark_bar\x00https://canonical", Path: "bookmark_bar", URL: "https://canonical", CanonicalPresent: true, Selected: "canonical"},
		{Key: "bookmark_bar\x00https://shared", Path: "bookmark_bar", URL: "https://shared", LocalPresent: true, CanonicalPresent: true, Selected: "canonical"},
	})
	urls := bookmarkNodeByURL(merged)
	if _, ok := urls["https://canonical"]; !ok {
		t.Fatalf("expected canonical-only row to be inserted")
	}
	if str(urls["https://shared"]["name"]) != "Shared Canon" {
		t.Fatalf("expected canonical shared row to replace live entry")
	}
}

func TestUsageAndVersionScreens(t *testing.T) {
	usage := UsageScreen()
	if !strings.Contains(usage, "completion") || !strings.Contains(usage, "Windows fork") {
		t.Fatalf("unexpected usage screen: %s", usage)
	}
	version := VersionScreen("1.0.0", "abc123", "windows/amd64 go1.26.2")
	if !strings.Contains(version, "helium-sync") || !strings.Contains(version, "abc123") || !strings.Contains(version, "embedded goleveldb") {
		t.Fatalf("unexpected version screen: %s", version)
	}
}

func TestDefaultResolvedStateForTabGroupsKeepsCanonicalOnly(t *testing.T) {
	live := groupState(
		map[string]any{"local": map[string]any{"title": "Local"}},
		map[string]any{"tab-local": map[string]any{"title": "Local"}},
	)
	canonical := groupState(
		map[string]any{"canon": map[string]any{"title": "Canonical"}},
		map[string]any{"tab-canon": map[string]any{"title": "Canonical"}},
	)
	merged := asMap(defaultResolvedState("saved_tab_groups", live, canonical))
	if _, ok := mapValue(merged, "groups")["local"]; !ok {
		t.Fatalf("missing local-only group")
	}
	if _, ok := mapValue(merged, "groups")["canon"]; !ok {
		t.Fatalf("missing canonical-only group")
	}
	if _, ok := mapValue(merged, "tabs")["tab-canon"]; !ok {
		t.Fatalf("missing canonical-only tab")
	}
}

func TestMergeBookmarksDefaultMatchesPythonLiveTreeBehavior(t *testing.T) {
	live := bookmarkTree([]any{
		urlNode("Live", "https://live"),
		urlNode("Conflict Live", "https://same"),
	})
	canonical := bookmarkTree([]any{
		urlNode("Canonical", "https://canonical"),
		urlNode("Conflict Canon", "https://same"),
	})
	merged := MergeBookmarksDefault(live, canonical)
	urls := bookmarkNodeByURL(merged)
	if _, ok := urls["https://live"]; !ok {
		t.Fatalf("missing live URL")
	}
	if _, ok := urls["https://canonical"]; ok {
		t.Fatalf("canonical-only URL should not be added by Python-compatible merge")
	}
	if str(urls["https://same"]["name"]) != "Conflict Live" {
		t.Fatalf("default conflict should keep live name")
	}
}
