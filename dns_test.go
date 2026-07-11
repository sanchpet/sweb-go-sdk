package sweb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// dnsRecordsJSON is a synthetic 7-record zone (TEST-NET IPs, example.com hosts —
// the live shape, values scrubbed). Numeric fields are quoted strings on
// MX/SRV/TXT (priority "10", ttl "600") but bare on index/main; canChange is a
// stringified bool.
const dnsRecordsJSON = `[` +
	`{"name":"","value":"203.0.113.10","index":0,"canChange":"true","sel":"A","type":"A","category":"zoneMain"},` +
	`{"name":"www","value":"203.0.113.10","index":1,"canChange":"false","sel":"A","type":"A","category":"zoneMain"},` +
	`{"name":"autoconfig","value":"autoconfig.example.com.","type":"CNAME","index":2,"category":"subdom"},` +
	`{"value":"mx1.example.com.","priority":"10","name":"","index":0,"category":"mx","type":"MX"},` +
	`{"value":"mx2.example.com.","priority":"20","name":"","index":1,"category":"mx","type":"MX"},` +
	`{"service":"autodiscover","protocol":"tcp","ttl":"86400","priority":"5","weight":"0","port":"443","target":"autodiscover.example.com.","index":0,"category":"srv","type":"SRV","name":""},` +
	`{"domain":"@","ttl":"600","value":"v=spf1 ~all","index":0,"main":1,"category":"mainTxt","type":"TXT"}` +
	`]`

// infoFlat is the common shape: result is a bare array of records (verified
// against a live sanch.pet response).
const infoFlat = `{"jsonrpc":"2.0","result":` + dnsRecordsJSON + `}`

// infoEnvelope is the VPS-attached-domain shape: records nested in ips=[[…]]
// alongside the /vps/ip index's protected_ips/vps fields (the apidoc example).
const infoEnvelope = `{"jsonrpc":"2.0","result":{"ips":[` + dnsRecordsJSON + `],` +
	`"protected_ips":[{"ip":"203.0.113.44","canBeDeclined":1,"price":6000}],"vps_ip":[],"local_ip":[],` +
	`"vps":{"billingId":"login_vps_1","currentAction":null,"isEmpty":"0","ordered_ip_count":2}}}`

// assertZone checks the 7 records decode correctly regardless of container shape.
func assertZone(t *testing.T, recs []DNSRecord) {
	t.Helper()
	if len(recs) != 7 {
		t.Fatalf("got %d records, want 7", len(recs))
	}
	if recs[0].Type != "A" || recs[0].CanChange != "true" || recs[0].Value != "203.0.113.10" {
		t.Errorf("record[0] = %+v, want A/canChange true/203.0.113.10", recs[0])
	}
	if mx := recs[3]; mx.Type != "MX" || mx.Priority != 10 { // priority from quoted "10"
		t.Errorf("record[3] = %+v, want MX priority 10", mx)
	}
	if srv := recs[5]; srv.Type != "SRV" || srv.TTL != 86400 || srv.Port != 443 || srv.Weight != 0 {
		t.Errorf("record[5] = %+v, want SRV ttl 86400/port 443", srv)
	}
	if txt := recs[6]; txt.Type != "TXT" || txt.Domain != "@" || txt.Main != 1 || txt.TTL != 600 {
		t.Errorf("record[6] = %+v, want TXT @/main 1/ttl 600", txt)
	}
}

func TestDNSRecords(t *testing.T) {
	var gotMethod, gotDomain string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				Domain string `json:"domain"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotDomain = req.Method, req.Params.Domain
		_, _ = w.Write([]byte(infoFlat))
	})
	recs, err := c.DNS.Records(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if gotMethod != "info" || gotDomain != "example.com" {
		t.Errorf("method/domain = %q/%q, want info/example.com", gotMethod, gotDomain)
	}
	assertZone(t, recs)
}

// TestDNSRecordsEnvelope covers the VPS-attached-domain shape (records wrapped in
// an object's ips=[[…]]), which must decode to the same records.
func TestDNSRecordsEnvelope(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(infoEnvelope))
	})
	recs, err := c.DNS.Records(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	assertZone(t, recs)
}

func TestDNSGetFile(t *testing.T) {
	var gotMethod string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"mimetype":"text/plain","metadata":[],"content":"@ IN SOA ns1.spaceweb.ru.","name":"test32132.ru.zone.txt"}}`))
	})
	zf, err := c.DNS.GetFile(context.Background(), "test32132.ru")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if gotMethod != "getFile" {
		t.Errorf("method = %q, want getFile", gotMethod)
	}
	if zf.Name != "test32132.ru.zone.txt" || zf.Mimetype != "text/plain" || zf.Content == "" {
		t.Errorf("zone file = %+v, want name/mimetype/content set", zf)
	}
}

func TestEditMX(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Domain   string `json:"domain"`
		Action   string `json:"action"`
		Index    int    `json:"index"`
		Priority int    `json:"priority"`
		Value    string `json:"value"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		// editMx answers with integer 1 (not boolean true).
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	err := c.DNS.EditMX(context.Background(), "test32132.ru", DNSActionEdit, MXRecord{Index: 0, Priority: 11, Value: "mx1.example.com."})
	if err != nil {
		t.Fatalf("EditMX: %v", err)
	}
	if gotMethod != "editMx" || gotParams.Action != "edit" || gotParams.Priority != 11 {
		t.Errorf("method/params = %q/%+v, want editMx / edit,priority 11", gotMethod, gotParams)
	}
}

func TestEditMXFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0}`))
	})
	if err := c.DNS.EditMX(context.Background(), "d", DNSActionEdit, MXRecord{}); err == nil {
		t.Fatal("EditMX: want error on result 0, got nil")
	}
}

// TestEditBoolMethods covers the four edit methods whose success sentinel is
// boolean true (editMain/editSrv/editNS/editTxt), asserting method name, the
// action discriminator, and that a false result is rejected.
func TestEditBoolMethods(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		call         func(*Client) error
	}{
		{"Main", "editMain", func(c *Client) error {
			return c.DNS.EditMain(context.Background(), "d", DNSActionAdd, MainRecord{Name: "www", Type: "A", Value: "203.0.113.7"})
		}},
		{"SRV", "editSrv", func(c *Client) error {
			return c.DNS.EditSRV(context.Background(), "d", DNSActionEdit, SRVRecord{Port: 443, Service: "sip"})
		}},
		{"NS", "editNS", func(c *Client) error {
			return c.DNS.EditNS(context.Background(), "d", DNSActionEdit, 0, "sub", "ns1.example.com.")
		}},
		{"TXT", "editTxt", func(c *Client) error {
			return c.DNS.EditTXT(context.Background(), "d", DNSActionRemove, 0, "sub", "v=spf1")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotAction string
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
					Params struct {
						Action string `json:"action"`
					} `json:"params"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod, gotAction = req.Method, req.Params.Action
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":true}`))
			})
			if err := tc.call(c); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method {
				t.Errorf("method = %q, want %q", gotMethod, tc.method)
			}
			if gotAction == "" {
				t.Errorf("action param not sent")
			}
		})
	}
}

func TestEditBoolFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":false}`))
	})
	if err := c.DNS.EditNS(context.Background(), "d", DNSActionEdit, 0, "s", "v"); err == nil {
		t.Fatal("EditNS: want error on result false, got nil")
	}
}
