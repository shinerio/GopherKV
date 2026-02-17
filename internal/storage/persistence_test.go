package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAOFReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	cm := NewConcurrentMap(16)
	p := NewAOFPersister(path, 1024*1024, cm)
	if err := p.OpenForAppend(); err != nil {
		t.Fatal(err)
	}
	if err := p.AppendSet("k1", []byte("v1"), 0); err != nil {
		t.Fatal(err)
	}
	if err := p.AppendSet("k2", []byte("v2"), time.Now().Add(5*time.Second).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := p.AppendDel("k1"); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	recovered := NewConcurrentMap(16)
	p2 := NewAOFPersister(path, 1024*1024, recovered)
	n, err := p2.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("expected 3 replayed commands, got %d", n)
	}
	if recovered.Exists("k1") {
		t.Fatal("k1 should be deleted after replay")
	}
	v, _, ok := recovered.Get("k2")
	if !ok || string(v) != "v2" {
		t.Fatal("k2 should be restored")
	}
}

func TestAOFTruncateOnCorruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")
	content := "SET\tk1\tdjE=\t0\nBROKEN\tline\nSET\tk2\tdjI=\t0\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cm := NewConcurrentMap(16)
	p := NewAOFPersister(path, 1024*1024, cm)
	if _, err := p.Replay(); err != nil {
		t.Fatal(err)
	}

	if !cm.Exists("k1") {
		t.Fatal("k1 should be recovered")
	}
	if cm.Exists("k2") {
		t.Fatal("k2 should not be replayed after corruption")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "BROKEN") {
		t.Fatal("corrupted part should be truncated")
	}
}

func TestRDBSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.rdb")

	orig := NewConcurrentMap(16)
	orig.Set("k1", []byte("v1"), 0)
	orig.Set("k2", []byte("v2"), time.Now().Add(2*time.Second).UnixMilli())

	rdb := NewRDBManager(path)
	if _, err := rdb.Save(orig); err != nil {
		t.Fatal(err)
	}

	restored := NewConcurrentMap(16)
	n, err := rdb.Load(restored)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 entries loaded, got %d", n)
	}
	v, _, ok := restored.Get("k1")
	if !ok || string(v) != "v1" {
		t.Fatal("k1 should be restored from rdb")
	}
}
