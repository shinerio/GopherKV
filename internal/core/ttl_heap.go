package core

import (
	"container/heap"
	"sync"
	"time"
)

type TTLItem struct {
	Key       string
	ExpiresAt int64
	index     int
}

type TTLHeap []*TTLItem

func (h TTLHeap) Len() int           { return len(h) }
func (h TTLHeap) Less(i, j int) bool { return h[i].ExpiresAt < h[j].ExpiresAt }
func (h TTLHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *TTLHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*TTLItem)
	item.index = n
	*h = append(*h, item)
}

func (h *TTLHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

type TTLManager struct {
	heap     TTLHeap
	mu       sync.Mutex
	stopCh   chan struct{}
	stopped  bool
	onExpire func(key string)
}

func NewTTLManager(onExpire func(key string)) *TTLManager {
	h := &TTLHeap{}
	heap.Init(h)
	return &TTLManager{
		heap:     *h,
		stopCh:   make(chan struct{}),
		onExpire: onExpire,
	}
}

func (tm *TTLManager) Add(key string, expiresAt int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.stopped {
		return
	}

	heap.Push(&tm.heap, &TTLItem{
		Key:       key,
		ExpiresAt: expiresAt,
	})
}

func (tm *TTLManager) Start() {
	go tm.run()
}

func (tm *TTLManager) Stop() {
	tm.mu.Lock()
	if tm.stopped {
		tm.mu.Unlock()
		return
	}
	tm.stopped = true
	close(tm.stopCh)
	tm.mu.Unlock()
}

func (tm *TTLManager) run() {
	for {
		select {
		case <-tm.stopCh:
			return
		default:
			tm.process()
		}
	}
}

func (tm *TTLManager) process() {
	tm.mu.Lock()
	if tm.heap.Len() == 0 {
		tm.mu.Unlock()
		time.Sleep(1 * time.Second)
		return
	}

	now := time.Now().UnixMilli()
	item := tm.heap[0]
	if item.ExpiresAt > now {
		waitTime := time.Duration(item.ExpiresAt-now) * time.Millisecond
		tm.mu.Unlock()

		select {
		case <-tm.stopCh:
			return
		case <-time.After(waitTime):
		}
		return
	}

	heap.Pop(&tm.heap)
	key := item.Key
	tm.mu.Unlock()

	if tm.onExpire != nil {
		tm.onExpire(key)
	}
}
