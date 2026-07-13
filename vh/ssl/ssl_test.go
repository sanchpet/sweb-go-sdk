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
		// index returns a bare {list, filterInfo} object. id arrives quoted
		// and ip populated ("sni"), exercising flex.Int and the VH-only IP field.
		_, _ = w.Write([]byte(`{"result":{"filterInfo":{"orderDirect":"desc","orderField":"id","page":1,"perPage":20,"totalCount":6},"list":[{"id":"466893","status":"issued","ip":"sni","domain":"mysite.ru","name":"Let's Encrypt","valid_to":"2023-01-23","prolong_available":0,"autoprolong":true,"autoprolongAllowed":false,"autoprolongAddition":{"full_name":"GlobalSign AlphaSSL на 1 год","id":"35","name":"GlobalSign AlphaSSL","price":1900}}]}}`))
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
	if cert.ID != 466893 || cert.Domain != "mysite.ru" || cert.IP != "sni" || !cert.Autoprolong || cert.AutoprolongAllowed {
		t.Errorf("cert = %+v, want id 466893, ip sni, autoprolong on, allowed off", cert)
	}
	if cert.AutoprolongAddition == nil || cert.AutoprolongAddition.ID != "35" || cert.AutoprolongAddition.Price != 1900 {
		t.Errorf("addition = %+v, want id 35 price 1900", cert.AutoprolongAddition)
	}
	if got.FilterInfo.TotalCount != 6 || got.FilterInfo.Page != 1 {
		t.Errorf("filterInfo = %+v, want totalCount 6 page 1", got.FilterInfo)
	}
}

