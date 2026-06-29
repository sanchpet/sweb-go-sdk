package sweb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestVPSRemove(t *testing.T) {
	var gotMethod, gotBillingID string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				BillingID string `json:"billingId"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotBillingID = req.Method, req.Params.BillingID
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})

	if _, err := c.VPS.Remove(context.Background(), "login_vps_6"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if gotMethod != "remove" {
		t.Errorf("method = %q, want remove", gotMethod)
	}
	if gotBillingID != "login_vps_6" {
		t.Errorf("billingId = %q, want login_vps_6", gotBillingID)
	}
}

// fixtureServer serves the bytes of testdata/<file> for any request.
func fixtureServer(t *testing.T, file string) *Client {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return serve(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
}

func serve(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithToken("test-token"))
}

func TestCreateToken(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notAuthorized/" || r.Method != http.MethodPost {
			t.Errorf("got %s %s, want POST /notAuthorized/", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"tok_abc123"}`))
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

	_, err := c.VPS.List(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *Error, got %T: %v", err, err)
	}
	if apiErr.Code != -32400 || apiErr.Message != "Wrong password" {
		t.Errorf("error = %+v, want code -32400 / Wrong password", apiErr)
	}
}

func TestVPSList(t *testing.T) {
	c := fixtureServer(t, "vps_index.json")

	list, err := c.VPS.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	v := list[0]
	if v.Name != "example-node" || v.UID != "vps-example-0001" {
		t.Errorf("name/uid = %q/%q", v.Name, v.UID)
	}
	if v.CPU != 2 || v.RAM != 2048 || v.DiskGB != 40 {
		t.Errorf("specs cpu/ram/disk = %d/%d/%d", v.CPU, v.RAM, v.DiskGB)
	}
	if v.IP != "203.0.113.10" || len(v.ExtIPs) != 1 {
		t.Errorf("ip/extips = %q/%v", v.IP, v.ExtIPs)
	}
	if len(v.SSHKeys) != 1 || v.SSHKeys[0].Name != "laptop" {
		t.Errorf("ssh_keys = %+v", v.SSHKeys)
	}
	if !v.Features.AllowBackups || v.Features.MaxIPCount != 4 {
		t.Errorf("features = %+v", v.Features)
	}
}

func TestAvailableConfig(t *testing.T) {
	c := fixtureServer(t, "available_config.json")

	cfg, err := c.VPS.AvailableConfig(context.Background())
	if err != nil {
		t.Fatalf("AvailableConfig: %v", err)
	}
	if len(cfg.VPSPlans) != 1 || cfg.VPSPlans[0].ID != 4 || cfg.VPSPlans[0].CPUCores != "2" {
		t.Errorf("vpsPlans = %+v", cfg.VPSPlans)
	}
	if len(cfg.Datacenters) != 1 || cfg.Datacenters[0].Location != "Saint Petersburg" {
		t.Errorf("datacenters = %+v", cfg.Datacenters)
	}
	if len(cfg.OSPanel) != 1 || cfg.OSPanel[0].MinRAM != 1024 {
		t.Errorf("osPanel = %+v", cfg.OSPanel)
	}
	if len(cfg.SelectOS) != 1 || cfg.SelectOS[0].Name != "Debian" {
		t.Errorf("selectOs = %+v", cfg.SelectOS)
	}
}

func TestNon200IsError(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
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
