package sweb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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

	if _, err := c.VPS.List(context.Background()); err != nil {
		t.Fatalf("List: %v", err)
	}
	if indexCalls != 2 {
		t.Errorf("index calls = %d, want 2 (expired + retry)", indexCalls)
	}
	if refreshed != "fresh-token" || c.Token() != "fresh-token" {
		t.Errorf("refresh: callback=%q token=%q, want fresh-token", refreshed, c.Token())
	}
}

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

func TestVPSRename(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		Alias     string `json:"alias"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				BillingID string `json:"billingId"`
				Alias     string `json:"alias"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		gotParams.BillingID, gotParams.Alias = req.Params.BillingID, req.Params.Alias
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})

	if err := c.VPS.Rename(context.Background(), "login_vps_6", "infra-01"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if gotMethod != "rename" {
		t.Errorf("method = %q, want rename", gotMethod)
	}
	if gotParams.BillingID != "login_vps_6" {
		t.Errorf("billingId = %q, want login_vps_6", gotParams.BillingID)
	}
	if gotParams.Alias != "infra-01" {
		t.Errorf("alias = %q, want infra-01", gotParams.Alias)
	}
}

func TestGetConstructorPlanID(t *testing.T) {
	var gotMethod string
	var gotParams map[string]int
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]int `json:"params"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		gotMethod, gotParams = req.Method, req.Params
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1234}`))
	})

	id, err := c.VPS.GetConstructorPlanID(context.Background(), 2, 6, 15, 1)
	if err != nil {
		t.Fatalf("GetConstructorPlanID: %v", err)
	}
	if id != 1234 {
		t.Errorf("id = %d, want 1234", id)
	}
	if gotMethod != "getConstructorPlanId" {
		t.Errorf("method = %q, want getConstructorPlanId", gotMethod)
	}
	if gotParams["cpu_cores"] != 2 || gotParams["ram"] != 6 || gotParams["volume_disk"] != 15 || gotParams["category_id"] != 1 {
		t.Errorf("params = %+v, want cpu_cores=2 ram=6 volume_disk=15 category_id=1", gotParams)
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
	// plan_price is money — the API returns fractional values; must decode as float
	// (regression: an int field failed on 0.9 → "cannot unmarshal number 0.9 into int").
	if v.PlanPrice != 0.9 {
		t.Errorf("plan_price = %v, want 0.9", v.PlanPrice)
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
	// price_per_month is money — same fractional-decode contract as plan_price.
	if cfg.VPSPlans[0].PricePerMonth != 500.5 {
		t.Errorf("price_per_month = %v, want 500.5", cfg.VPSPlans[0].PricePerMonth)
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
