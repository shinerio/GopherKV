package storage

import (
	"testing"
	"time"
)

func TestConcurrentMap_SetGet(t *testing.T) {
	cm := NewConcurrentMap(16)

	cm.Set("key1", []byte("value1"), 0)
	value, _, exists := cm.Get("key1")
	if !exists {
		t.Fatal("key1 should exist")
	}
	if string(value) != "value1" {
		t.Errorf("expected value1, got %s", string(value))
	}
}

func TestConcurrentMap_Delete(t *testing.T) {
	cm := NewConcurrentMap(16)

	cm.Set("key1", []byte("value1"), 0)
	cm.Delete("key1")
	_, _, exists := cm.Get("key1")
	if exists {
		t.Fatal("key1 should not exist after delete")
	}
}

func TestConcurrentMap_Exists(t *testing.T) {
	cm := NewConcurrentMap(16)

	cm.Set("key1", []byte("value1"), 0)
	if !cm.Exists("key1") {
		t.Error("key1 should exist")
	}

	cm.Delete("key1")
	if cm.Exists("key1") {
		t.Error("key1 should not exist after delete")
	}
}

func TestConcurrentMap_Expiration(t *testing.T) {
	cm := NewConcurrentMap(16)

	expiresAt := time.Now().Add(100 * time.Millisecond).Unix()
	cm.Set("expired", []byte("test"), expiresAt)

	time.Sleep(200 * time.Millisecond)
	_, _, exists := cm.Get("expired")
	if exists {
		t.Error("expired key should not exist")
	}
}

func TestConcurrentMap_ConcurrentAccess(t *testing.T) {
	cm := NewConcurrentMap(256)
	done := make(chan bool)

	for i := 0; i < 100; i++ {
		go func(idx int) {
			key := string(rune('a' + idx%26))
			for j := 0; j < 100; j++ {
				cm.Set(key, []byte("value"), 0)
				cm.Get(key)
				cm.Delete(key)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}
