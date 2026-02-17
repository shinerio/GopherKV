package storage

import (
	"crypto/sha256"
	"math"
	"sync"
	"time"
)

type Entry struct {
	Value     []byte
	ExpiresAt int64
}

type Shard struct {
	mu    sync.RWMutex
	items map[string]Entry
	mem   int64
}

type ConcurrentMap struct {
	shards    []*Shard
	shardMask uint32
}

func NewConcurrentMap(shardCount int) *ConcurrentMap {
	actualShardCount := int(math.Ceil(math.Log2(float64(shardCount))))
	actualShardCount = 1 << actualShardCount
	if actualShardCount > shardCount {
		actualShardCount = shardCount
	}
	shards := make([]*Shard, actualShardCount)
	for i := range shards {
		shards[i] = &Shard{
			items: make(map[string]Entry),
		}
	}
	return &ConcurrentMap{
		shards:    shards,
		shardMask: uint32(actualShardCount - 1),
	}
}

func (cm *ConcurrentMap) getShard(key string) *Shard {
	h := sha256.Sum256([]byte(key))
	idx := uint32(h[0]) | uint32(h[1])<<8 | uint32(h[2])<<16 | uint32(h[3])<<24
	return cm.shards[idx&cm.shardMask]
}

func (cm *ConcurrentMap) Set(key string, value []byte, expiresAt int64) int64 {
	shard := cm.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	oldEntry, exists := shard.items[key]
	var memDelta int64

	if exists {
		memDelta -= int64(len(key) + len(oldEntry.Value))
	}

	newEntry := Entry{
		Value:     value,
		ExpiresAt: expiresAt,
	}
	shard.items[key] = newEntry
	memDelta += int64(len(key) + len(value))
	shard.mem += memDelta

	return memDelta
}

func (cm *ConcurrentMap) Get(key string) ([]byte, int64, bool) {
	shard := cm.getShard(key)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	entry, exists := shard.items[key]
	if !exists {
		return nil, 0, false
	}

	if entry.ExpiresAt > 0 && time.Now().UnixMilli() > entry.ExpiresAt {
		return nil, 0, false
	}

	return entry.Value, entry.ExpiresAt, true
}

func (cm *ConcurrentMap) Delete(key string) int64 {
	shard := cm.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	oldEntry, exists := shard.items[key]
	if !exists {
		return 0
	}

	memDelta := -(int64(len(key) + len(oldEntry.Value)))
	delete(shard.items, key)
	shard.mem += memDelta

	return memDelta
}

func (cm *ConcurrentMap) Exists(key string) bool {
	shard := cm.getShard(key)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	entry, exists := shard.items[key]
	if !exists {
		return false
	}

	if entry.ExpiresAt > 0 && time.Now().UnixMilli() > entry.ExpiresAt {
		return false
	}

	return true
}

func (cm *ConcurrentMap) Keys() int {
	total := 0
	for _, shard := range cm.shards {
		shard.mu.RLock()
		total += len(shard.items)
		shard.mu.RUnlock()
	}
	return total
}

func (cm *ConcurrentMap) MemUsage() int64 {
	var total int64
	for _, shard := range cm.shards {
		shard.mu.RLock()
		total += shard.mem
		shard.mu.RUnlock()
	}
	return total
}

func (cm *ConcurrentMap) Iterate(fn func(key string, entry Entry) bool) {
	for _, shard := range cm.shards {
		shard.mu.RLock()
		for key, entry := range shard.items {
			if !fn(key, entry) {
				shard.mu.RUnlock()
				return
			}
		}
		shard.mu.RUnlock()
	}
}
