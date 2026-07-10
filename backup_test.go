package sweb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestBackupList(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"name":"backup_1","prettyName":"10 Jul 2026","unic_id":"u1","attach_type":"","updatedAt":"2026-07-10"}]}`))
	})
	list, err := c.Backup.List(context.Background(), "login_vps_1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "backup_1" || list[0].PrettyName != "10 Jul 2026" {
		t.Errorf("list = %+v, want one backup_1", list)
	}
}

func TestBackupCreate(t *testing.T) {
	var gotMethod string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := c.Backup.Create(context.Background(), "login_vps_1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotMethod != "create" {
		t.Errorf("method = %q, want create", gotMethod)
	}
}

func TestBackupNameActions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		call   func(*Client) error
		method string
	}{
		{"Restore", func(c *Client) error { return c.Backup.Restore(context.Background(), "login_vps_1", "backup_1") }, "restore"},
		{"Attach", func(c *Client) error { return c.Backup.Attach(context.Background(), "login_vps_1", "backup_1") }, "attach"},
		{"Detach", func(c *Client) error { return c.Backup.Detach(context.Background(), "login_vps_1", "backup_1") }, "detach"},
		{"Remove", func(c *Client) error { return c.Backup.Remove(context.Background(), "login_vps_1", "backup_1") }, "remove"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotName string
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
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
			if err := tc.call(c); err != nil {
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
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"mode":"auto","frequency":7,"time":3,"next_data_backup":"2026-07-17"}]}`))
	})
	set, err := c.Backup.Settings(context.Background(), "login_vps_1")
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
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := c.Backup.SaveSettings(context.Background(), "login_vps_1", "auto", 7, 3); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if gotMethod != "saveSettings" || gotParams.Mode != "auto" || gotParams.Frequency != 7 || gotParams.Time != 3 {
		t.Errorf("method/params = %q/%+v, want saveSettings / auto,7,3", gotMethod, gotParams)
	}
}

func TestRemoteBackupList(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":42,"billing_id":"login_vps_1","size":1024,"status":"ready","name":"cb1","comment":"nightly","price":90}]}`))
	})
	list, err := c.RemoteBackup.List(context.Background())
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
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":{"id":43}}`))
	})
	if _, err := c.RemoteBackup.Create(context.Background(), "login_vps_1", "cb2", "manual"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotMethod != "create" || gotParams.Name != "cb2" || gotParams.Comment != "manual" {
		t.Errorf("method/params = %q/%+v, want create / cb2,manual", gotMethod, gotParams)
	}
}

func TestRemoteBackupActions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		call   func(*Client) error
		method string
	}{
		{"EditComment", func(c *Client) error { return c.RemoteBackup.EditComment(context.Background(), 42, "x") }, "editComment"},
		{"Restore", func(c *Client) error { return c.RemoteBackup.Restore(context.Background(), 42) }, "restore"},
		{"RestoreInto", func(c *Client) error { return c.RemoteBackup.RestoreInto(context.Background(), 42, "login_vps_2") }, "restoreInto"},
		{"Remove", func(c *Client) error { return c.RemoteBackup.Remove(context.Background(), 42) }, "remove"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			var gotID int
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
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
			if err := tc.call(c); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method || gotID != 42 {
				t.Errorf("method/id = %q/%d, want %s/42", gotMethod, gotID, tc.method)
			}
		})
	}
}
