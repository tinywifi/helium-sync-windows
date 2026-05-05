package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

func createTestDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close test db: %v", err)
	}
	return dir
}

func writeOpsFile(t *testing.T, ops []Op) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ops.json")
	raw, err := json.Marshal(ops)
	if err != nil {
		t.Fatalf("marshal ops: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write ops: %v", err)
	}
	return path
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}

func readEntries(t *testing.T, dbDir string) map[string]string {
	t.Helper()
	out := captureStdout(t, func() {
		cmdRead([]string{"-db", dbDir})
	})
	var entries []Entry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("unmarshal read output %q: %v", out, err)
	}
	got := map[string]string{}
	for _, entry := range entries {
		got[entry.Key] = entry.ValHex
	}
	return got
}

func TestWritePutAndReadReturnsValues(t *testing.T) {
	dbDir := createTestDB(t)
	opsPath := writeOpsFile(t, []Op{
		{Op: "put", Key: "alpha", ValHex: "68656c6c6f"},
		{Op: "put", Key: "beta", ValHex: "00ff"},
	})

	out := captureStdout(t, func() {
		cmdWrite([]string{"-db", dbDir, "-ops", opsPath})
	})
	if !strings.Contains(out, "ok: 2 puts, 0 deletes") {
		t.Fatalf("unexpected write output: %q", out)
	}

	got := readEntries(t, dbDir)
	if got["alpha"] != "68656c6c6f" {
		t.Fatalf("alpha = %q", got["alpha"])
	}
	if got["beta"] != "00ff" {
		t.Fatalf("beta = %q", got["beta"])
	}
}

func TestWriteDeleteRemovesKey(t *testing.T) {
	dbDir := createTestDB(t)
	cmdWrite([]string{"-db", dbDir, "-ops", writeOpsFile(t, []Op{
		{Op: "put", Key: "keep", ValHex: "01"},
		{Op: "put", Key: "drop", ValHex: "02"},
	})})
	cmdWrite([]string{"-db", dbDir, "-ops", writeOpsFile(t, []Op{
		{Op: "delete", Key: "drop"},
	})})

	got := readEntries(t, dbDir)
	if got["keep"] != "01" {
		t.Fatalf("keep = %q", got["keep"])
	}
	if _, ok := got["drop"]; ok {
		t.Fatalf("drop key still present: %#v", got)
	}
}

func TestReadIgnoresLockDuringTempCopy(t *testing.T) {
	dbDir := createTestDB(t)
	cmdWrite([]string{"-db", dbDir, "-ops", writeOpsFile(t, []Op{
		{Op: "put", Key: "locked", ValHex: "6f6b"},
	})})
	if err := os.WriteFile(filepath.Join(dbDir, "LOCK"), []byte("held"), 0o600); err != nil {
		t.Fatalf("write LOCK: %v", err)
	}

	got := readEntries(t, dbDir)
	if got["locked"] != "6f6b" {
		t.Fatalf("locked = %q", got["locked"])
	}
}

func TestCopyLevelDBSkipsLockFile(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "000001.log"), []byte("data"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "LOCK"), []byte("held"), 0o600); err != nil {
		t.Fatalf("write LOCK: %v", err)
	}

	if err := copyLevelDB(src, dst); err != nil {
		t.Fatalf("copyLevelDB: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "000001.log")); err != nil {
		t.Fatalf("copied log missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "LOCK")); !os.IsNotExist(err) {
		t.Fatalf("LOCK should not be copied, stat err = %v", err)
	}
}
