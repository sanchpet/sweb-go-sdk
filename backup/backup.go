// Package backup groups local (on-node) backup operations (endpoint
// /vps/backup): list/create/restore/remove, attach/detach a backup as a disk,
// and the auto-backup schedule. All calls dispatch through the shared transport.
package backup

import (
	"context"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const backupEndpoint = "/vps/backup"

// Service groups local (on-node) backup operations (endpoint /vps/backup):
// list/create/restore/remove, attach/detach a backup as a disk, and the
// auto-backup schedule.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Backup is one local backup of a VPS, as returned by List. name is the key the
// restore/attach/detach/remove methods take.
type Backup struct {
	Name       string `json:"name"`
	PrettyName string `json:"prettyName"`
	UnicID     string `json:"unic_id"`
	AttachType string `json:"attach_type"`
	UpdatedAt  string `json:"updatedAt"`
}

// Settings is the auto-backup schedule (getSettings/saveSettings). mode is
// "manual" or "auto"; frequency/time are null in manual mode (flex.Int → 0).
type Settings struct {
	Mode           string   `json:"mode"`
	Frequency      flex.Int `json:"frequency"`
	Time           flex.Int `json:"time"`
	NextDataBackup string   `json:"next_data_backup"`
}

// List returns a VPS's local backups (method "index"). Read-only.
func (s *Service) List(ctx context.Context, billingID string) ([]Backup, error) {
	var out []Backup
	err := s.t.Call(ctx, backupEndpoint, "index", map[string]string{"billingId": billingID}, &out)
	return out, err
}

// Create takes a new local backup of a VPS (method "create"). Action 1/0 result.
func (s *Service) Create(ctx context.Context, billingID string) error {
	var out flex.Int
	if err := s.t.Call(ctx, backupEndpoint, "create", map[string]string{"billingId": billingID}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: backup create returned %d, want 1 (0 = failure)", int64(out))
	}
	return nil
}

// UpdateIndex refreshes the server-side list of a VPS's local backups (method
// "updateIndex") so a subsequent List reflects newly created/removed backups.
// Action 1/0 result.
func (s *Service) UpdateIndex(ctx context.Context, billingID string) error {
	var out flex.Int
	if err := s.t.Call(ctx, backupEndpoint, "updateIndex", map[string]string{"billingId": billingID}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: backup updateIndex returned %d, want 1 (0 = failure)", int64(out))
	}
	return nil
}

// Restore restores a VPS from a local backup (method "restore"). DESTRUCTIVE —
// overwrites the current disk. name is a Backup.Name from List. Action 1/0.
func (s *Service) Restore(ctx context.Context, billingID, name string) error {
	return s.nameAction(ctx, "restore", billingID, name)
}

// Attach mounts a backup on the VPS as an extra disk (method "attach"). Action 1/0.
func (s *Service) Attach(ctx context.Context, billingID, name string) error {
	return s.nameAction(ctx, "attach", billingID, name)
}

// Detach unmounts a previously attached backup disk (method "detach"). Action 1/0.
func (s *Service) Detach(ctx context.Context, billingID, name string) error {
	return s.nameAction(ctx, "detach", billingID, name)
}

// Remove deletes a local backup (method "remove"). Action 1/0.
func (s *Service) Remove(ctx context.Context, billingID, name string) error {
	return s.nameAction(ctx, "remove", billingID, name)
}

// nameAction issues a /vps/backup method keyed by billingId + backupName.
func (s *Service) nameAction(ctx context.Context, method, billingID, name string) error {
	var out flex.Int
	if err := s.t.Call(ctx, backupEndpoint, method, map[string]string{
		"billingId":  billingID,
		"backupName": name,
	}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: backup %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}

// Settings returns a VPS's auto-backup schedule (method "getSettings"). The API
// wraps it in a one-element array; this unwraps it (nil if empty). Read-only.
func (s *Service) Settings(ctx context.Context, billingID string) (*Settings, error) {
	var out []Settings
	if err := s.t.Call(ctx, backupEndpoint, "getSettings", map[string]string{"billingId": billingID}, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
}

// SaveSettings sets the auto-backup schedule (method "saveSettings"). mode is
// "manual" or "auto"; frequency and time are the schedule knobs (ignored in
// manual mode). Action 1/0 result.
func (s *Service) SaveSettings(ctx context.Context, billingID, mode string, frequency, backupTime int) error {
	var out flex.Int
	if err := s.t.Call(ctx, backupEndpoint, "saveSettings", map[string]any{
		"billingId": billingID,
		"mode":      mode,
		"frequency": frequency,
		"time":      backupTime,
	}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: saveSettings returned %d, want 1 (0 = failure)", int64(out))
	}
	return nil
}
