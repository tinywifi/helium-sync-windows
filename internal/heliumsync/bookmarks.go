package heliumsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Bookmarks struct{}

func (Bookmarks) Name() string          { return "bookmarks" }
func (Bookmarks) StateFilename() string { return "bookmarks.json" }

func (Bookmarks) livePath(profile string) string {
	return filepath.Join(profile, "Default", "Bookmarks")
}

func (b Bookmarks) Extract(profile string) (any, error) {
	raw, err := os.ReadFile(b.livePath(profile))
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	data["checksum"] = ""
	return data, nil
}

func (b Bookmarks) Apply(profile string, data any, backupDir string) error {
	live := b.livePath(profile)
	if err := os.MkdirAll(filepath.Dir(live), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(live); err == nil {
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return err
		}
		if err := copyFile(live, filepath.Join(backupDir, "Bookmarks")); err != nil {
			return err
		}
	}
	text, err := b.Serialize(data)
	if err != nil {
		return err
	}
	tmp := live + ".helium-sync-tmp"
	if err := os.WriteFile(tmp, []byte(text), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, live)
}

func (Bookmarks) Serialize(data any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "   ")
	if err := enc.Encode(data); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func (Bookmarks) Deserialize(text string) (any, error) {
	var data map[string]any
	err := json.Unmarshal([]byte(text), &data)
	return data, err
}

func (Bookmarks) SemanticallyEqual(a, b any) bool {
	au, af := flattenBookmarks(asMap(a))
	bu, bf := flattenBookmarks(asMap(b))
	if len(au) != len(bu) || len(af) != len(bf) {
		return false
	}
	for k := range af {
		if !bf[k] {
			return false
		}
	}
	for k, av := range au {
		bv, ok := bu[k]
		if !ok || str(av["name"]) != str(bv["name"]) || str(av["url"]) != str(bv["url"]) {
			return false
		}
	}
	return true
}

func (Bookmarks) Summary(data any) string {
	return fmt.Sprintf("%d URLs", len(WalkURLs(asMap(data))))
}
