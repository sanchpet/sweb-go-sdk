package sweb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestPowerActions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		call   func(*Client) error
		method string
	}{
		{"PowerOn", func(c *Client) error { return c.VPS.PowerOn(context.Background(), "login_vps_1") }, "powerOn"},
		{"PowerOff", func(c *Client) error { return c.VPS.PowerOff(context.Background(), "login_vps_1") }, "powerOff"},
		{"Reboot", func(c *Client) error { return c.VPS.Reboot(context.Background(), "login_vps_1") }, "reboot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotBilling string
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
					Params struct {
						BillingID string `json:"billingId"`
					} `json:"params"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod, gotBilling = req.Method, req.Params.BillingID
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
			})
			if err := tc.call(c); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method || gotBilling != "login_vps_1" {
				t.Errorf("method/billing = %q/%q, want %s/login_vps_1", gotMethod, gotBilling, tc.method)
			}
		})
	}
}

func TestPowerActionFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0}`))
	})
	if err := c.VPS.Reboot(context.Background(), "login_vps_1"); err == nil {
		t.Fatal("Reboot: want error on result 0, got nil")
	}
}

func TestReinstallOS(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID      string `json:"billingId"`
		DistributiveID int    `json:"distributiveId"`
		SaveDisk       bool   `json:"save_disk"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	if err := c.VPS.ReinstallOS(context.Background(), "login_vps_1", 42, true); err != nil {
		t.Fatalf("ReinstallOS: %v", err)
	}
	if gotMethod != "reinstallOs" || gotParams.BillingID != "login_vps_1" ||
		gotParams.DistributiveID != 42 || !gotParams.SaveDisk {
		t.Errorf("method/params = %q/%+v, want reinstallOs / login_vps_1,42,save_disk=true", gotMethod, gotParams)
	}
}

func TestReinstallOSFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0}`))
	})
	if err := c.VPS.ReinstallOS(context.Background(), "login_vps_1", 42, false); err == nil {
		t.Fatal("ReinstallOS: want error on result 0, got nil")
	}
}

func TestCopy(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		VPSPlanID int    `json:"vpsPlanId"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"billingId":"login_vps_2"}}`))
	})
	if _, err := c.VPS.Copy(context.Background(), "login_vps_1", 7); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if gotMethod != "copy" || gotParams.BillingID != "login_vps_1" || gotParams.VPSPlanID != 7 {
		t.Errorf("method/params = %q/%+v, want copy / login_vps_1,7", gotMethod, gotParams)
	}
}

func TestLogs(t *testing.T) {
	var gotMethod string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":[{"type":"create","status":"done","started_at":"2026-07-10 10:00:00","ended_at":"2026-07-10 10:03:00"}]}`))
	})
	logs, err := c.VPS.Logs(context.Background(), "login_vps_1")
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if gotMethod != "logs" {
		t.Errorf("method = %q, want logs", gotMethod)
	}
	if len(logs) != 1 || logs[0].Type != "create" || logs[0].Status != "done" {
		t.Errorf("logs = %+v, want one create/done entry", logs)
	}
}

func TestGetCurrentAction(t *testing.T) {
	var gotMethod, gotBilling string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				BillingID string `json:"billingId"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotBilling = req.Method, req.Params.BillingID
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"start"}`))
	})
	got, err := c.VPS.GetCurrentAction(context.Background(), "login_vps_1")
	if err != nil {
		t.Fatalf("GetCurrentAction: %v", err)
	}
	if gotMethod != "getCurrentAction" || gotBilling != "login_vps_1" {
		t.Errorf("method/billing = %q/%q, want getCurrentAction/login_vps_1", gotMethod, gotBilling)
	}
	if got != "start" {
		t.Errorf("action = %q, want start", got)
	}
}

