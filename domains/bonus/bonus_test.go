package bonus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestBonusIndex(t *testing.T) {
	// index answers a single-element array wrapping the object; the bonus fields
	// arrive as quoted strings/nulls.
	var gotMethod string
	var gotParams struct {
		Page      int    `json:"page"`
		OrderBy   string `json:"orderBy"`
		OrderType string `json:"orderType"`
		Used      *bool  `json:"used"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":[{
			"bonuses":[{
				"bonus_id":"0","bonus_title":null,"customer_id":"testvps123","domain":null,
				"id":"106067","payment_id":"4690785","tld":"online","ts_close":null,
				"ts_create":"2023-01-30 16:53:49","type":"3","type_title":"Доменный бонус .ONLINE",
				"use_type":null,"used":"n","valid_till":"2024-01-30 23:59:59"
			}],
			"count":1,"unusedCount":1
		}]}`))
	})
	used := false
	res, err := s.Index(context.Background(), IndexOptions{Page: 0, OrderBy: "valid_till", OrderType: "DESC", Used: &used})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if gotParams.OrderBy != "valid_till" || gotParams.OrderType != "DESC" || gotParams.Used == nil || *gotParams.Used {
		t.Errorf("params = %+v, want orderBy valid_till / DESC / used false", gotParams)
	}
	if res.Count != 1 || res.UnusedCount != 1 || len(res.Bonuses) != 1 {
		t.Fatalf("result = %+v, want count 1 / unusedCount 1 / one bonus", res)
	}
	b := res.Bonuses[0]
	if b.ID != 106067 || b.BonusID != 0 || b.PaymentID != 4690785 || b.Type != 3 {
		t.Errorf("numeric ids = %+v, want id 106067 / bonusId 0 / payment 4690785 / type 3", b)
	}
	if b.CustomerID != "testvps123" || b.TLD != "online" || b.Used != "n" {
		t.Errorf("strings = %+v, want customer testvps123 / tld online / used n", b)
	}
	if b.BonusTitle != "" || b.Domain != "" || b.TSClose != "" || b.UseType != "" {
		t.Errorf("nullable fields = %+v, want empty for null", b)
	}
}

func TestBonusIndexOptionalParamsOmittedWhenUnset(t *testing.T) {
	var params map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		params = req.Params
		_, _ = w.Write([]byte(`{"result":[{"bonuses":[],"count":0,"unusedCount":0}]}`))
	})
	if _, err := s.Index(context.Background(), IndexOptions{Page: 2}); err != nil {
		t.Fatalf("Index: %v", err)
	}
	if _, ok := params["page"]; !ok {
		t.Errorf("params missing required page key: %v", params)
	}
	for _, k := range []string{"orderBy", "orderType", "used"} {
		if _, ok := params[k]; ok {
			t.Errorf("params carried empty %q key, want it omitted", k)
		}
	}
}

func TestBonusIndexBareObject(t *testing.T) {
	// A bare object (not array-wrapped) must also decode.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"bonuses":[],"count":5,"unusedCount":2}}`))
	})
	res, err := s.Index(context.Background(), IndexOptions{Page: 0})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if res.Count != 5 || res.UnusedCount != 2 {
		t.Errorf("result = %+v, want count 5 / unusedCount 2", res)
	}
}

func TestBonusGetList(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":[
			{"descr":"Приобретение 100 доменных бонусов","domains":100,"id":30,
			 "price":17000,"price_for_domain":"170 ₽ за домен","price_old":40000,
			 "title":"100 доменных бонусов"},
			{"descr":"Приобретение 3 доменных бонусов","domains":3,"id":1,
			 "price":1050,"price_for_domain":"350 ₽ за домен","price_old":1200,
			 "title":"3 доменных бонуса"}
		]}`))
	})
	list, err := s.GetList(context.Background())
	if err != nil {
		t.Fatalf("GetList: %v", err)
	}
	if gotMethod != "getList" {
		t.Errorf("method = %q, want getList", gotMethod)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	p := list[0]
	if p.ID != 30 || p.Domains != 100 || p.Price != 17000 || p.PriceOld != 40000 {
		t.Errorf("package = %+v, want id 30 / domains 100 / price 17000 / old 40000", p)
	}
	if p.PriceForDomain != "170 ₽ за домен" || p.Title != "100 доменных бонусов" {
		t.Errorf("strings = %+v, want priceForDomain/title populated", p)
	}
}

func TestBonusBuy(t *testing.T) {
	var gotMethod string
	var gotBonusID int
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				BonusID int `json:"bonusId"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotBonusID = req.Method, req.Params.BonusID
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Buy(context.Background(), 30); err != nil {
		t.Fatalf("Buy: %v", err)
	}
	if gotMethod != "buy" || gotBonusID != 30 {
		t.Errorf("method/bonusId = %q/%d, want buy/30", gotMethod, gotBonusID)
	}
}

func TestBonusBuySentinelFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.Buy(context.Background(), 30); err == nil {
		t.Error("Buy with result 0: got nil error, want failure")
	}
}

// serve spins up a mock JSON-RPC server for h and returns a bonus.Service
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
