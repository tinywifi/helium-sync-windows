package heliumsync

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T, repo string) {
	t.Helper()
	if err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").Run(); err != nil {
		t.Fatal(err)
	}
	_ = exec.Command("git", "-C", repo, "config", "user.email", "test@example.invalid").Run()
	_ = exec.Command("git", "-C", repo, "config", "user.name", "helium-sync-test").Run()
}

func seedProfile(t *testing.T, profile string, data map[string]any) {
	t.Helper()
	live := filepath.Join(profile, "Default", "Bookmarks")
	if err := os.MkdirAll(filepath.Dir(live), 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(data)
	if err := os.WriteFile(live, raw, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRepoPrecedence(t *testing.T) {
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(cfg, []byte(`repo = "`+filepath.ToSlash(filepath.Join(tmp, "cfg"))+`"`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := ResolveRepo(filepath.Join(tmp, "cli"), []string{"HELIUM_SYNC_REPO=" + filepath.Join(tmp, "env")}, cfg); got != abs(filepath.Join(tmp, "cli")) {
		t.Fatalf("cli did not win: %s", got)
	}
	if got := ResolveRepo("", []string{"HELIUM_SYNC_REPO=" + filepath.Join(tmp, "env")}, cfg); got != abs(filepath.Join(tmp, "env")) {
		t.Fatalf("env did not win: %s", got)
	}
	if got := ResolveRepo("", nil, cfg); got != abs(filepath.Join(tmp, "cfg")) {
		t.Fatalf("config did not win: %s", got)
	}
}

func TestPushPullExportImportRestore(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	initRepo(t, repo)
	source := filepath.Join(tmp, "source")
	dest := filepath.Join(tmp, "dest")
	original := bookmarkTree([]any{urlNode("Example", "https://example.com")})
	seedProfile(t, source, original)
	app := New(repo, source)
	app.Targets = []Target{Bookmarks{}}
	if rc := app.Push("", false, false); rc != 0 {
		t.Fatalf("push rc=%d", rc)
	}
	if _, err := os.Stat(filepath.Join(repo, "state", "bookmarks.json")); err != nil {
		t.Fatalf("state not written: %v", err)
	}
	seedProfile(t, dest, bookmarkTree([]any{urlNode("Old", "https://old")}))
	app.Profile = dest
	if rc := app.Pull("", true, false); rc != 0 {
		t.Fatalf("pull rc=%d", rc)
	}
	got, err := (Bookmarks{}).Extract(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !(Bookmarks{}).SemanticallyEqual(original, got) {
		t.Fatalf("dest did not match canonical")
	}
	exportPath := filepath.Join(tmp, "export.json")
	if rc := app.Export(exportPath, ""); rc != 0 {
		t.Fatalf("export rc=%d", rc)
	}
	seedProfile(t, dest, bookmarkTree([]any{urlNode("Changed", "https://changed")}))
	if rc := app.Import(exportPath, "", true); rc != 0 {
		t.Fatalf("import rc=%d", rc)
	}
	if rc := app.Restore(true); rc != 0 {
		t.Fatalf("restore rc=%d", rc)
	}
}

func TestDryRunsDoNotWrite(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	_ = os.MkdirAll(repo, 0755)
	initRepo(t, repo)
	profile := filepath.Join(tmp, "profile")
	seedProfile(t, profile, bookmarkTree([]any{urlNode("A", "https://a")}))
	app := New(repo, profile)
	app.Targets = []Target{Bookmarks{}}
	if rc := app.Push("", false, true); rc != 0 {
		t.Fatalf("dry push rc=%d", rc)
	}
	if _, err := os.Stat(filepath.Join(repo, "state", "bookmarks.json")); !os.IsNotExist(err) {
		t.Fatalf("dry push wrote state")
	}
}
