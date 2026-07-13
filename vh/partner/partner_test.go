package partner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

// ---------------------------------------------------------------------------
// Referral catalog.
// ---------------------------------------------------------------------------

func TestStandardAndVIPPlans(t *testing.T) {
	// standardPlans/vipPlans share the Plan shape and differ only in method; one
	// table exercises both. Limits arrive as bare ints, id quoted.
	body := `{"result":[{
		"id":"7110","name":"Взлёт","disk":5,"sites":5,"dbCount":512,
		"ftpCount":1025,"mailCount":1025,
		"period":[
			{"length":1,"price":199,"ssl":1,"domain":0,"domainZone":""},
			{"length":12,"price":1908,"ssl":1,"domain":1,"domainZone":".ru, .рф"}
		]
	}]}`
	for _, tc := range []struct {
		name, method string
		call         func(*Service) ([]Plan, error)
	}{
		{"standard", "standardPlans", func(s *Service) ([]Plan, error) { return s.StandardPlans(context.Background()) }},
		{"vip", "vipPlans", func(s *Service) ([]Plan, error) { return s.VipPlans(context.Background()) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			s := serve(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, _ = decodeReq(r)
				_, _ = w.Write([]byte(body))
			})
			plans, err := tc.call(s)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method {
				t.Errorf("method = %q, want %q", gotMethod, tc.method)
			}
			if len(plans) != 1 || plans[0].ID != "7110" || plans[0].Disk != 5 || plans[0].Sites != 5 {
				t.Fatalf("plans = %+v, want one 7110/disk 5/sites 5", plans)
			}
			p := plans[0]
			if len(p.Period) != 2 || p.Period[0].Length != 1 || p.Period[0].Price != 199 {
				t.Errorf("period[0] = %+v, want length 1 / price 199", p.Period)
			}
			if p.Period[1].Domain != 1 || p.Period[1].DomainZone != ".ru, .рф" {
				t.Errorf("period[1] = %+v, want domain 1 / zone .ru, .рф", p.Period[1])
			}
		})
	}
}

