// Package httpx holds HTTP plumbing: JSON responses, routing, and handlers.
//
// Nothing here logs. Structured logging is not built yet and every call path
// carries a context so that adding slog later is a wiring change rather than a
// refactor (CLAUDE.md golden rule 7).
package httpx

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with the given status.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

// ErrorBody is the single error shape the API returns.
//
// Message is written for the person reading it in the interface: what happened
// and what to do, never an apology and never vague.
type ErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Error writes a structured error response.
func Error(w http.ResponseWriter, status int, code, msg string) {
	JSON(w, status, ErrorBody{Error: code, Message: msg})
}
