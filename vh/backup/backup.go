// Package backup groups shared-hosting account-backup operations (endpoint
// /vh/backup): listing daily backups, browsing their file/MySQL contents, and
// the restore/receive/download lifecycle over an account's files and databases.
// All calls dispatch through the shared transport.
//
// This is DISTINCT from the /vps/backup service (package backup at the module
// root): that one snapshots a whole VPS disk, this one operates on a
// shared-hosting account's home directory and databases.
package backup

import (
	"context"
	"encoding/json"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const backupEndpoint = "/vh/backup"

// Service groups shared-hosting account-backup operations (endpoint /vh/backup):
// listing daily backups, browsing their contents, and restore/receive/download.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// DateEntry is one day's backup as returned by List (method "getList"): a folder
// on the server named by Date, holding a file backup and/or a MySQL backup.
//
// Files and Mysql are the item counts for that day; per the spec they are null
// while the server is still counting (→ flex.Int 0), so they decode through
// flex.Int rather than a strict int. BackupFilesExists reports whether a file
// backup is present; WarnQuota flags a quota overrun for that day.
type DateEntry struct {
	Date              string   `json:"date"`  // e.g. "23.06.2023" (display format)
	Mysql             flex.Int `json:"mysql"` // db count; null (→0) while counting
	Files             flex.Int `json:"files"` // file count; null (→0) while counting
	BackupFilesExists bool     `json:"backupFilesExists"`
	WarnQuota         bool     `json:"warnQuota"`
}

// File is one entry inside a file backup, as returned by ListFiles (method
// "getListFiles"). Dir marks a directory; Size is a human-readable string
// ("0 B", "4 KB", "309,89 KB") — note the API's locale uses a comma decimal
// separator, so it is kept as a string, not parsed to a number.
type File struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Size string `json:"size"`
}

// MysqlDump is one entry inside a MySQL backup, as returned by ListMysql (method
// "getListMysql"): a database (or a directory within it). Unlike File it carries
// no size in the recorded response.
type MysqlDump struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
}

// List returns the full set of daily backups grouped by day (method "getList"),
// covering both files and databases. Read-only. No parameters.
func (s *Service) List(ctx context.Context) ([]DateEntry, error) {
	var out []DateEntry
	err := s.t.Call(ctx, backupEndpoint, "getList", nil, &out)
	return out, err
}

// ListFiles returns the contents of a directory inside a file backup for a given
// day (method "getListFiles"). Read-only. date is the backup's folder name in the
// server's strict format ("2023-02-27", NOT the DateEntry.Date display format);
// dir is the path within the backup ("/" for the root).
func (s *Service) ListFiles(ctx context.Context, date, dir string) ([]File, error) {
	var out []File
	err := s.t.Call(ctx, backupEndpoint, "getListFiles", map[string]string{
		"date": date,
		"dir":  dir,
	}, &out)
	return out, err
}

// ListMysql returns the contents of a MySQL backup for a given day (method
// "getListMysql"). Read-only. date is the backup's folder name in the server's
// strict format ("2023-02-27"); dir is the path within the backup ("/" for the
// root).
func (s *Service) ListMysql(ctx context.Context, date, dir string) ([]MysqlDump, error) {
	var out []MysqlDump
	err := s.t.Call(ctx, backupEndpoint, "getListMysql", map[string]string{
		"date": date,
		"dir":  dir,
	}, &out)
	return out, err
}

// FileRef identifies one file or folder to act on within a backup, matching the
// API's positional [type, path] pair. Dir reports whether the target is a folder
// (the spec's files[][0]: 0 = file, 1 = folder); Path is the concatenated
// directory and name (the spec's files[][1], e.g. "/.authfile").
//
// It marshals to the wire form [0|1, "path"] — a JSON array, not an object —
// which is what restoreFiles/receiveFiles/downloadFile expect for each element
// of their files parameter.
type FileRef struct {
	Dir  bool
	Path string
}

// MarshalJSON emits the API's positional [type, path] pair: [0,"/f"] for a file,
// [1,"/d"] for a folder.
func (f FileRef) MarshalJSON() ([]byte, error) {
	typ := 0
	if f.Dir {
		typ = 1
	}
	return json.Marshal([]any{typ, f.Path})
}

