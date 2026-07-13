package load

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestLoadPeriods(t *testing.T) {
	// index returns a bare array of {month, year} string pairs.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":[
			{"month":"11","year":"2022"},
			{"month":"12","year":"2022"},
			{"month":"1","year":"2023"}
		]}`))
	})
	periods, err := s.Periods(context.Background())
	if err != nil {
		t.Fatalf("Periods: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if len(periods) != 3 {
		t.Fatalf("periods len = %d, want 3", len(periods))
	}
	if periods[0].Month != "11" || periods[0].Year != "2022" {
		t.Errorf("periods[0] = %+v, want 11/2022", periods[0])
	}
	if periods[2].Month != "1" || periods[2].Year != "2023" {
		t.Errorf("periods[2] = %+v, want 1/2023", periods[2])
	}
}

func TestLoadTable(t *testing.T) {
	// getLoadTable returns a single-element array wrapping one Table; cpu arrives
	// quoted ("0.00") and mysql bare (0), and csv is an object (not the string[]
	// the spec's content descriptor claims).
	var gotMethod string
	var gotParams struct {
		Year  int    `json:"year"`
		Month int    `json:"month"`
		Type  string `json:"type"`
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
			"csv":{
				"content":"MjAyMy0wNi0wMTswLjAwCg==",
				"metadata":[],
				"mimetype":"application/csv;base64",
				"name":"loading_lina199302_6.csv"
			},
			"dbLevels":[2000,3000,6000,12000],
			"hostingLevels":[120,260,500,1000],
			"list":[
				{"cpu":"0.00","date":"2023-06-01","mysql":0},
				{"cpu":"1.50","date":"2023-06-02","mysql":3}
			]
		}]}`))
	})
	tbl, err := s.LoadTable(context.Background(), 2023, 6, "cpu")
	if err != nil {
		t.Fatalf("LoadTable: %v", err)
	}
	if gotMethod != "getLoadTable" {
		t.Errorf("method = %q, want getLoadTable", gotMethod)
	}
	if gotParams.Year != 2023 || gotParams.Month != 6 || gotParams.Type != "cpu" {
		t.Errorf("params = %+v, want year 2023 / month 6 / type cpu", gotParams)
	}
	if len(tbl.List) != 2 {
		t.Fatalf("list len = %d, want 2", len(tbl.List))
	}
	if tbl.List[0].Date != "2023-06-01" || tbl.List[0].CPU != 0 || tbl.List[0].Mysql != 0 {
		t.Errorf("list[0] = %+v, want 2023-06-01/0/0", tbl.List[0])
	}
	if tbl.List[1].CPU != 1.5 || tbl.List[1].Mysql != 3 {
		t.Errorf("list[1] = %+v, want cpu 1.5 / mysql 3", tbl.List[1])
	}
	if len(tbl.HostingLevels) != 4 || tbl.HostingLevels[0] != 120 || tbl.HostingLevels[3] != 1000 {
		t.Errorf("hostingLevels = %+v, want [120 260 500 1000]", tbl.HostingLevels)
	}
	if len(tbl.DBLevels) != 4 || tbl.DBLevels[0] != 2000 || tbl.DBLevels[3] != 12000 {
		t.Errorf("dbLevels = %+v, want [2000 3000 6000 12000]", tbl.DBLevels)
	}
	if tbl.CSV.Name != "loading_lina199302_6.csv" || tbl.CSV.Mimetype != "application/csv;base64" {
		t.Errorf("csv = %+v, want loading_lina199302_6.csv / application/csv;base64", tbl.CSV)
	}
	if tbl.CSV.Content == "" {
		t.Errorf("csv content empty, want base64 body")
	}
}

func TestLoadTableOmitsEmptyParams(t *testing.T) {
	// year/month 0 and type "" must be omitted, not sent as zero-valued keys.
	var params map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		params = req.Params
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	tbl, err := s.LoadTable(context.Background(), 0, 0, "")
	if err != nil {
		t.Fatalf("LoadTable: %v", err)
	}
	if _, ok := params["year"]; ok {
		t.Error("params carried a year key, want it omitted")
	}
	if _, ok := params["month"]; ok {
		t.Error("params carried a month key, want it omitted")
	}
	if _, ok := params["type"]; ok {
		t.Error("params carried a type key, want it omitted")
	}
	if len(tbl.List) != 0 {
		t.Errorf("empty result yielded %+v, want zero Table", tbl)
	}
}

// serve spins up a mock JSON-RPC server for h and returns a load.Service backed
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
