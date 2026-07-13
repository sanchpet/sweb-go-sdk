// Package hosting groups shared-hosting database operations (endpoint
// /vh/hosting): listing the account's MySQL/PgSQL databases, the MySQL and
// PgSQL create/delete/change-password lifecycle, MySQL import/backup and remote
// access rules, comment editing, and the temporary PhpMyAdmin user. All calls
// dispatch through the shared transport.
package hosting

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const hostingEndpoint = "/vh/hosting"

// Service groups shared-hosting database operations (endpoint /vh/hosting):
// the database list, the MySQL/PgSQL lifecycle, MySQL import/backup, remote
// access rules, comment editing, and the temporary PhpMyAdmin user.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// List is the object returned by DatabaseList (method "databaseGetList"): the
// account-level quota/Redis metadata under Params and the databases under List.
type List struct {
	Params ListParams `json:"params"`
	List   []Database `json:"list"`
}

// ListParams is the account-level metadata that accompanies a database list:
// server name, per-engine quotas, Redis status, and pagination.
//
// Doc-vs-reality: the mysql*/pgsql* counts are documented int but decode through
// flex.Int for the API's usual polymorphic numbers; canCreate is documented int
// yet the recorded example returns bool, so those are typed bool. Redis lives
// under a variable nested object (RedisInfo) left raw — its shape (session sites,
// nullable ip/protocol/port) is only needed to render the Redis UI.
type ListParams struct {
	Server         string          `json:"server"`
	MysqlCount     flex.Int        `json:"mysqlCount"`
	MysqlMaxCount  flex.Int        `json:"mysqlMaxCount"`
	MysqlCanCreate bool            `json:"mysqlCanCreate"`
	PgsqlCount     flex.Int        `json:"pgsqlCount"`
	PgsqlMaxCount  flex.Int        `json:"pgsqlMaxCount"`
	PgsqlCanCreate bool            `json:"pgsqlCanCreate"`
	RedisAvailable bool            `json:"redisAvailable"`
	RedisEnabled   bool            `json:"redisEnabled"`
	RedisInfo      json.RawMessage `json:"redisInfo"`
	Page           flex.Int        `json:"page"`
	PerPage        flex.Int        `json:"perPage"`
	TotalPages     flex.Int        `json:"totalPages"`
	TotalCount     flex.Int        `json:"totalCount"`
	SortingType    string          `json:"sortingType"`
	DirectOrder    bool            `json:"directOrder"`
}

// Database is one database in a List. Version is present only for type "mysql";
// PgAdminURL only for type "pgsql". SizeTables is in MB.
type Database struct {
	Type        string     `json:"type"` // "mysql" | "pgsql"
	Version     string     `json:"version"`
	Name        string     `json:"name"`
	Login       string     `json:"login"`
	CountTables flex.Int   `json:"countTables"`
	SizeTables  flex.Float `json:"sizeTables"` // MB
	Charset     string     `json:"charset"`
	Comment     string     `json:"comment"`
	PgAdminURL  string     `json:"pgAdminUrl"`
}

// ListOptions are the (all optional) inputs to DatabaseList. A zero Page lets
// the API default it (1-based); an empty SortingType/Filter is omitted.
type ListOptions struct {
	Page        int
	SortingType string
	DirectOrder bool
	Filter      string
}

