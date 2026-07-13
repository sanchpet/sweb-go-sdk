package pay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestPayIndex(t *testing.T) {
	// index is a single-element array wrapping the account; balance is nested
	// under "balance" and blockInfo is an object (doc-vs-reality, both handled).
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = decodeMethod(r)
		_, _ = w.Write([]byte(`{"result":[{
			"auto_payment_enable":0,"isAutopaymentEnable":1,"domainBonuses":7,
			"status":"active","blockedMoney":36033,"edgeDate":"2024-08-30",
			"blockInfo":{"days":2274,"days_date":"25.04.2032","days_word":"дня"},
			"deferment":{"show":false,"value":0},
			"balance":{"real_balance":11886,"bonus_balance":0,"cloud_balance":11886,
				"other_balance":11886,"cloud_balance_view":11886,"other_balance_view":0,
				"credit_balance":0,"type":1,"vat_balance":{"2":"11886.0000"}}
		}]}`))
	})
	acc, err := s.Index(context.Background())
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if acc.Status != "active" || acc.IsAutopaymentEnable != 1 || acc.DomainBonuses != 7 {
		t.Errorf("account = %+v, want active / isAutopaymentEnable 1 / domainBonuses 7", acc)
	}
	if acc.Balance.RealBalance != 11886 || acc.Balance.Type != 1 || acc.Balance.CloudBalance != 11886 {
		t.Errorf("balance = %+v, want real/cloud 11886", acc.Balance)
	}
	if acc.BlockInfo.Days != 2274 || acc.BlockInfo.DaysDate != "25.04.2032" {
		t.Errorf("blockInfo = %+v, want days 2274 / 25.04.2032", acc.BlockInfo)
	}
	if acc.Deferment.Show || acc.Deferment.Value != 0 {
		t.Errorf("deferment = %+v, want show false / value 0", acc.Deferment)
	}
	if acc.Balance.VATBalance["2"] != "11886.0000" {
		t.Errorf("vat_balance = %+v, want [2]=11886.0000", acc.Balance.VATBalance)
	}
}

func TestPayIndexEmpty(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	acc, err := s.Index(context.Background())
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if acc == nil || acc.Status != "" {
		t.Errorf("empty index = %+v, want zero Account", acc)
	}
}

func TestPayIsAutopaymentEnable(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{`{"result":true}`, true},
		{`{"result":false}`, false},
		{`{"result":1}`, true}, // 1/0 form tolerated alongside the boolean
		{`{"result":0}`, false},
	} {
		var gotMethod string
		s := serve(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod = decodeMethod(r)
			_, _ = w.Write([]byte(tc.body))
		})
		got, err := s.IsAutopaymentEnable(context.Background())
		if err != nil {
			t.Fatalf("IsAutopaymentEnable(%s): %v", tc.body, err)
		}
		if gotMethod != "isAutopaymentEnable" {
			t.Errorf("method = %q, want isAutopaymentEnable", gotMethod)
		}
		if got != tc.want {
			t.Errorf("IsAutopaymentEnable(%s) = %v, want %v", tc.body, got, tc.want)
		}
	}
}

