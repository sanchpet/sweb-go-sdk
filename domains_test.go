package sweb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// indexJSON is a synthetic "index" result: one plain domain with a wildcard
// subdomain (fields per the apidoc, values scrubbed to example.com).
const indexJSON = `{"jsonrpc":"2.0","result":[` +
	`{"fqdn":"example.com","fqdn_readable":"example.com","siteAlias":"default",` +
	`"docroot":"/home/e/example","fqdn_tech":"example.com.swtest.ru","reg_price":199,` +
	`"bonus_available":false,"subdomains":[` +
	`{"docroot":"/home/e/example","machine":"*","machine_readable":"*","siteAlias":"default",` +
	`"fqdn":"*.example.com","fqdn_readable":"*.example.com","parent_fqdn":"example.com",` +
	`"parent_fqdn_readable":"example.com"}]}` +
	`]}`

func TestDomainsList(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Type         string `json:"type"`
		ShowPackages bool   `json:"showPackages"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(indexJSON))
	})
	domains, err := c.Domains.List(context.Background(), &DomainListOptions{Type: "all"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "index" || gotParams.Type != "all" {
		t.Errorf("method/type = %q/%q, want index/all", gotMethod, gotParams.Type)
	}
	if len(domains) != 1 {
		t.Fatalf("got %d domains, want 1", len(domains))
	}
	d := domains[0]
	if d.FQDNReadable != "example.com" || d.RegPrice != 199 || d.BonusAvailable {
		t.Errorf("domain = %+v, want example.com/199/bonus false", d)
	}
	if len(d.Subdomains) != 1 || d.Subdomains[0].Machine != "*" || d.Subdomains[0].ParentFQDN != "example.com" {
		t.Errorf("subdomains = %+v, want one wildcard under example.com", d.Subdomains)
	}
}

// getDomainInfoJSON exercises the awkward corners: nullable strings (registrar,
// transferLink, redirectUrl), a negative transfer_price, real booleans, and the
// nested prolong_confirm object.
const getDomainInfoJSON = `{"jsonrpc":"2.0","result":{` +
	`"is_active_task":0,"autoreg":0,"is_taken":0,"registrar":null,"is_our":1,` +
	`"expired":"2027-01-01","can_prolong":1,"prolong_price":399,"prolong_by_bonus":true,` +
	`"prolong_confirm":{"domain":"example.com","confirm":false,"price":399,"link":null},` +
	`"reg_price":199,"transfer_price":-1,"autoprolong":"no","docRoot":"/home/e/example",` +
	`"siteAlias":"default","bonus_available":true,"transferLink":null,"redirectUrl":null}}`

func TestDomainsInfo(t *testing.T) {
	var gotMethod string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(getDomainInfoJSON))
	})
	info, err := c.Domains.Info(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if gotMethod != "getDomainInfo" {
		t.Errorf("method = %q, want getDomainInfo", gotMethod)
	}
	if info.IsOur != 1 || info.Registrar != "" || info.TransferPrice != -1 || !info.ProlongByBonus {
		t.Errorf("info = %+v, want isOur 1 / registrar \"\" / transfer -1 / prolongByBonus", info)
	}
	if info.DocRoot != "/home/e/example" || info.RedirectURL != "" {
		t.Errorf("info docRoot/redirect = %q/%q", info.DocRoot, info.RedirectURL)
	}
	if info.ProlongConfirm == nil || info.ProlongConfirm.Price != 399 || info.ProlongConfirm.Link != "" {
		t.Errorf("prolongConfirm = %+v, want price 399 / link \"\"", info.ProlongConfirm)
	}
}

func TestDomainsInfoNilProlongConfirm(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"is_our":0,"prolong_confirm":null,"autoprolong":"no"}}`))
	})
	info, err := c.Domains.Info(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.ProlongConfirm != nil {
		t.Errorf("prolongConfirm = %+v, want nil for JSON null", info.ProlongConfirm)
	}
}

func TestDomainsSubdomains(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":[{"value":"*.example.com","name":"*.example.com"}]}`))
	})
	subs, err := c.Domains.Subdomains(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Subdomains: %v", err)
	}
	if len(subs) != 1 || subs[0].Value != "*.example.com" {
		t.Errorf("subs = %+v, want one *.example.com", subs)
	}
}

// TestDomainsAvailablePackages covers the quoted-string numerics (id/price/priority
// arrive as strings, decoded through FlexInt/FlexFloat).
func TestDomainsAvailablePackages(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":[{"id":"3","name_readable":".ru+.shop",` +
			`"price":"249","price2":"398","priority":"20","available":true,"order_package_id":696,` +
			`"domains":[{"name":"example.ru","name_readable":"example.ru"},` +
			`{"name":"example.shop","name_readable":"example.shop"}]}]}`))
	})
	pkgs, err := c.Domains.AvailablePackages(context.Background(), "example.ru", "example.shop")
	if err != nil {
		t.Fatalf("AvailablePackages: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("got %d packages, want 1", len(pkgs))
	}
	p := pkgs[0]
	if p.ID != 3 || p.Price != 249 || p.Priority != 20 || p.OrderPackageID != 696 || len(p.Domains) != 2 {
		t.Errorf("package = %+v, want id 3 / price 249 / priority 20 / order 696 / 2 domains", p)
	}
}

