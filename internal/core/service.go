package core

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"github.com/shinerio/gopher-kv/internal/config"
	"github.com/shinerio/gopher-kv/internal/storage"
	"github.com/shinerio/gopher-kv/pkg/protocol"
)

var (
	ErrKeyNotFound   = errors.New("key not found")
	ErrKeyTooLong    = errors.New("key too long")
	ErrValueTooLarge = errors.New("value too large")
	ErrMemoryFull    = errors.New("memory full")
)

type Service struct {
	cfg       *config.Config
	storage   *storage.ConcurrentMap
	ttlMgr    *TTLManager
	memUsage  int64
	hits      int64
	misses    int64
	requests  atomic.Value
	startTime time.Time
}

func NewService(cfg *config.Config) *Service {
	s := &Service{
		cfg:       cfg,
		storage:   storage.NewConcurrentMap(cfg.Storage.ShardCount),
		startTime: time.Now(),
	}
	s.ttlMgr = NewTTLManager(func(key string) {
		s.storage.Delete(key)
		slog.Debug("TTL expired", "key", key)
	})
	s.requests.Store(make(map[string]int64))
	return s
}

func (s *Service) Start() {
	s.ttlMgr.Start()
}

func (s *Service) Stop() {
	s.ttlMgr.Stop()
}

func (s *Service) validateKey(key string) error {
	if len(key) == 0 {
		return fmt.Errorf("%w: empty key", ErrKeyTooLong)
	}
	if len(key) > s.cfg.Storage.MaxKeySize {
		return fmt.Errorf("%w: max %d bytes", ErrKeyTooLong, s.cfg.Storage.MaxKeySize)
	}
	return nil
}

func (s *Service) validateValue(value []byte) error {
	if len(value) > s.cfg.Storage.MaxValueSize {
		return fmt.Errorf("%w: max %d bytes", ErrValueTooLarge, s.cfg.Storage.MaxValueSize)
	}
	return nil
}

func (s *Service) recordRequest(op string) {
	reqs := s.requests.Load().(map[string]int64)
	newReqs := make(map[string]int64)
	for k, v := range reqs {
		newReqs[k] = v
	}
	newReqs[op]++
	s.requests.Store(newReqs)
}

func (s *Service) Set(key string, value []byte, ttl time.Duration) error {
	s.recordRequest("set")

	if err := s.validateKey(key); err != nil {
		return err
	}
	if err := s.validateValue(value); err != nil {
		return err
	}

	var expiresAt int64
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).UnixMilli()
	}

	currentMem := atomic.LoadInt64(&s.memUsage)
	estimatedDelta := int64(len(key) + len(value))

	if currentMem+estimatedDelta > s.cfg.Storage.MaxMemory {
		return ErrMemoryFull
	}

	memDelta := s.storage.Set(key, value, expiresAt)
	atomic.AddInt64(&s.memUsage, memDelta)

	if ttl > 0 {
		s.ttlMgr.Add(key, expiresAt)
	}

	return nil
}

func (s *Service) Get(key string) ([]byte, time.Duration, error) {
	s.recordRequest("get")

	if err := s.validateKey(key); err != nil {
		return nil, 0, err
	}

	value, expiresAt, exists := s.storage.Get(key)
	if !exists {
		atomic.AddInt64(&s.misses, 1)
		return nil, 0, ErrKeyNotFound
	}

	atomic.AddInt64(&s.hits, 1)

	var ttlRemaining time.Duration
	if expiresAt > 0 {
		ttlRemaining = time.Until(time.UnixMilli(expiresAt))
		if ttlRemaining < 0 {
			ttlRemaining = 0
		}
	}

	return value, ttlRemaining, nil
}

func (s *Service) Delete(key string) error {
	s.recordRequest("del")

	if err := s.validateKey(key); err != nil {
		return err
	}

	memDelta := s.storage.Delete(key)
	atomic.AddInt64(&s.memUsage, memDelta)

	return nil
}

func (s *Service) Exists(key string) (bool, error) {
	if err := s.validateKey(key); err != nil {
		return false, err
	}

	return s.storage.Exists(key), nil
}

func (s *Service) TTL(key string) (time.Duration, error) {
	if err := s.validateKey(key); err != nil {
		return 0, err
	}

	_, expiresAt, exists := s.storage.Get(key)
	if !exists {
		return -1 * time.Second, ErrKeyNotFound
	}

	if expiresAt == 0 {
		return -1 * time.Second, nil
	}

	remaining := time.Until(time.UnixMilli(expiresAt))
	if remaining < 0 {
		return -2 * time.Second, nil
	}

	// Return second-level TTL semantics aligned with Redis style integer TTL.
	return time.Duration(math.Ceil(remaining.Seconds())) * time.Second, nil
}

func (s *Service) Keys() int {
	return s.storage.Keys()
}

func (s *Service) MemUsage() int64 {
	return atomic.LoadInt64(&s.memUsage)
}

func (s *Service) Stats() *protocol.StatsResponseData {
	return &protocol.StatsResponseData{
		Keys:     s.Keys(),
		Memory:   s.MemUsage(),
		Hits:     atomic.LoadInt64(&s.hits),
		Misses:   atomic.LoadInt64(&s.misses),
		Requests: s.requests.Load().(map[string]int64),
		Uptime:   int64(time.Since(s.startTime).Seconds()),
	}
}

func (s *Service) ErrorToCode(err error) int {
	if err == nil {
		return protocol.CodeSuccess
	}
	switch {
	case errors.Is(err, ErrKeyNotFound):
		return protocol.CodeKeyNotFound
	case errors.Is(err, ErrKeyTooLong):
		return protocol.CodeKeyTooLong
	case errors.Is(err, ErrValueTooLarge):
		return protocol.CodeValueTooLarge
	case errors.Is(err, ErrMemoryFull):
		return protocol.CodeMemoryFull
	default:
		return protocol.CodeInternalError
	}
}
