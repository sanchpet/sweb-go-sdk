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
		// local_ip populated; ordered_ip_count as a quoted string (FlexInt path).
		_, _ = w.Write([]byte(`{"result":{"ips":[],"protected_ips":[],"local_ip":[{"ip":"10.0.0.24","mac":"00:16:3e:aa:bb:cc","mask":"10.0.0.0/27"}],"vps":{"billingId":"login_vps_1","currentAction":null,"isEmpty":"0","ordered_ip_count":"2"}}}`))
	})
	info, err := c.IP.Info(context.Background(), "login_vps_1")
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if len(info.LocalIP) != 1 || info.LocalIP[0].IP != "10.0.0.24" || info.LocalIP[0].Mask != "10.0.0.0/27" {
		t.Errorf("local_ip = %+v, want one 10.0.0.24 /27", info.LocalIP)
	}
	if info.VPS.OrderedIPCount != 2 {
		t.Errorf("ordered_ip_count = %d, want 2", int64(info.VPS.OrderedIPCount))
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
