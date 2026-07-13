package diskusage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestDiskUsageList(t *testing.T) {
	// index returns a bare Usage object; numeric fields arrive polymorphic
	// (bare and quoted) and decode through flex.Float/flex.Int.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":{
			"tariffQuota":5000,"realQuota":"1","dbQuota":0,
			"mailQuota":0,"filesQuota":1,"filesNum":36,"subscription":null
		}}`))
	})
	list, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	u := list[0]
	if u.TariffQuota != 5000 || u.RealQuota != 1 || u.FilesNum != 36 {
		t.Errorf("usage = %+v, want tariffQuota 5000 / realQuota 1 / filesNum 36", u)
	}
	if u.DBQuota != 0 || u.FilesQuota != 1 {
		t.Errorf("usage = %+v, want dbQuota 0 / filesQuota 1", u)
	}
}

func TestDiskUsageTasksInfo(t *testing.T) {
	// getTasksInfo returns an object; activeTasksCount arrives quoted ("0").
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":{"activeTasksCount":"0","lastDoneTaskDate":"2023-02-28 23:52:26"}}`))
	})
	info, err := s.TasksInfo(context.Background())
	if err != nil {
		t.Fatalf("TasksInfo: %v", err)
	}
	if gotMethod != "getTasksInfo" {
		t.Errorf("method = %q, want getTasksInfo", gotMethod)
	}
	if info.ActiveTasksCount != 0 || info.LastDoneTaskDate != "2023-02-28 23:52:26" {
		t.Errorf("info = %+v, want activeTasksCount 0 / lastDoneTaskDate 2023-02-28 23:52:26", info)
	}
}

func TestDiskUsageEmail(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":"test@gmail.com"}`))
	})
	email, err := s.Email(context.Background())
	if err != nil {
		t.Fatalf("Email: %v", err)
	}
	if gotMethod != "getEmail" {
		t.Errorf("method = %q, want getEmail", gotMethod)
	}
	if email != "test@gmail.com" {
		t.Errorf("email = %q, want test@gmail.com", email)
	}
}

func TestDiskUsageStartTask(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.StartTask(context.Background()); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if gotMethod != "startTask" {
		t.Errorf("method = %q, want startTask", gotMethod)
	}
}

func TestDiskUsageChangeEmail(t *testing.T) {
	var gotMethod, gotEmail string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				Email string `json:"email"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotEmail = req.Method, req.Params.Email
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.ChangeEmail(context.Background(), "newtestemail@gmail.com"); err != nil {
		t.Fatalf("ChangeEmail: %v", err)
	}
	if gotMethod != "changeEmail" || gotEmail != "newtestemail@gmail.com" {
		t.Errorf("method/email = %q/%q, want changeEmail/newtestemail@gmail.com", gotMethod, gotEmail)
	}
}

func TestDiskUsageSentinelFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.StartTask(context.Background()); err == nil {
		t.Error("StartTask with result 0: got nil error, want failure")
	}
}

// serve spins up a mock JSON-RPC server for h and returns a diskusage.Service
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
