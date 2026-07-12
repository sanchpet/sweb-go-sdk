package sweb

import "fmt"

// Error is an API-level failure: either a JSON-RPC error object returned by the
// API, or a synthesized error for a non-200 HTTP response (Code = status code).
//
// The JSON-RPC error shape is confirmed against real responses: SpaceWeb returns
// {"code": -32500, "message": "<human text>", "data": []} — a numeric code, a
// (Russian) message, and a data field that is an empty array in the observed
// cases.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("sweb: api error %d: %s", e.Code, e.Message)
}
