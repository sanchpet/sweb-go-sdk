package backup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestBackupList(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"name":"backup_1","prettyName":"10 Jul 2026","unic_id":"u1","attach_type":"","updatedAt":"2026-07-10"}]}`))
	})
	list, err := s.List(context.Background(), "login_vps_1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "backup_1" || list[0].PrettyName != "10 Jul 2026" {
		t.Errorf("list = %+v, want one backup_1", list)
	}
}

func TestBackupCreate(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.Create(context.Background(), "login_vps_1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotMethod != "create" {
		t.Errorf("method = %q, want create", gotMethod)
	}
}

func TestBackupUpdateIndex(t *testing.T) {
	var gotMethod, gotBillingID string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				BillingID string `json:"billingId"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotBillingID = req.Method, req.Params.BillingID
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.UpdateIndex(context.Background(), "login_vps_1"); err != nil {
		t.Fatalf("UpdateIndex: %v", err)
	}
	if gotMethod != "updateIndex" || gotBillingID != "login_vps_1" {
		t.Errorf("method/billingId = %q/%q, want updateIndex/login_vps_1", gotMethod, gotBillingID)
	}
}

func TestBackupNameActions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		call   func(*Service) error
		method string
	}{
		{"Restore", func(s *Service) error { return s.Restore(context.Background(), "login_vps_1", "backup_1") }, "restore"},
		{"Attach", func(s *Service) error { return s.Attach(context.Background(), "login_vps_1", "backup_1") }, "attach"},
		{"Detach", func(s *Service) error { return s.Detach(context.Background(), "login_vps_1", "backup_1") }, "detach"},
		{"Remove", func(s *Service) error { return s.Remove(context.Background(), "login_vps_1", "backup_1") }, "remove"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotName string
			s := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
					Params struct {
						BackupName string `json:"backupName"`
					} `json:"params"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod, gotName = req.Method, req.Params.BackupName
				_, _ = w.Write([]byte(`{"result":1}`))
			})
			if err := tc.call(s); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method || gotName != "backup_1" {
				t.Errorf("method/name = %q/%q, want %s/backup_1", gotMethod, gotName, tc.method)
			}
		})
	}
}

func TestBackupSettings(t *testing.T) {
	// getSettings wraps the object in a one-element array; frequency/time nullable.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"mode":"auto","frequency":7,"time":3,"next_data_backup":"2026-07-17"}]}`))
	})
	set, err := s.Settings(context.Background(), "login_vps_1")
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if set == nil || set.Mode != "auto" || set.Frequency != 7 || set.Time != 3 {
		t.Errorf("settings = %+v, want auto/7/3", set)
	}
}

func TestSaveSettings(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Mode      string `json:"mode"`
		Frequency int    `json:"frequency"`
		Time      int    `json:"time"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.SaveSettings(context.Background(), "login_vps_1", "auto", 7, 3); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if gotMethod != "saveSettings" || gotParams.Mode != "auto" || gotParams.Frequency != 7 || gotParams.Time != 3 {
		t.Errorf("method/params = %q/%+v, want saveSettings / auto,7,3", gotMethod, gotParams)
	}
}

// serve spins up a mock JSON-RPC server for h and returns a backup.Service
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