// DatabaseList returns the account's databases and quota/Redis metadata (method
// "databaseGetList"). Read-only.
func (s *Service) DatabaseList(ctx context.Context, o ListOptions) (*List, error) {
	params := map[string]any{"directOrder": o.DirectOrder}
	if o.Page != 0 {
		params["page"] = o.Page
	}
	if o.SortingType != "" {
		params["sortingType"] = o.SortingType
	}
	if o.Filter != "" {
		params["filter"] = o.Filter
	}
	var out List
	if err := s.t.Call(ctx, hostingEndpoint, "databaseGetList", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MysqlAccess is the result of MysqlAccessList (method "databaseMysqlAccessList"):
// the server name and whether another rule may be added, plus the current rules
// (each a "localhost", an IP, or a subnet).
type MysqlAccess struct {
	Params MysqlAccessParams `json:"params"`
	List   []string          `json:"list"`
}

// MysqlAccessParams is the metadata of a MysqlAccess result.
type MysqlAccessParams struct {
	Server    string `json:"server"`
	CanCreate bool   `json:"canCreate"`
}

// MysqlAccessList returns the remote-access rules for a MySQL database (method
// "databaseMysqlAccessList"). Read-only. dbName may be empty per the spec.
func (s *Service) MysqlAccessList(ctx context.Context, dbName string) (*MysqlAccess, error) {
	params := map[string]any{}
	if dbName != "" {
		params["dbName"] = dbName
	}
	var out MysqlAccess
	if err := s.t.Call(ctx, hostingEndpoint, "databaseMysqlAccessList", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PmaUser is the temporary PhpMyAdmin user returned by GetPmaUser (method
// "getPmaUser"): a one-shot login URL and DB/user/password credentials.
type PmaUser struct {
	URL  string `json:"url"`
	DB   string `json:"db"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

// GetPmaUser provisions a temporary PhpMyAdmin user for a database (method
// "getPmaUser") and returns its login URL and credentials. dbName is a
// Database.Name from DatabaseList.
//
// Read-only in effect (it grants a scratch login rather than mutating account
// data), so it is safe to exercise; the result is fully typed against the spec
// example.
func (s *Service) GetPmaUser(ctx context.Context, dbName string) (*PmaUser, error) {
	var out PmaUser
	if err := s.t.Call(ctx, hostingEndpoint, "getPmaUser", map[string]any{"dbName": dbName}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MysqlCreateOptions are the inputs to MysqlCreate. Name and Password are
// required; Comment and Version are optional and omitted when empty.
type MysqlCreateOptions struct {
	Name     string
	Password string
	Comment  string
	Version  string
}

// MysqlCreate creates a MySQL database (method "databaseMysqlCreate"). MUTATING.
// Success is the 1/0 sentinel (1 = created).
func (s *Service) MysqlCreate(ctx context.Context, o MysqlCreateOptions) error {
	params := map[string]any{"dbName": o.Name, "dbPassword": o.Password}
	if o.Comment != "" {
		params["dbComment"] = o.Comment
	}
	if o.Version != "" {
		params["dbVersion"] = o.Version
	}
	return s.sentinelAction(ctx, "databaseMysqlCreate", params)
}

// MysqlDelete removes a MySQL database (method "databaseMysqlDelete").
// DESTRUCTIVE. dbName is a Database.Name. Success is the 1/0 sentinel.
func (s *Service) MysqlDelete(ctx context.Context, dbName string) error {
	return s.sentinelAction(ctx, "databaseMysqlDelete", map[string]any{"dbName": dbName})
}

// MysqlChangePass changes a MySQL database password (method
// "databaseMysqlChangePass"). MUTATING. Success is the 1/0 sentinel.
func (s *Service) MysqlChangePass(ctx context.Context, dbName, password string) error {
	return s.sentinelAction(ctx, "databaseMysqlChangePass", map[string]any{
		"dbName":     dbName,
		"dbPassword": password,
	})
}

// MysqlImport imports a MySQL database from a file in the user's home directory
// (method "databaseMysqlImport"). MUTATING — overwrites the target database.
// dbName is an existing Database.Name; filePath is a path in the user's folder.
// Success is the 1/0 sentinel.
//
// Doc-vs-reality: the API spells the parameter "filePatch" (sic); the Go
// signature exposes it as filePath but sends the documented key.
func (s *Service) MysqlImport(ctx context.Context, dbName, filePath string) error {
	return s.sentinelAction(ctx, "databaseMysqlImport", map[string]any{
		"dbName":    dbName,
		"filePatch": filePath,
	})
}

// MysqlMakeCopy queues a task to archive a MySQL database (method
// "databaseMysqlMakeCopy"). MUTATING (enqueues a backup job). dbName is a full
// Database.Name. Success is the 1/0 sentinel (1 = the task was queued).
func (s *Service) MysqlMakeCopy(ctx context.Context, dbName string) error {
	return s.sentinelAction(ctx, "databaseMysqlMakeCopy", map[string]any{"dbName": dbName})
}

// MysqlAccessCreate adds a remote-access rule to a MySQL database (method
// "databaseMysqlAccessCreate"). MUTATING. rule is "localhost", an IP, or a
// subnet. Success is the 1/0 sentinel.
func (s *Service) MysqlAccessCreate(ctx context.Context, dbName, rule string) error {
	return s.sentinelAction(ctx, "databaseMysqlAccessCreate", map[string]any{
		"dbName": dbName,
		"rule":   rule,
	})
}

// MysqlAccessDelete removes a remote-access rule from a MySQL database (method
// "databaseMysqlAccessDelete"). MUTATING. rule is one from MysqlAccessList.
// Success is the 1/0 sentinel.
func (s *Service) MysqlAccessDelete(ctx context.Context, dbName, rule string) error {
	return s.sentinelAction(ctx, "databaseMysqlAccessDelete", map[string]any{
		"dbName": dbName,
		"rule":   rule,
	})
}

// PgsqlCreateOptions are the inputs to PgsqlCreate. Name, Password and Charset
// are required; Comment is optional and omitted when empty.
type PgsqlCreateOptions struct {
	Name     string
	Password string
	Charset  string
	Comment  string
}

// PgsqlCreate creates a PostgreSQL database (method "databasePgsqlCreate").
// MUTATING. Success is the 1/0 sentinel (1 = created).
func (s *Service) PgsqlCreate(ctx context.Context, o PgsqlCreateOptions) error {
	params := map[string]any{
		"dbName":     o.Name,
		"dbPassword": o.Password,
		"dbCharset":  o.Charset,
	}
	if o.Comment != "" {
		params["dbComment"] = o.Comment
	}
	return s.sentinelAction(ctx, "databasePgsqlCreate", params)
}

// PgsqlDelete removes a PostgreSQL database (method "databasePgsqlDelete").
// DESTRUCTIVE. dbName is a Database.Name. Success is the 1/0 sentinel.
func (s *Service) PgsqlDelete(ctx context.Context, dbName string) error {
	return s.sentinelAction(ctx, "databasePgsqlDelete", map[string]any{"dbName": dbName})
}

// PgsqlChangePass changes a PostgreSQL database password (method
// "databasePgsqlChangePass"). MUTATING. Success is the 1/0 sentinel.
func (s *Service) PgsqlChangePass(ctx context.Context, dbName, password string) error {
	return s.sentinelAction(ctx, "databasePgsqlChangePass", map[string]any{
		"dbName":     dbName,
		"dbPassword": password,
	})
}

// EditComment sets (or creates) the comment on a database (method
// "databaseEditComment"). MUTATING. dbType is "mysql" or "pgsql"; dbName is a
// full Database.Name. Success is the 1/0 sentinel.
func (s *Service) EditComment(ctx context.Context, dbType, dbName, comment string) error {
	return s.sentinelAction(ctx, "databaseEditComment", map[string]any{
		"dbType":    dbType,
		"dbName":    dbName,
		"dbComment": comment,
	})
}

// sentinelAction runs a /vh/hosting mutating method whose success is the integer
// sentinel 1 (per the spec every mutating method here answers resultInt: 1 =
// success, 0 = failure). A real failure usually surfaces as a JSON-RPC error via
// Call; the non-1 check is defensive. The result is decoded via json.RawMessage
// first so that a shape not yet observed live — the 1/0 sentinel is documented
// but not reconciled against a recorded response — does not silently pass: only
// a plain 1 is accepted as success.
func (s *Service) sentinelAction(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, hostingEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: hosting %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: hosting %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
