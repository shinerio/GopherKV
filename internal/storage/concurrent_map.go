package storage

import (
	"container/heap"
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/shinerio/gopher-kv/pkg/protocol"
	"github.com/shinerio/gopher-kv/pkg/utils"
)

type Storage interface {
	Set(key string, value []byte, ttl time.Duration) error
	Get(key string) ([]byte, bool)
	Delete(key string) error
	Exists(key string) bool
	TTL(key string) (time.Duration, bool)
	Keys() int
	MemUsage() int64
	Close() error
}

type Entry struct {
	Value     []byte
	ExpiresAt int64
}

type PersistRecord struct {
	Key       string `json:"key"`
	Value     []byte `json:"value"`
	ExpiresAt int64  `json:"expires_at"`
}

type shard struct {
	mu sync.RWMutex
	m  map[string]Entry
}

type Options struct {
	ShardCount   int
	MaxKeySize   int
	MaxValueSize int
	MaxMemory    int64
}

type Engine struct {
	shards       []shard
	shardMask    uint64
	shardCount   int
	maxKeySize   int
	maxValueSize int
	maxMemory    int64
	memUsage     atomic.Int64

	ttlq      ttlHeap
	ttlqMu    sync.Mutex
	stopCh    chan struct{}
	stopped   chan struct{}
	closed    atomic.Bool
	closeOnce sync.Once
}

type ttlItem struct {
	key       string
	expiresAt int64
}

type ttlHeap []ttlItem

func (h ttlHeap) Len() int            { return len(h) }
func (h ttlHeap) Less(i, j int) bool  { return h[i].expiresAt < h[j].expiresAt }
func (h ttlHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *ttlHeap) Push(x interface{}) { *h = append(*h, x.(ttlItem)) }
func (h *ttlHeap) Pop() interface{} {
	old := *h
	n := len(old)
	it := old[n-1]
	*h = old[:n-1]
	return it
}

func NewEngine(opt Options) *Engine {
	if opt.ShardCount <= 0 {
		opt.ShardCount = 256
	}
	e := &Engine{
		shards:       make([]shard, opt.ShardCount),
		shardCount:   opt.ShardCount,
		maxKeySize:   opt.MaxKeySize,
		maxValueSize: opt.MaxValueSize,
		maxMemory:    opt.MaxMemory,
		stopCh:       make(chan struct{}),
		stopped:      make(chan struct{}),
	}
	for i := range e.shards {
		e.shards[i].m = make(map[string]Entry)
	}
	heap.Init(&e.ttlq)
	if opt.ShardCount&(opt.ShardCount-1) == 0 {
		e.shardMask = uint64(opt.ShardCount - 1)
	}
	go e.runTTLWorker()
	return e
}

func (e *Engine) Set(key string, value []byte, ttl time.Duration) error {
	expiresAt := int64(0)
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).Unix()
	}
	return e.SetWithExpiresAt(key, value, expiresAt)
}

func (e *Engine) SetWithExpiresAt(key string, value []byte, expiresAt int64) error {
	if e.closed.Load() {
		return errors.New("storage closed")
	}
	if err := e.validate(key, value); err != nil {
		return err
	}
	idx := e.shardIndex(key)
	s := &e.shards[idx]

	s.mu.Lock()
	defer s.mu.Unlock()

	old, ok := s.m[key]
	newSize := estimateEntrySize(key, value)
	delta := newSize
	if ok {
		delta -= estimateEntrySize(key, old.Value)
	}
	if delta > 0 && e.memUsage.Load()+delta > e.maxMemory {
		return protocol.NewError(protocol.CodeMemoryFull, "memory limit reached")
	}
	buf := make([]byte, len(value))
	copy(buf, value)
	s.m[key] = Entry{Value: buf, ExpiresAt: expiresAt}
	e.memUsage.Add(delta)

	if expiresAt > 0 {
		e.ttlqMu.Lock()
		heap.Push(&e.ttlq, ttlItem{key: key, expiresAt: expiresAt})
		e.ttlqMu.Unlock()
	}
	return nil
}

