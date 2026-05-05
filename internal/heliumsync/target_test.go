package heliumsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func urlNode(name, url string) map[string]any {
	return map[string]any{"type": "url", "name": name, "url": url, "id": "0", "guid": "g-" + name}
}

func folderNode(name string, children []any) map[string]any {
	return map[string]any{"type": "folder", "name": name, "children": children, "id": "0", "guid": "g-" + name}
}

func bookmarkTree(children []any) map[string]any {
	return map[string]any{
		"checksum": "abc",
		"version":  float64(1),
		"roots": map[string]any{
			"bookmark_bar": folderNode("Bookmarks Bar", children),
			"other":        folderNode("Other", nil),
			"synced":       folderNode("Mobile", nil),
		},
	}
}

func TestBookmarksExtractApplyAndSemanticEqual(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, "profile")
	live := filepath.Join(profile, "Default", "Bookmarks")
	if err := os.MkdirAll(filepath.Dir(live), 0755); err != nil {
		t.Fatal(err)
	}
	original := bookmarkTree([]any{urlNode("A", "https://a")})
	raw, _ := json.MarshalIndent(original, "", "   ")
	if err := os.WriteFile(live, raw, 0644); err != nil {
		t.Fatal(err)
	}
	b := Bookmarks{}
	data, err := b.Extract(profile)
	if err != nil {
		t.Fatal(err)
	}
	if asMap(data)["checksum"] != "" {
		t.Fatalf("checksum was not cleared: %#v", asMap(data)["checksum"])
	}
	next := bookmarkTree([]any{folderNode("F", []any{urlNode("B", "https://b")})})
	if err := b.Apply(profile, next, filepath.Join(tmp, "backup")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "backup", "Bookmarks")); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	got, err := b.Extract(profile)
	if err != nil {
		t.Fatal(err)
	}
	if !b.SemanticallyEqual(next, got) {
		t.Fatalf("applied bookmarks not semantically equal")
	}
}

func TestBookmarksSerializeRoundTripDeterministicAndIndented(t *testing.T) {
	b := Bookmarks{}
	data := bookmarkTree([]any{urlNode("A", "https://a"), urlNode("B", "https://b")})
	text, err := b.Serialize(data)
	if err != nil {
		t.Fatal(err)
	}
	back, err := b.Deserialize(text)
	if err != nil {
		t.Fatal(err)
	}
	if !b.SemanticallyEqual(data, back) {
		t.Fatalf("roundtrip not semantically equal")
	}
	text2, _ := b.Serialize(data)
	if text != text2 {
		t.Fatalf("serialization not deterministic")
	}
	if !strings.Contains(text, "\n   ") {
		t.Fatalf("expected Chromium-style indent=3 JSON")
	}
}

func TestBookmarksSemanticEqualIgnoresIDsAndOrder(t *testing.T) {
	b := Bookmarks{}
	a := bookmarkTree([]any{urlNode("A", "https://a"), urlNode("B", "https://b")})
	c := bookmarkTree([]any{urlNode("B", "https://b"), urlNode("A", "https://a")})
	asMap(sliceValue(asMap(asMap(a["roots"])["bookmark_bar"])["children"])[0])["id"] = "1"
	asMap(sliceValue(asMap(asMap(c["roots"])["bookmark_bar"])["children"])[1])["id"] = "999"
	if !b.SemanticallyEqual(a, c) {
		t.Fatalf("expected semantic equality")
	}
}

func TestBookmarksSemanticEqualNegativeCases(t *testing.T) {
	b := Bookmarks{}
	if b.SemanticallyEqual(bookmarkTree([]any{urlNode("A", "https://a")}), bookmarkTree([]any{urlNode("A", "https://b")})) {
		t.Fatalf("different URLs should not be equal")
	}
	if b.SemanticallyEqual(bookmarkTree([]any{urlNode("A", "https://a")}), bookmarkTree([]any{urlNode("B", "https://a")})) {
		t.Fatalf("different names should not be equal")
	}
	a := bookmarkTree([]any{folderNode("F", nil)})
	c := bookmarkTree([]any{folderNode("G", nil)})
	if b.SemanticallyEqual(a, c) {
		t.Fatalf("different folder sets should not be equal")
	}
	otherRoot := bookmarkTree(nil)
	asMap(asMap(otherRoot["roots"])["other"])["children"] = []any{urlNode("A", "https://a")}
	if b.SemanticallyEqual(bookmarkTree([]any{urlNode("A", "https://a")}), otherRoot) {
		t.Fatalf("same URL in different roots should not be equal")
	}
}

func TestSavedTabGroupsSerializeAndSemanticEqual(t *testing.T) {
	s := SavedTabGroups{}
	a := map[string]any{
		"groups": map[string]any{"g1": map[string]any{"guid": "g1", "title": "Dev", "color": float64(1), "position": float64(0), "creation_time": float64(1), "update_time": float64(1), "version": float64(1)}},
		"tabs":   map[string]any{"t1": map[string]any{"guid": "t1", "group_guid": "g1", "url": "https://example.com", "title": "Example", "position": float64(0)}},
	}
	text, err := s.Serialize(a)
	if err != nil {
		t.Fatal(err)
	}
	back, err := s.Deserialize(text)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SemanticallyEqual(a, back) {
		t.Fatalf("roundtrip not semantically equal")
	}
}

