package server

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/shinerio/gopher-kv/internal/core"
	"github.com/shinerio/gopher-kv/pkg/protocol"
)

type HTTPHandler struct {
	svc    *core.Service
	logger *slog.Logger
}

func NewHTTPHandler(svc *core.Service, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{svc: svc, logger: logger}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/key", h.handleKey)
	mux.HandleFunc("/v1/exists", h.handleExists)
	mux.HandleFunc("/v1/ttl", h.handleTTL)
	mux.HandleFunc("/v1/stats", h.handleStats)
	mux.HandleFunc("/v1/snapshot", h.handleSnapshot)
	mux.HandleFunc("/v1/health", h.handleHealth)
}

type putKeyReq struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	TTL   int64  `json:"ttl"`
}

func (h *HTTPHandler) handleKey(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		var req putKeyReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "invalid json"))
			return
		}
		value, err := base64.StdEncoding.DecodeString(req.Value)
		if err != nil {
			writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "value must be base64"))
			return
		}
		if err := h.svc.Set(r.Context(), req.Key, value, req.TTL); err != nil {
			writeErr(w, err)
			return
		}
		writeOK(w, nil)
	case http.MethodGet:
		key := r.URL.Query().Get("k")
		if key == "" {
			writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "missing key"))
			return
		}
		val, err := h.svc.Get(r.Context(), key)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeOK(w, map[string]interface{}{
			"value":         base64.StdEncoding.EncodeToString(val.Value),
			"ttl_remaining": val.TTLRemaining,
		})
	case http.MethodDelete:
		key := r.URL.Query().Get("k")
		if key == "" {
			writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "missing key"))
			return
		}
		if err := h.svc.Delete(r.Context(), key); err != nil {
			writeErr(w, err)
			return
		}
		writeOK(w, nil)
	default:
		writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "method not allowed"))
	}
}

func (h *HTTPHandler) handleExists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "method not allowed"))
		return
	}
	key := r.URL.Query().Get("k")
	if key == "" {
		writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "missing key"))
		return
	}
	writeOK(w, map[string]bool{"exists": h.svc.Exists(r.Context(), key)})
}

func (h *HTTPHandler) handleTTL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "method not allowed"))
		return
	}
	key := r.URL.Query().Get("k")
	if key == "" {
		writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "missing key"))
		return
	}
	ttl, err := h.svc.TTL(r.Context(), key)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, map[string]int64{"ttl": ttl})
}

func (h *HTTPHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "method not allowed"))
		return
	}
	writeOK(w, h.svc.Stats(r.Context()))
}

func (h *HTTPHandler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "method not allowed"))
		return
	}
	path, err := h.svc.Snapshot(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, map[string]string{"status": "ok", "path": path})
}

func (h *HTTPHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, protocol.NewError(protocol.CodeInvalidRequest, "method not allowed"))
		return
	}
	writeOK(w, map[string]string{"status": "healthy"})
}