func (e *Engine) Get(key string) ([]byte, bool) {
	idx := e.shardIndex(key)
	s := &e.shards[idx]

	s.mu.RLock()
	ent, ok := s.m[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if isExpired(ent.ExpiresAt) {
		_ = e.deleteIfExpired(key, ent.ExpiresAt)
		return nil, false
	}
	buf := make([]byte, len(ent.Value))
	copy(buf, ent.Value)
	return buf, true
}

func (e *Engine) Delete(key string) error {
	idx := e.shardIndex(key)
	s := &e.shards[idx]
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.m[key]
	if !ok {
		return nil
	}
	delete(s.m, key)
	e.memUsage.Add(-estimateEntrySize(key, old.Value))
	return nil
}

func (e *Engine) Exists(key string) bool {
	_, ok := e.Get(key)
	return ok
}

func (e *Engine) TTL(key string) (time.Duration, bool) {
	idx := e.shardIndex(key)
	s := &e.shards[idx]
	s.mu.RLock()
	ent, ok := s.m[key]
	s.mu.RUnlock()
	if !ok {
		return 0, false
	}
	if isExpired(ent.ExpiresAt) {
		_ = e.deleteIfExpired(key, ent.ExpiresAt)
		return 0, false
	}
	if ent.ExpiresAt == 0 {
		return -1, true
	}
	remaining := time.Until(time.Unix(ent.ExpiresAt, 0)).Round(time.Second)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, true
}

func (e *Engine) Keys() int {
	total := 0
	for i := range e.shards {
		s := &e.shards[i]
		s.mu.RLock()
		total += len(s.m)
		s.mu.RUnlock()
	}
	return total
}

func (e *Engine) MemUsage() int64 {
	return e.memUsage.Load()
}

func (e *Engine) Close() error {
	e.closeOnce.Do(func() {
		e.closed.Store(true)
		close(e.stopCh)
		<-e.stopped
	})
	return nil
}

func (e *Engine) Snapshot() []PersistRecord {
	records := make([]PersistRecord, 0, e.Keys())
	now := time.Now().Unix()
	for i := range e.shards {
		s := &e.shards[i]
		s.mu.RLock()
		for k, v := range s.m {
			if v.ExpiresAt > 0 && v.ExpiresAt <= now {
				continue
			}
			vv := make([]byte, len(v.Value))
			copy(vv, v.Value)
			records = append(records, PersistRecord{Key: k, Value: vv, ExpiresAt: v.ExpiresAt})
		}
		s.mu.RUnlock()
	}
	return records
}

func (e *Engine) Restore(records []PersistRecord) error {
	for _, r := range records {
		if r.ExpiresAt > 0 && r.ExpiresAt <= time.Now().Unix() {
			continue
		}
		if err := e.SetWithExpiresAt(r.Key, r.Value, r.ExpiresAt); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) runTTLWorker() {
	ticker := time.NewTicker(time.Second)
	defer func() {
		ticker.Stop()
		close(e.stopped)
	}()

	for {
		select {
		case <-ticker.C:
			e.evictExpired()
		case <-e.stopCh:
			return
		}
	}
}

func (e *Engine) evictExpired() {
	now := time.Now().Unix()
	for {
		e.ttlqMu.Lock()
		if len(e.ttlq) == 0 || e.ttlq[0].expiresAt > now {
			e.ttlqMu.Unlock()
			return
		}
		it := heap.Pop(&e.ttlq).(ttlItem)
		e.ttlqMu.Unlock()
		_ = e.deleteIfExpired(it.key, it.expiresAt)
	}
}

func (e *Engine) deleteIfExpired(key string, expiresAt int64) error {
	if expiresAt == 0 {
		return nil
	}
	idx := e.shardIndex(key)
	s := &e.shards[idx]
	now := time.Now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	ent, ok := s.m[key]
	if !ok {
		return nil
	}
	if ent.ExpiresAt != expiresAt || ent.ExpiresAt == 0 || ent.ExpiresAt > now {
		return nil
	}
	delete(s.m, key)
	e.memUsage.Add(-estimateEntrySize(key, ent.Value))
	return nil
}

func (e *Engine) validate(key string, value []byte) error {
	if key == "" || !utf8.ValidString(key) {
		return protocol.NewError(protocol.CodeInvalidRequest, "invalid key")
	}
	if e.maxKeySize > 0 && len([]byte(key)) > e.maxKeySize {
		return protocol.NewError(protocol.CodeKeyTooLong, "key too long")
	}
	if e.maxValueSize > 0 && len(value) > e.maxValueSize {
		return protocol.NewError(protocol.CodeValueTooLarge, "value too large")
	}
	return nil
}

func estimateEntrySize(key string, value []byte) int64 {
	return int64(len([]byte(key)) + len(value) + 32)
}

func isExpired(expiresAt int64) bool {
	return expiresAt > 0 && expiresAt <= time.Now().Unix()
}

func (e *Engine) shardIndex(key string) int {
	h := utils.HashString(key)
	if e.shardMask > 0 {
		return int(h & e.shardMask)
	}
	return int(h % uint64(e.shardCount))
}