func TestPayGetPayRecommendations(t *testing.T) {
	var gotMethod string
	var gotAddBalance bool
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				AddBalanceRecommendations bool `json:"addBalanceRecommendations"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotAddBalance = req.Method, req.Params.AddBalanceRecommendations
		_, _ = w.Write([]byte(`{"result":[{
			"recommended_for_pay":[{"id":1,"name":"Домен test32132.ru (1 год)","date":"не зарегистрирован","cost":175}],
			"recommended_for_pay_balance":[],
			"exist_domain_bonus":0,"total_frp_balance":2800,
			"tariff_domain_bonus":0,"tariff_domain_bonus_tld":0,
			"domain_bonuses_by_tld":{"any":0}
		}]}`))
	})
	rec, err := s.GetPayRecommendations(context.Background(), true)
	if err != nil {
		t.Fatalf("GetPayRecommendations: %v", err)
	}
	if gotMethod != "getPayRecommendations" || !gotAddBalance {
		t.Errorf("method/param = %q/%v, want getPayRecommendations/true", gotMethod, gotAddBalance)
	}
	if len(rec.RecommendedForPay) != 1 || rec.RecommendedForPay[0].Cost != 175 || rec.RecommendedForPay[0].ID != 1 {
		t.Errorf("recommended = %+v, want one id 1 / cost 175", rec.RecommendedForPay)
	}
	if rec.TotalFRPBalance != 2800 || rec.DomainBonusesByTLD["any"] != 0 {
		t.Errorf("bundle = %+v, want total 2800 / any 0", rec)
	}
}

func TestPayGetRecommendationTotalCost(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = decodeMethod(r)
		_, _ = w.Write([]byte(`{"result":949}`))
	})
	cost, err := s.GetRecommendationTotalCost(context.Background())
	if err != nil {
		t.Fatalf("GetRecommendationTotalCost: %v", err)
	}
	if gotMethod != "getRecommendationTotalCost" {
		t.Errorf("method = %q, want getRecommendationTotalCost", gotMethod)
	}
	if cost != 949 {
		t.Errorf("cost = %v, want 949", cost)
	}
}

func TestPayGetUpcomingPaymentsVh(t *testing.T) {
	// Heterogeneous entries: a domain reg (base_cost quoted, tld) and an
	// antivirus (bare type, no base_cost). Both must decode.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = decodeMethod(r)
		_, _ = w.Write([]byte(`{"result":[
			{"action":0,"base_cost":"175","base_cost_str":"175 руб.","bonus_cost":0,
			 "checkbox_available":1,"cost":175,"cost_str":"175 руб.","date":"не зарегистрирован",
			 "entity_type":"domain","id":2,"name":"Домен test865.ru (1 год)","ready_for_bonus":1,
			 "service_id":"dyasyuc794_domain_2","tld":"ru","vat_name":"НДС 22%","vat_type":"other","vat_value":"22"},
			{"action":0,"checkbox_available":0,"cost":199,"cost_str":"199 р.","date":"19.02.2026",
			 "entity_type":"antivirus","id":3,"name":"Лечение антивирусом","service_id":"dyasyuc794_antivirus",
			 "type":"antivirus","vat_name":"НДС 22%","vat_type":"other","vat_value":"22"}
		]}`))
	})
	list, err := s.GetUpcomingPaymentsVh(context.Background())
	if err != nil {
		t.Fatalf("GetUpcomingPaymentsVh: %v", err)
	}
	if gotMethod != "getUpcomingPaymentsVh" {
		t.Errorf("method = %q, want getUpcomingPaymentsVh", gotMethod)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list[0].ID != 2 || list[0].Cost != 175 || list[0].BaseCost != 175 || list[0].TLD != "ru" || list[0].VATValue != 22 {
		t.Errorf("entry 0 = %+v, want id 2 / cost 175 / base 175 / tld ru / vat 22", list[0])
	}
	if list[1].ID != 3 || list[1].Cost != 199 || list[1].Type != "antivirus" || list[1].BaseCost != 0 {
		t.Errorf("entry 1 = %+v, want id 3 / cost 199 / type antivirus / base 0", list[1])
	}
}

func TestPayChangeDeferment(t *testing.T) {
	var gotMethod string
	var gotTurnOn bool
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				TurnOn bool `json:"turnOn"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotTurnOn = req.Method, req.Params.TurnOn
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.ChangeDeferment(context.Background(), true); err != nil {
		t.Fatalf("ChangeDeferment: %v", err)
	}
	if gotMethod != "changeDeferment" || !gotTurnOn {
		t.Errorf("method/turnOn = %q/%v, want changeDeferment/true", gotMethod, gotTurnOn)
	}
}

func TestPayChangeDefermentFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.ChangeDeferment(context.Background(), false); err == nil {
		t.Error("ChangeDeferment with result 0: got nil error, want failure")
	}
}

