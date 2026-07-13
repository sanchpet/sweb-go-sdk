package cron

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestCronGetTasks(t *testing.T) {
	// getTasks returns a bare array; schedule fields arrive quoted.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":[{
			"command":"test","day":"1","hour":"12","minute":"30","month":"12",
			"task":"30 12 1 12 7 test","task_escaped":"30%2012%201%2012%207%20test","weekday":"7"
		}]}`))
	})
	tasks, err := s.GetTasks(context.Background())
	if err != nil {
		t.Fatalf("GetTasks: %v", err)
	}
	if gotMethod != "getTasks" {
		t.Errorf("method = %q, want getTasks", gotMethod)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks len = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.Minute != 30 || task.Hour != 12 || task.Day != 1 || task.Month != 12 || task.Weekday != 7 {
		t.Errorf("schedule = %+v, want 30/12/1/12/7", task)
	}
	if task.Command != "test" || task.Task != "30 12 1 12 7 test" {
		t.Errorf("task = %+v, want command test / task '30 12 1 12 7 test'", task)
	}
	if task.TaskEscaped != "30%2012%201%2012%207%20test" {
		t.Errorf("taskEscaped = %q, want the URL-escaped line", task.TaskEscaped)
	}
}

func TestCronAddTask(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Minute  int    `json:"minute"`
		Hour    int    `json:"hour"`
		Day     int    `json:"day"`
		Month   int    `json:"month"`
		Weekday int    `json:"weekday"`
		Command string `json:"command"`
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
	err := s.AddTask(context.Background(), Schedule{
		Minute: 30, Hour: 0, Day: 31, Month: 1, Weekday: 1, Command: "test",
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if gotMethod != "addTask" {
		t.Errorf("method = %q, want addTask", gotMethod)
	}
	if gotParams.Minute != 30 || gotParams.Day != 31 || gotParams.Month != 1 || gotParams.Command != "test" {
		t.Errorf("params = %+v, want minute 30 / day 31 / month 1 / command test", gotParams)
	}
}

func TestCronEditTask(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		OldTask string `json:"oldTask"`
		Minute  int    `json:"minute"`
		Weekday int    `json:"weekday"`
		Command string `json:"command"`
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
	err := s.EditTask(context.Background(), "30 12 1 12 7 test", Schedule{
		Minute: 30, Hour: 12, Day: 1, Month: 12, Weekday: 7, Command: "test",
	})
	if err != nil {
		t.Fatalf("EditTask: %v", err)
	}
	if gotMethod != "editTask" {
		t.Errorf("method = %q, want editTask", gotMethod)
	}
	if gotParams.OldTask != "30 12 1 12 7 test" {
		t.Errorf("oldTask = %q, want '30 12 1 12 7 test'", gotParams.OldTask)
	}
	if gotParams.Minute != 30 || gotParams.Weekday != 7 || gotParams.Command != "test" {
		t.Errorf("params = %+v, want minute 30 / weekday 7 / command test", gotParams)
	}
}

func TestCronRemoveTask(t *testing.T) {
	var gotMethod, gotTask string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				Task string `json:"task"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotTask = req.Method, req.Params.Task
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.RemoveTask(context.Background(), "30 0 31 1 1 test"); err != nil {
		t.Fatalf("RemoveTask: %v", err)
	}
	if gotMethod != "removeTask" || gotTask != "30 0 31 1 1 test" {
		t.Errorf("method/task = %q/%q, want removeTask/'30 0 31 1 1 test'", gotMethod, gotTask)
	}
}

func TestCronSentinelFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.RemoveTask(context.Background(), "x"); err == nil {
		t.Error("RemoveTask with result 0: got nil error, want failure")
	}
}

// serve spins up a mock JSON-RPC server for h and returns a cron.Service
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