func TestDomainsQueryFlags(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result string
		want   bool
	}{
		{"available", "1", true},
		{"unavailable", "0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + tc.result + `}`))
			})
			ok, err := c.Domains.RegAvailable(context.Background(), "example.com", PayBalance)
			if err != nil {
				t.Fatalf("RegAvailable: %v", err)
			}
			if ok != tc.want {
				t.Errorf("RegAvailable = %v, want %v", ok, tc.want)
			}
			// TransferAvailable shares the queryFlag helper.
			ok, err = c.Domains.TransferAvailable(context.Background(), "example.com")
			if err != nil {
				t.Fatalf("TransferAvailable: %v", err)
			}
			if ok != tc.want {
				t.Errorf("TransferAvailable = %v, want %v", ok, tc.want)
			}
		})
	}
}

func TestDomainsRegistrationPrice(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		// Doc types it as string; the example returns a bare number. FlexFloat takes both.
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":199}`))
	})
	price, err := c.Domains.RegistrationPrice(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("RegistrationPrice: %v", err)
	}
	if price != 199 {
		t.Errorf("price = %v, want 199", price)
	}
}

// TestDomainsActionOne covers the 1=success mutating methods and the non-1 failure.
func TestDomainsActionOne(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Domain      string `json:"domain"`
		ProlongType string `json:"prolongType"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	if err := c.Domains.ChangeProlong(context.Background(), "example.com", ProlongBonusMoney); err != nil {
		t.Fatalf("ChangeProlong: %v", err)
	}
	if gotMethod != "changeProlong" || gotParams.ProlongType != "bonus_money" {
		t.Errorf("method/prolong = %q/%q, want changeProlong/bonus_money", gotMethod, gotParams.ProlongType)
	}
}

func TestDomainsActionOneFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0}`))
	})
	if err := c.Domains.Remove(context.Background(), "example.com"); err == nil {
		t.Fatal("Remove: want error on result 0, got nil")
	}
}

func TestDomainsMoveSuccess(t *testing.T) {
	var gotMethod string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	if err := c.Domains.Move(context.Background(), "example.com", ProlongBonusMoney, "/home/e/example"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if gotMethod != "move" {
		t.Errorf("method = %q, want move", gotMethod)
	}
}

// TestDomainsMoveExtendedFailure covers the failure branch: move answers an
// ExtendedResult (non-1 code) carrying per-domain errors, surfaced as an error.
func TestDomainsMoveExtendedFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"code":0,"message":"failed",` +
			`"data":[["example.com","already on another account"]]}}`))
	})
	err := c.Domains.Move(context.Background(), "example.com", ProlongNo, "")
	if err == nil {
		t.Fatal("Move: want error on code 0 ExtendedResult, got nil")
	}
}

func TestDomainsMoveListParams(t *testing.T) {
	var gotDomains []MoveItem
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Domains []MoveItem `json:"domains"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotDomains = req.Params.Domains
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	err := c.Domains.MoveList(context.Background(),
		MoveItem{FQDN: "a.com", ProlongType: ProlongManual, Dir: "/home/a"},
		MoveItem{FQDN: "b.com", ProlongType: ProlongBonusMoney},
	)
	if err != nil {
		t.Fatalf("MoveList: %v", err)
	}
	if len(gotDomains) != 2 || gotDomains[0].FQDN != "a.com" || gotDomains[1].ProlongType != "bonus_money" {
		t.Errorf("domains = %+v, want a.com/manual + b.com/bonus_money", gotDomains)
	}
}

// TestDomainsProlongList covers the always-wrapped {"extendedResult":{…}} envelope.
func TestDomainsProlongList(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"extendedResult":` +
			`{"code":1,"message":"domains prolonged","data":[]}}}`))
	})
	er, err := c.Domains.ProlongList(context.Background(), "a.com", "b.com")
	if err != nil {
		t.Fatalf("ProlongList: %v", err)
	}
	if er == nil || er.Code != 1 || er.Message != "domains prolonged" {
		t.Errorf("extendedResult = %+v, want code 1 / message set", er)
	}
}

func TestDomainsProlongListFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"extendedResult":` +
			`{"code":0,"message":"insufficient balance","data":[]}}}`))
	})
	er, err := c.Domains.ProlongList(context.Background(), "a.com")
	if err == nil {
		t.Fatal("ProlongList: want error on code 0, got nil")
	}
	if er == nil || er.Message != "insufficient balance" {
		t.Errorf("extendedResult = %+v, want message returned alongside the error", er)
	}
}

func TestDomainsSubdomainAndRedirect(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		call         func(*Client) error
	}{
		{"CreateSubdomain", "createSubdomain", func(c *Client) error {
			return c.Domains.CreateSubdomain(context.Background(), "example.com", "test1", "/test")
		}},
		{"RemoveSubdomain", "removeSubdomain", func(c *Client) error {
			return c.Domains.RemoveSubdomain(context.Background(), "example.com", "test1")
		}},
		{"SetRedirect", "setRedirectVh", func(c *Client) error {
			return c.Domains.SetRedirect(context.Background(), "example.com", "https://example.org")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod = req.Method
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
			})
			if err := tc.call(c); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method {
				t.Errorf("method = %q, want %q", gotMethod, tc.method)
			}
		})
	}
}

func TestDomainsGetRedirect(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"https://example.org"}`))
	})
	url, err := c.Domains.Redirect(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Redirect: %v", err)
	}
	if url != "https://example.org" {
		t.Errorf("url = %q, want https://example.org", url)
	}
}