func TestSavedTabGroupsSemanticEqualNegativeCases(t *testing.T) {
	s := SavedTabGroups{}
	base := map[string]any{
		"groups": map[string]any{"g1": map[string]any{"guid": "g1", "title": "Dev", "color": float64(1), "position": float64(0), "creation_time": float64(1), "update_time": float64(1)}},
		"tabs":   map[string]any{"t1": map[string]any{"guid": "t1", "group_guid": "g1", "url": "https://a", "title": "A", "position": float64(0)}},
	}
	diffTitle := map[string]any{
		"groups": map[string]any{"g1": map[string]any{"guid": "g1", "title": "Other", "color": float64(1), "position": float64(0)}},
		"tabs":   map[string]any{"t1": map[string]any{"guid": "t1", "group_guid": "g1", "url": "https://a", "title": "A", "position": float64(0)}},
	}
	if s.SemanticallyEqual(base, diffTitle) {
		t.Fatalf("different group title should not be equal")
	}
	diffURL := map[string]any{
		"groups": map[string]any{"g1": map[string]any{"guid": "g1", "title": "Dev", "color": float64(1), "position": float64(0)}},
		"tabs":   map[string]any{"t1": map[string]any{"guid": "t1", "group_guid": "g1", "url": "https://b", "title": "A", "position": float64(0)}},
	}
	if s.SemanticallyEqual(base, diffURL) {
		t.Fatalf("different tab URL should not be equal")
	}
	diffPosition := map[string]any{
		"groups": map[string]any{"g1": map[string]any{"guid": "g1", "title": "Dev", "color": float64(1), "position": float64(0)}},
		"tabs":   map[string]any{"t1": map[string]any{"guid": "t1", "group_guid": "g1", "url": "https://a", "title": "A", "position": float64(1)}},
	}
	if s.SemanticallyEqual(base, diffPosition) {
		t.Fatalf("different tab position should not be equal")
	}
	timeOnly := map[string]any{
		"groups": map[string]any{"g1": map[string]any{"guid": "g1", "title": "Dev", "color": float64(1), "position": float64(0), "creation_time": float64(999), "update_time": float64(999)}},
		"tabs":   map[string]any{"t1": map[string]any{"guid": "t1", "group_guid": "g1", "url": "https://a", "title": "A", "position": float64(0)}},
	}
	if !s.SemanticallyEqual(base, timeOnly) {
		t.Fatalf("creation/update times should be ignored")
	}
}

func TestValidation(t *testing.T) {
	if issues := ValidateBookmarks(bookmarkTree([]any{urlNode("HTTP", "http://example.com"), urlNode("HTTPS", "https://example.com")})); len(issues) != 0 {
		t.Fatalf("http/https should pass: %#v", issues)
	}
	if issues := ValidateBookmarks(map[string]any{}); len(issues) == 0 {
		t.Fatalf("missing roots should fail")
	}
	if issues := ValidateBookmarks(bookmarkTree([]any{map[string]any{"type": "url", "name": "Bad", "url": ""}})); len(issues) == 0 {
		t.Fatalf("empty URL should warn")
	}
	if issues := ValidateBookmarks(bookmarkTree([]any{urlNode("FTP", "ftp://files")})); len(issues) == 0 {
		t.Fatalf("expected bookmark scheme warning")
	}
	data := map[string]any{
		"groups": map[string]any{},
		"tabs":   map[string]any{"t1": map[string]any{"guid": "t1", "group_guid": "missing", "url": "https://a"}},
	}
	if issues := ValidateTabGroups(data); len(issues) == 0 {
		t.Fatalf("expected tab group warning")
	}
}

func TestProtoRoundTrip(t *testing.T) {
	g := map[string]any{"guid": "g1", "title": "Dev", "color": float64(5), "position": float64(2), "creation_time": float64(100), "update_time": float64(101), "version": float64(1)}
	spec, err := decodeSpecifics(decodeMustWrapper(encodeWrapperSpecifics(encodeGroup(g))))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Kind != "group" || spec.GUID != "g1" || spec.Title != "Dev" || spec.Color != 5 || spec.Position != 2 {
		t.Fatalf("bad group decode: %#v", spec)
	}
	tab := map[string]any{"guid": "t1", "group_guid": "g1", "url": "https://a", "title": "A", "position": float64(3)}
	spec, err = decodeSpecifics(decodeMustWrapper(encodeWrapperSpecifics(encodeTab(tab))))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Kind != "tab" || spec.GroupGUID != "g1" || spec.URL != "https://a" || spec.Position != 3 {
		t.Fatalf("bad tab decode: %#v", spec)
	}
}

func decodeMustWrapper(b []byte) []byte {
	v, err := decodeWrapperSpecifics(b)
	if err != nil {
		panic(err)
	}
	return v
}
