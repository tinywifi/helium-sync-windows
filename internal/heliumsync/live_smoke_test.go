package heliumsync

import (
	"os"
	"path/filepath"
	"testing"
)

func liveProfilePath() string {
	if profile := os.Getenv("HELIUM_PROFILE"); profile != "" {
		return profile
	}
	return DefaultProfile()
}

func requireLiveBookmarks(t *testing.T) string {
	t.Helper()
	profile := liveProfilePath()
	if _, err := os.Stat(filepath.Join(profile, "Default", "Bookmarks")); err != nil {
		t.Skip("real Helium bookmarks profile not available")
	}
	return profile
}

func requireLiveLevelDB(t *testing.T) string {
	t.Helper()
	profile := liveProfilePath()
	if _, err := os.Stat(filepath.Join(profile, "Default", "Sync Data", "LevelDB")); err != nil {
		t.Skip("real Helium LevelDB profile not available")
	}
	return profile
}

func TestLiveBookmarksExtract(t *testing.T) {
	profile := requireLiveBookmarks(t)
	b := Bookmarks{}
	data, err := b.Extract(profile)
	if err != nil {
		t.Fatal(err)
	}
	m := asMap(data)
	if str(m["checksum"]) != "" {
		t.Fatalf("expected checksum to be cleared")
	}
	if len(mapValue(m, "roots")) == 0 {
		t.Fatalf("expected bookmark roots")
	}
	text, err := b.Serialize(data)
	if err != nil {
		t.Fatal(err)
	}
	back, err := b.Deserialize(text)
	if err != nil {
		t.Fatal(err)
	}
	if !b.SemanticallyEqual(data, back) {
		t.Fatalf("live bookmark serialize roundtrip changed semantics")
	}
}

func TestLiveSavedTabGroupsExtract(t *testing.T) {
	profile := requireLiveLevelDB(t)
	s := SavedTabGroups{}
	data, err := s.Extract(profile)
	if err != nil {
		t.Fatal(err)
	}
	m := asMap(data)
	if _, ok := m["groups"]; !ok {
		t.Fatalf("expected groups key")
	}
	if _, ok := m["tabs"]; !ok {
		t.Fatalf("expected tabs key")
	}
	text, err := s.Serialize(data)
	if err != nil {
		t.Fatal(err)
	}
	back, err := s.Deserialize(text)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SemanticallyEqual(data, back) {
		t.Fatalf("live saved tab groups serialize roundtrip changed semantics")
	}
}

func TestLiveSavedTabGroupsApplyOnTempCopy(t *testing.T) {
	profile := requireLiveLevelDB(t)
	s := SavedTabGroups{}
	original, err := s.Extract(profile)
	if err != nil {
		t.Fatal(err)
	}
	tmpProfile := filepath.Join(t.TempDir(), "profile")
	src := filepath.Join(profile, "Default", "Sync Data", "LevelDB")
	dst := filepath.Join(tmpProfile, "Default", "Sync Data", "LevelDB")
	if err := copyDir(src, dst, false); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(tmpProfile, original, filepath.Join(tmpProfile, "logs")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Extract(tmpProfile)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SemanticallyEqual(original, got) {
		t.Fatalf("apply to temp copy changed saved tab group semantics")
	}
	if _, err := os.Stat(filepath.Join(tmpProfile, "logs", "LevelDB")); err != nil {
		t.Fatalf("expected LevelDB backup: %v", err)
	}
}
