package storage

import (
	"testing"
	"time"
)

func TestSetGetDelete(t *testing.T) {
	e := NewEngine(Options{ShardCount: 16, MaxKeySize: 256, MaxValueSize: 1024, MaxMemory: 1024 * 1024})
	defer e.Close()

	if err := e.Set("k1", []byte("v1"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	v, ok := e.Get("k1")
	if !ok || string(v) != "v1" {
		t.Fatalf("unexpected get result: ok=%v v=%s", ok, string(v))
	}
	if err := e.Delete("k1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, ok := e.Get("k1"); ok {
		t.Fatalf("key should be deleted")
	}
}

func TestTTLExpiration(t *testing.T) {
	e := NewEngine(Options{ShardCount: 16, MaxKeySize: 256, MaxValueSize: 1024, MaxMemory: 1024 * 1024})
	defer e.Close()

	if err := e.Set("k2", []byte("v2"), time.Second); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, ok := e.Get("k2"); ok {
		t.Fatalf("key should be expired")
	}
}

func TestMemoryLimit(t *testing.T) {
	e := NewEngine(Options{ShardCount: 16, MaxKeySize: 256, MaxValueSize: 1024, MaxMemory: 64})
	defer e.Close()

	if err := e.Set("k3", []byte("1234567890123456789012345678901234567890"), 0); err == nil {
		t.Fatalf("expected memory limit error")
	}
}
