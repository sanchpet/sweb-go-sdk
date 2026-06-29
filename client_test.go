package sweb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient spins an httptest server and returns a Client pointed at it.
// (Inline JSON here stands in for recorded fixtures until the Evidence phase
// captures anonymized real responses under testdata/.)
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithToken("test-token"))
}

func TestCreateToken(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notAuthorized/" {
			t.Errorf("path = %s, want /notAuthorized/", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		_, _ = w.Write([]byte(`{"result":"tok_abc123"}`))
	})

	got, err := c.CreateToken(context.Background(), "user", "pass")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if got != "tok_abc123" {
		t.Errorf("token = %q, want tok_abc123", got)
	}
}

func TestAuthHeaderAndAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		_, _ = w.Write([]byte(`{"error":{"code":42,"message":"boom"}}`))
	})

	_, err := c.VPS.AvailableConfig(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *Error, got %T: %v", err, err)
	}
	if apiErr.Code != 42 || apiErr.Message != "boom" {
		t.Errorf("error = %+v, want code 42 / boom", apiErr)
	}
}

func TestVPSList(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":[{"id":101,"alias":"hub","status":"active"}]}`))
	})

	list, err := c.VPS.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Alias != "hub" || list[0].ID.String() != "101" {
		t.Errorf("list = %+v, want one item {101 hub active}", list)
	}
}

func TestNon200IsError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`upstream exploded`))
	})

	_, err := c.VPS.List(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *Error, got %T: %v", err, err)
	}
	if apiErr.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", apiErr.Code)
	}
}