func TestOSConfig(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{
			"categories":[{"id":"1","name":"NVMe","slug":"nvme","prior":"1"}],
			"datacenters":[{"id":"1","name":"spb","location":"Санкт-Петербург","site_name":"ДЦ в СПБ"}],
			"osPanel":[{"distributive":"8","os":"4","panel":"7","minRam":0,"minStorage":0,"availablePlanIds":[4,5,6]}],
			"selectOs":[{"id":"20","name":"ubuntu-20-04","description":"Ubuntu 20.04","full_description":null,
				"order":"1","os_distribution_id":"32","plan_id":"4","panel_type":["empty","isp"],"url":null}],
			"selectPanel":[{"id":"4","name":"vps_isp6_lite","description":"ISPmanager 6 Lite","full_description":null,
				"order":"2","os_distribution_id":"77","plan_id":"5","price":300,"old_price":300,
				"action":0,"creation_time":"20-30","url":null}]
		}}`))
	})
	cfg, err := s.OSConfig(context.Background())
	if err != nil {
		t.Fatalf("OSConfig: %v", err)
	}
	if gotMethod != "vpsOsConfig" {
		t.Errorf("method = %q, want vpsOsConfig", gotMethod)
	}
	if len(cfg.Categories) != 1 || cfg.Categories[0].Slug != "nvme" {
		t.Errorf("categories = %+v, want one nvme", cfg.Categories)
	}
	if len(cfg.Datacenters) != 1 || cfg.Datacenters[0].Name != "spb" {
		t.Errorf("datacenters = %+v, want one spb", cfg.Datacenters)
	}
	if len(cfg.OSPanel) != 1 || len(cfg.OSPanel[0].AvailablePlanIDs) != 3 || cfg.OSPanel[0].AvailablePlanIDs[0] != 4 {
		t.Errorf("osPanel = %+v, want one with plan ids [4,5,6]", cfg.OSPanel)
	}
	if len(cfg.SelectOS) != 1 || len(cfg.SelectOS[0].PanelType) != 2 || cfg.SelectOS[0].FullDescription != "" {
		t.Errorf("selectOs = %+v, want one with 2 panels and null full_description", cfg.SelectOS)
	}
	if len(cfg.SelectPanel) != 1 || cfg.SelectPanel[0].Price != 300 || cfg.SelectPanel[0].CreationTime != "20-30" {
		t.Errorf("selectPanel = %+v, want one price 300 / creation 20-30", cfg.SelectPanel)
	}
}

// ---------------------------------------------------------------------------
// checkLogin and order placement (offline only — orders bill).
// ---------------------------------------------------------------------------

func TestCheckLogin(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{`{"result":true}`, true},
		{`{"result":false}`, false},
	} {
		var gotMethod string
		var gotLogin string
		s := serve(t, func(w http.ResponseWriter, r *http.Request) {
			var params map[string]json.RawMessage
			gotMethod, params = decodeReq(r)
			_ = json.Unmarshal(params["login"], &gotLogin)
			_, _ = w.Write([]byte(tc.body))
		})
		got, err := s.CheckLogin(context.Background(), "newuser")
		if err != nil {
			t.Fatalf("CheckLogin(%s): %v", tc.body, err)
		}
		if gotMethod != "checkLogin" || gotLogin != "newuser" {
			t.Errorf("method/login = %q/%q, want checkLogin/newuser", gotMethod, gotLogin)
		}
		if got != tc.want {
			t.Errorf("CheckLogin(%s) = %v, want %v", tc.body, got, tc.want)
		}
	}
}

func TestCreateHostingOrders(t *testing.T) {
	// createOrderVh/createOrderVip share params and the {login,password} result;
	// mock server only — a live call would place a billed order.
	for _, tc := range []struct {
		name, method string
		call         func(*Service, StandardOrder) (*OrderResult, error)
	}{
		{"standard", "createOrderVh", (*Service).createStandard},
		{"vip", "createOrderVip", (*Service).createVIP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			var gotParams struct {
				Email    string `json:"email"`
				TariffID int    `json:"tariffId"`
				Period   int    `json:"period"`
				Login    string `json:"login"`
			}
			s := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string          `json:"method"`
					Params json.RawMessage `json:"params"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod = req.Method
				_ = json.Unmarshal(req.Params, &gotParams)
				_, _ = w.Write([]byte(`{"result":{"login":"in****ly82","password":"secret"}}`))
			})
			res, err := tc.call(s, StandardOrder{Email: "a@b.c", TariffID: 7110, Period: 1, Login: "newuser", Password: "pw"})
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method {
				t.Errorf("method = %q, want %q", gotMethod, tc.method)
			}
			if gotParams.TariffID != 7110 || gotParams.Period != 1 || gotParams.Login != "newuser" {
				t.Errorf("params = %+v, want tariff 7110 / period 1 / login newuser", gotParams)
			}
			if res.Login != "in****ly82" || res.Password != "secret" {
				t.Errorf("result = %+v, want login/password", res)
			}
		})
	}
}

func TestCreateVPSOrder(t *testing.T) {
	// Mock server only — a live createOrderVps would bill.
	var gotMethod string
	var gotParams struct {
		DistributiveID int `json:"distributiveId"`
		Datacenter     int `json:"datacenter"`
		TariffID       int `json:"tariffId"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":{"login":"vp****01","password":"pw"}}`))
	})
	res, err := s.CreateVPSOrder(context.Background(), VPSOrder{
		Email: "a@b.c", TariffID: 42, DistributiveID: 20, Period: 1, Login: "v", Password: "p", Datacenter: 2,
	})
	if err != nil {
		t.Fatalf("CreateVPSOrder: %v", err)
	}
	if gotMethod != "createOrderVps" {
		t.Errorf("method = %q, want createOrderVps", gotMethod)
	}
	if gotParams.DistributiveID != 20 || gotParams.Datacenter != 2 || gotParams.TariffID != 42 {
		t.Errorf("params = %+v, want distributive 20 / dc 2 / tariff 42", gotParams)
	}
	if res.Login != "vp****01" {
		t.Errorf("result = %+v, want login vp****01", res)
	}
}

// ---------------------------------------------------------------------------
// Sentinel-1 mutations.
// ---------------------------------------------------------------------------

