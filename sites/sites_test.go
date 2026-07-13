package sites

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestSitesList(t *testing.T) {
	// index returns a bare array; id arrives quoted, antivirus* as bare numbers.
	var gotMethod string
	var gotParams struct {
		Page    int    `json:"page"`
		PerPage int    `json:"perPage"`
		Filter  string `json:"filter"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":[
			{"alias":"Site 3","antivirusActive":0,"antivirusAvailable":true,
			 "antivirusExpired":null,"antivirusPrice":199,"docRoot":"/test",
			 "docRootFull":"/home/i/imalysheva/test","domainTech":null,"id":"105394",
			 "redisSessionEnabled":false,"redisSessionSelected":false},
			{"alias":"Site 10","antivirusActive":1,"antivirusAvailable":true,
			 "antivirusExpired":null,"antivirusPrice":199,"docRoot":"/dir2",
			 "docRootFull":"/home/i/imalysheva/dir2","domainTech":null,"id":"105417",
			 "redisSessionEnabled":true,"redisSessionSelected":true}
		]}`))
	})
	list, err := s.List(context.Background(), &ListOptions{Page: 1, PerPage: 20, Filter: "SITE"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if gotParams.Page != 1 || gotParams.PerPage != 20 || gotParams.Filter != "SITE" {
		t.Errorf("params = %+v, want page 1 / perPage 20 / filter SITE", gotParams)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list[0].ID != 105394 || list[0].Alias != "Site 3" || list[0].AntivirusPrice != 199 {
		t.Errorf("site[0] = %+v, want id 105394 / Site 3 / price 199", list[0])
	}
	if list[0].DomainTech != "" || list[0].AntivirusExpired != "" {
		t.Errorf("site[0] nullables = %+v, want empty domainTech/antivirusExpired", list[0])
	}
	if list[1].ID != 105417 || list[1].AntivirusActive != 1 || !list[1].RedisSessionEnabled {
		t.Errorf("site[1] = %+v, want id 105417 / active 1 / redis on", list[1])
	}
}

func TestSitesListNilOptions(t *testing.T) {
	// nil options must omit page/perPage/filter entirely.
	var params map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		params = req.Params
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	if _, err := s.List(context.Background(), nil); err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, k := range []string{"page", "perPage", "filter"} {
		if _, ok := params[k]; ok {
			t.Errorf("params carried empty %q key, want it omitted", k)
		}
	}
}

func TestSitesGetSiteInfo(t *testing.T) {
	// backEndId arrives quoted; domains is a string array; many bools.
	var gotMethod, gotDocRoot string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				DocRoot string `json:"docRoot"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotDocRoot = req.Method, req.Params.DocRoot
		_, _ = w.Write([]byte(`{"result":{
			"backEnd":"Apache 2.2 + PHP 7.1 (current)","backEndId":"8",
			"domains":["example.ru"],"encoding":"UTF-8","program":[],
			"redisAvailable":true,"redisBackendAvailable":true,"redisCanEnableSession":true,
			"redisEnabled":true,"redisNeedTransfer":false,"redisSessionEnabled":false,
			"redisSessionSelected":false,"runScripts":true,"viewFiles":false
		}}`))
	})
	info, err := s.GetSiteInfo(context.Background(), "/dir")
	if err != nil {
		t.Fatalf("GetSiteInfo: %v", err)
	}
	if gotMethod != "getSiteInfo" || gotDocRoot != "/dir" {
		t.Errorf("method/docRoot = %q/%q, want getSiteInfo//dir", gotMethod, gotDocRoot)
	}
	if info.BackEndID != 8 || info.Encoding != "UTF-8" {
		t.Errorf("info = %+v, want backEndId 8 / UTF-8", info)
	}
	if !info.RedisAvailable || !info.RunScripts || info.ViewFiles {
		t.Errorf("flags = %+v, want redisAvailable/runScripts true, viewFiles false", info)
	}
	if len(info.Domains) != 1 || info.Domains[0] != "example.ru" || len(info.Program) != 0 {
		t.Errorf("domains/program = %+v/%+v, want [example.ru]/[]", info.Domains, info.Program)
	}
}

func TestSitesBackEndsList(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":[
			{"id":32,"name":"Apache 2.4 + PHP 8.0 Bitrix"},
			{"id":23,"name":"Apache 2.4 + PHP 8.1 opcache"}
		]}`))
	})
	list, err := s.BackEndsList(context.Background())
	if err != nil {
		t.Fatalf("BackEndsList: %v", err)
	}
	if gotMethod != "getBackEndsList" {
		t.Errorf("method = %q, want getBackEndsList", gotMethod)
	}
	if len(list) != 2 || list[0].ID != 32 || list[1].ID != 23 {
		t.Fatalf("list = %+v, want ids 32 and 23", list)
	}
	if list[0].Name != "Apache 2.4 + PHP 8.0 Bitrix" {
		t.Errorf("name = %q, want Apache 2.4 + PHP 8.0 Bitrix", list[0].Name)
	}
}

