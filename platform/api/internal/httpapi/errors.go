package httpapi

import (
	"encoding/json"
	"net/http"
)

// Error is the error body shared by every endpoint: {code,message,fields,request_id} (web-v1.md §10).
type Error struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id"`
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	WriteJSON(w, status, Error{Code: code, Message: message, RequestID: RequestID(r.Context())})
}
