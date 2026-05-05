package heliumsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const keyPrefix = "saved_tab_group-dt-"

type SavedTabGroups struct{}

func (SavedTabGroups) Name() string          { return "saved_tab_groups" }
func (SavedTabGroups) StateFilename() string { return "saved_tab_groups.json" }

func (SavedTabGroups) levelDBDir(profile string) string {
	return filepath.Join(profile, "Default", "Sync Data", "LevelDB")
}

func (s SavedTabGroups) Extract(profile string) (any, error) {
	entries, err := readLevelDB(s.levelDBDir(profile))
	if err != nil {
		return nil, err
	}
	state := map[string]any{"groups": map[string]any{}, "tabs": map[string]any{}}
	groups := state["groups"].(map[string]any)
	tabs := state["tabs"].(map[string]any)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Key, keyPrefix) {
			continue
		}
		entity, err := decodeLevelDBEntity(entry.ValHex)
		if err != nil {
			return nil, err
		}
		if entity.Kind == "group" {
			groups[entity.GUID] = map[string]any{
				"guid": entity.GUID, "creation_time": entity.CreationTime, "update_time": entity.UpdateTime,
				"version": entity.Version, "title": entity.Title, "color": entity.Color, "position": entity.Position,
			}
		} else if entity.Kind == "tab" {
			tabs[entity.GUID] = map[string]any{
				"guid": entity.GUID, "creation_time": entity.CreationTime, "update_time": entity.UpdateTime,
				"version": entity.Version, "group_guid": entity.GroupGUID, "url": entity.URL,
				"title": entity.Title, "position": entity.Position,
			}
		}
	}
	return state, nil
}

func (s SavedTabGroups) Apply(profile string, data any, backupDir string) error {
	ldb := s.levelDBDir(profile)
	if _, err := os.Stat(ldb); err != nil {
		return fmt.Errorf("no LevelDB at %s", ldb)
	}
	backup := filepath.Join(backupDir, "LevelDB")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}
	_ = os.RemoveAll(backup)
	if err := copyDir(ldb, backup, false); err != nil {
		return err
	}
	existingAny, err := s.Extract(profile)
	if err != nil {
		return err
	}
	existing := asMap(existingAny)
	target := asMap(data)
	existingKeys := stgKeys(existing)
	targetKeys := stgKeys(target)

	db, err := leveldb.OpenFile(ldb, &opt.Options{ErrorIfMissing: true})
	if err != nil {
		return err
	}
	defer db.Close()
	batch := new(leveldb.Batch)
	for guid, raw := range mapValue(target, "groups") {
		batch.Put([]byte(keyPrefix+guid), encodeWrapperSpecifics(encodeGroup(asMap(raw))))
	}
	for guid, raw := range mapValue(target, "tabs") {
		batch.Put([]byte(keyPrefix+guid), encodeWrapperSpecifics(encodeTab(asMap(raw))))
	}
	for key := range existingKeys {
		if !targetKeys[key] {
			batch.Delete([]byte(key))
		}
	}
	return db.Write(batch, &opt.WriteOptions{Sync: true})
}

func (SavedTabGroups) Serialize(data any) (string, error) {
	ordered := map[string]any{
		"groups": sortedMap(mapValue(asMap(data), "groups")),
		"tabs":   sortedMap(mapValue(asMap(data), "tabs")),
	}
	out, err := json.MarshalIndent(ordered, "", "  ")
	return string(out), err
}

func (SavedTabGroups) Deserialize(text string) (any, error) {
	var data map[string]any
	err := json.Unmarshal([]byte(text), &data)
	return data, err
}

func (SavedTabGroups) SemanticallyEqual(a, b any) bool {
	ag, bg := mapValue(asMap(a), "groups"), mapValue(asMap(b), "groups")
	at, bt := mapValue(asMap(a), "tabs"), mapValue(asMap(b), "tabs")
	if len(ag) != len(bg) || len(at) != len(bt) {
		return false
	}
	for k, av := range ag {
		bv, ok := bg[k]
		if !ok {
			return false
		}
		am, bm := asMap(av), asMap(bv)
		if str(am["title"]) != str(bm["title"]) ||
			int64Value(am["color"]) != int64Value(bm["color"]) ||
			int64Value(am["position"]) != int64Value(bm["position"]) {
			return false
		}
	}
	for k, av := range at {
		bv, ok := bt[k]
		if !ok {
			return false
		}
		am, bm := asMap(av), asMap(bv)
		if str(am["group_guid"]) != str(bm["group_guid"]) ||
			str(am["url"]) != str(bm["url"]) ||
			str(am["title"]) != str(bm["title"]) ||
			int64Value(am["position"]) != int64Value(bm["position"]) {
			return false
		}
	}
	return true
}

func (SavedTabGroups) Summary(data any) string {
	m := asMap(data)
	return fmt.Sprintf("%d groups, %d tabs", len(mapValue(m, "groups")), len(mapValue(m, "tabs")))
}

func stgKeys(m map[string]any) map[string]bool {
	out := map[string]bool{}
	for guid := range mapValue(m, "groups") {
		out[keyPrefix+guid] = true
	}
	for guid := range mapValue(m, "tabs") {
		out[keyPrefix+guid] = true
	}
	return out
}