func TestActionOneMethods(t *testing.T) {
	// startPartnership/fillPartnerRequisites/savePartnerClientComment/
	// sendWithdrawalOrder all answer the resultInt 1/0 sentinel. Mock only —
	// sendWithdrawalOrder moves real money.
	for _, tc := range []struct {
		name, method string
		call         func(*Service) error
	}{
		{"startPartnership", "startPartnership",
			func(s *Service) error { return s.StartPartnership(context.Background()) }},
		{"fillRequisites", "fillPartnerRequisites",
			func(s *Service) error {
				return s.FillRequisites(context.Background(), Requisites{INN: "1", SNILS: "2", RegAddress: "addr"})
			}},
		{"saveComment", "savePartnerClientComment",
			func(s *Service) error { return s.SaveClientComment(context.Background(), "cid", "note") }},
		{"sendWithdrawal", "sendWithdrawalOrder",
			func(s *Service) error {
				return s.SendWithdrawalOrder(context.Background(), WithdrawalOrder{OrderType: 1, CountMoney: 390})
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			s := serve(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, _ = decodeReq(r)
				_, _ = w.Write([]byte(`{"result":1}`))
			})
			if err := tc.call(s); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method {
				t.Errorf("method = %q, want %q", gotMethod, tc.method)
			}
		})
	}
}

func TestActionOneFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.StartPartnership(context.Background()); err == nil {
		t.Error("StartPartnership with result 0: got nil error, want failure")
	}
}

func TestSendWithdrawalOrderParams(t *testing.T) {
	// Confirm the bank requisites map through under their req* keys.
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	err := s.SendWithdrawalOrder(context.Background(), WithdrawalOrder{
		OrderType: 1, CountMoney: 390, ReqUserName: "Ivanov", ReqBIC: "044030858",
	})
	if err != nil {
		t.Fatalf("SendWithdrawalOrder: %v", err)
	}
	var bic string
	_ = json.Unmarshal(gotParams["reqBIC"], &bic)
	if bic != "044030858" {
		t.Errorf("reqBIC = %q, want 044030858", bic)
	}
}

// ---------------------------------------------------------------------------
// Advertising materials.
// ---------------------------------------------------------------------------

func TestAdvertMaterialTypes(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":[{"name":"Все","value":"all"},{"name":"160x600","value":"160x600"}]}`))
	})
	types, err := s.AdvertMaterialTypes(context.Background())
	if err != nil {
		t.Fatalf("AdvertMaterialTypes: %v", err)
	}
	if gotMethod != "getTypesAdvertMaterials" {
		t.Errorf("method = %q, want getTypesAdvertMaterials", gotMethod)
	}
	if len(types) != 2 || types[0].Value != "all" || types[1].Value != "160x600" {
		t.Errorf("types = %+v, want all + 160x600", types)
	}
}

func TestAdvertMaterials(t *testing.T) {
	var gotMethod string
	var gotType string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var params map[string]json.RawMessage
		gotMethod, params = decodeReq(r)
		_ = json.Unmarshal(params["type"], &gotType)
		_, _ = w.Write([]byte(`{"result":[{"code":"<img/>","filesize":"29 КБ","sizes":"160x600"}]}`))
	})
	mats, err := s.AdvertMaterials(context.Background(), "160x600")
	if err != nil {
		t.Fatalf("AdvertMaterials: %v", err)
	}
	if gotMethod != "getAdvertMaterials" || gotType != "160x600" {
		t.Errorf("method/type = %q/%q, want getAdvertMaterials/160x600", gotMethod, gotType)
	}
	if len(mats) != 1 || mats[0].Sizes != "160x600" || mats[0].FileSize != "29 КБ" {
		t.Errorf("materials = %+v, want one 160x600/29 КБ", mats)
	}
}

// ---------------------------------------------------------------------------
// Client roster, card, and logs.
// ---------------------------------------------------------------------------

