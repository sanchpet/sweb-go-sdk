// Package diskusage groups disk-usage (quota scan) operations
// (endpoint /vh/utils/diskUsage): reading the per-backend quota breakdown and
// the scan-task state, triggering a new scan, and managing the over-quota
// notification email. All calls dispatch through the shared transport.
package diskusage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const diskUsageEndpoint = "/vh/utils/diskUsage"

// Service groups disk-usage operations (endpoint /vh/utils/diskUsage):
// index/getTasksInfo/getEmail reads plus the startTask/changeEmail actions.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Usage is one backend's disk-usage breakdown as returned by List (method
// "index"). All quota figures are megabytes.
//
// Types are reconciled against the spec's recorded example: the quota fields are
// documented "float" and file counts "int", but SpaceWeb quotes numerics
// inconsistently, so every field decodes through flex.Float/flex.Int rather than
// a bare float64/int.
type Usage struct {
	TariffQuota flex.Float `json:"tariffQuota"` // plan quota, MB
	RealQuota   flex.Float `json:"realQuota"`   // actual usage, MB
	DBQuota     flex.Float `json:"dbQuota"`     // databases, MB
	MailQuota   flex.Float `json:"mailQuota"`   // mail, MB
	FilesQuota  flex.Float `json:"filesQuota"`  // files, MB
	FilesNum    flex.Int   `json:"filesNum"`    // number of files
}

// TasksInfo is the disk-usage scan-task state as returned by TasksInfo (method
// "getTasksInfo").
//
// ActiveTasksCount is documented "int" but the spec example quotes it ("0"), so
// it decodes through flex.Int. LastDoneTaskDate is a plain timestamp string.
type TasksInfo struct {
	ActiveTasksCount flex.Int `json:"activeTasksCount"` // scans running now
	LastDoneTaskDate string   `json:"lastDoneTaskDate"` // "2006-01-02 15:04:05"
}

// List returns the per-backend disk-usage breakdown (method "index"). Read-only.
func (s *Service) List(ctx context.Context) ([]Usage, error) {
	var out []Usage
	if err := s.t.Call(ctx, diskUsageEndpoint, "index", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// TasksInfo returns the disk-usage scan-task state (method "getTasksInfo"):
// whether a scan is running now and when the last one finished. Read-only.
func (s *Service) TasksInfo(ctx context.Context) (*TasksInfo, error) {
	var out TasksInfo
	if err := s.t.Call(ctx, diskUsageEndpoint, "getTasksInfo", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Email returns the address that receives over-quota notifications
// (method "getEmail"). Read-only.
func (s *Service) Email(ctx context.Context) (string, error) {
	var out string
	if err := s.t.Call(ctx, diskUsageEndpoint, "getEmail", nil, &out); err != nil {
		return "", err
	}
	return out, nil
}

// StartTask queues a new disk-usage scan (method "startTask"). MUTATING. Takes
// no parameters. Returns on the 1/0 sentinel (1 = the scan was queued).
func (s *Service) StartTask(ctx context.Context) error {
	return s.sentinelAction(ctx, "startTask", nil)
}

// ChangeEmail sets the address for over-quota notifications
// (method "changeEmail"). MUTATING. Returns on the 1/0 sentinel (1 = success).
func (s *Service) ChangeEmail(ctx context.Context, email string) error {
	return s.sentinelAction(ctx, "changeEmail", map[string]any{"email": email})
}

// sentinelAction runs a /vh/utils/diskUsage method whose success is the integer
// sentinel 1 (startTask and changeEmail both answer 1 on success, 0 on failure
// per the spec's resultInt: "1 - успешно, 0 - ошибка"). A real failure usually
// surfaces as a JSON-RPC error via Call; the non-1 check is defensive. The
// result is decoded via json.RawMessage first so a shape not yet observed live
// (should the API ever answer richer than a bare 1) does not silently pass —
// only a plain 1 is accepted as success.
func (s *Service) sentinelAction(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, diskUsageEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: diskusage %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: diskusage %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
