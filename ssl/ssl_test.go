package ssl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestSSLList(t *testing.T) {
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
		// index wraps {list, filterInfo} in a one-element array.
		_, _ = w.Write([]byte(`{"result":[{"filterInfo":{"orderDirect":"desc","orderField":"id","page":1,"perPage":20,"totalCount":8},"list":[{"id":622297,"status":"Заказ в обработке","ip":null,"domain":"www.kommersant.ru","name":"GlobalSign AlphaSSL","valid_to":null,"prolong_available":0,"autoprolong":true,"autoprolongAllowed":false,"isFree":true,"autoprolongAddition":{"full_name":"GlobalSign AlphaSSL на 1 год","id":"35","name":"GlobalSign AlphaSSL","price":1900}}]}]}`))
	})
	got, err := s.List(context.Background(), &ListOptions{Page: 1, PerPage: 20, OrderField: "id", OrderDirect: "desc"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if gotParams.Page != 1 || gotParams.PerPage != 20 || gotParams.OrderField != "id" || gotParams.OrderDirect != "desc" {
		t.Errorf("params = %+v, want page=1 perPage=20 id/desc", gotParams)
	}
	if got == nil || len(got.List) != 1 {
		t.Fatalf("list = %+v, want one certificate", got)
	}
	cert := got.List[0]
	if cert.ID != 622297 || cert.Domain != "www.kommersant.ru" || !cert.Autoprolong || cert.AutoprolongAllowed {
		t.Errorf("cert = %+v, want id 622297, autoprolong on, allowed off", cert)
	}
	if cert.AutoprolongAddition == nil || cert.AutoprolongAddition.ID != "35" || cert.AutoprolongAddition.Price != 1900 {
		t.Errorf("addition = %+v, want id 35 price 1900", cert.AutoprolongAddition)
	}
	if got.FilterInfo.TotalCount != 8 || got.FilterInfo.Page != 1 {
		t.Errorf("filterInfo = %+v, want totalCount 8 page 1", got.FilterInfo)
	}
}

func TestSSLListEmpty(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	got, err := s.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got != nil {
		t.Errorf("list = %+v, want nil for empty result", got)
	}
}

func TestSSLOrderList(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":[{"id":"12","name":"GlobalSign AlphaSSL","type":"dv","advantage_text":"Подтверждает домен","advantages":["a","b"],"persons":["u","f","ip"],"periods":["12"],"prices":{"12":"1900.00"},"prices_old":null,"autoprolongAddition":null}]}`))
	})
	got, err := s.OrderList(context.Background())
	if err != nil {
		t.Fatalf("OrderList: %v", err)
	}
	if gotMethod != "getOrderList" {
		t.Errorf("method = %q, want getOrderList", gotMethod)
	}
	if len(got) != 1 {
		t.Fatalf("orderList = %+v, want one option", got)
	}
	o := got[0]
	if o.ID != "12" || o.Type != "dv" || len(o.Persons) != 3 || len(o.Periods) != 1 {
		t.Errorf("option = %+v, want id 12 dv 3 persons 1 period", o)
	}
	if o.Prices["12"] != 1900.00 {
		t.Errorf("price[12] = %v, want 1900.00", o.Prices["12"])
	}
	if o.PricesOld != nil {
		t.Errorf("pricesOld = %v, want nil", o.PricesOld)
	}
}

func TestSSLDownload(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		ID       int    `json:"id"`
		Password string `json:"password"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":[{"mimetype":"application/zip;base64","metadata":[],"content":"UEsDBBQ=","name":"test.ru.2023.348533.zip"}]}`))
	})
	got, err := s.Download(context.Background(), 348533, "secret")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if gotMethod != "download" || gotParams.ID != 348533 || gotParams.Password != "secret" {
		t.Errorf("method/params = %q/%+v, want download / id 348533 pw secret", gotMethod, gotParams)
	}
	if len(got) != 1 || got[0].Name != "test.ru.2023.348533.zip" || got[0].Mimetype != "application/zip;base64" || got[0].Content != "UEsDBBQ=" {
		t.Errorf("files = %+v, want one zip descriptor", got)
	}
}

