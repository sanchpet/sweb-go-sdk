package checks

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

func TestChecksIndex(t *testing.T) {
	s, method, params := capture(t, `{"filterInfo":{"page":1,"perPage":2,"totalCount":1},`+
		`"list":[{"disabled":false,"id":"339","lastResult":true,"name":"sweb.ru","status":true,`+
		`"tsDeltaResult":null,"tsLastResult":null,"type":"1"}]}`)
	got, err := s.Index(context.Background(), &ListOptions{Page: 1, PerPage: 2})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if *method != "index" {
		t.Errorf("method = %q, want index", *method)
	}
	var p struct {
		Page    int `json:"page"`
		PerPage int `json:"perPage"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.Page != 1 || p.PerPage != 2 {
		t.Errorf("params = %s, want page=1 perPage=2", *params)
	}
	if got.FilterInfo.TotalCount != 1 || len(got.List) != 1 {
		t.Fatalf("filterInfo/list = %+v", got)
	}
	if got.List[0].ID != "339" || got.List[0].Type != "1" || !got.List[0].Status || got.List[0].Disabled {
		t.Errorf("list[0] = %+v", got.List[0])
	}
}

func TestChecksReferenceLists(t *testing.T) {
	t.Run("getTypes", func(t *testing.T) {
		s, method, _ := capture(t, `[{"code":"ping","id":"1","name":"Ping"},{"code":"http","id":"2","name":"HTTP"}]`)
		got, err := s.GetTypes(context.Background())
		if err != nil || *method != "getTypes" {
			t.Fatalf("GetTypes: err=%v method=%q", err, *method)
		}
		if len(got) != 2 || got[0].Code != "ping" || got[1].Name != "HTTP" {
			t.Errorf("types = %+v", got)
		}
	})
	t.Run("getIntervals", func(t *testing.T) {
		s, method, _ := capture(t, `[{"id":"1","name":"1 мин","time":"1"},{"id":"7","name":"1 час","time":"60"}]`)
		got, err := s.GetIntervals(context.Background())
		if err != nil || *method != "getIntervals" {
			t.Fatalf("GetIntervals: err=%v method=%q", err, *method)
		}
		if len(got) != 2 || got[1].Time != "60" {
			t.Errorf("intervals = %+v", got)
		}
	})
	t.Run("getPorts", func(t *testing.T) {
		s, method, _ := capture(t, `[{"name":"HTTPS","nameFull":"Hypertext Transfer Protocol Secure","value":"443"}]`)
		got, err := s.GetPorts(context.Background())
		if err != nil || *method != "getPorts" {
			t.Fatalf("GetPorts: err=%v method=%q", err, *method)
		}
		if len(got) != 1 || got[0].Value != "443" {
			t.Errorf("ports = %+v", got)
		}
	})
	t.Run("getKeywordModes", func(t *testing.T) {
		s, method, _ := capture(t, `[{"id":"1","name":"на странице должны быть все слова"}]`)
		got, err := s.GetKeywordModes(context.Background())
		if err != nil || *method != "getKeywordModes" {
			t.Fatalf("GetKeywordModes: err=%v method=%q", err, *method)
		}
		if len(got) != 1 || got[0].ID != "1" {
			t.Errorf("modes = %+v", got)
		}
	})
}

func TestChecksGetInfo(t *testing.T) {
	s, method, _ := capture(t, `{"active":true,"availableChecks":0,"availableSms":6,"currentChecks":1,`+
		`"currentSms":0,"expired":"29.10.2025","totalChecks":1,"totalSms":6,`+
		`"types":[{"code":"ping","id":"1","name":"Ping"}],`+
		`"intervals":[{"id":"1","name":"1 мин","time":"1"}],`+
		`"keywordModes":[{"id":"1","name":"m"}],`+
		`"ports":[{"name":"WWW","nameFull":"Web","value":"80"}],"tariff":null}`)
	got, err := s.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if *method != "getInfo" {
		t.Errorf("method = %q, want getInfo", *method)
	}
	if !got.Active || got.AvailableSMS != 6 || got.CurrentChecks != 1 {
		t.Errorf("counters = %+v", got)
	}
	if len(got.Types) != 1 || len(got.Intervals) != 1 || len(got.Ports) != 1 || len(got.KeywordModes) != 1 {
		t.Errorf("nested lists = %+v", got)
	}
}

func TestChecksGetFullCheckInfo(t *testing.T) {
	s, method, params := capture(t, `{"contacts":[{"id":3205,"name":"Тест","type":"email","value":"test@sweb.ru","verified":true}],`+
		`"id":1,"lastResult":true,"name":"sweb.ru","settings":[{"type":"target","value":"192.168.122.75"},`+
		`{"type":"interval","value":"4"}],"status":true,"type":1}`)
	got, err := s.GetFullCheckInfo(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetFullCheckInfo: %v", err)
	}
	if *method != "getFullCheckInfo" {
		t.Errorf("method = %q", *method)
	}
	var p struct {
		ID int `json:"id"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.ID != 1 {
		t.Errorf("id param = %s", *params)
	}
	if got.ID != 1 || got.Type != 1 || len(got.Settings) != 2 || len(got.Contacts) != 1 {
		t.Errorf("full info = %+v", got)
	}
	if got.Settings[0].Type != "target" || got.Contacts[0].Value != "test@sweb.ru" {
		t.Errorf("nested = %+v / %+v", got.Settings, got.Contacts)
	}
}

func TestChecksCreate(t *testing.T) {
	s, method, params := capture(t, `1`)
	err := s.Create(context.Background(), Spec{
		Type: 2, Target: "https://example.com", Name: "web", Interval: 3,
		ContactIDs: []int{3205}, SSL: true, Keywords: []string{"ok"}, KeywordMode: 1,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if *method != "create" {
		t.Errorf("method = %q, want create", *method)
	}
	var p struct {
		Type        int      `json:"type"`
		Target      string   `json:"target"`
		ContactIDs  []int    `json:"contactIds"`
		SSL         bool     `json:"ssl"`
		Keywords    []string `json:"keywords"`
		KeywordMode int      `json:"keywordMode"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.Type != 2 || p.Target != "https://example.com" || len(p.ContactIDs) != 1 || !p.SSL || len(p.Keywords) != 1 || p.KeywordMode != 1 {
		t.Errorf("params = %s", *params)
	}
}

func TestChecksCreateFailure(t *testing.T) {
	s, _, _ := capture(t, `0`)
	if err := s.Create(context.Background(), Spec{Type: 1, Target: "x", Name: "n", Interval: 1, ContactIDs: []int{1}}); err == nil {
		t.Fatal("Create: want error on result 0")
	}
}

func TestChecksEdit(t *testing.T) {
	s, method, params := capture(t, `1`)
	err := s.Edit(context.Background(), 42, Spec{
		Type: 3, Target: "1.2.3.4", Name: "port", Interval: 2, ContactIDs: []int{1}, Port: 443,
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if *method != "edit" {
		t.Errorf("method = %q, want edit", *method)
	}
	var p struct {
		ID   int `json:"id"`
		Port int `json:"port"`
		Type int `json:"type"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.ID != 42 || p.Port != 443 {
		t.Errorf("params = %s, want id=42 port=443", *params)
	}
	if p.Type != 0 {
		t.Errorf("edit must not send type, got %s", *params)
	}
}

func TestChecksToggleAndRemove(t *testing.T) {
	cases := []struct {
		name   string
		call   func(*Service) error
		method string
		bulk   bool
	}{
		{"Activate", func(s *Service) error { return s.Activate(context.Background(), 1) }, "activate", false},
		{"ActivateList", func(s *Service) error { return s.ActivateList(context.Background(), 1, 2) }, "activateList", true},
		{"Deactivate", func(s *Service) error { return s.Deactivate(context.Background(), 1) }, "deactivate", false},
		{"DeactivateList", func(s *Service) error { return s.DeactivateList(context.Background(), 1, 2) }, "deactivateList", true},
		{"Remove", func(s *Service) error { return s.Remove(context.Background(), 1) }, "remove", false},
		{"RemoveList", func(s *Service) error { return s.RemoveList(context.Background(), 1, 2) }, "removeList", true},
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
				ID  int   `json:"id"`
				IDs []int `json:"ids"`
			}
			_ = json.Unmarshal(*params, &p)
			if tc.bulk && len(p.IDs) != 2 {
				t.Errorf("bulk ids = %s, want 2", *params)
			}
			if !tc.bulk && p.ID != 1 {
				t.Errorf("id = %s, want 1", *params)
			}
		})
	}
}

func TestChecksActionFailure(t *testing.T) {
	s, _, _ := capture(t, `0`)
	if err := s.Activate(context.Background(), 1); err == nil {
		t.Fatal("Activate: want error on result 0")
	}
}

func TestChecksHistory(t *testing.T) {
	s, method, params := capture(t, `{"filterInfo":{"page":1,"perPage":20,"totalCount":1},`+
		`"list":[{"id":"7","check_id":"339","ts":"2025-01-01 00:00:00","success":"y"}]}`)
	got, err := s.History(context.Background(), 339, &HistoryOptions{StartDate: "2025-01-01", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if *method != "history" {
		t.Errorf("method = %q, want history", *method)
	}
	var p struct {
		ID        int    `json:"id"`
		StartDate string `json:"startDate"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.ID != 339 || p.StartDate != "2025-01-01" {
		t.Errorf("params = %s", *params)
	}
	if len(got.List) != 1 || got.List[0].CheckID != "339" || got.List[0].Success != "y" {
		t.Errorf("history = %+v", got)
	}
}
