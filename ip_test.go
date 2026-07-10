package sweb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestAddLocal(t *testing.T) {
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
	if err := c.IP.AddLocal(context.Background(), "login_vps_1"); err != nil {
		t.Fatalf("AddLocal: %v", err)
	}
	if gotMethod != "addLocal" || gotBilling != "login_vps_1" {
		t.Errorf("method/billing = %q/%q, want addLocal/login_vps_1", gotMethod, gotBilling)
	}
}

func TestRemoveLocal(t *testing.T) {
	var gotMethod string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	if err := c.IP.RemoveLocal(context.Background(), "login_vps_1"); err != nil {
		t.Fatalf("RemoveLocal: %v", err)
	}
	if gotMethod != "removeLocal" {
		t.Errorf("method = %q, want removeLocal", gotMethod)
	}
}

func TestAddLocalFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0}`))
	})
	if err := c.IP.AddLocal(context.Background(), "login_vps_1"); err == nil {
		t.Fatal("AddLocal: want error on result 0, got nil")
	}
}

func TestIPInfoLocalIP(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		// Real attached shape: local_ip and ips come as BARE OBJECTS (not arrays) when
		// populated; a public IP has a FRACTIONAL price (142.06, money-as-float).
		_, _ = w.Write([]byte(`{"result":{"ips":{"ip":"77.222.43.225","gateway":"77.222.43.1","netmask":"77.222.43.0/24","datacenter":1,"ptr":"x","price":142.06},"protected_ips":[],"local_ip":{"ip":"10.0.0.24","mac":"00:16:3e:aa:bb:cc","mask":"10.0.0.0/27"},"vps":{"billingId":"login_vps_1","currentAction":null,"isEmpty":"0","ordered_ip_count":"2"}}}`))
	})
	info, err := c.IP.Info(context.Background(), "login_vps_1")
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if len(info.LocalIP) != 1 || info.LocalIP[0].IP != "10.0.0.24" || info.LocalIP[0].Mask != "10.0.0.0/27" {
		t.Errorf("local_ip = %+v, want one 10.0.0.24 /27", info.LocalIP)
	}
	if len(info.IPs) != 1 || info.IPs[0].Price < 142 || info.IPs[0].Price > 143 {
		t.Errorf("ips[0].price = %v, want ~142.06 (fractional, FlexFloat)", info.IPs)
	}
	if info.VPS.OrderedIPCount != 2 {
		t.Errorf("ordered_ip_count = %d, want 2", int64(info.VPS.OrderedIPCount))
	}
}

func TestAddIP(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		Number    int    `json:"number"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"ip":"203.0.113.7"}}`))
	})
	if _, err := c.IP.Add(context.Background(), "login_vps_1", 2); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if gotMethod != "add" || gotParams.BillingID != "login_vps_1" || gotParams.Number != 2 {
		t.Errorf("method/params = %q/%+v, want add / login_vps_1,2", gotMethod, gotParams)
	}
}

func TestRemoveIP(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		IP        string `json:"ip"`
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
	if err := c.IP.Remove(context.Background(), "login_vps_1", "203.0.113.7"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if gotMethod != "remove" || gotParams.IP != "203.0.113.7" {
		t.Errorf("method/ip = %q/%q, want remove/203.0.113.7", gotMethod, gotParams.IP)
	}
}

func TestMoveIP(t *testing.T) {
	// Attach sends billingId; detach (empty) sends null.
	for _, tc := range []struct {
		name      string
		billingID string
		wantNull  bool
	}{
		{"attach", "login_vps_2", false},
		{"detach", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rawParams json.RawMessage
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
					Params json.RawMessage
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				rawParams = req.Params
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
			})
			if err := c.IP.Move(context.Background(), "203.0.113.7", tc.billingID); err != nil {
				t.Fatalf("Move: %v", err)
			}
			var p struct {
				BillingID *string `json:"billingId"`
			}
			_ = json.Unmarshal(rawParams, &p)
			if tc.wantNull && p.BillingID != nil {
				t.Errorf("detach: billingId = %v, want null", *p.BillingID)
			}
			if !tc.wantNull && (p.BillingID == nil || *p.BillingID != tc.billingID) {
				t.Errorf("attach: billingId = %v, want %q", p.BillingID, tc.billingID)
			}
		})
	}
}

func TestEditPtr(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		IP  string `json:"ip"`
		PTR string `json:"ptr"`
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
	if err := c.IP.EditPtr(context.Background(), "203.0.113.7", "host.example.com"); err != nil {
		t.Fatalf("EditPtr: %v", err)
	}
	if gotMethod != "editPtr" || gotParams.IP != "203.0.113.7" || gotParams.PTR != "host.example.com" {
		t.Errorf("method/params = %q/%+v, want editPtr / ip,ptr", gotMethod, gotParams)
	}
}

func TestGetPtr(t *testing.T) {
	// Tolerate both a bare-string result and a {"ptr": …} object.
	for _, tc := range []struct{ name, result string }{
		{"string", `"host.example.com"`},
		{"object", `{"ptr":"host.example.com"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + tc.result + `}`))
			})
			ptr, err := c.IP.GetPtr(context.Background(), "203.0.113.7")
			if err != nil {
				t.Fatalf("GetPtr: %v", err)
			}
			if ptr != "host.example.com" {
				t.Errorf("ptr = %q, want host.example.com", ptr)
			}
		})
	}
}

func TestWaitForLocalIP(t *testing.T) {
	var calls int
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 2 {
			_, _ = w.Write([]byte(`{"result":{"local_ip":[],"vps":{"billingId":"login_vps_1"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"local_ip":[{"ip":"10.0.0.24","mac":"x","mask":"10.0.0.0/27"}],"vps":{"billingId":"login_vps_1"}}}`))
	})
	lip, err := c.IP.WaitForLocalIP(context.Background(), "login_vps_1", time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForLocalIP: %v", err)
	}
	if lip.IP != "10.0.0.24" {
		t.Errorf("local ip = %q, want 10.0.0.24", lip.IP)
	}
	if calls < 2 {
		t.Errorf("calls = %d, want >= 2 (polled)", calls)
	}
}
