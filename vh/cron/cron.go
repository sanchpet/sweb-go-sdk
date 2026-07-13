// Package cron groups shared-hosting crontab operations (endpoint /vh/cron):
// listing the account's cron tasks plus the add/edit/remove lifecycle. All
// calls dispatch through the shared transport.
package cron

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const cronEndpoint = "/vh/cron"

// Service groups shared-hosting crontab operations (endpoint /vh/cron):
// GetTasks plus the AddTask/EditTask/RemoveTask lifecycle.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Task is one crontab entry as returned by GetTasks. The schedule fields are
// the five cron positions; Command is the command line to run.
//
// Types are reconciled against the spec's recorded example: the schedule fields
// arrive quoted ("30", "12", …) so they decode through flex.Int. Task is the
// job identifier — the raw crontab line ("30 12 1 12 7 test"), which is the
// value editTask/removeTask expect back as oldTask/task. TaskEscaped is the
// same value URL-escaped, present only for correct display of special
// characters.
type Task struct {
	Minute      flex.Int `json:"minute"`  // 0..59
	Hour        flex.Int `json:"hour"`    // 0..23
	Day         flex.Int `json:"day"`     // 1..31
	Month       flex.Int `json:"month"`   // 0..12
	Weekday     flex.Int `json:"weekday"` // 0..7 (0 and 7 both = Sunday)
	Command     string   `json:"command"`
	Task        string   `json:"task"`         // job id = the raw crontab line
	TaskEscaped string   `json:"task_escaped"` // display-only, URL-escaped
}

// Schedule is the five cron positions shared by AddTask and EditTask.
// Ranges follow the API: Minute 0..59, Hour 0..23, Day 1..31, Month 0..12,
// Weekday 0..7 (0 and 7 both mean Sunday).
type Schedule struct {
	Minute  int
	Hour    int
	Day     int
	Month   int
	Weekday int
	Command string // command line to run
}

func (sc Schedule) params() map[string]any {
	return map[string]any{
		"minute":  sc.Minute,
		"hour":    sc.Hour,
		"day":     sc.Day,
		"month":   sc.Month,
		"weekday": sc.Weekday,
		"command": sc.Command,
	}
}

// GetTasks returns the account's cron tasks (method "getTasks"). Read-only.
func (s *Service) GetTasks(ctx context.Context) ([]Task, error) {
	var out []Task
	if err := s.t.Call(ctx, cronEndpoint, "getTasks", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddTask adds a cron task (method "addTask"). MUTATING. Returns on the 1/0
// sentinel (1 = success).
//
// The spec declares positional params (minute, hour, day, month, weekday,
// command); consistent with the rest of the SDK (balancer, dns, …) and the
// live API, they are sent as a named-key object rather than a JSON array.
func (s *Service) AddTask(ctx context.Context, sc Schedule) error {
	return s.sentinelAction(ctx, "addTask", sc.params())
}

// EditTask replaces an existing cron task (method "editTask"). MUTATING.
// oldTask identifies the entry to change — it is the Task.Task value (the raw
// crontab line) of the entry as returned by GetTasks; sc is the new schedule.
// Returns on the 1/0 sentinel (1 = success).
func (s *Service) EditTask(ctx context.Context, oldTask string, sc Schedule) error {
	params := sc.params()
	params["oldTask"] = oldTask
	return s.sentinelAction(ctx, "editTask", params)
}

// RemoveTask deletes a cron task (method "removeTask"). MUTATING. task is a
// Task.Task value (the raw crontab line) as returned by GetTasks. Returns on
// the 1/0 sentinel (1 = success).
func (s *Service) RemoveTask(ctx context.Context, task string) error {
	return s.sentinelAction(ctx, "removeTask", map[string]any{"task": task})
}

// sentinelAction runs a /vh/cron method whose success is the integer sentinel 1
// (addTask/editTask/removeTask all answer 1 on success, 0 on failure per the
// spec). A real failure usually surfaces as a JSON-RPC error via Call; the
// non-1 check is defensive. The result is decoded via json.RawMessage first so
// that a shape not yet observed live (should the API ever answer richer than a
// bare 1) does not silently pass — only a plain 1 is accepted as success.
func (s *Service) sentinelAction(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, cronEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: cron %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: cron %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
