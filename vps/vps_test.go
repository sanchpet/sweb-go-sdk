package vps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestPowerActions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		call   func(*Service) error
		method string
	}{
		{"PowerOn", func(s *Service) error { return s.PowerOn(context.Background(), "login_vps_1") }, "powerOn"},
		{"PowerOff", func(s *Service) error { return s.PowerOff(context.Background(), "login_vps_1") }, "powerOff"},
		{"Reboot", func(s *Service) error { return s.Reboot(context.Background(), "login_vps_1") }, "reboot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotBilling string
			s := serve(t, func(w http.ResponseWriter, r *http.Request) {
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
			if err := tc.call(s); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method || gotBilling != "login_vps_1" {
				t.Errorf("method/billing = %q/%q, want %s/login_vps_1", gotMethod, gotBilling, tc.method)
			}
		})
	}
}

func TestPowerActionFailure(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0}`))
	})
	if err := s.Reboot(context.Background(), "login_vps_1"); err == nil {
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
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	if err := s.ReinstallOS(context.Background(), "login_vps_1", 42, true); err != nil {
		t.Fatalf("ReinstallOS: %v", err)
	}
	if gotMethod != "reinstallOs" || gotParams.BillingID != "login_vps_1" ||
		gotParams.DistributiveID != 42 || !gotParams.SaveDisk {
		t.Errorf("method/params = %q/%+v, want reinstallOs / login_vps_1,42,save_disk=true", gotMethod, gotParams)
	}
}

func TestReinstallOSFailure(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0}`))
	})
	if err := s.ReinstallOS(context.Background(), "login_vps_1", 42, false); err == nil {
		t.Fatal("ReinstallOS: want error on result 0, got nil")
	}
}

func TestCopy(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		VPSPlanID int    `json:"vpsPlanId"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"billingId":"login_vps_2"}}`))
	})
	if _, err := s.Copy(context.Background(), "login_vps_1", 7); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if gotMethod != "copy" || gotParams.BillingID != "login_vps_1" || gotParams.VPSPlanID != 7 {
		t.Errorf("method/params = %q/%+v, want copy / login_vps_1,7", gotMethod, gotParams)
	}
}

func TestLogs(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":[{"type":"create","status":"done","started_at":"2026-07-10 10:00:00","ended_at":"2026-07-10 10:03:00"}]}`))
	})
	logs, err := s.Logs(context.Background(), "login_vps_1")
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
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
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
	got, err := s.GetCurrentAction(context.Background(), "login_vps_1")
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
		{"available-quoted", `"1"`, true}, // API may quote numeric fields (flex.Int)
		{"unavailable", `0`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			s := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod = req.Method
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + tc.result + `}`))
			})
			got, err := s.CreateEnable(context.Background())
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
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"login_vps_2"}`))
	})
	raw, err := s.CreateFirst(context.Background(), CreateFirstRequest{
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
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotParams = req.Params
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"login_vps_2"}`))
	})
	if _, err := s.CreateFirst(context.Background(), CreateFirstRequest{DistributiveID: 32, VPSPlanID: 4}); err != nil {
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
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotParams = req.Method, req.Params
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	raw, err := s.RemoveFirst(context.Background())
	if err != nil {
		t.Fatalf("RemoveFirst: %v", err)
	}
	if gotMethod != "removeFirst" {
		t.Errorf("method = %q, want removeFirst", gotMethod)
	}
	// removeFirst takes no params; the client sends an empty object.
	if str := string(gotParams); str != "{}" && str != "" && str != "null" {
		t.Errorf("params = %s, want no parameters", str)
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
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"mimetype":"image/png;base64","metadata":[],"content":"aGVsbG8="}}`))
	})
	g, err := s.Load(context.Background(), "login_vps_1", LoadCPU, "08-03-2023", "15-03-2023", 640)
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
		{"running-quoted", `"1"`, true}, // API may quote numeric fields (flex.Int)
		{"stopped", `0`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			s := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod = req.Method
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + tc.result + `}`))
			})
			got, err := s.IsRunning(context.Background(), "login_vps_1")
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

func TestRemove(t *testing.T) {
	var gotMethod, gotBillingID string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
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

	if _, err := s.Remove(context.Background(), "login_vps_6"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if gotMethod != "remove" {
		t.Errorf("method = %q, want remove", gotMethod)
	}
	if gotBillingID != "login_vps_6" {
		t.Errorf("billingId = %q, want login_vps_6", gotBillingID)
	}
}

func TestRename(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		Alias     string `json:"alias"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
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

	if err := s.Rename(context.Background(), "login_vps_6", "infra-01"); err != nil {
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

func TestChangePlan(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		VPSPlanID int    `json:"planId"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
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

	if err := s.ChangePlan(context.Background(), "login_vps_1", 4); err != nil {
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

func TestChangePlanFailure(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0}`))
	})
	if err := s.ChangePlan(context.Background(), "login_vps_1", 4); err == nil {
		t.Fatal("ChangePlan: want error on result 0, got nil")
	}
}

func TestWaitForIdle(t *testing.T) {
	var calls int
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		action := "" // idle from the 3rd poll on
		switch calls {
		case 1:
			action = "Modify"
		case 2:
			action = "ExtIpAdd"
		}
		_, _ = fmt.Fprintf(w, `{"result":[{"billingId":"login_vps_1","current_action":%q,"is_running":1}]}`, action)
	})

	var phases []string
	node, err := s.WaitForIdle(context.Background(), "login_vps_1", time.Millisecond, func(a string) {
		phases = append(phases, a)
	})
	if err != nil {
		t.Fatalf("WaitForIdle: %v", err)
	}
	if node.CurrentAction != "" {
		t.Errorf("current_action = %q, want empty (idle)", node.CurrentAction)
	}
	// Must poll through the whole action sequence, not stop at the first change.
	if calls < 3 {
		t.Errorf("polled %d times, want >= 3 (Modify → ExtIpAdd → idle)", calls)
	}
	if len(phases) < 3 || phases[0] != "Modify" || phases[1] != "ExtIpAdd" {
		t.Errorf("phases = %v, want [Modify ExtIpAdd ...]", phases)
	}
}

func TestWaitForIdleTimeout(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"billingId":"login_vps_1","current_action":"Modify","is_running":1}]}`))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := s.WaitForIdle(ctx, "login_vps_1", time.Millisecond, nil); err == nil {
		t.Fatal("WaitForIdle: want timeout error, got nil")
	}
}

func TestGetConstructorPlanID(t *testing.T) {
	var gotParams map[string]int
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
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

	id, err := s.GetConstructorPlanID(context.Background(), 2, 6, 15, 1)
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
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
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

	if _, err := s.GetConstructorPlanID(context.Background(), 1, 1, 10, 1); err == nil {
		t.Fatal("want error when the configurator resolves to a sold-out plan")
	}
}

func TestList(t *testing.T) {
	s := fixtureServer(t, "vps_index.json")

	list, err := s.List(context.Background())
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
	// Numeric-ish fields SpaceWeb quotes inconsistently — flex.Int/flex.Float decode
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
	s := fixtureServer(t, "available_config.json")

	cfg, err := s.AvailableConfig(context.Background())
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
	s := fixtureServer(t, "first_order_info.json")

	info, err := s.GetFirstOrderInfo(context.Background())
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

// fixtureServer serves the bytes of testdata/<file> for any request.
func fixtureServer(t *testing.T, file string) *Service {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return serve(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
}

// serve spins up a mock JSON-RPC server for h and returns a vps.Service backed
// by a transport pointed at it.
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
