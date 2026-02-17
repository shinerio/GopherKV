package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shinerio/gopher-kv/internal/config"
)

func TestServiceAutoSnapshotWithoutFurtherWrites(t *testing.T) {
	dir := t.TempDir()
	aofPath := filepath.Join(dir, "appendonly.aof")
	rdbPath := filepath.Join(dir, "dump.rdb")

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:            6380,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		Storage: config.StorageConfig{
			ShardCount:   16,
			MaxKeySize:   256,
			MaxValueSize: 1024 * 1024,
			MaxMemory:    256 * 1024 * 1024,
		},
		AOF: config.AOFConfig{
			Enabled:          false,
			FilePath:         aofPath,
			RewriteThreshold: 1024 * 1024,
		},
		RDB: config.RDBConfig{
			Enabled:  true,
			FilePath: rdbPath,
			SaveRules: []config.SaveRule{
				{Seconds: 1, Changes: 1},
			},
		},
		Log: config.LogConfig{Level: "error"},
	}

	svc := NewService(cfg)
	svc.Start()
	defer svc.Stop()

	if err := svc.Set("k1", []byte("v1"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(rdbPath); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("expected auto snapshot file %s to be created while service is running", rdbPath)
}
