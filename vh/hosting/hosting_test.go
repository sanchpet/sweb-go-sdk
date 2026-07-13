package hosting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestDatabaseList(t *testing.T) {
	// The list wraps databases under "list" and account metadata under "params";
	// counts arrive polymorphic (bare ints here) and canCreate as bool.
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{
			"params":{"server":"VH297","mysqlCount":1,"mysqlMaxCount":512,"mysqlCanCreate":true,
				"pgsqlCount":0,"pgsqlMaxCount":512,"pgsqlCanCreate":true,
				"redisAvailable":true,"redisEnabled":true,"redisInfo":{"ip":"127.0.0.79","port":15121},
				"page":1,"perPage":20,"totalPages":1,"totalCount":1,
				"sortingType":"default","directOrder":true},
			"list":[{"type":"mysql","version":"5.7","name":"in****","login":"in****",
				"countTables":0,"sizeTables":0.01,"charset":"","comment":"test"}]
		}}`))
	})
	got, err := s.DatabaseList(context.Background(), ListOptions{Page: 1, SortingType: "default", DirectOrder: true, Filter: "inn"})
	if err != nil {
		t.Fatalf("DatabaseList: %v", err)
	}
	if gotMethod != "databaseGetList" {
		t.Errorf("method = %q, want databaseGetList", gotMethod)
	}
	if _, ok := gotParams["filter"]; !ok {
		t.Errorf("params = %v, want filter present", gotParams)
	}
	if got.Params.Server != "VH297" || got.Params.MysqlCount != 1 || !got.Params.MysqlCanCreate {
		t.Errorf("params = %+v, want VH297 / mysqlCount 1 / canCreate true", got.Params)
	}
	if got.Params.TotalCount != 1 || !got.Params.DirectOrder || got.Params.SortingType != "default" {
		t.Errorf("pagination = %+v, want totalCount 1 / directOrder true / default", got.Params)
	}
	if len(got.List) != 1 {
		t.Fatalf("list len = %d, want 1", len(got.List))
	}
	db := got.List[0]
	if db.Type != "mysql" || db.Version != "5.7" || db.SizeTables != 0.01 || db.Comment != "test" {
		t.Errorf("database = %+v, want mysql/5.7/0.01/test", db)
	}
}

func TestDatabaseListOmitsEmptyOptions(t *testing.T) {
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{"params":{},"list":[]}}`))
	})
	if _, err := s.DatabaseList(context.Background(), ListOptions{}); err != nil {
		t.Fatalf("DatabaseList: %v", err)
	}
	for _, k := range []string{"page", "sortingType", "filter"} {
		if _, ok := gotParams[k]; ok {
			t.Errorf("params carried empty %q, want it omitted", k)
		}
	}
}

func TestMysqlAccessList(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{"list":["localhost","127.0.0.2"],
			"params":{"canCreate":true,"server":"VH297"}}}`))
	})
	got, err := s.MysqlAccessList(context.Background(), "mydb")
	if err != nil {
		t.Fatalf("MysqlAccessList: %v", err)
	}
	if gotMethod != "databaseMysqlAccessList" {
		t.Errorf("method = %q, want databaseMysqlAccessList", gotMethod)
	}
	if got.Params.Server != "VH297" || !got.Params.CanCreate {
		t.Errorf("params = %+v, want VH297 / canCreate true", got.Params)
	}
	if len(got.List) != 2 || got.List[0] != "localhost" || got.List[1] != "127.0.0.2" {
		t.Errorf("list = %v, want [localhost 127.0.0.2]", got.List)
	}
}

func TestMysqlAccessListOmitsEmptyDBName(t *testing.T) {
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":{"list":[],"params":{}}}`))
	})
	if _, err := s.MysqlAccessList(context.Background(), ""); err != nil {
		t.Fatalf("MysqlAccessList: %v", err)
	}
	if _, ok := gotParams["dbName"]; ok {
		t.Errorf("params carried empty dbName, want it omitted")
	}
}

