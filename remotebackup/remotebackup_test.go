package remotebackup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestRemoteBackupList(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":42,"billing_id":"login_vps_1","size":1024,"status":"ready","name":"cb1","comment":"nightly","price":90}]}`))
	})
	list, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != 42 || list[0].Name != "cb1" || list[0].Status != "ready" {
		t.Errorf("list = %+v, want one cb1/42", list)
	}
}

func TestRemoteBackupCreate(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		Name      string `json:"name"`
		Comment   string `json:"comment"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":{"id":43}}`))
	})
	if _, err := s.Create(context.Background(), "login_vps_1", "cb2", "manual"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotMethod != "create" || gotParams.Name != "cb2" || gotParams.Comment != "manual" {
		t.Errorf("method/params = %q/%+v, want create / cb2,manual", gotMethod, gotParams)
	}
}

func TestRemoteBackupActions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		call   func(*Service) error
		method string
	}{
		{"EditComment", func(s *Service) error { return s.EditComment(context.Background(), 42, "x") }, "editComment"},
		{"Restore", func(s *Service) error { return s.Restore(context.Background(), 42) }, "restore"},
		{"RestoreInto", func(s *Service) error { return s.RestoreInto(context.Background(), 42, "login_vps_2") }, "restoreInto"},
		{"Remove", func(s *Service) error { return s.Remove(context.Background(), 42) }, "remove"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			var gotID int
			s := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
					Params struct {
						RemoteBackupID int `json:"remoteBackupId"`
					} `json:"params"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod, gotID = req.Method, req.Params.RemoteBackupID
				_, _ = w.Write([]byte(`{"result":1}`))
			})
			if err := tc.call(s); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method || gotID != 42 {
				t.Errorf("method/id = %q/%d, want %s/42", gotMethod, gotID, tc.method)
			}
		})
	}
}

// serve spins up a mock JSON-RPC server for h and returns a remotebackup.Service
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
