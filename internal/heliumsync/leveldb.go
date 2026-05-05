package heliumsync

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type LevelDBEntry struct {
	Key    string `json:"key"`
	ValHex string `json:"val_hex"`
}

func readLevelDB(path string) ([]LevelDBEntry, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("no LevelDB at %s", path)
	}
	tmp, err := os.MkdirTemp("", "helium-sync-read-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := copyDir(path, tmp, true); err != nil {
		return nil, err
	}
	db, err := leveldb.OpenFile(tmp, &opt.Options{ErrorIfMissing: true})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "lock") {
			return nil, errors.New("LevelDB is locked; close Helium before syncing saved tab groups")
		}
		return nil, err
	}
	defer db.Close()

	var out []LevelDBEntry
	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		out = append(out, LevelDBEntry{
			Key:    string(iter.Key()),
			ValHex: hex.EncodeToString(iter.Value()),
		})
	}
	iter.Release()
	return out, iter.Error()
}
