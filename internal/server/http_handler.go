package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/shinerio/gopher-kv/internal/core"
	"github.com/shinerio/gopher-kv/pkg/protocol"
)

type Handler struct {
	service *core.Service
}

type Middleware func(http.Handler) http.Handler

func NewHandler(service *core.Service) *Handler {
	return &Handler{
		service: service,
	}
}

func respondJSON(w http.ResponseWriter, code int, data interface{}, msg string) {
	w.Header().Set("Content-Type", "application/json")
	httpCode := http.StatusOK
	switch code {
	case protocol.CodeKeyNotFound, protocol.CodeKeyExpired:
		httpCode = http.StatusNotFound
	case protocol.CodeKeyTooLong, protocol.CodeValueTooLarge, protocol.CodeInvalidParam:
		httpCode = http.StatusBadRequest
	case protocol.CodeMemoryFull:
		httpCode = http.StatusInsufficientStorage
	case protocol.CodeInternalError:
		httpCode = http.StatusInternalServerError
	}
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(protocol.Response{
		Code: code,
		Data: data,
		Msg:  msg,
	})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, protocol.CodeSuccess, &protocol.HealthResponseData{
		Status: "healthy",
	}, "ok")
}

func (h *Handler) SetKey(w http.ResponseWriter, r *http.Request) {
	var req protocol.SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, protocol.CodeInvalidParam, nil, "invalid request body")
		return
	}

	value, err := base64.StdEncoding.DecodeString(req.Value)
	if err != nil {
		respondJSON(w, protocol.CodeInvalidParam, nil, "invalid base64 value")
		return
	}

	var ttl time.Duration
	if req.TTL > 0 {
		ttl = time.Duration(req.TTL) * time.Second
	}

	err = h.service.Set(req.Key, value, ttl)
	if err != nil {
		code := h.service.ErrorToCode(err)
		respondJSON(w, code, nil, protocol.CodeMessages[code])
		return
	}

	respondJSON(w, protocol.CodeSuccess, nil, "ok")
}

func (h *Handler) GetKey(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("k")
	if key == "" {
		respondJSON(w, protocol.CodeInvalidParam, nil, "missing key parameter")
		return
	}

	value, ttlRemaining, err := h.service.Get(key)
	if err != nil {
		code := h.service.ErrorToCode(err)
		respondJSON(w, code, nil, protocol.CodeMessages[code])
		return
	}

	data := &protocol.GetResponseData{
		Value: base64.StdEncoding.EncodeToString(value),
	}
	if ttlRemaining > 0 {
		data.TTLRemaining = int(ttlRemaining.Seconds())
	}

	respondJSON(w, protocol.CodeSuccess, data, "ok")
}

func (h *Handler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("k")
	if key == "" {
		respondJSON(w, protocol.CodeInvalidParam, nil, "missing key parameter")
		return
	}

	err := h.service.Delete(key)
	if err != nil {
		code := h.service.ErrorToCode(err)
		respondJSON(w, code, nil, protocol.CodeMessages[code])
		return
	}

	respondJSON(w, protocol.CodeSuccess, nil, "ok")
}

func (h *Handler) TTLKey(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("k")
	if key == "" {
		respondJSON(w, protocol.CodeInvalidParam, nil, "missing key parameter")
		return
	}

	ttl, err := h.service.TTL(key)
	if err != nil {
		code := h.service.ErrorToCode(err)
		respondJSON(w, code, nil, protocol.CodeMessages[code])
		return
	}

	respondJSON(w, protocol.CodeSuccess, &protocol.TTLResponseData{
		TTL: int(ttl.Seconds()),
	}, "ok")
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.service.Stats()
	respondJSON(w, protocol.CodeSuccess, stats, "ok")
}

func (h *Handler) Snapshot(w http.ResponseWriter, r *http.Request) {
	path, err := h.service.Snapshot()
	if err != nil {
		respondJSON(w, protocol.CodeInternalError, nil, protocol.CodeMessages[protocol.CodeInternalError])
		return
	}
	respondJSON(w, protocol.CodeSuccess, &protocol.SnapshotResponseData{
		Status: "ok",
		Path:   path,
	}, "ok")
}

func NewHTTPServer(addr string, handler *Handler, middlewares ...Middleware) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", handler.Health)
	mux.HandleFunc("PUT /v1/key", handler.SetKey)
	mux.HandleFunc("GET /v1/key", handler.GetKey)
	mux.HandleFunc("DELETE /v1/key", handler.DeleteKey)
	mux.HandleFunc("GET /v1/ttl", handler.TTLKey)
	mux.HandleFunc("GET /v1/stats", handler.Stats)
	mux.HandleFunc("POST /v1/snapshot", handler.Snapshot)

	var root http.Handler = mux
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] != nil {
			root = middlewares[i](root)
		}
	}

	return &http.Server{
		Addr:    addr,
		Handler: root,
	}
}
