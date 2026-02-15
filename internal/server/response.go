package server

import (
	"encoding/json"
	"net/http"

	"github.com/shinerio/gopher-kv/pkg/protocol"
)

func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, protocol.APIResponse{Code: protocol.CodeOK, Data: data, Msg: "ok"})
}

func writeErr(w http.ResponseWriter, err error) {
	apiErr, ok := err.(*protocol.APIError)
	if !ok {
		apiErr = protocol.NewError(protocol.CodeInternal, err.Error())
	}
	writeJSON(w, apiErr.HTTPStatus, protocol.APIResponse{Code: apiErr.Code, Data: nil, Msg: apiErr.Message})
}

func writeJSON(w http.ResponseWriter, status int, body protocol.APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
