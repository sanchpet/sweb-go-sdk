package ddg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestDDGList(t *testing.T) {
	// index returns a bare array; ip/expired/blocked are null when the service is
	// not connected, and "" must represent that null.
	var gotMethod string
	var gotParams listParams
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":[
			{"blocked":"2024-06-10","expired":"2024-07-10","fqdn":"testdomen.shop","fqdnReadable":"testdomen.shop","ip":"77.222.44.5","status":"active_blocked"},
			{"blocked":null,"expired":null,"fqdn":"itomen.ru","fqdnReadable":"itomen.ru","ip":null,"status":"disabled"}
		]}`))
	})
	got, err := s.List(context.Background(), &ListOptions{Page: 1, PerPage: 20, OrderField: "fqdn", OrderDirect: "DESC"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if gotParams.Page != 1 || gotParams.PerPage != 20 || gotParams.OrderField != "fqdn" || gotParams.OrderDirect != "DESC" {
		t.Errorf("params = %+v, want page=1 perPage=20 fqdn/DESC", gotParams)
	}
	if len(got) != 2 {
		t.Fatalf("list len = %d, want 2", len(got))
	}
	if got[0].FQDN != "testdomen.shop" || got[0].IP != "77.222.44.5" || got[0].Status != "active_blocked" {
		t.Errorf("domain[0] = %+v, want testdomen.shop/77.222.44.5/active_blocked", got[0])
	}
	if got[0].Blocked != "2024-06-10" || got[0].Expired != "2024-07-10" {
		t.Errorf("domain[0] dates = %q/%q, want 2024-06-10/2024-07-10", got[0].Blocked, got[0].Expired)
	}
	// nulls decode to "".
	if got[1].IP != "" || got[1].Expired != "" || got[1].Blocked != "" || got[1].Status != "disabled" {
		t.Errorf("domain[1] = %+v, want empty ip/expired/blocked and status disabled", got[1])
	}
}

func TestDDGListNilOptions(t *testing.T) {
	// A nil ListOptions must not panic and sends the zero-value paging.
	var gotParams listParams
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	got, err := s.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List(nil): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("list len = %d, want 0", len(got))
	}
	if gotParams.OrderField != "" || gotParams.OrderDirect != "" {
		t.Errorf("params = %+v, want empty order fields omitted", gotParams)
	}
}

func TestDDGCountAllDomains(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":21}`))
	})
	got, err := s.CountAllDomains(context.Background())
	if err != nil {
		t.Fatalf("CountAllDomains: %v", err)
	}
	if gotMethod != "countAllDomains" {
		t.Errorf("method = %q, want countAllDomains", gotMethod)
	}
	if got != 21 {
		t.Errorf("count = %d, want 21", got)
	}
}

func TestDDGEnableInfo(t *testing.T) {
	// price arrives bare (290); ssl is an object when present, null otherwise.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":{
			"domains":[
				{"fqdn":"test1.ru","fqdnReadable":"test1.ru","isOnOurNs":true,"ssl":{"isFilled":true,"isOur":true}},
				{"fqdn":"test2.com","fqdnReadable":"test2.com","isOnOurNs":false,"ssl":null}
			],
			"price":290
		}}`))
	})
	got, err := s.EnableInfo(context.Background())
	if err != nil {
		t.Fatalf("EnableInfo: %v", err)
	}
	if gotMethod != "enableInfo" {
		t.Errorf("method = %q, want enableInfo", gotMethod)
	}
	if got.Price != 290 {
		t.Errorf("price = %v, want 290", got.Price)
	}
	if len(got.Domains) != 2 {
		t.Fatalf("domains len = %d, want 2", len(got.Domains))
	}
	d0 := got.Domains[0]
	if d0.FQDN != "test1.ru" || !d0.IsOnOurNS || d0.SSL == nil || !d0.SSL.IsFilled || !d0.SSL.IsOur {
		t.Errorf("domain[0] = %+v (ssl %+v), want test1.ru/onOurNs/ssl filled&our", d0, d0.SSL)
	}
	if got.Domains[1].SSL != nil {
		t.Errorf("domain[1].SSL = %+v, want nil", got.Domains[1].SSL)
	}
}

func TestDDGGetPrice(t *testing.T) {
	// money is polymorphic: a quoted string must decode through flex.Float.
	for _, tc := range []struct {
		body string
		want float64
	}{
		{`{"result":290}`, 290},
		{`{"result":"290.50"}`, 290.5},
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
		got, err := s.GetPrice(context.Background())
		if err != nil {
			t.Fatalf("GetPrice(%s): %v", tc.body, err)
		}
		if gotMethod != "getPrice" {
			t.Errorf("method = %q, want getPrice", gotMethod)
		}
		if got != tc.want {
			t.Errorf("GetPrice(%s) = %v, want %v", tc.body, got, tc.want)
		}
	}
}

func TestDDGPriceWidget(t *testing.T) {
	// the example quotes both prices as strings; flex.Float must decode them.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":{"current":"290","new":"990"}}`))
	})
	got, err := s.PriceWidget(context.Background())
	if err != nil {
		t.Fatalf("PriceWidget: %v", err)
	}
	if gotMethod != "priceWidget" {
		t.Errorf("method = %q, want priceWidget", gotMethod)
	}
	if got.Current != 290 || got.New != 990 {
		t.Errorf("price = %+v, want current 290 / new 990", got)
	}
}

func TestDDGEnable(t *testing.T) {
	var gotMethod, gotDomain string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				Domain string `json:"domain"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotDomain = req.Method, req.Params.Domain
		_, _ = w.Write([]byte(`{"result":{"fqdn":"test.ru","fqdnReadable":"test.ru","ip":"77.222.44.8","isOnOurNs":false}}`))
	})
	got, err := s.Enable(context.Background(), "test.ru")
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if gotMethod != "enable" || gotDomain != "test.ru" {
		t.Errorf("method/domain = %q/%q, want enable/test.ru", gotMethod, gotDomain)
	}
	if got.FQDN != "test.ru" || got.IP != "77.222.44.8" || got.IsOnOurNS {
		t.Errorf("enable = %+v, want test.ru/77.222.44.8/isOnOurNs false", got)
	}
}

func TestDDGDisable(t *testing.T) {
	var gotMethod, gotDomain string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				Domain string `json:"domain"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotDomain = req.Method, req.Params.Domain
		_, _ = w.Write([]byte(`{"result":{"fqdn":"test.ru","fqdnReadable":"test.ru","expire":"10.07.24","isOnOurNs":false}}`))
	})
	got, err := s.Disable(context.Background(), "test.ru")
	if err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if gotMethod != "disable" || gotDomain != "test.ru" {
		t.Errorf("method/domain = %q/%q, want disable/test.ru", gotMethod, gotDomain)
	}
	if got.FQDN != "test.ru" || got.Expire != "10.07.24" || got.IsOnOurNS {
		t.Errorf("disable = %+v, want test.ru/10.07.24/isOnOurNs false", got)
	}
}

func TestDDGCallError(t *testing.T) {
	// a JSON-RPC error envelope must surface as an error, not a zero value.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"code":-32000,"message":"Доступ запрещен"}}`))
	})
	if _, err := s.Enable(context.Background(), "test.ru"); err == nil {
		t.Error("Enable with error envelope: got nil error, want failure")
	}
}

// serve spins up a mock JSON-RPC server for h and returns a ddg.Service backed by a
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