func TestCreateEnable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result string
		want   bool
	}{
		{"available", `1`, true},
		{"available-quoted", `"1"`, true}, // API may quote numeric fields (FlexInt)
		{"unavailable", `0`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod = req.Method
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + tc.result + `}`))
			})
			got, err := c.VPS.CreateEnable(context.Background())
			if err != nil {
				t.Fatalf("CreateEnable: %v", err)
			}
			if gotMethod != "createEnable" {
				t.Errorf("method = %q, want createEnable", gotMethod)
			}
			if got != tc.want {
				t.Errorf("CreateEnable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCreateFirst(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		DistributiveID  int    `json:"distributiveId"`
		VPSPlanID       int    `json:"vpsPlanId"`
		Period          int    `json:"period"`
		StartTestPeriod bool   `json:"startTestPeriod"`
		Alias           string `json:"alias"`
		ProtectedIPs    []int  `json:"protectedIps"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"login_vps_2"}`))
	})
	raw, err := c.VPS.CreateFirst(context.Background(), CreateFirstRequest{
		DistributiveID:  32,
		VPSPlanID:       4,
		Period:          12,
		StartTestPeriod: true,
		Alias:           "trialVPS",
		ProtectedIPs:    []int{1, 2},
	})
	if err != nil {
		t.Fatalf("CreateFirst: %v", err)
	}
	if gotMethod != "createFirst" {
		t.Errorf("method = %q, want createFirst", gotMethod)
	}
	if gotParams.DistributiveID != 32 || gotParams.VPSPlanID != 4 || gotParams.Period != 12 ||
		!gotParams.StartTestPeriod || gotParams.Alias != "trialVPS" || len(gotParams.ProtectedIPs) != 2 {
		t.Errorf("params = %+v, want distr=32 plan=4 period=12 test=true alias=trialVPS ips=[1 2]", gotParams)
	}
	if string(raw) != `"login_vps_2"` {
		t.Errorf("raw result = %s, want \"login_vps_2\"", raw)
	}
}

func TestCreateFirstOmitsZeroValues(t *testing.T) {
	// Only the two required fields set — the optional params must not appear.
	var gotParams map[string]any
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotParams = req.Params
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"login_vps_2"}`))
	})
	if _, err := c.VPS.CreateFirst(context.Background(), CreateFirstRequest{DistributiveID: 32, VPSPlanID: 4}); err != nil {
		t.Fatalf("CreateFirst: %v", err)
	}
	for _, k := range []string{"datacenter", "alias", "period", "startTestPeriod", "ipCount", "protectedIps"} {
		if _, present := gotParams[k]; present {
			t.Errorf("optional param %q sent despite being zero-valued", k)
		}
	}
}

func TestRemoveFirst(t *testing.T) {
	var gotMethod string
	var gotParams json.RawMessage
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotParams = req.Method, req.Params
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	raw, err := c.VPS.RemoveFirst(context.Background())
	if err != nil {
		t.Fatalf("RemoveFirst: %v", err)
	}
	if gotMethod != "removeFirst" {
		t.Errorf("method = %q, want removeFirst", gotMethod)
	}
	// removeFirst takes no params; the client sends an empty object.
	if s := string(gotParams); s != "{}" && s != "" && s != "null" {
		t.Errorf("params = %s, want no parameters", s)
	}
	if string(raw) != "1" {
		t.Errorf("raw result = %s, want 1", raw)
	}
}

func TestLoad(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		Type      string `json:"type"`
		From      string `json:"from"`
		To        string `json:"to"`
		Width     int    `json:"width"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"mimetype":"image/png;base64","metadata":[],"content":"aGVsbG8="}}`))
	})
	g, err := c.VPS.Load(context.Background(), "login_vps_1", LoadCPU, "08-03-2023", "15-03-2023", 640)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotMethod != "load" || gotParams.BillingID != "login_vps_1" || gotParams.Type != "cpu" ||
		gotParams.From != "08-03-2023" || gotParams.To != "15-03-2023" || gotParams.Width != 640 {
		t.Errorf("method/params = %q/%+v, want load / login_vps_1,cpu,08-03-2023,15-03-2023,640", gotMethod, gotParams)
	}
	if g.MIMEType != "image/png;base64" || g.Content != "aGVsbG8=" {
		t.Errorf("graph = %+v, want png/base64 content", g)
	}
}

func TestIsRunning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result string
		want   bool
	}{
		{"running-number", `1`, true},
		{"running-quoted", `"1"`, true}, // API may quote numeric fields (FlexInt)
		{"stopped", `0`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod = req.Method
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + tc.result + `}`))
			})
			got, err := c.VPS.IsRunning(context.Background(), "login_vps_1")
			if err != nil {
				t.Fatalf("IsRunning: %v", err)
			}
			if gotMethod != "isRunning" {
				t.Errorf("method = %q, want isRunning", gotMethod)
			}
			if got != tc.want {
				t.Errorf("IsRunning = %v, want %v", got, tc.want)
			}
		})
	}
}