func TestSSLListEmpty(t *testing.T) {
	// No certificates: the bare object carries an empty list and a zero totalCount.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"list":[],"filterInfo":{"page":1,"perPage":20,"orderField":"id","orderDirect":"desc","totalCount":0}}}`))
	})
	got, err := s.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got == nil || len(got.List) != 0 {
		t.Errorf("list = %+v, want empty list for no certificates", got)
	}
	if got.FilterInfo.TotalCount != 0 {
		t.Errorf("totalCount = %v, want 0", got.FilterInfo.TotalCount)
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
		_, _ = w.Write([]byte(`{"result":[{"id":"7","name":"GlobalSign DomainSSL","type":"dv","advantage_text":"Подтверждает домен","advantages":["a","b"],"persons":["u","f","ip"],"periods":["12"],"prices":{"12":"4100.00"},"prices_old":null,"autoprolongAddition":null}]}`))
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
	if o.ID != "7" || o.Type != "dv" || len(o.Persons) != 3 || len(o.Periods) != 1 {
		t.Errorf("option = %+v, want id 7 dv 3 persons 1 period", o)
	}
	if o.Prices["12"] != 4100.00 {
		t.Errorf("price[12] = %v, want 4100.00", o.Prices["12"])
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
		_, _ = w.Write([]byte(`{"result":[{"mimetype":"application/zip;base64","metadata":[],"content":"UEsDBBQ=","name":"my_cert.zip"}]}`))
	})
	got, err := s.Download(context.Background(), 466893, "secret")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if gotMethod != "download" || gotParams.ID != 466893 || gotParams.Password != "secret" {
		t.Errorf("method/params = %q/%+v, want download / id 466893 pw secret", gotMethod, gotParams)
	}
	if len(got) != 1 || got[0].Name != "my_cert.zip" || got[0].Mimetype != "application/zip;base64" || got[0].Content != "UEsDBBQ=" {
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
		_, _ = w.Write([]byte(`{"result":[{"currentCertificateId":348397,"title":"GlobalSign DomainSSL","isFreeCertificate":false,"orderData":{"domain":"testetste.com","sub_domain":null,"mailbox":"admin@dfhbvjd.org.ru","person_id":"359688","company_link":"","auth_type":"dns","is_machine":"N","nic_customer_id":"777","nic_order_id":null},"prices":{"12":"4100.00"},"ids":{"12":"27"}}]}`))
	})
	got, err := s.ProlongInfo(context.Background(), 348397)
	if err != nil {
		t.Fatalf("ProlongInfo: %v", err)
	}
	if gotMethod != "getProlongInfo" || gotParams.CertificateID != 348397 {
		t.Errorf("method/params = %q/%+v, want getProlongInfo / 348397", gotMethod, gotParams)
	}
	if got == nil || got.CurrentCertificateID != 348397 || got.Title != "GlobalSign DomainSSL" || got.IsFreeCertificate {
		t.Fatalf("prolongInfo = %+v, want id 348397 DomainSSL not-free", got)
	}
	if got.OrderData.Domain != "testetste.com" || got.OrderData.PersonID != "359688" || got.OrderData.AuthType != "dns" || got.OrderData.IsMachine != "N" {
		t.Errorf("orderData = %+v, want testetste.com / 359688 / dns / N", got.OrderData)
	}
	if got.Prices["12"] != 4100.00 || got.IDs["12"] != "27" {
		t.Errorf("prices/ids = %v/%v, want 4100.00 / 27", got.Prices["12"], got.IDs["12"])
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
	}{
		{"enable", true},
		{"disable", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			var gotParams struct {
				CertificateID int  `json:"certificateId"`
				Autoprolong   bool `json:"autoprolong"`
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
			raw, err := s.EditAutoprolong(context.Background(), 466893, tc.enabled)
			if err != nil {
				t.Fatalf("EditAutoprolong: %v", err)
			}
			if gotMethod != "editAutoprolong" || gotParams.CertificateID != 466893 || gotParams.Autoprolong != tc.enabled {
				t.Errorf("method/params = %q/%+v, want editAutoprolong / 466893,%v", gotMethod, gotParams, tc.enabled)
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
	raw, err := s.RemoveCertificate(context.Background(), 466893)
	if err != nil {
		t.Fatalf("RemoveCertificate: %v", err)
	}
	if gotMethod != "removeCertificate" || gotParams.CertificateID != 466893 {
		t.Errorf("method/params = %q/%+v, want removeCertificate / 466893", gotMethod, gotParams)
	}
	if string(raw) != "1" {
		t.Errorf("raw = %s, want 1", raw)
	}
}

func TestSSLProlongCertificate(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		CurrentCertificateID int `json:"currentCertificateId"`
		CertificateProlongID int `json:"certificateProlongId"`
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
	raw, err := s.ProlongCertificate(context.Background(), 466893, 987234)
	if err != nil {
		t.Fatalf("ProlongCertificate: %v", err)
	}
	if gotMethod != "prolongCertificate" || gotParams.CurrentCertificateID != 466893 || gotParams.CertificateProlongID != 987234 {
		t.Errorf("method/params = %q/%+v, want prolongCertificate / 466893,987234", gotMethod, gotParams)
	}
	if string(raw) != "1" {
		t.Errorf("raw = %s, want 1", raw)
	}
}

func TestSSLInstallLetsEncrypt(t *testing.T) {
	var gotMethod string
	var gotParams installLetsEncryptParams
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
	raw, err := s.InstallLetsEncrypt(context.Background(), "mysite.ru", false, &InstallLetsEncryptOptions{
		Virtdom:   "poddomen.mysite.ru",
		IP:        "sni",
		Challenge: "dns",
	})
	if err != nil {
		t.Fatalf("InstallLetsEncrypt: %v", err)
	}
	if gotMethod != "installLetsEncrypt" {
		t.Errorf("method = %q, want installLetsEncrypt", gotMethod)
	}
	if gotParams.Domain != "mysite.ru" || gotParams.Wildcard != 0 || gotParams.Virtdom != "poddomen.mysite.ru" || gotParams.IP != "sni" || gotParams.Challenge != "dns" {
		t.Errorf("params = %+v, want mysite.ru / wildcard 0 / poddomen / sni / dns", gotParams)
	}
	if string(raw) != "1" {
		t.Errorf("raw = %s, want 1", raw)
	}
}

func TestSSLInstallLetsEncryptDefaults(t *testing.T) {
	// Nil opts must omit the optional fields; wildcard true sends 1.
	var raw json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		raw = req.Params
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if _, err := s.InstallLetsEncrypt(context.Background(), "mysite.ru", true, nil); err != nil {
		t.Fatalf("InstallLetsEncrypt: %v", err)
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	for _, k := range []string{"virtdom", "ip", "challenge"} {
		if _, ok := m[k]; ok {
			t.Errorf("param %q present, want omitted for nil opts", k)
		}
	}
	if string(m["wildcard"]) != "1" {
		t.Errorf("wildcard = %s, want 1 for true", m["wildcard"])
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
