package core

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shinerio/gopher-kv/internal/config"
	"github.com/shinerio/gopher-kv/internal/storage"
	"github.com/shinerio/gopher-kv/pkg/protocol"
)

type stats struct {
	Hits     atomic.Uint64
	Misses   atomic.Uint64
	ReqSet   atomic.Uint64
	ReqGet   atomic.Uint64
	ReqDel   atomic.Uint64
	ReqExist atomic.Uint64
	ReqTTL   atomic.Uint64
}

type Service struct {
	store *storage.Engine
	aof   *storage.AOF
	rdb   *storage.RDB
	cfg   config.Config

	startedAt time.Time
	stats     stats

	changeMu       sync.Mutex
	changeTimes    []time.Time
	maxRuleSeconds int

	bgStop chan struct{}
	bgDone chan struct{}
}

type KeyValue struct {
	Value        []byte
	TTLRemaining int64
}

type StatsResponse struct {
	Keys     int               `json:"keys"`
	Memory   int64             `json:"memory"`
	Hits     uint64            `json:"hits"`
	Misses   uint64            `json:"misses"`
	Requests map[string]uint64 `json:"requests"`
	Uptime   int64             `json:"uptime"`
}

func NewService(cfg config.Config, store *storage.Engine, aof *storage.AOF, rdb *storage.RDB) *Service {
	s := &Service{
		cfg:       cfg,
		store:     store,
		aof:       aof,
		rdb:       rdb,
		startedAt: time.Now(),
		bgStop:    make(chan struct{}),
		bgDone:    make(chan struct{}),
	}
	for _, r := range cfg.RDB.SaveRules {
		if r.Seconds > s.maxRuleSeconds {
			s.maxRuleSeconds = r.Seconds
		}
	}
	go s.background()
	return s
}

func (s *Service) Restore(records []storage.PersistRecord) error {
	return s.store.Restore(records)
}

func (s *Service) Set(ctx context.Context, key string, value []byte, ttlSeconds int64) error {
	_ = ctx
	s.stats.ReqSet.Add(1)
	var expiresAt int64
	if ttlSeconds > 0 {
		expiresAt = time.Now().Unix() + ttlSeconds
	}
	if err := s.store.SetWithExpiresAt(key, value, expiresAt); err != nil {
		return mapStorageErr(err)
	}
	if s.aof != nil {
		if err := s.aof.AppendSet(key, value, expiresAt); err != nil {
			return protocol.NewError(protocol.CodeInternal, err.Error())
		}
	}
	s.recordChange()
	return nil
}

func (s *Service) Get(ctx context.Context, key string) (KeyValue, error) {
	_ = ctx
	s.stats.ReqGet.Add(1)
	val, ok := s.store.Get(key)
	if !ok {
		s.stats.Misses.Add(1)
		return KeyValue{}, protocol.NewError(protocol.CodeKeyNotFound, "key not found")
	}
	ttl, _ := s.store.TTL(key)
	s.stats.Hits.Add(1)
	resp := KeyValue{Value: val, TTLRemaining: int64(ttl.Seconds())}
	if ttl < 0 {
		resp.TTLRemaining = -1
	}
	return resp, nil
}

func (s *Service) Delete(ctx context.Context, key string) error {
	_ = ctx
	s.stats.ReqDel.Add(1)
	if err := s.store.Delete(key); err != nil {
		return protocol.NewError(protocol.CodeInternal, err.Error())
	}
	if s.aof != nil {
		if err := s.aof.AppendDel(key); err != nil {
			return protocol.NewError(protocol.CodeInternal, err.Error())
		}
	}
	s.recordChange()
	return nil
}

func (s *Service) Exists(ctx context.Context, key string) bool {
	_ = ctx
	s.stats.ReqExist.Add(1)
	ok := s.store.Exists(key)
	if ok {
		s.stats.Hits.Add(1)
	} else {
		s.stats.Misses.Add(1)
	}
	return ok
}

