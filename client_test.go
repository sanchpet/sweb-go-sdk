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

func TestVPSChangePlan(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		VPSPlanID int    `json:"planId"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				BillingID string `json:"billingId"`
				VPSPlanID int    `json:"planId"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		gotParams.BillingID, gotParams.VPSPlanID = req.Params.BillingID, req.Params.VPSPlanID
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})

	if err := c.VPS.ChangePlan(context.Background(), "login_vps_1", 4); err != nil {
		t.Fatalf("ChangePlan: %v", err)
	}
	if gotMethod != "changePlan" {
		t.Errorf("method = %q, want changePlan", gotMethod)
	}
	// The wire param is "planId" (the docs' EXAMPLE wrongly shows "vpsPlanId").
	if gotParams.BillingID != "login_vps_1" || gotParams.VPSPlanID != 4 {
		t.Errorf("params = %q/%d, want login_vps_1/4", gotParams.BillingID, gotParams.VPSPlanID)
	}
}

func TestVPSChangePlanFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0}`))
	})
	if err := c.VPS.ChangePlan(context.Background(), "login_vps_1", 4); err == nil {
		t.Fatal("ChangePlan: want error on result 0, got nil")
	}
}

func TestGetConstructorPlanID(t *testing.T) {
	var gotParams map[string]int
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]int `json:"params"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		switch req.Method {
		case "getConstructorPlanId":
			gotParams = req.Params
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1234}`))
		case "getAvailableConfig": // sold-out guard: plan 1234 present and orderable
			_, _ = w.Write([]byte(`{"result":{"vpsPlans":[{"id":1234,"name":"OK","sold_out":false}]}}`))
		}
	})

	id, err := c.VPS.GetConstructorPlanID(context.Background(), 2, 6, 15, 1)
	if err != nil {
		t.Fatalf("GetConstructorPlanID: %v", err)
	}
	if id != 1234 {
		t.Errorf("id = %d, want 1234", id)
	}
	if gotParams["cpu_cores"] != 2 || gotParams["ram"] != 6 || gotParams["volume_disk"] != 15 || gotParams["category_id"] != 1 {
		t.Errorf("params = %+v, want cpu_cores=2 ram=6 volume_disk=15 category_id=1", gotParams)
	}
}

// The resolver can map out-of-range inputs onto a sold-out plan (e.g. 1/1/10 →
// the "Промо" plan); GetConstructorPlanID must surface that instead of returning
// an id that would fail create with "-32500 Тариф распродан".
func TestGetConstructorPlanIDSoldOut(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		switch req.Method {
		case "getConstructorPlanId":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":216}`))
		case "getAvailableConfig":
			_, _ = w.Write([]byte(`{"result":{"vpsPlans":[{"id":216,"name":"Облако Промо","sold_out":true}]}}`))
		}
	})

	if _, err := c.VPS.GetConstructorPlanID(context.Background(), 1, 1, 10, 1); err == nil {
		t.Fatal("want error when the configurator resolves to a sold-out plan")
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
	// Numeric-ish fields SpaceWeb quotes inconsistently — FlexInt/FlexFloat decode
	// both forms. Regression guards: ram "1024" and plan_id "4" (strings) and
	// plan_price 0.9 (fractional) would each crash a plain int field.
	if v.CPU != 2 || v.RAM != 1024 || v.PlanID != 4 {
		t.Errorf("cpu/ram/plan_id = %d/%d/%d", v.CPU, v.RAM, v.PlanID)
	}
	if v.PlanPrice != 0.9 {
		t.Errorf("plan_price = %v, want 0.9", v.PlanPrice)
	}
	if v.Disk != "10 ГБ" || v.Datacenter != "Moscow" || v.OrderedIPCount != 2 {
		t.Errorf("disk/datacenter/ordered = %q/%q/%d", v.Disk, v.Datacenter, v.OrderedIPCount)
	}
	// ext_ips is crash-safe raw pending a populated example; empty here.
	if v.IP != "203.0.113.10" || len(v.ExtIPs) != 0 {
		t.Errorf("ip/ext_ips = %q/%v", v.IP, v.ExtIPs)
	}
	if len(v.ISP) != 1 || v.ISP[0].LicenseType != "LEMP" {
		t.Errorf("isp = %+v", v.ISP)
	}
	if len(v.ProtectedIPs) != 1 || v.ProtectedIPs[0] != "203.0.113.53" {
		t.Errorf("protected_ips = %v", v.ProtectedIPs)
	}
	// parent_plan_id / local_* are null in the response → zero values, no error.
	if v.ParentPlanID != 0 || v.LocalIP != "" {
		t.Errorf("nullable parent_plan_id/local_ip = %d/%q", v.ParentPlanID, v.LocalIP)
	}
	if v.BlockUI != 1 || v.IsRunning != 0 || v.IsTest != 0 || v.IsNew {
		t.Errorf("flags block/run/test/new = %d/%d/%d/%t", v.BlockUI, v.IsRunning, v.IsTest, v.IsNew)
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

func TestGetFirstOrderInfo(t *testing.T) {
	c := fixtureServer(t, "first_order_info.json")

	info, err := c.VPS.GetFirstOrderInfo(context.Background())
	if err != nil {
		t.Fatalf("GetFirstOrderInfo: %v", err)
	}
	if info == nil {
		t.Fatal("info = nil, want unwrapped object")
	}
	// Unwrapped from the nested JSON-RPC envelope (result[0].result); cpu_cores
	// and ram arrive as quoted strings.
	if info.Plan != "CLOUD PROMO на год" || info.CPUCores != 1 || info.RAM != 1024 {
		t.Errorf("plan/cpu/ram = %q/%d/%d", info.Plan, info.CPUCores, info.RAM)
	}
	// Authoritative by name+value (doc swaps the *_with_stock descriptions):
	// pay_period is months, price_for_period is the period total.
	if info.PayPeriod != 12 || info.PriceForPeriodWithStock != 1668 || info.PricePerMonthWithStock != 139 {
		t.Errorf("period/prices = %d/%v/%v", info.PayPeriod, info.PriceForPeriodWithStock, info.PricePerMonthWithStock)
	}
	if info.Promocode != "" || info.ClearAvailable {
		t.Errorf("promocode/clear = %q/%t", info.Promocode, info.ClearAvailable)
	}
	if len(info.ProtectedIPs) != 2 || info.ProtectedIPs[0] != 1 || info.ProtectedIPs[1] != 2 {
		t.Errorf("protectedIps = %v", info.ProtectedIPs)
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
