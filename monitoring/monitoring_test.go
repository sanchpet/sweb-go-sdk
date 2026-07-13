package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

// serve spins up a mock JSON-RPC server for h and returns a Service backed by a
// transport pointed at it.
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

// capture records the method and raw params of a single JSON-RPC call and
// replies with the given result JSON (the bare result value, wrapped here in the
// envelope). It returns pointers the test can assert against after the call.
func capture(t *testing.T, result string) (s *Service, method *string, params *json.RawMessage) {
	t.Helper()
	var gotMethod string
	var gotParams json.RawMessage
	s = serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		gotParams = req.Params
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + result + `}`))
	})
	return s, &gotMethod, &gotParams
}

func TestMonitoringPlans(t *testing.T) {
	s, method, _ := capture(t, `[{"id":1,"name":"Базовый","checks":1,"sms":6,"price":30},`+
		`{"id":2,"name":"Стандартный","checks":10,"sms":30,"price":150.5}]`)
	plans, err := s.Plans(context.Background())
	if err != nil {
		t.Fatalf("Plans: %v", err)
	}
	if *method != "plans" {
		t.Errorf("method = %q, want plans", *method)
	}
	if len(plans) != 2 {
		t.Fatalf("got %d plans, want 2", len(plans))
	}
	if plans[0].ID != 1 || plans[0].Name != "Базовый" || plans[0].Checks != 1 || plans[0].SMS != 6 || plans[0].Price != 30 {
		t.Errorf("plans[0] = %+v", plans[0])
	}
	if plans[1].Price != 150.5 {
		t.Errorf("plans[1].Price = %v, want 150.5 (flex.Float)", plans[1].Price)
	}
}

func TestMonitoringTariffActions(t *testing.T) {
	cases := []struct {
		name   string
		call   func(*Service) error
		method string
	}{
		{"Enable", func(s *Service) error { return s.Enable(context.Background(), 2) }, "enable"},
		{"Disable", func(s *Service) error { return s.Disable(context.Background(), 2) }, "disable"},
		{"Change", func(s *Service) error { return s.Change(context.Background(), 3) }, "change"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, method, params := capture(t, `1`)
			if err := tc.call(s); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if *method != tc.method {
				t.Errorf("method = %q, want %q", *method, tc.method)
			}
			var p struct {
				ID int `json:"id"`
			}
			_ = json.Unmarshal(*params, &p)
			if p.ID == 0 {
				t.Errorf("id param not sent: %s", *params)
			}
		})
	}
}

func TestMonitoringTariffActionFailure(t *testing.T) {
	s, _, _ := capture(t, `0`)
	if err := s.Enable(context.Background(), 1); err == nil {
		t.Fatal("Enable: want error on result 0, got nil")
	}
}
