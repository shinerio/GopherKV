package storage

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AOFPersister struct {
	path             string
	rewriteThreshold int64
	storage          *ConcurrentMap

	mu        sync.Mutex
	file      *os.File
	rewriting bool
	incrBuf   [][]byte
}

func NewAOFPersister(path string, rewriteThreshold int64, storage *ConcurrentMap) *AOFPersister {
	return &AOFPersister{
		path:             path,
		rewriteThreshold: rewriteThreshold,
		storage:          storage,
	}
}

func (p *AOFPersister) OpenForAppend() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.openForAppendLocked()
}

func (p *AOFPersister) openForAppendLocked() error {
	if p.file != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	p.file = f
	return nil
}

func (p *AOFPersister) AppendSet(key string, value []byte, expiresAt int64) error {
	line := fmt.Sprintf("SET\t%s\t%s\t%d\n", key, base64.StdEncoding.EncodeToString(value), expiresAt)
	return p.appendLine([]byte(line))
}

func (p *AOFPersister) AppendDel(key string) error {
	line := fmt.Sprintf("DEL\t%s\n", key)
	return p.appendLine([]byte(line))
}

func (p *AOFPersister) appendLine(line []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.openForAppendLocked(); err != nil {
		return err
	}

	if _, err := p.file.Write(line); err != nil {
		return err
	}

	if p.rewriting {
		dup := make([]byte, len(line))
		copy(dup, line)
		p.incrBuf = append(p.incrBuf, dup)
	}

	p.maybeTriggerRewriteLocked()
	return nil
}

func (p *AOFPersister) maybeTriggerRewriteLocked() {
	if p.rewriteThreshold <= 0 || p.rewriting || p.file == nil {
		return
	}
	st, err := p.file.Stat()
	if err != nil || st.Size() < p.rewriteThreshold {
		return
	}
	p.rewriting = true
	p.incrBuf = p.incrBuf[:0]
	go p.rewrite()
}

func (p *AOFPersister) rewrite() {
	tmpPath := p.path + ".rewrite.tmp"
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		p.finishRewriteWithError(err)
		return
	}

	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		p.finishRewriteWithError(err)
		return
	}

	now := time.Now().UnixMilli()
	p.storage.Iterate(func(key string, entry Entry) bool {
		if entry.ExpiresAt > 0 && entry.ExpiresAt <= now {
			return true
		}
		line := fmt.Sprintf("SET\t%s\t%s\t%d\n", key, base64.StdEncoding.EncodeToString(entry.Value), entry.ExpiresAt)
		_, err = tmp.WriteString(line)
		return err == nil
	})
	if err != nil {
		tmp.Close()
		p.finishRewriteWithError(err)
		return
	}

	p.mu.Lock()
	buf := make([][]byte, len(p.incrBuf))
	copy(buf, p.incrBuf)
	p.mu.Unlock()

	for _, line := range buf {
		if _, err := tmp.Write(line); err != nil {
			tmp.Close()
			p.finishRewriteWithError(err)
			return
		}
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		p.finishRewriteWithError(err)
		return
	}
	if err := tmp.Close(); err != nil {
		p.finishRewriteWithError(err)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.file != nil {
		_ = p.file.Close()
		p.file = nil
	}
	_ = os.Remove(p.path)
	if err := os.Rename(tmpPath, p.path); err != nil {
		p.rewriting = false
		return
	}
	_ = p.openForAppendLocked()
	p.rewriting = false
	p.incrBuf = nil
}

func (p *AOFPersister) finishRewriteWithError(_ error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rewriting = false
	p.incrBuf = nil
}

func (p *AOFPersister) Replay() (int, error) {
	if _, err := os.Stat(p.path); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}

	f, err := os.OpenFile(p.path, os.O_RDWR, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	var (
		offset         int64
		lastGoodOffset int64
		loaded         int
	)

	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return loaded, err
		}
		if line != "" {
			offset += int64(len(line))
			if parseErr := p.applyLine(strings.TrimRight(line, "\r\n")); parseErr != nil {
				if trErr := f.Truncate(lastGoodOffset); trErr != nil {
					return loaded, trErr
				}
				break
			}
			lastGoodOffset = offset
			loaded++
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return loaded, nil
}

func (p *AOFPersister) applyLine(line string) error {
	if line == "" {
		return nil
	}
	parts := strings.Split(line, "\t")
	if len(parts) < 2 {
		return fmt.Errorf("invalid aof line")
	}
	switch parts[0] {
	case "SET":
		if len(parts) != 4 {
			return fmt.Errorf("invalid set line")
		}
		value, err := base64.StdEncoding.DecodeString(parts[2])
		if err != nil {
			return err
		}
		expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return err
		}
		if expiresAt > 0 && expiresAt <= time.Now().UnixMilli() {
			return nil
		}
		p.storage.Set(parts[1], value, expiresAt)
	case "DEL":
		if len(parts) != 2 {
			return fmt.Errorf("invalid del line")
		}
		p.storage.Delete(parts[1])
	default:
		return fmt.Errorf("invalid aof op")
	}
	return nil
}

func (p *AOFPersister) Sync() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.file == nil {
		return nil
	}
	return p.file.Sync()
}

func (p *AOFPersister) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.file == nil {
		return nil
	}
	err := p.file.Close()
	p.file = nil
	return err
}
