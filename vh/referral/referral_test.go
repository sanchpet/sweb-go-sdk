package referral

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestReferralList(t *testing.T) {
	// index wraps the sites under "list" with "filterInfo" pagination; numeric
	// fields (id, partner_id, clientsCount) arrive quoted and decode via flex.Int.
	var gotMethod string
	var gotParams struct {
		Page  int `json:"page"`
		Limit int `json:"limit"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":{
			"filterInfo":{"limit":20,"page":1,"totalCount":2},
			"list":[{
				"clientsCount":0,
				"confirmationFile":{
					"content":"YmIwZTliNjhmYWZjNWM4YTg1NjYxZDI4ZDc1MWMwYmQ=",
					"metadata":[],
					"mimetype":"application/plain;base64",
					"name":"bb0e9b68fafc5c8a85661d28d751c0bd.txt"
				},
				"confirmed":true,
				"created":"2019-12-09 12:35:07",
				"domain":"2rush.ru",
				"id":"911",
				"partner_id":"3523",
				"verification_code":"bb0e9b68fafc5c8a85661d28d751c0bd"
			}]
		}}`))
	})
	res, err := s.List(context.Background(), &ListOptions{Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if gotParams.Page != 1 || gotParams.Limit != 20 {
		t.Errorf("params = %+v, want page 1 / limit 20", gotParams)
	}
	if res.FilterInfo.Page != 1 || res.FilterInfo.Limit != 20 || res.FilterInfo.TotalCount != 2 {
		t.Errorf("filterInfo = %+v, want page 1 / limit 20 / totalCount 2", res.FilterInfo)
	}
	if len(res.List) != 1 {
		t.Fatalf("list len = %d, want 1", len(res.List))
	}
	site := res.List[0]
	if site.ID != "911" || site.PartnerID != "3523" || site.Domain != "2rush.ru" {
		t.Errorf("site = %+v, want 911 / 3523 / 2rush.ru", site)
	}
	if !site.Confirmed || site.ClientsCount != 0 || site.VerificationCode != "bb0e9b68fafc5c8a85661d28d751c0bd" {
		t.Errorf("site flags = %+v, want confirmed true / clientsCount 0", site)
	}
	if site.ConfirmationFile.Name != "bb0e9b68fafc5c8a85661d28d751c0bd.txt" || site.ConfirmationFile.Mimetype != "application/plain;base64" {
		t.Errorf("confirmationFile = %+v, want the .txt file", site.ConfirmationFile)
	}
}

func TestReferralListParamsOmittedWhenZero(t *testing.T) {
	// Nil options must not send page/limit keys so the API applies its defaults.
	var hasPage, hasLimit bool
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_, hasPage = req.Params["page"]
		_, hasLimit = req.Params["limit"]
		_, _ = w.Write([]byte(`{"result":{"list":[],"filterInfo":{"page":1,"limit":20,"totalCount":0}}}`))
	})
	if _, err := s.List(context.Background(), nil); err != nil {
		t.Fatalf("List: %v", err)
	}
	if hasPage || hasLimit {
		t.Errorf("params carried empty page/limit keys (page=%v limit=%v), want them omitted", hasPage, hasLimit)
	}
}

func TestReferralAdd(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Add(context.Background(), "mysite.ru"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if gotMethod != "addReferralSite" || gotDomain != "mysite.ru" {
		t.Errorf("method/domain = %q/%q, want addReferralSite/mysite.ru", gotMethod, gotDomain)
	}
}

func TestReferralConfirm(t *testing.T) {
	var gotMethod string
	var gotID int
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				ID int `json:"id"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotID = req.Method, req.Params.ID
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Confirm(context.Background(), 1664); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if gotMethod != "confirmReferralSite" || gotID != 1664 {
		t.Errorf("method/id = %q/%d, want confirmReferralSite/1664", gotMethod, gotID)
	}
}

func TestReferralRemove(t *testing.T) {
	var gotMethod string
	var gotID int
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				ID int `json:"id"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotID = req.Method, req.Params.ID
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Remove(context.Background(), 1664); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if gotMethod != "removeReferralSite" || gotID != 1664 {
		t.Errorf("method/id = %q/%d, want removeReferralSite/1664", gotMethod, gotID)
	}
}

func TestReferralSentinelFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error, across every
	// mutating method.
	for _, tc := range []struct {
		name string
		call func(*Service) error
	}{
		{"Add", func(s *Service) error { return s.Add(context.Background(), "x.ru") }},
		{"Confirm", func(s *Service) error { return s.Confirm(context.Background(), 1) }},
		{"Remove", func(s *Service) error { return s.Remove(context.Background(), 1) }},
	} {
		s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"result":0}`))
		})
		if err := tc.call(s); err == nil {
			t.Errorf("%s with result 0: got nil error, want failure", tc.name)
		}
	}
}

// serve spins up a mock JSON-RPC server for h and returns a referral.Service
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
