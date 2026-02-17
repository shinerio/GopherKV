package storage

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type RDBManager struct {
	path string
}

type rdbEntry struct {
	Key       string
	Value     []byte
	ExpiresAt int64
}

func NewRDBManager(path string) *RDBManager {
	return &RDBManager{path: path}
}

func (r *RDBManager) Save(storage *ConcurrentMap) (string, error) {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return "", err
	}

	tmpPath := r.path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}

	enc := gob.NewEncoder(f)
	entries := make([]rdbEntry, 0, storage.Keys())
	now := time.Now().UnixMilli()
	storage.Iterate(func(key string, entry Entry) bool {
		if entry.ExpiresAt > 0 && entry.ExpiresAt <= now {
			return true
		}
		val := make([]byte, len(entry.Value))
		copy(val, entry.Value)
		entries = append(entries, rdbEntry{
			Key:       key,
			Value:     val,
			ExpiresAt: entry.ExpiresAt,
		})
		return true
	})

	if err := enc.Encode(entries); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	_ = os.Remove(r.path)
	if err := os.Rename(tmpPath, r.path); err != nil {
		return "", err
	}
	return r.path, nil
}

func (r *RDBManager) Load(storage *ConcurrentMap) (int, error) {
	if _, err := os.Stat(r.path); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	f, err := os.Open(r.path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var entries []rdbEntry
	if err := gob.NewDecoder(f).Decode(&entries); err != nil {
		return 0, err
	}

	now := time.Now().UnixMilli()
	loaded := 0
	for _, e := range entries {
		if e.ExpiresAt > 0 && e.ExpiresAt <= now {
			continue
		}
		storage.Set(e.Key, e.Value, e.ExpiresAt)
		loaded++
	}
	return loaded, nil
}
