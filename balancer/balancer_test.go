package balancer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestBalancerList(t *testing.T) {
	// index wraps the balancers under "ips"; numeric fields arrive polymorphic.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":{"ips":[{
			"billingId":"dyasyuc578_balancer_1","name":"LB 1","type":"leastconn",
			"plan_id":"4298","plan_name":"Standard","price":375,"active":true,
			"removeAllowed":false,"blockUi":false,"currentAction":null,
			"tsCreate":"2025-09-25 10:53:24","ipBalancer":"192.168.122.66","datacenter":1,
			"healthCheck":false,"proxyProto":false,"keepalive":false,"saveSession":false,
			"rules":[{"protocolBalancer":"https","portBalancer":"443","protocolServer":"https","portServer":"443"}],
			"servers":[{"ip":"1.1.1.1","vpsName":null}]
		}]}}`))
	})
	list, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	b := list[0]
	if b.BillingID != "dyasyuc578_balancer_1" || b.Type != "leastconn" || b.PlanID != 4298 || b.Price != 375 {
		t.Errorf("balancer = %+v, want billingId/leastconn/4298/375", b)
	}
	if b.Datacenter != 1 || !b.Active || b.RemoveAllowed {
		t.Errorf("flags = %+v, want datacenter 1 / active true / removeAllowed false", b)
	}
	if len(b.Rules) != 1 || b.Rules[0].PortBalancer != "443" || b.Rules[0].ProtocolServer != "https" {
		t.Errorf("rules = %+v, want one https:443", b.Rules)
	}
	if len(b.Servers) != 1 || b.Servers[0].IP != "1.1.1.1" || b.Servers[0].VPSName != "" {
		t.Errorf("servers = %+v, want one 1.1.1.1", b.Servers)
	}
}

// TestBalancerListEmpty reconciles the dual index shape against the live API: an
// account with no balancers answers with a bare empty array (not {"ips":[]}),
// which must decode to an empty list rather than a type error.
func TestBalancerListEmpty(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	list, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("list = %+v, want empty for an account with no balancers", list)
	}
}

func TestBalancerIsCreateEnable(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{`{"result":1}`, true},
		{`{"result":0}`, false},
	} {
		var gotMethod string
		s := serve(t, func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Method string `json:"method"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			gotMethod = req.Method
			_, _ = w.Write([]byte(tc.body))
		})
		got, err := s.IsCreateEnable(context.Background())
		if err != nil {
			t.Fatalf("IsCreateEnable(%s): %v", tc.body, err)
		}
		if gotMethod != "isCreateEnable" {
			t.Errorf("method = %q, want isCreateEnable", gotMethod)
		}
		if got != tc.want {
			t.Errorf("IsCreateEnable(%s) = %v, want %v", tc.body, got, tc.want)
		}
	}
}