func TestClientsList(t *testing.T) {
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{
			"filterInfo":{"filterStatus":-1,"page":1,"perPage":20,"totalCount":1},
			"list":[{"id":"cid1","cust_login":"in****ly82","is_promo":false,
				"pays_all":0,"pays_month":0,"plan":["Взлёт"],"status":2,"ts":"14.03.2023","type":1}]
		}}`))
	})
	out, err := s.ClientsList(context.Background(), -1, 1)
	if err != nil {
		t.Fatalf("ClientsList: %v", err)
	}
	if gotMethod != "getPartnerClientsList" {
		t.Errorf("method = %q, want getPartnerClientsList", gotMethod)
	}
	var fs int
	_ = json.Unmarshal(gotParams["filterStatus"], &fs)
	if fs != -1 {
		t.Errorf("filterStatus = %d, want -1", fs)
	}
	if out.FilterInfo.TotalCount != 1 || out.FilterInfo.FilterStatus != -1 {
		t.Errorf("filterInfo = %+v, want totalCount 1 / filterStatus -1", out.FilterInfo)
	}
	if len(out.List) != 1 || out.List[0].ID != "cid1" || out.List[0].Status != 2 || len(out.List[0].Plan) != 1 {
		t.Errorf("list = %+v, want one cid1/status 2/one plan", out.List)
	}
}

func TestClientCard(t *testing.T) {
	var gotMethod string
	var gotClientID string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var params map[string]json.RawMessage
		gotMethod, params = decodeReq(r)
		_ = json.Unmarshal(params["clientId"], &gotClientID)
		_, _ = w.Write([]byte(`{"result":{
			"id":"cid1","login":"in****ly82","planName":["Взлёт"],"status":2,"type":1,
			"attraction":"MIMAIDAZ","comment":"","contractNumber":"","regDate":"14.03.2023",
			"amountsPeriod":0,"amountsLastMonth":0
		}}`))
	})
	card, err := s.ClientCard(context.Background(), "cid1")
	if err != nil {
		t.Fatalf("ClientCard: %v", err)
	}
	if gotMethod != "getPartnerClientCard" || gotClientID != "cid1" {
		t.Errorf("method/clientId = %q/%q, want getPartnerClientCard/cid1", gotMethod, gotClientID)
	}
	if card.ID != "cid1" || card.Attraction != "MIMAIDAZ" || card.Status != 2 || len(card.PlanName) != 1 {
		t.Errorf("card = %+v, want cid1/MIMAIDAZ/status 2/one plan", card)
	}
}

func TestClientLogEvents(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{
			"filterInfo":{"page":1,"perPage":20,"totalCount":1},
			"list":[{"eventName":"регистрация клиента","ts":"14.03.2023"}]
		}}`))
	})
	log, err := s.ClientLogEvents(context.Background(), 1)
	if err != nil {
		t.Fatalf("ClientLogEvents: %v", err)
	}
	if gotMethod != "getPartnerClientLogEvents" {
		t.Errorf("method = %q, want getPartnerClientLogEvents", gotMethod)
	}
	if log.FilterInfo.TotalCount != 1 || len(log.List) != 1 || log.List[0].EventName != "регистрация клиента" {
		t.Errorf("log = %+v, want one event", log)
	}
}

func TestClientLogFinance(t *testing.T) {
	// withdrawal/payment/lock arrive as number-or-empty-string; flex.Int folds
	// "" to 0.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{
			"filterInfo":{"page":1,"perPage":20,"totalCount":201},
			"list":[
				{"eventName":"оплата тарифа","lock":"","payment":"","ts":"14.03.2023","withdrawal":199},
				{"eventName":"отправлена заявка","lock":390,"payment":"","ts":"22.02.2023","withdrawal":""}
			]
		}}`))
	})
	log, err := s.ClientLogFinance(context.Background(), 1)
	if err != nil {
		t.Fatalf("ClientLogFinance: %v", err)
	}
	if gotMethod != "getPartnerClientLogFinance" {
		t.Errorf("method = %q, want getPartnerClientLogFinance", gotMethod)
	}
	if log.FilterInfo.TotalCount != 201 || len(log.List) != 2 {
		t.Fatalf("log = %+v, want totalCount 201 / 2 rows", log)
	}
	if log.List[0].Withdrawal != 199 || log.List[0].Lock != 0 {
		t.Errorf("row0 = %+v, want withdrawal 199 / lock 0 (from \"\")", log.List[0])
	}
	if log.List[1].Lock != 390 || log.List[1].Withdrawal != 0 {
		t.Errorf("row1 = %+v, want lock 390 / withdrawal 0 (from \"\")", log.List[1])
	}
}

// ---------------------------------------------------------------------------
// Withdrawal requisites and statistics.
// ---------------------------------------------------------------------------

func TestWithdrawalRequisites(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{
			"balance":2878.44,
			"orderTypes":{
				"hosting":{"description":"...","enable":true,"error":"","maximumMonthAmount":14,
					"minimumAmount":199,"name":"Оплата тарифа","sort":1,"type":"3","typeName":"hosting"},
				"score":{"description":"...","enable":true,"error":"","minimumAmount":390,
					"name":"На расчетный счет","sort":2,"type":"1","typeName":"score"}
			},
			"reqBIC":"044030858","reqBankName":"ЮниКредит","reqCheckAccount":"40702810500024452823",
			"reqCorrAccount":"30101810800000000858","reqUserName":"Иванов Петр Сидорович"
		}}`))
	})
	req, err := s.WithdrawalRequisites(context.Background())
	if err != nil {
		t.Fatalf("WithdrawalRequisites: %v", err)
	}
	if gotMethod != "getRequisitesWithdrawal" {
		t.Errorf("method = %q, want getRequisitesWithdrawal", gotMethod)
	}
	if req.Balance != 2878.44 || req.ReqBIC != "044030858" || req.ReqUserName != "Иванов Петр Сидорович" {
		t.Errorf("req = %+v, want balance 2878.44 / bic / name", req)
	}
	if len(req.OrderTypes) != 2 {
		t.Fatalf("orderTypes = %+v, want 2", req.OrderTypes)
	}
	hosting := req.OrderTypes["hosting"]
	if hosting.Type != "3" || hosting.MaximumMonthAmount != 14 || hosting.MinimumAmount != 199 || !hosting.Enable {
		t.Errorf("hosting way = %+v, want type 3 / maxMonth 14 / min 199 / enabled", hosting)
	}
	if req.OrderTypes["score"].MaximumMonthAmount != 0 {
		t.Errorf("score maxMonth = %d, want 0 (absent field)", int64(req.OrderTypes["score"].MaximumMonthAmount))
	}
}

