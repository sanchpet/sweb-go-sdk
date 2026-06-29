package sweb

import "fmt"

// Error is an API-level failure: either a JSON-RPC error object returned by the
// API, or a synthesized error for a non-200 HTTP response (Code = status code).
//
// NOTE: the exact JSON-RPC error shape SpaceWeb returns is not yet confirmed
// against a real response (Evidence phase) — Code/Message/Data are the standard
// JSON-RPC fields and may need adjusting once real error payloads are recorded.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("sweb: api error %d: %s", e.Code, e.Message)
}