// MakeAccountCopy queues creation of a fresh backup of all databases and the
// account's home directory (method "makeAccountCopy"). MUTATING. No parameters.
//
// The spec documents the result as the integer sentinel 1 (success) / 0
// (failure), but that shape has not been observed against the live API, so the
// raw JSON-RPC result is returned rather than reduced to a bool. A transport-
// level failure still surfaces as an error via Call; the caller may inspect the
// payload. (Evidence-first: promote to a typed sentinel once recorded live.)
func (s *Service) MakeAccountCopy(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, backupEndpoint, "makeAccountCopy", nil, &out)
	return out, err
}

// RestoreFiles restores the given files/folders from a day's backup in place
// (method "restoreFiles"). MUTATING and DESTRUCTIVE — overwrites live files.
// date is the backup folder in the server's strict format ("2023-02-27"); files
// are the targets to restore.
//
// The spec documents a 1/0 integer sentinel, but the live result shape is
// unobserved, so the raw result is returned unreduced (see MakeAccountCopy).
func (s *Service) RestoreFiles(ctx context.Context, date string, files []FileRef) (json.RawMessage, error) {
	return s.filesAction(ctx, "restoreFiles", date, files)
}

// DownloadFile downloads a single file from a day's backup (method
// "downloadFile"). date is the backup folder in the server's strict format;
// files carries the target(s) as for RestoreFiles.
//
// Per the spec example the result is an array of {"file":{mimetype, metadata,
// content, name}} objects where content is base64 (mimetype e.g.
// "application/gzip;base64"). That shape is not yet reconciled against a live
// response, so the raw result is returned for the caller to decode rather than a
// guessed struct. (Evidence-first: promote once recorded.)
func (s *Service) DownloadFile(ctx context.Context, date string, files []FileRef) (json.RawMessage, error) {
	return s.filesAction(ctx, "downloadFile", date, files)
}

// ReceiveFiles queues the given files/folders from a day's backup to be prepared
// for download ("Получить бэкап") rather than restored in place (method
// "receiveFiles"). MUTATING. Parameters as for RestoreFiles.
//
// The spec documents a 1/0 integer sentinel; the live result shape is
// unobserved, so the raw result is returned unreduced (see MakeAccountCopy).
func (s *Service) ReceiveFiles(ctx context.Context, date string, files []FileRef) (json.RawMessage, error) {
	return s.filesAction(ctx, "receiveFiles", date, files)
}

// filesAction issues a /vh/backup method taking (date, files[]) — shared by
// restoreFiles/downloadFile/receiveFiles. The result is returned raw: none of
// these shapes has been reconciled against the live API (see MakeAccountCopy).
func (s *Service) filesAction(ctx context.Context, method, date string, files []FileRef) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, backupEndpoint, method, map[string]any{
		"date":  date,
		"files": files,
	}, &out)
	return out, err
}

// ReceiveMysql queues the named databases from a day's backup to be prepared for
// download ("Получить бэкап") (method "receiveMysql"). MUTATING. date is the
// backup folder in the server's strict format; databases is the list of database
// names.
//
// The spec documents a 1/0 integer sentinel; the live result shape is
// unobserved, so the raw result is returned unreduced (see MakeAccountCopy).
func (s *Service) ReceiveMysql(ctx context.Context, date string, databases []string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, backupEndpoint, "receiveMysql", map[string]any{
		"date":      date,
		"databases": databases,
	}, &out)
	return out, err
}

// RestoreMysql restores databases/tables from a day's backup in place (method
// "restoreMysql"). MUTATING and DESTRUCTIVE. date is the backup folder in the
// server's strict format.
//
// Doc-vs-reality: the spec types the databases parameter as a bare String ("the
// database name"), whereas its sibling receiveMysql takes an Array of names and
// this method's own description is "single and bulk actions". We forward a
// []string (the general case); a single name is a one-element slice. Confirm the
// wire shape against a live call before relying on bulk restore.
//
// The spec documents a 1/0 integer sentinel; the live result shape is
// unobserved, so the raw result is returned unreduced (see MakeAccountCopy).
func (s *Service) RestoreMysql(ctx context.Context, date string, databases []string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, backupEndpoint, "restoreMysql", map[string]any{
		"date":      date,
		"databases": databases,
	}, &out)
	return out, err
}