func TestGetPmaUser(t *testing.T) {
	var gotMethod, gotDBName string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				DBName string `json:"dbName"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotDBName = req.Method, req.Params.DBName
		_, _ = w.Write([]byte(`{"result":{"db":"mydb","pass":"secret",
			"url":"qlnpd02knh8ogot21u5iie0gqj.75902b85","user":"_pma_3610018962"}}`))
	})
	got, err := s.GetPmaUser(context.Background(), "mydb")
	if err != nil {
		t.Fatalf("GetPmaUser: %v", err)
	}
	if gotMethod != "getPmaUser" || gotDBName != "mydb" {
		t.Errorf("method/dbName = %q/%q, want getPmaUser/mydb", gotMethod, gotDBName)
	}
	if got.DB != "mydb" || got.User != "_pma_3610018962" || got.Pass != "secret" || got.URL == "" {
		t.Errorf("pmaUser = %+v, want mydb / _pma_3610018962 / secret / url set", got)
	}
}

func TestMysqlCreate(t *testing.T) {
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	err := s.MysqlCreate(context.Background(), MysqlCreateOptions{
		Name: "mydb", Password: "pw", Comment: "c", Version: "5.7",
	})
	if err != nil {
		t.Fatalf("MysqlCreate: %v", err)
	}
	if gotMethod != "databaseMysqlCreate" {
		t.Errorf("method = %q, want databaseMysqlCreate", gotMethod)
	}
	assertString(t, gotParams, "dbName", "mydb")
	assertString(t, gotParams, "dbComment", "c")
	assertString(t, gotParams, "dbVersion", "5.7")
}

func TestMysqlCreateOmitsEmptyOptionals(t *testing.T) {
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.MysqlCreate(context.Background(), MysqlCreateOptions{Name: "mydb", Password: "pw"}); err != nil {
		t.Fatalf("MysqlCreate: %v", err)
	}
	for _, k := range []string{"dbComment", "dbVersion"} {
		if _, ok := gotParams[k]; ok {
			t.Errorf("params carried empty %q, want it omitted", k)
		}
	}
}

func TestMysqlDelete(t *testing.T) {
	assertSentinelCall(t, "databaseMysqlDelete", func(s *Service) error {
		return s.MysqlDelete(context.Background(), "mydb")
	})
}

func TestMysqlChangePass(t *testing.T) {
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.MysqlChangePass(context.Background(), "mydb", "newpw"); err != nil {
		t.Fatalf("MysqlChangePass: %v", err)
	}
	assertString(t, gotParams, "dbName", "mydb")
	assertString(t, gotParams, "dbPassword", "newpw")
}

func TestMysqlImportSendsDocumentedKey(t *testing.T) {
	// The Go signature exposes filePath but the API key is the misspelled "filePatch".
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.MysqlImport(context.Background(), "mydb", "/dump.sql.gz"); err != nil {
		t.Fatalf("MysqlImport: %v", err)
	}
	if gotMethod != "databaseMysqlImport" {
		t.Errorf("method = %q, want databaseMysqlImport", gotMethod)
	}
	assertString(t, gotParams, "filePatch", "/dump.sql.gz")
	if _, ok := gotParams["filePath"]; ok {
		t.Errorf("params carried filePath, want the documented filePatch key")
	}
}

func TestMysqlMakeCopy(t *testing.T) {
	assertSentinelCall(t, "databaseMysqlMakeCopy", func(s *Service) error {
		return s.MysqlMakeCopy(context.Background(), "mydb")
	})
}

func TestMysqlAccessCreate(t *testing.T) {
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.MysqlAccessCreate(context.Background(), "mydb", "127.0.0.2"); err != nil {
		t.Fatalf("MysqlAccessCreate: %v", err)
	}
	if gotMethod != "databaseMysqlAccessCreate" {
		t.Errorf("method = %q, want databaseMysqlAccessCreate", gotMethod)
	}
	assertString(t, gotParams, "dbName", "mydb")
	assertString(t, gotParams, "rule", "127.0.0.2")
}

func TestMysqlAccessDelete(t *testing.T) {
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.MysqlAccessDelete(context.Background(), "mydb", "127.0.0.2"); err != nil {
		t.Fatalf("MysqlAccessDelete: %v", err)
	}
	if gotMethod != "databaseMysqlAccessDelete" {
		t.Errorf("method = %q, want databaseMysqlAccessDelete", gotMethod)
	}
	assertString(t, gotParams, "rule", "127.0.0.2")
}

func TestPgsqlCreate(t *testing.T) {
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	err := s.PgsqlCreate(context.Background(), PgsqlCreateOptions{
		Name: "mydb", Password: "pw", Charset: "unicode", Comment: "c",
	})
	if err != nil {
		t.Fatalf("PgsqlCreate: %v", err)
	}
	if gotMethod != "databasePgsqlCreate" {
		t.Errorf("method = %q, want databasePgsqlCreate", gotMethod)
	}
	assertString(t, gotParams, "dbCharset", "unicode")
	assertString(t, gotParams, "dbComment", "c")
}

func TestPgsqlCreateOmitsEmptyComment(t *testing.T) {
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	err := s.PgsqlCreate(context.Background(), PgsqlCreateOptions{Name: "mydb", Password: "pw", Charset: "unicode"})
	if err != nil {
		t.Fatalf("PgsqlCreate: %v", err)
	}
	if _, ok := gotParams["dbComment"]; ok {
		t.Errorf("params carried empty dbComment, want it omitted")
	}
}

func TestPgsqlDelete(t *testing.T) {
	assertSentinelCall(t, "databasePgsqlDelete", func(s *Service) error {
		return s.PgsqlDelete(context.Background(), "mydb")
	})
}

func TestPgsqlChangePass(t *testing.T) {
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.PgsqlChangePass(context.Background(), "mydb", "newpw"); err != nil {
		t.Fatalf("PgsqlChangePass: %v", err)
	}
	assertString(t, gotParams, "dbPassword", "newpw")
}

func TestEditComment(t *testing.T) {
	var gotMethod string
	var gotParams map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.EditComment(context.Background(), "mysql", "mydb", "note"); err != nil {
		t.Fatalf("EditComment: %v", err)
	}
	if gotMethod != "databaseEditComment" {
		t.Errorf("method = %q, want databaseEditComment", gotMethod)
	}
	assertString(t, gotParams, "dbType", "mysql")
	assertString(t, gotParams, "dbComment", "note")
}

func TestSentinelFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.MysqlDelete(context.Background(), "mydb"); err == nil {
		t.Error("MysqlDelete with result 0: got nil error, want failure")
	}
}

func TestSentinelUnexpectedShape(t *testing.T) {
	// A non-integer result must not silently pass as success.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"unexpected":true}}`))
	})
	if err := s.MysqlDelete(context.Background(), "mydb"); err == nil {
		t.Error("MysqlDelete with object result: got nil error, want failure")
	}
}

