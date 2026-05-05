// leveldb-writer — read from and write to a LevelDB directory.
//
// Commands:
//
//	read  -db PATH              iterate all entries, output JSON array to stdout
//	write -db PATH -ops FILE    apply a batch of put/delete operations
//
// read copies the LevelDB directory to a temp location (excluding the LOCK
// file) before opening, so it works even while another process (e.g. Helium)
// holds the lock on the original directory.
//
// read outputs:
//
//	[{"key":"...", "val_hex":"deadbeef"}, ...]
//
// write reads a JSON ops array from FILE (or stdin):
//
//	{"op":"put",    "key":"string", "val_hex":"deadbeef"}
//	{"op":"delete", "key":"string"}
//
// All write ops land in a single atomic batch via leveldb.DB.Write.
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "leveldb-writer: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		die("usage: leveldb-writer <read|write> [flags]")
	}
	switch os.Args[1] {
	case "read":
		cmdRead(os.Args[2:])
	case "write":
		cmdWrite(os.Args[2:])
	default:
		die("unknown command %q (want read or write)", os.Args[1])
	}
}

// ── read ─────────────────────────────────────────────────────────────────────

type Entry struct {
	Key    string `json:"key"`
	ValHex string `json:"val_hex"`
}

func cmdRead(args []string) {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	dbPath := fs.String("db", "", "path to the LevelDB directory (required)")
	fs.Parse(args)
	if *dbPath == "" {
		die("read: -db PATH is required")
	}

	// Copy to a temp dir (excluding LOCK) so we can open it without
	// contending for the lock that Helium holds while running.
	tmpDir, err := os.MkdirTemp("", "helium-sync-read-*")
	if err != nil {
		die("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := copyLevelDB(*dbPath, tmpDir); err != nil {
		die("copying LevelDB: %v", err)
	}

	db, err := leveldb.OpenFile(tmpDir, &opt.Options{ErrorIfMissing: true})
	if err != nil {
		die("opening %q: %v", *dbPath, err)
	}
	defer db.Close()

	entries := []Entry{}
	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		entries = append(entries, Entry{
			Key:    string(iter.Key()),
			ValHex: hex.EncodeToString(iter.Value()),
		})
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		die("iterating: %v", err)
	}

	out, err := json.Marshal(entries)
	if err != nil {
		die("encoding JSON: %v", err)
	}
	os.Stdout.Write(out)
}

// copyLevelDB copies src to dst, skipping the LOCK file.
func copyLevelDB(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "LOCK" {
			continue
		}
		if err := copyFile(
			filepath.Join(src, e.Name()),
			filepath.Join(dst, e.Name()),
		); err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// ── write ────────────────────────────────────────────────────────────────────

type Op struct {
	Op     string `json:"op"`
	Key    string `json:"key"`
	ValHex string `json:"val_hex"`
}

func cmdWrite(args []string) {
	fs := flag.NewFlagSet("write", flag.ExitOnError)
	dbPath := fs.String("db", "", "path to the LevelDB directory (required)")
	opsFile := fs.String("ops", "", "path to JSON ops file (default: stdin)")
	fs.Parse(args)
	if *dbPath == "" {
		die("write: -db PATH is required")
	}

	var raw []byte
	var err error
	if *opsFile == "" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(*opsFile)
	}
	if err != nil {
		die("reading ops: %v", err)
	}

	var ops []Op
	if err := json.Unmarshal(raw, &ops); err != nil {
		die("parsing ops JSON: %v", err)
	}

	// ErrorIfMissing: never accidentally create a blank DB at a wrong path.
	db, err := leveldb.OpenFile(*dbPath, &opt.Options{ErrorIfMissing: true})
	if err != nil {
		die("opening %q: %v", *dbPath, err)
	}
	defer db.Close()

	batch := new(leveldb.Batch)
	puts, dels := 0, 0
	for i, op := range ops {
		switch op.Op {
		case "put":
			val, err := hex.DecodeString(op.ValHex)
			if err != nil {
				die("op[%d] bad hex: %v", i, err)
			}
			batch.Put([]byte(op.Key), val)
			puts++
		case "delete":
			batch.Delete([]byte(op.Key))
			dels++
		default:
			die("op[%d] unknown op %q (want put or delete)", i, op.Op)
		}
	}

	if err := db.Write(batch, &opt.WriteOptions{Sync: true}); err != nil {
		die("writing batch: %v", err)
	}
	fmt.Printf("ok: %d puts, %d deletes\n", puts, dels)
}