func TestSSLProlongInfo(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		CertificateID int `json:"certificateId"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":[{"currentCertificateId":348533,"title":"GlobalSign AlphaSSL","isFreeCertificate":false,"orderData":{"domain":"test.ru","sub_domain":null,"mailbox":"admin@test.ru","person_id":"360120","company_link":"","auth_type":null,"is_machine":"N","nic_customer_id":"777","nic_order_id":null},"prices":{"12":"1900.00"},"ids":{"12":"35"}}]}`))
	})
	got, err := s.ProlongInfo(context.Background(), 348533)
	if err != nil {
		t.Fatalf("ProlongInfo: %v", err)
	}
	if gotMethod != "getProlongInfo" || gotParams.CertificateID != 348533 {
		t.Errorf("method/params = %q/%+v, want getProlongInfo / 348533", gotMethod, gotParams)
	}
	if got == nil || got.CurrentCertificateID != 348533 || got.Title != "GlobalSign AlphaSSL" || got.IsFreeCertificate {
		t.Fatalf("prolongInfo = %+v, want id 348533 AlphaSSL not-free", got)
	}
	if got.OrderData.Domain != "test.ru" || got.OrderData.PersonID != "360120" || got.OrderData.IsMachine != "N" {
		t.Errorf("orderData = %+v, want test.ru / 360120 / N", got.OrderData)
	}
	if got.Prices["12"] != 1900.00 || got.IDs["12"] != "35" {
		t.Errorf("prices/ids = %v/%v, want 1900.00 / 35", got.Prices["12"], got.IDs["12"])
	}
}

func TestSSLProlongInfoEmpty(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	got, err := s.ProlongInfo(context.Background(), 1)
	if err != nil {
		t.Fatalf("ProlongInfo: %v", err)
	}
	if got != nil {
		t.Errorf("prolongInfo = %+v, want nil for empty result", got)
	}
}

func TestSSLEditAutoprolong(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		want    int
	}{
		{"enable", true, 1},
		{"disable", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			var gotParams struct {
				CertificateID int `json:"certificateId"`
				Autoprolong   int `json:"autoprolong"`
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
			raw, err := s.EditAutoprolong(context.Background(), 348533, tc.enabled)
			if err != nil {
				t.Fatalf("EditAutoprolong: %v", err)
			}
			if gotMethod != "editAutoprolong" || gotParams.CertificateID != 348533 || gotParams.Autoprolong != tc.want {
				t.Errorf("method/params = %q/%+v, want editAutoprolong / 348533,%d", gotMethod, gotParams, tc.want)
			}
			if string(raw) != "1" {
				t.Errorf("raw = %s, want 1", raw)
			}
		})
	}
}

func TestSSLRemoveCertificate(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		CertificateID int `json:"certificateId"`
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
	raw, err := s.RemoveCertificate(context.Background(), 348533)
	if err != nil {
		t.Fatalf("RemoveCertificate: %v", err)
	}
	if gotMethod != "removeCertificate" || gotParams.CertificateID != 348533 {
		t.Errorf("method/params = %q/%+v, want removeCertificate / 348533", gotMethod, gotParams)
	}
	if string(raw) != "1" {
		t.Errorf("raw = %s, want 1", raw)
	}
}

func TestSSLOrderSubmit(t *testing.T) {
	var gotMethod string
	var gotParams orderSubmitParams
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		// tri-state: 2 = queued for manual processing.
		_, _ = w.Write([]byte(`{"result":2}`))
	})
	raw, err := s.OrderSubmit(context.Background(), "test.ru", 348533, "admin@test.ru", &OrderSubmitOptions{
		PersonID:    360129,
		Autoprolong: true,
	})
	if err != nil {
		t.Fatalf("OrderSubmit: %v", err)
	}
	if gotMethod != "orderSubmit" {
		t.Errorf("method = %q, want orderSubmit", gotMethod)
	}
	if gotParams.Domain != "test.ru" || gotParams.CertificateID != 348533 || gotParams.CertificateConfirmMail != "admin@test.ru" {
		t.Errorf("params = %+v, want test.ru / 348533 / admin@test.ru", gotParams)
	}
	if gotParams.PersonID != 360129 || gotParams.Autoprolong != 1 {
		t.Errorf("params = %+v, want personId 360129, autoprolong 1", gotParams)
	}
	if string(raw) != "2" {
		t.Errorf("raw = %s, want 2", raw)
	}
}

func TestSSLOrderSubmitDefaults(t *testing.T) {
	// Nil opts must not emit the optional fields (omitempty) — subdomain absent.
	var raw json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		raw = req.Params
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if _, err := s.OrderSubmit(context.Background(), "test.ru", 12, "admin@test.ru", nil); err != nil {
		t.Fatalf("OrderSubmit: %v", err)
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	for _, k := range []string{"personId", "companyPageLink", "subdomain", "oldCertificateId", "fromProlongation"} {
		if _, ok := m[k]; ok {
			t.Errorf("param %q present, want omitted for nil opts", k)
		}
	}
	if _, ok := m["autoprolong"]; !ok {
		t.Errorf("autoprolong absent, want 0 present (always sent)")
	}
}

// serve spins up a mock JSON-RPC server for h and returns an ssl.Service backed
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
