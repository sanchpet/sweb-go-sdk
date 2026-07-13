package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/apierr"
)

func TestAutoRefreshOnSessionExpired(t *testing.T) {
	var indexCalls int
	var refreshed string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		switch req.Method {
		case "getToken":
			_, _ = w.Write([]byte(`{"result":"fresh-token"}`))
		case "index":
			indexCalls++
			if indexCalls == 1 {
				_, _ = w.Write([]byte(`{"error":{"code":-32603,"message":"Время сеанса истекло.","data":[]}}`))
				return
			}
			if got := r.Header.Get("Authorization"); got != "Bearer fresh-token" {
				t.Errorf("retry Authorization = %q, want Bearer fresh-token", got)
			}
			_, _ = w.Write([]byte(`{"result":[]}`))
		}
	}))
	t.Cleanup(srv.Close)

	c := New(
		WithBaseURL(srv.URL), WithHTTPClient(srv.Client()),
		WithToken("stale-token"),
		WithCredentials("user", "pass"),
		WithOnTokenRefresh(func(tok string) { refreshed = tok }),
	)

	var out []json.RawMessage
	if err := c.Call(context.Background(), "/vps", "index", nil, &out); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if indexCalls != 2 {
		t.Errorf("index calls = %d, want 2 (expired + retry)", indexCalls)
	}
	if refreshed != "fresh-token" || c.Token() != "fresh-token" {
		t.Errorf("refresh: callback=%q token=%q, want fresh-token", refreshed, c.Token())
	}
}

func TestGetToken(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notAuthorized/" || r.Method != http.MethodPost {
			t.Errorf("got %s %s, want POST /notAuthorized/", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"tok_abc123"}`))
	})

	got, err := c.GetToken(context.Background(), "user", "pass")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got != "tok_abc123" {
		t.Errorf("token = %q, want tok_abc123", got)
	}
}

func TestAuthHeaderAndAPIError(t *testing.T) {
	// Mirrors the real envelope observed from the API: {..., "error":{code,message,data}}.
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","version":"1.0","id":"x","error":{"code":-32400,"message":"Wrong password","data":[]}}`))
	})

	err := c.Call(context.Background(), "/vps", "index", nil, nil)
	var apiErr *apierr.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *apierr.Error, got %T: %v", err, err)
	}
	if apiErr.Code != -32400 || apiErr.Message != "Wrong password" {
		t.Errorf("error = %+v, want code -32400 / Wrong password", apiErr)
	}
}

func TestNon200IsError(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`upstream exploded`))
	})

	err := c.Call(context.Background(), "/vps", "index", nil, nil)
	var apiErr *apierr.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *apierr.Error, got %T: %v", err, err)
	}
	if apiErr.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", apiErr.Code)
	}
}

// serve spins up a mock JSON-RPC server for h and returns a Client pointed at it
// with a fixed test token.
func serve(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithToken("test-token"))
}
