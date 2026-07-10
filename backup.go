package sweb

import (
	"context"
	"fmt"
)

const backupEndpoint = "/vps/backup"

// BackupService groups local (on-node) backup operations (endpoint /vps/backup):
// list/create/restore/remove, attach/detach a backup as a disk, and the
// auto-backup schedule.
type BackupService struct{ c *Client }

// Backup is one local backup of a VPS, as returned by List. name is the key the
// restore/attach/detach/remove methods take.
type Backup struct {
	Name       string `json:"name"`
	PrettyName string `json:"prettyName"`
	UnicID     string `json:"unic_id"`
	AttachType string `json:"attach_type"`
	UpdatedAt  string `json:"updatedAt"`
}

// BackupSettings is the auto-backup schedule (getSettings/saveSettings). mode is
// "manual" or "auto"; frequency/time are null in manual mode (FlexInt → 0).
type BackupSettings struct {
	Mode           string  `json:"mode"`
	Frequency      FlexInt `json:"frequency"`
	Time           FlexInt `json:"time"`
	NextDataBackup string  `json:"next_data_backup"`
}

// List returns a VPS's local backups (method "index"). Read-only.
func (s *BackupService) List(ctx context.Context, billingID string) ([]Backup, error) {
	var out []Backup
	err := s.c.call(ctx, backupEndpoint, "index", map[string]string{"billingId": billingID}, &out)
	return out, err
}

// Create takes a new local backup of a VPS (method "create"). Action 1/0 result.
func (s *BackupService) Create(ctx context.Context, billingID string) error {
	var out FlexInt
	if err := s.c.call(ctx, backupEndpoint, "create", map[string]string{"billingId": billingID}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: backup create returned %d, want 1 (0 = failure)", int64(out))
	}
	return nil
}

// Restore restores a VPS from a local backup (method "restore"). DESTRUCTIVE —
// overwrites the current disk. name is a Backup.Name from List. Action 1/0.
func (s *BackupService) Restore(ctx context.Context, billingID, name string) error {
	return s.nameAction(ctx, "restore", billingID, name)
}

// Attach mounts a backup on the VPS as an extra disk (method "attach"). Action 1/0.
func (s *BackupService) Attach(ctx context.Context, billingID, name string) error {
	return s.nameAction(ctx, "attach", billingID, name)
}

// Detach unmounts a previously attached backup disk (method "detach"). Action 1/0.
func (s *BackupService) Detach(ctx context.Context, billingID, name string) error {
	return s.nameAction(ctx, "detach", billingID, name)
}

// Remove deletes a local backup (method "remove"). Action 1/0.
func (s *BackupService) Remove(ctx context.Context, billingID, name string) error {
	return s.nameAction(ctx, "remove", billingID, name)
}

// nameAction issues a /vps/backup method keyed by billingId + backupName.
func (s *BackupService) nameAction(ctx context.Context, method, billingID, name string) error {
	var out FlexInt
	if err := s.c.call(ctx, backupEndpoint, method, map[string]string{
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
func (s *BackupService) Settings(ctx context.Context, billingID string) (*BackupSettings, error) {
	var out []BackupSettings
	if err := s.c.call(ctx, backupEndpoint, "getSettings", map[string]string{"billingId": billingID}, &out); err != nil {
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
func (s *BackupService) SaveSettings(ctx context.Context, billingID, mode string, frequency, backupTime int) error {
	var out FlexInt
	if err := s.c.call(ctx, backupEndpoint, "saveSettings", map[string]any{
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
