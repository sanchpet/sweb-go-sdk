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