func TestBalancerAvailableConfig(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":{
			"plans":[{"id":"4298","tag":"balancer_2","title":"Standard","price":"375.0000"}],
			"protocols":[{"id":"3","name":"HTTPS","restrictions":["HTTP","HTTPS"]}],
			"descriptions":[{"service_id":"balancer_2","service_plan_id":"4298","description":"desc"}]
		}}`))
	})
	cfg, err := s.AvailableConfig(context.Background())
	if err != nil {
		t.Fatalf("AvailableConfig: %v", err)
	}
	if gotMethod != "getAvailableConfig" {
		t.Errorf("method = %q, want getAvailableConfig", gotMethod)
	}
	if len(cfg.Plans) != 1 || cfg.Plans[0].ID != "4298" || cfg.Plans[0].Price != 375 {
		t.Errorf("plans = %+v, want one 4298/375", cfg.Plans)
	}
	if len(cfg.Protocols) != 1 || cfg.Protocols[0].Name != "HTTPS" || len(cfg.Protocols[0].Restrictions) != 2 {
		t.Errorf("protocols = %+v, want one HTTPS with 2 restrictions", cfg.Protocols)
	}
	if len(cfg.Descriptions) != 1 || cfg.Descriptions[0].ServiceID != "balancer_2" {
		t.Errorf("descriptions = %+v, want one balancer_2", cfg.Descriptions)
	}
}

func TestBalancerCreate(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Datacenter   int      `json:"datacenter"`
		Type         string   `json:"type"`
		Servers      []Server `json:"servers"`
		Rules        []Rule   `json:"rules"`
		PlanID       int      `json:"planId"`
		HealthCheck  bool     `json:"healthCheck"`
		SaveSession  bool     `json:"saveSession"`
		Alias        string   `json:"alias"`
		IsFirstOrder bool     `json:"isFirstOrder"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	err := s.Create(context.Background(), CreateOptions{
		Datacenter:   1,
		Type:         "leastconn",
		Servers:      []Server{{IP: "1.1.1.1"}},
		Rules:        []Rule{{ProtocolBalancer: "https", PortBalancer: "443", ProtocolServer: "https", PortServer: "443"}},
		PlanID:       4298,
		SaveSession:  true,
		Alias:        "LB 3",
		IsFirstOrder: false,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotMethod != "create" {
		t.Errorf("method = %q, want create", gotMethod)
	}
	if gotParams.Datacenter != 1 || gotParams.Type != "leastconn" || gotParams.PlanID != 4298 {
		t.Errorf("params = %+v, want datacenter 1 / leastconn / plan 4298", gotParams)
	}
	if !gotParams.SaveSession || gotParams.Alias != "LB 3" {
		t.Errorf("optional params = %+v, want saveSession true / alias LB 3", gotParams)
	}
	if len(gotParams.Servers) != 1 || gotParams.Servers[0].IP != "1.1.1.1" {
		t.Errorf("servers = %+v, want one 1.1.1.1", gotParams.Servers)
	}
	if len(gotParams.Rules) != 1 || gotParams.Rules[0].PortServer != "443" {
		t.Errorf("rules = %+v, want one :443", gotParams.Rules)
	}
}

func TestBalancerCreateAliasOmittedWhenEmpty(t *testing.T) {
	var hasAlias bool
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_, hasAlias = req.Params["alias"]
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Create(context.Background(), CreateOptions{Type: "leastconn"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if hasAlias {
		t.Errorf("params carried an empty alias key, want it omitted")
	}
}

func TestBalancerEdit(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID  string `json:"billingId"`
		Type       string `json:"type"`
		ProxyProto bool   `json:"proxyProto"`
		Keepalive  bool   `json:"keepalive"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	err := s.Edit(context.Background(), EditOptions{
		BillingID:  "dyasyuc578_balancer_1",
		Type:       "roundrobin",
		Servers:    []Server{{IP: "2.2.2.2", Weight: 3}},
		Rules:      []Rule{{ProtocolBalancer: "http", PortBalancer: "80", ProtocolServer: "http", PortServer: "80"}},
		ProxyProto: true,
		Keepalive:  true,
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if gotMethod != "edit" {
		t.Errorf("method = %q, want edit", gotMethod)
	}
	if gotParams.BillingID != "dyasyuc578_balancer_1" || gotParams.Type != "roundrobin" {
		t.Errorf("params = %+v, want billingId / roundrobin", gotParams)
	}
	if !gotParams.ProxyProto || !gotParams.Keepalive {
		t.Errorf("toggles = %+v, want proxyProto & keepalive true", gotParams)
	}
}

func TestBalancerRemove(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Remove(context.Background(), "dyasyuc578_balancer_1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if gotMethod != "remove" || gotBillingID != "dyasyuc578_balancer_1" {
		t.Errorf("method/billingId = %q/%q, want remove/dyasyuc578_balancer_1", gotMethod, gotBillingID)
	}
}

func TestBalancerSentinelFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.Remove(context.Background(), "x"); err == nil {
		t.Error("Remove with result 0: got nil error, want failure")
	}
}

// serve spins up a mock JSON-RPC server for h and returns a balancer.Service
// backed by a transport pointed at it.
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