// assertSentinelCall runs a one-argument sentinel method against a mock that
// answers a bare 1 and asserts the JSON-RPC method name reached the server.
func assertSentinelCall(t *testing.T, wantMethod string, call func(*Service) error) {
	t.Helper()
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decodeReq(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := call(s); err != nil {
		t.Fatalf("%s: %v", wantMethod, err)
	}
	if gotMethod != wantMethod {
		t.Errorf("method = %q, want %s", gotMethod, wantMethod)
	}
}

// assertString fails unless params[key] is the JSON string want.
func assertString(t *testing.T, params map[string]json.RawMessage, key, want string) {
	t.Helper()
	raw, ok := params[key]
	if !ok {
		t.Errorf("params missing %q, want %q", key, want)
		return
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Errorf("params[%q] = %s, not a string: %v", key, raw, err)
		return
	}
	if got != want {
		t.Errorf("params[%q] = %q, want %q", key, got, want)
	}
}

// decodeReq reads the JSON-RPC method and raw params from a request body.
func decodeReq(r *http.Request) (method string, params map[string]json.RawMessage) {
	var req struct {
		Method string                     `json:"method"`
		Params map[string]json.RawMessage `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	return req.Method, req.Params
}

// serve spins up a mock JSON-RPC server for h and returns a hosting.Service
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