func TestSitesAdd(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Alias              string `json:"alias"`
		DocRoot            string `json:"docRoot"`
		Domain             string `json:"domain"`
		Machine            string `json:"machine"`
		EnableRedisSession bool   `json:"enableRedisSession"`
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
	err := s.Add(context.Background(), AddOptions{
		Alias:              "My site",
		DocRoot:            "/dir",
		Domain:             "mysite.ru",
		Machine:            "subdomain",
		EnableRedisSession: true,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if gotMethod != "add" {
		t.Errorf("method = %q, want add", gotMethod)
	}
	if gotParams.Alias != "My site" || gotParams.DocRoot != "/dir" || gotParams.Domain != "mysite.ru" {
		t.Errorf("params = %+v, want My site / /dir / mysite.ru", gotParams)
	}
	if gotParams.Machine != "subdomain" || !gotParams.EnableRedisSession {
		t.Errorf("optional params = %+v, want machine subdomain / redis true", gotParams)
	}
}

func TestSitesAddMachineOmittedWhenEmpty(t *testing.T) {
	var hasMachine bool
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_, hasMachine = req.Params["machine"]
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Add(context.Background(), AddOptions{Alias: "s", DocRoot: "/d", Domain: "d.ru"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if hasMachine {
		t.Errorf("params carried an empty machine key, want it omitted")
	}
}

func TestSitesEdit(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		DocRoot    string `json:"docRoot"`
		Alias      string `json:"alias"`
		DocRootNew string `json:"docRootNew"`
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
	if err := s.Edit(context.Background(), "/dir", "New name", "newDir"); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if gotMethod != "edit" {
		t.Errorf("method = %q, want edit", gotMethod)
	}
	if gotParams.DocRoot != "/dir" || gotParams.Alias != "New name" || gotParams.DocRootNew != "newDir" {
		t.Errorf("params = %+v, want /dir / New name / newDir", gotParams)
	}
}

func TestSitesEditDocRootNewOmittedWhenEmpty(t *testing.T) {
	var hasDocRootNew bool
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_, hasDocRootNew = req.Params["docRootNew"]
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Edit(context.Background(), "/dir", "name", ""); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if hasDocRootNew {
		t.Errorf("params carried an empty docRootNew key, want it omitted")
	}
}

func TestSitesDel(t *testing.T) {
	var gotMethod, gotDocRoot string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				DocRoot string `json:"docRoot"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotDocRoot = req.Method, req.Params.DocRoot
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Del(context.Background(), "/dir"); err != nil {
		t.Fatalf("Del: %v", err)
	}
	if gotMethod != "del" || gotDocRoot != "/dir" {
		t.Errorf("method/docRoot = %q/%q, want del//dir", gotMethod, gotDocRoot)
	}
}

func TestSitesChangeDomainSite(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Domain  string `json:"domain"`
		DocRoot string `json:"docRoot"`
		Machine string `json:"machine"`
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
	if err := s.ChangeDomainSite(context.Background(), "mysite.ru", "/dir", "sub"); err != nil {
		t.Fatalf("ChangeDomainSite: %v", err)
	}
	if gotMethod != "changeDomainSite" {
		t.Errorf("method = %q, want changeDomainSite", gotMethod)
	}
	if gotParams.Domain != "mysite.ru" || gotParams.DocRoot != "/dir" || gotParams.Machine != "sub" {
		t.Errorf("params = %+v, want mysite.ru / /dir / sub", gotParams)
	}
}

func TestSitesChangeDomainSiteMachineOmittedWhenEmpty(t *testing.T) {
	var hasMachine bool
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_, hasMachine = req.Params["machine"]
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.ChangeDomainSite(context.Background(), "mysite.ru", "/dir", ""); err != nil {
		t.Fatalf("ChangeDomainSite: %v", err)
	}
	if hasMachine {
		t.Errorf("params carried an empty machine key, want it omitted")
	}
}

func TestSitesChangeBackEnd(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		DocRoot   string `json:"docRoot"`
		BackEndID int    `json:"backEndId"`
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
	if err := s.ChangeBackEnd(context.Background(), "/dir", 23); err != nil {
		t.Fatalf("ChangeBackEnd: %v", err)
	}
	if gotMethod != "changeBackEnd" {
		t.Errorf("method = %q, want changeBackEnd", gotMethod)
	}
	if gotParams.DocRoot != "/dir" || gotParams.BackEndID != 23 {
		t.Errorf("params = %+v, want /dir / 23", gotParams)
	}
}

func TestSitesSentinelFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.Del(context.Background(), "/dir"); err == nil {
		t.Error("Del with result 0: got nil error, want failure")
	}
}

// serve spins up a mock JSON-RPC server for h and returns a sites.Service backed
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