func (s *Service) TTL(ctx context.Context, key string) (int64, error) {
	_ = ctx
	s.stats.ReqTTL.Add(1)
	ttl, ok := s.store.TTL(key)
	if !ok {
		s.stats.Misses.Add(1)
		return 0, protocol.NewError(protocol.CodeKeyNotFound, "key not found")
	}
	s.stats.Hits.Add(1)
	if ttl < 0 {
		return -1, nil
	}
	return int64(ttl.Seconds()), nil
}

func (s *Service) Stats(ctx context.Context) StatsResponse {
	_ = ctx
	return StatsResponse{
		Keys:   s.store.Keys(),
		Memory: s.store.MemUsage(),
		Hits:   s.stats.Hits.Load(),
		Misses: s.stats.Misses.Load(),
		Requests: map[string]uint64{
			"set":    s.stats.ReqSet.Load(),
			"get":    s.stats.ReqGet.Load(),
			"del":    s.stats.ReqDel.Load(),
			"exists": s.stats.ReqExist.Load(),
			"ttl":    s.stats.ReqTTL.Load(),
		},
		Uptime: int64(time.Since(s.startedAt).Seconds()),
	}
}

func (s *Service) Snapshot(ctx context.Context) (string, error) {
	_ = ctx
	if s.rdb == nil {
		return "", protocol.NewError(protocol.CodeInvalidRequest, "rdb disabled")
	}
	path, err := s.rdb.Save(s.store.Snapshot())
	if err != nil {
		return "", protocol.NewError(protocol.CodeInternal, err.Error())
	}
	return path, nil
}

func (s *Service) Close(ctx context.Context) error {
	_ = ctx
	close(s.bgStop)
	<-s.bgDone
	if s.rdb != nil {
		_, _ = s.rdb.Save(s.store.Snapshot())
	}
	if s.aof != nil {
		if err := s.aof.Sync(); err != nil {
			return err
		}
		if err := s.aof.Close(); err != nil {
			return err
		}
	}
	return s.store.Close()
}

func (s *Service) background() {
	ticker := time.NewTicker(time.Second)
	rewriteTicker := time.NewTicker(10 * time.Second)
	defer func() {
		ticker.Stop()
		rewriteTicker.Stop()
		close(s.bgDone)
	}()

	for {
		select {
		case <-ticker.C:
			s.maybeAutoSnapshot()
		case <-rewriteTicker.C:
			if s.aof != nil && s.aof.NeedsRewrite() {
				_ = s.aof.Rewrite(s.store.Snapshot())
			}
		case <-s.bgStop:
			return
		}
	}
}

func (s *Service) maybeAutoSnapshot() {
	if s.rdb == nil || len(s.cfg.RDB.SaveRules) == 0 {
		return
	}
	now := time.Now()
	s.changeMu.Lock()
	if s.maxRuleSeconds > 0 {
		cutoff := now.Add(-time.Duration(s.maxRuleSeconds) * time.Second)
		i := 0
		for ; i < len(s.changeTimes); i++ {
			if s.changeTimes[i].After(cutoff) {
				break
			}
		}
		if i > 0 {
			s.changeTimes = append([]time.Time(nil), s.changeTimes[i:]...)
		}
	}
	changes := append([]time.Time(nil), s.changeTimes...)
	s.changeMu.Unlock()

	for _, rule := range s.cfg.RDB.SaveRules {
		if rule.Seconds <= 0 || rule.Changes <= 0 {
			continue
		}
		count := 0
		from := now.Add(-time.Duration(rule.Seconds) * time.Second)
		for i := len(changes) - 1; i >= 0; i-- {
			if changes[i].Before(from) {
				break
			}
			count++
		}
		if count >= rule.Changes {
			_, _ = s.rdb.Save(s.store.Snapshot())
			s.changeMu.Lock()
			s.changeTimes = nil
			s.changeMu.Unlock()
			return
		}
	}
}

func (s *Service) recordChange() {
	s.changeMu.Lock()
	s.changeTimes = append(s.changeTimes, time.Now())
	s.changeMu.Unlock()
}

func mapStorageErr(err error) error {
	var apiErr *protocol.APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return protocol.NewError(protocol.CodeInternal, err.Error())
}
