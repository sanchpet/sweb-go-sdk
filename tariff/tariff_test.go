package tariff

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestTariffIndex(t *testing.T) {
	// index wraps a single record in a one-element array; info numbers arrive
	// bare, real counters arrive quoted, and quota is a locale comma-decimal
	// string ("0,00") kept unparsed.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":[{
			"info":{"category":"standart","duration":12,"mail_quota":0,"mysql":512,
				"name":"Ракета","plan_id":7112,"postgresql":512,"price":339,
				"price_12":3348,"price_6":0,"quota":10000,"site":10},
			"real":{"firebird":"0","mail_quota":"0,00","mailbox":"1","mysql":"0",
				"noHosting":0,"postgresql":"0","prolongChangeDisable":false,
				"prolongDuration":12,"prolongPrice":3348,"quota":"0,00",
				"realDuration":1,"realPrice":339,"site":"1"}
		}]}`))
	})
	tf, err := s.Index(context.Background())
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if tf == nil {
		t.Fatal("Index returned nil tariff")
	}
	if tf.Info.PlanID != 7112 || tf.Info.Name != "Ракета" || tf.Info.Quota != 10000 || tf.Info.Site != 10 {
		t.Errorf("info = %+v, want plan 7112 / Ракета / quota 10000 / site 10", tf.Info)
	}
	if tf.Info.Price != 339 || tf.Info.Price12 != 3348 || tf.Info.Duration != 12 {
		t.Errorf("price ladder = %+v, want 339 / 3348 / duration 12", tf.Info)
	}
	if tf.Usage.Quota != "0,00" || tf.Usage.MailQuota != "0,00" {
		t.Errorf("usage quota = %q/%q, want locale comma \"0,00\" kept as string", tf.Usage.Quota, tf.Usage.MailQuota)
	}
	if tf.Usage.Mailbox != 1 || tf.Usage.RealPrice != 339 || tf.Usage.Site != 1 {
		t.Errorf("usage counters = %+v, want mailbox 1 / realPrice 339 / site 1", tf.Usage)
	}
	if tf.Usage.NoHosting != 0 || tf.Usage.ProlongChangeDisable {
		t.Errorf("usage flags = %+v, want noHosting 0 / prolongChangeDisable false", tf.Usage)
	}
}

func TestTariffIndexEmpty(t *testing.T) {
	// An empty array unwraps to a nil tariff, not a panic.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	tf, err := s.Index(context.Background())
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if tf != nil {
		t.Errorf("Index on empty array = %+v, want nil", tf)
	}
}

func TestTariffServerInfo(t *testing.T) {
	// serverInfo wraps a single record in a one-element array; backend drifts
	// from the spec's "string" to an array of backends, port arrives quoted,
	// and absent stacks (python/ruby) are empty strings.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":[{
			"apache":"2.2.29","ip":"203.0.113.7","mysql":"5.7.27","name":"VH293",
			"os":"Linux 3.10","perl":"5.20.2","python":"","ruby":"",
			"backend":[
				{"descr":"Apache 2.4 + PHP 8.1 opcache","id":23,"php_info":"https://vh293.example.ru/phpinfo.php81","port":"8093","release_version":"3.0gamma","type":"php8.1"},
				{"descr":"Apache 2.2 + PHP 5.3 (legacy)","id":2,"php_info":"https://vh293.example.ru/phpinfo.php53","port":"8081","type":"php5.3"}
			]
		}]}`))
	})
	si, err := s.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if gotMethod != "serverInfo" {
		t.Errorf("method = %q, want serverInfo", gotMethod)
	}
	if si == nil {
		t.Fatal("ServerInfo returned nil")
	}
	if si.Name != "VH293" || si.IP != "203.0.113.7" || si.OS != "Linux 3.10" || si.MySQL != "5.7.27" {
		t.Errorf("server = %+v, want VH293 / 203.0.113.7 / Linux 3.10 / 5.7.27", si)
	}
	if si.Python != "" || si.Ruby != "" {
		t.Errorf("absent stacks = python %q / ruby %q, want empty", si.Python, si.Ruby)
	}
	if len(si.Backend) != 2 {
		t.Fatalf("backend len = %d, want 2", len(si.Backend))
	}
	if si.Backend[0].ID != 23 || si.Backend[0].Type != "php8.1" || si.Backend[0].Port != 8093 {
		t.Errorf("backend[0] = %+v, want id 23 / php8.1 / port 8093", si.Backend[0])
	}
	if si.Backend[1].ReleaseVersion != "" {
		t.Errorf("legacy backend release = %q, want empty (absent field)", si.Backend[1].ReleaseVersion)
	}
}

func TestTariffServerInfoEmpty(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	si, err := s.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if si != nil {
		t.Errorf("ServerInfo on empty array = %+v, want nil", si)
	}
}

// serve spins up a mock JSON-RPC server for h and returns a tariff.Service
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
