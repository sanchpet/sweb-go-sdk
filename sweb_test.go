package sweb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFacadeWiring proves New wires every service field over one shared
// transport (no nil field) and that the re-exported options are callable as
// sweb.WithToken(…) etc.
func TestFacadeWiring(t *testing.T) {
	c := New(
		WithBaseURL("https://example.invalid"),
		WithToken("tok"),
		WithHTTPClient(http.DefaultClient),
		WithCredentials("u", "p"),
		WithOnTokenRefresh(func(string) {}),
	)
	if c.VPS == nil || c.IP == nil || c.Backup == nil || c.RemoteBackup == nil ||
		c.DNS == nil || c.Domains == nil || c.Balancer == nil || c.DBaaS == nil ||
		c.SSL == nil || c.Monitoring == nil || c.MonitoringChecks == nil || c.MonitoringContacts == nil ||
		c.Mail == nil || c.HostingDB == nil || c.Sites == nil || c.VHSSL == nil ||
		c.VHBackup == nil || c.Cron == nil || c.DDoSGuard == nil || c.HostingLoad == nil ||
		c.SSH == nil || c.DiskUsage == nil || c.Tariff == nil || c.Pay == nil ||
		c.Persons == nil || c.Bonus == nil || c.PartnerProgram == nil || c.ReferralProgram == nil {
		t.Fatal("New left a service field nil")
	}
	if c.Token() != "tok" {
		t.Errorf("Token() = %q, want tok", c.Token())
	}
	if DefaultBaseURL == "" {
		t.Error("DefaultBaseURL is empty")
	}
}

// TestCreateToken confirms the facade delegates CreateToken to the transport's
// getToken exchange (/notAuthorized/, method getToken).
func TestCreateToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notAuthorized/" || r.Method != http.MethodPost {
			t.Errorf("got %s %s, want POST /notAuthorized/", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"tok_abc123"}`))
	}))
	t.Cleanup(srv.Close)

	c := New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	got, err := c.CreateToken(context.Background(), "user", "pass")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if got != "tok_abc123" {
		t.Errorf("token = %q, want tok_abc123", got)
	}
}
