package main

import (
	"context"
	"fmt"
	"time"

	"github.com/shinerio/gopher-kv/pkg/client"
)

// Result is the structured response returned to the frontend.
// All App methods return this type so the JS layer can check success uniformly.
type Result struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// App is the Wails application struct. Its exported methods are bound to the
// frontend via the Wails JS runtime.
type App struct {
	ctx    context.Context
	client *client.Client
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Connect establishes a connection to the GopherKV server at host:port.
// It performs a health-check to verify the server is reachable.
func (a *App) Connect(host string, port int) Result {
	baseURL := fmt.Sprintf("http://%s:%d", host, port)
	c := client.NewClient(baseURL)
	if err := c.Health(); err != nil {
		return Result{Success: false, Message: fmt.Sprintf("connection failed: %v", err)}
	}
	a.client = c
	return Result{Success: true, Message: fmt.Sprintf("connected to %s:%d", host, port)}
}

// Disconnect clears the current client connection.
func (a *App) Disconnect() Result {
	a.client = nil
	return Result{Success: true, Message: "disconnected"}
}

// SetKey writes key=value with an optional TTL (0 means no expiry).
func (a *App) SetKey(key, value string, ttlSeconds int) Result {
	if a.client == nil {
		return Result{Success: false, Message: "not connected"}
	}
	var ttl time.Duration
	if ttlSeconds > 0 {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	if err := a.client.Set(key, []byte(value), ttl); err != nil {
		return Result{Success: false, Message: err.Error()}
	}
	return Result{Success: true, Message: "OK"}
}

// GetKey retrieves the value for key. Returns "(nil)" in Message when the key
// does not exist (mirrors CLI output).
func (a *App) GetKey(key string) Result {
	if a.client == nil {
		return Result{Success: false, Message: "not connected"}
	}
	val, err := a.client.Get(key)
	if err != nil {
		return Result{Success: false, Message: err.Error()}
	}
	if val == nil {
		return Result{Success: true, Message: "(nil)"}
	}
	str := string(val)
	return Result{Success: true, Message: fmt.Sprintf("%q", str), Data: str}
}

// DeleteKey removes key from the store. Silently succeeds for non-existent keys.
func (a *App) DeleteKey(key string) Result {
	if a.client == nil {
		return Result{Success: false, Message: "not connected"}
	}
	if err := a.client.Delete(key); err != nil {
		return Result{Success: false, Message: err.Error()}
	}
	return Result{Success: true, Message: "OK"}
}

// ExistsKey reports whether key exists in the store.
func (a *App) ExistsKey(key string) Result {
	if a.client == nil {
		return Result{Success: false, Message: "not connected"}
	}
	exists, err := a.client.Exists(key)
	if err != nil {
		return Result{Success: false, Message: err.Error()}
	}
	if exists {
		return Result{Success: true, Message: "(integer) 1", Data: true}
	}
	return Result{Success: true, Message: "(integer) 0", Data: false}
}

// GetTTL returns the remaining TTL in seconds for key.
func (a *App) GetTTL(key string) Result {
	if a.client == nil {
		return Result{Success: false, Message: "not connected"}
	}
	ttl, err := a.client.TTL(key)
	if err != nil {
		return Result{Success: false, Message: err.Error()}
	}
	return Result{Success: true, Message: fmt.Sprintf("(integer) %d", ttl), Data: ttl}
}

// GetStats fetches server monitoring statistics.
func (a *App) GetStats() Result {
	if a.client == nil {
		return Result{Success: false, Message: "not connected"}
	}
	stats, err := a.client.Stats()
	if err != nil {
		return Result{Success: false, Message: err.Error()}
	}
	return Result{Success: true, Message: "OK", Data: stats}
}

// TriggerSnapshot triggers a manual RDB snapshot on the server.
func (a *App) TriggerSnapshot() Result {
	if a.client == nil {
		return Result{Success: false, Message: "not connected"}
	}
	snap, err := a.client.Snapshot()
	if err != nil {
		return Result{Success: false, Message: err.Error()}
	}
	return Result{
		Success: true,
		Message: fmt.Sprintf("snapshot saved: %s", snap.Path),
		Data:    snap,
	}
}

