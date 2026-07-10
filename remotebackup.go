package sweb

import (
	"context"
	"encoding/json"
	"fmt"
)

const remoteBackupEndpoint = "/vps/remoteBackup"

// RemoteBackupService groups cloud (off-node) backup operations (endpoint
// /vps/remoteBackup): list/create/remove, edit the comment, and restore into the
// source or a different VPS.
type RemoteBackupService struct{ c *Client }

// RemoteBackup is one cloud backup, as returned by List. ID is the key the
// edit/restore/remove methods take.
type RemoteBackup struct {
	ID               FlexInt   `json:"id"`
	BillingID        string    `json:"billing_id"`
	DiskSize         FlexInt   `json:"disk_size"`
	Size             FlexInt   `json:"size"`
	Status           string    `json:"status"`
	OSDistributionID string    `json:"os_distribution_id"`
	Price            FlexFloat `json:"price"`
	Name             string    `json:"name"`
	Comment          string    `json:"comment"`
	TSCreate         string    `json:"ts_create"`
}

// List returns all cloud backups on the account (method "index"). Read-only.
func (s *RemoteBackupService) List(ctx context.Context) ([]RemoteBackup, error) {
	var out []RemoteBackup
	err := s.c.call(ctx, remoteBackupEndpoint, "index", nil, &out)
	return out, err
}

// Create takes a new cloud backup of a VPS (method "create"). Like the VPS
// create, the result shape is left raw pending a recorded response — read the new
// backup back via List.
func (s *RemoteBackupService) Create(ctx context.Context, billingID, name, comment string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, remoteBackupEndpoint, "create", map[string]string{
		"billingId": billingID,
		"name":      name,
		"comment":   comment,
	}, &out)
	return out, err
}

// EditComment updates a cloud backup's comment (method "editComment").
func (s *RemoteBackupService) EditComment(ctx context.Context, remoteBackupID int, comment string) error {
	return s.action(ctx, "editComment", map[string]any{
		"remoteBackupId": remoteBackupID,
		"comment":        comment,
	})
}

// Restore restores a cloud backup into its source VPS (method "restore").
// DESTRUCTIVE — overwrites the source disk.
func (s *RemoteBackupService) Restore(ctx context.Context, remoteBackupID int) error {
	return s.action(ctx, "restore", map[string]any{"remoteBackupId": remoteBackupID})
}

// RestoreInto restores a cloud backup into a DIFFERENT VPS (method "restoreInto").
// DESTRUCTIVE — overwrites the target disk.
func (s *RemoteBackupService) RestoreInto(ctx context.Context, remoteBackupID int, billingID string) error {
	return s.action(ctx, "restoreInto", map[string]any{
		"remoteBackupId": remoteBackupID,
		"billingId":      billingID,
	})
}

// Remove deletes a cloud backup (method "remove").
func (s *RemoteBackupService) Remove(ctx context.Context, remoteBackupID int) error {
	return s.action(ctx, "remove", map[string]any{"remoteBackupId": remoteBackupID})
}

// action issues a /vps/remoteBackup method and enforces the API's 1/0 action
// result (the cloud methods' return is undocumented, but the whole API answers
// actions this way; a JSON-RPC error also surfaces).
func (s *RemoteBackupService) action(ctx context.Context, method string, params map[string]any) error {
	var out FlexInt
	if err := s.c.call(ctx, remoteBackupEndpoint, method, params, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: remoteBackup %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
