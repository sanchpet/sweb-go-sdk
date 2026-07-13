package ssh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestSSHOn(t *testing.T) {
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.On(context.Background(), 24); err != nil {
		t.Fatalf("On: %v", err)
	}
	if gotMethod != "sshOn" {
		t.Errorf("method = %q, want sshOn", gotMethod)
	}
	period, ok := gotParams["period"]
	if !ok || string(period) != "24" {
		t.Errorf("params = %v, want period 24", gotParams)
	}
}

func TestSSHOff(t *testing.T) {
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Off(context.Background()); err != nil {
		t.Fatalf("Off: %v", err)
	}
	if gotMethod != "sshOff" {
		t.Errorf("method = %q, want sshOff", gotMethod)
	}
	if len(gotParams) != 0 {
		t.Errorf("params = %v, want none", gotParams)
	}
}

func TestSSHSentinelFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error, for both methods.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.On(context.Background(), 3); err == nil {
		t.Error("On with result 0: got nil error, want failure")
	}
	if err := s.Off(context.Background()); err == nil {
		t.Error("Off with result 0: got nil error, want failure")
	}
}

func decodeReq(r *http.Request) (method string, params map[string]json.RawMessage) {
	var req struct {
		Method string                     `json:"method"`
		Params map[string]json.RawMessage `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	return req.Method, req.Params
}

// serve spins up a mock JSON-RPC server for h and returns an ssh.Service
// backed by a transport pointed at it.
func serve(t *testing.T, h http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(transport.New(
		transport.WithBaseURL(srv.URL),
		transport.WithHTTPClient(srv.Client()),
		transport.WithToken("test-token"),
	))
}
