package storage

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AOF struct {
	path             string
	rewriteThreshold int64
	logger           *slog.Logger

	mu        sync.Mutex
	file      *os.File
	rewriting bool
	incBuf    bytes.Buffer
}

func NewAOF(path string, rewriteThreshold int64, logger *slog.Logger) *AOF {
	return &AOF{path: path, rewriteThreshold: rewriteThreshold, logger: logger}
}

func (a *AOF) OpenAndReplay(restore func([]PersistRecord) error) error {
	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	records, cutOffset, parseErr := parseAOF(f)
	if parseErr != nil {
		a.logger.Warn("aof parse error, truncating", "error", parseErr)
		if err := f.Truncate(cutOffset); err != nil {
			_ = f.Close()
			return err
		}
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		_ = f.Close()
		return err
	}
	if err := restore(records); err != nil {
		_ = f.Close()
		return err
	}
	a.file = f
	return nil
}

func (a *AOF) AppendSet(key string, value []byte, expiresAt int64) error {
	line := fmt.Sprintf("SET\t%s\t%s\t%d\n", key, base64.StdEncoding.EncodeToString(value), expiresAt)
	return a.append([]byte(line))
}

func (a *AOF) AppendDel(key string) error {
	line := fmt.Sprintf("DEL\t%s\n", key)
	return a.append([]byte(line))
}

func (a *AOF) append(b []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file == nil {
		return nil
	}
	if _, err := a.file.Write(b); err != nil {
		return err
	}
	if a.rewriting {
		_, _ = a.incBuf.Write(b)
	}
	if a.rewriteThreshold > 0 {
		if st, err := a.file.Stat(); err == nil && st.Size() >= a.rewriteThreshold && !a.rewriting {
			a.logger.Info("aof threshold reached, rewrite should be triggered")
		}
	}
	return nil
}

func (a *AOF) NeedsRewrite() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file == nil || a.rewriteThreshold <= 0 || a.rewriting {
		return false
	}
	st, err := a.file.Stat()
	if err != nil {
		return false
	}
	return st.Size() >= a.rewriteThreshold
}

func (a *AOF) Rewrite(snapshot []PersistRecord) error {
	a.mu.Lock()
	if a.file == nil || a.rewriting {
		a.mu.Unlock()
		return nil
	}
	a.rewriting = true
	a.incBuf.Reset()
	a.mu.Unlock()

	tmpPath := a.path + ".rewrite.tmp"
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		a.finishRewrite()
		return err
	}
	for _, r := range snapshot {
		line := fmt.Sprintf("SET\t%s\t%s\t%d\n", r.Key, base64.StdEncoding.EncodeToString(r.Value), r.ExpiresAt)
		if _, err := tmp.WriteString(line); err != nil {
			_ = tmp.Close()
			a.finishRewrite()
			return err
		}
	}

	a.mu.Lock()
	if _, err := tmp.Write(a.incBuf.Bytes()); err != nil {
		a.mu.Unlock()
		_ = tmp.Close()
		a.finishRewrite()
		return err
	}
	a.mu.Unlock()

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		a.finishRewrite()
		return err
	}
	if err := tmp.Close(); err != nil {
		a.finishRewrite()
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if err := os.Rename(tmpPath, a.path); err != nil {
		a.rewriting = false
		return err
	}
	_ = a.file.Close()
	a.file, err = os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	a.rewriting = false
	return err
}

func (a *AOF) finishRewrite() {
	a.mu.Lock()
	a.rewriting = false
	a.mu.Unlock()
}

func (a *AOF) Sync() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file == nil {
		return nil
	}
	return a.file.Sync()
}

func (a *AOF) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file == nil {
		return nil
	}
	err := a.file.Close()
	a.file = nil
	return err
}

func parseAOF(f *os.File) ([]PersistRecord, int64, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, 0, err
	}
	r := bufio.NewReader(f)
	kv := make(map[string]PersistRecord)
	var offset int64
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, offset, err
		}
		lineBytes := []byte(line)
		line = strings.TrimSpace(line)
		if line == "" {
			offset += int64(len(lineBytes))
			continue
		}
		parts := strings.Split(line, "\t")
		switch parts[0] {
		case "SET":
			if len(parts) != 4 {
				return nil, offset, fmt.Errorf("invalid set line")
			}
			val, err := base64.StdEncoding.DecodeString(parts[2])
			if err != nil {
				return nil, offset, err
			}
			expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
			if err != nil {
				return nil, offset, err
			}
			kv[parts[1]] = PersistRecord{Key: parts[1], Value: val, ExpiresAt: expiresAt}
		case "DEL":
			if len(parts) != 2 {
				return nil, offset, fmt.Errorf("invalid del line")
			}
			delete(kv, parts[1])
		default:
			return nil, offset, fmt.Errorf("invalid cmd: %s", parts[0])
		}
		offset += int64(len(lineBytes))
	}
	records := make([]PersistRecord, 0, len(kv))
	for _, v := range kv {
		records = append(records, v)
	}
	return records, offset, nil
}

type RDB struct {
	path   string
	logger *slog.Logger
}

func NewRDB(path string, logger *slog.Logger) *RDB {
	return &RDB{path: path, logger: logger}
}

func (r *RDB) Save(records []PersistRecord) (string, error) {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("dump-%d.rdb", time.Now().Unix())
	fullPath := filepath.Join(filepath.Dir(r.path), name)
	tmpPath := fullPath + ".tmp"

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	enc := gob.NewEncoder(f)
	if err := enc.Encode(records); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, fullPath); err != nil {
		return "", err
	}
	return fullPath, nil
}

func (r *RDB) LoadLatest() ([]PersistRecord, string, error) {
	dir := filepath.Dir(r.path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", err
	}
	var candidates []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "dump-") && strings.HasSuffix(name, ".rdb") {
			candidates = append(candidates, filepath.Join(dir, name))
		}
	}
	if len(candidates) == 0 {
		if _, err := os.Stat(r.path); err == nil {
			candidates = append(candidates, r.path)
		}
	}
	if len(candidates) == 0 {
		return nil, "", nil
	}
	sort.Strings(candidates)
	latest := candidates[len(candidates)-1]
	records, err := r.loadFile(latest)
	if err != nil {
		r.logger.Warn("rdb load failed", "path", latest, "error", err)
		return nil, latest, err
	}
	return records, latest, nil
}

func (r *RDB) loadFile(path string) ([]PersistRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var records []PersistRecord
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&records); err != nil {
		return nil, err
	}
	return records, nil
}