func TestGetStatistic(t *testing.T) {
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{
			"csv":{"content":"YWJj","metadata":[],"mimetype":"application/csv;base64","name":"s.csv"},
			"png":{"content":"aVZC","metadata":[],"mimetype":"image/png;base64","name":"s.png"},
			"data":[["2023-03-01",0,0],["2023-03-02",1,2]]
		}}`))
	})
	stat, err := s.GetStatistic(context.Background(), "hosters.ru", 2023, 3)
	if err != nil {
		t.Fatalf("GetStatistic: %v", err)
	}
	if gotMethod != "getStatistic" {
		t.Errorf("method = %q, want getStatistic", gotMethod)
	}
	var site string
	_ = json.Unmarshal(gotParams["site"], &site)
	if site != "hosters.ru" {
		t.Errorf("site = %q, want hosters.ru", site)
	}
	if stat.CSV.Mimetype != "application/csv;base64" || stat.CSV.Content != "YWJj" {
		t.Errorf("csv = %+v, want base64 payload", stat.CSV)
	}
	if len(stat.Data) != 2 || len(stat.Data[1]) != 3 || string(stat.Data[1][1]) != "1" {
		t.Errorf("data = %v, want 2 rows, row1 second cell 1", stat.Data)
	}
}

func TestGetLinkStatistics(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{
			"csv":{"content":"YWJj","metadata":[],"mimetype":"application/csv;base64","name":"l.csv"},
			"png":{"content":"aVZC","metadata":[],"mimetype":"image/png;base64","name":"l.png"},
			"data":[["2023-03-01",0,0,0],["2023-03-02",3,2,1]]
		}}`))
	})
	stat, err := s.GetLinkStatistics(context.Background(), 2023, 3)
	if err != nil {
		t.Fatalf("GetLinkStatistics: %v", err)
	}
	if gotMethod != "getLinkStatistics" {
		t.Errorf("method = %q, want getLinkStatistics", gotMethod)
	}
	if len(stat.Data) != 2 || len(stat.Data[1]) != 4 || string(stat.Data[1][0]) != `"2023-03-02"` {
		t.Errorf("data = %v, want 2 rows of 4 cells", stat.Data)
	}
	if stat.CSV.Name != "l.csv" {
		t.Errorf("csv name = %q, want l.csv", stat.CSV.Name)
	}
}

// ---------------------------------------------------------------------------
// Test helpers (mirroring vh/hosting).
// ---------------------------------------------------------------------------

// createStandard / createVIP adapt the two hosting-order methods to a common
// signature for the shared table in TestCreateHostingOrders.
func (s *Service) createStandard(o StandardOrder) (*OrderResult, error) {
	return s.CreateStandardOrder(context.Background(), o)
}

func (s *Service) createVIP(o StandardOrder) (*OrderResult, error) {
	return s.CreateVIPOrder(context.Background(), o)
}

// decodeReq extracts the method and raw params from a JSON-RPC request body.
func decodeReq(r *http.Request) (method string, params map[string]json.RawMessage) {
	var req struct {
		Method string                     `json:"method"`
		Params map[string]json.RawMessage `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	return req.Method, req.Params
}

// serve spins up a mock JSON-RPC server for h and returns a partner.Service
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