func TestPayGetRemainsDate(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = decodeMethod(r)
		_, _ = w.Write([]byte(`{"result":"01.10.2023"}`))
	})
	date, err := s.GetRemainsDate(context.Background())
	if err != nil {
		t.Fatalf("GetRemainsDate: %v", err)
	}
	if gotMethod != "getRemainsDate" {
		t.Errorf("method = %q, want getRemainsDate", gotMethod)
	}
	if date != "01.10.2023" {
		t.Errorf("date = %q, want 01.10.2023", date)
	}
}

func TestPayGetRemainsDays(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = decodeMethod(r)
		_, _ = w.Write([]byte(`{"result":15}`))
	})
	days, err := s.GetRemainsDays(context.Background())
	if err != nil {
		t.Fatalf("GetRemainsDays: %v", err)
	}
	if gotMethod != "getRemainsDays" {
		t.Errorf("method = %q, want getRemainsDays", gotMethod)
	}
	if days != 15 {
		t.Errorf("days = %v, want 15", days)
	}
}

func TestPayGetBalance(t *testing.T) {
	// getBalance is a single-element array wrapping the balance; money arrives
	// int-or-float and vat_balance is quoted.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = decodeMethod(r)
		_, _ = w.Write([]byte(`{"result":[{
			"real_balance":1544,"bonus_balance":0,"cloud_balance":1492,"other_balance":52,
			"cloud_balance_view":1492,"other_balance_view":52,"credit_balance":0,
			"type":1,"multiple_balance_enabled":true,
			"vat_balance":{"4":"1492.0000","5":"52.0000"}
		}]}`))
	})
	bal, err := s.GetBalance(context.Background())
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if gotMethod != "getBalance" {
		t.Errorf("method = %q, want getBalance", gotMethod)
	}
	if bal.RealBalance != 1544 || bal.CloudBalance != 1492 || bal.OtherBalance != 52 {
		t.Errorf("balance = %+v, want real 1544 / cloud 1492 / other 52", bal)
	}
	if !bal.MultipleBalanceEnabled || bal.Type != 1 {
		t.Errorf("flags = %+v, want multiple true / type 1", bal)
	}
	if bal.VATBalance["4"] != "1492.0000" {
		t.Errorf("vat_balance = %+v, want [4]=1492.0000", bal.VATBalance)
	}
}

func TestPayGetActiveReserves(t *testing.T) {
	// charge arrives both bare-float (2368.74) and quoted ("3120.00"); endDate
	// is nullable.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = decodeMethod(r)
		_, _ = w.Write([]byte(`{"result":[
			{"balance_type":"cloud","charge":2368.74,"type":"tariff",
			 "info":{"endDate":"2027-01-20","title":"Tariff Takeoff 1 year"}},
			{"balance_type":"other","charge":"3120.00","type":"other",
			 "info":{"endDate":null,"title":"Другое"}}
		]}`))
	})
	list, err := s.GetActiveReserves(context.Background())
	if err != nil {
		t.Fatalf("GetActiveReserves: %v", err)
	}
	if gotMethod != "getActiveReserves" {
		t.Errorf("method = %q, want getActiveReserves", gotMethod)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list[0].Charge != 2368.74 || list[0].BalanceType != "cloud" || list[0].Info.EndDate != "2027-01-20" {
		t.Errorf("reserve 0 = %+v, want charge 2368.74 / cloud / 2027-01-20", list[0])
	}
	if list[1].Charge != 3120 || list[1].Type != "other" || list[1].Info.EndDate != "" {
		t.Errorf("reserve 1 = %+v, want charge 3120 / other / empty endDate", list[1])
	}
}

// decodeMethod extracts the JSON-RPC method name from the request body.
func decodeMethod(r *http.Request) string {
	var req struct {
		Method string `json:"method"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	return req.Method
}

// serve spins up a mock JSON-RPC server for h and returns a pay.Service backed
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
