package backup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestBackupList(t *testing.T) {
	// getList groups backups by day; files/mysql arrive as int or null (counting).
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = methodOf(r)
		_, _ = w.Write([]byte(`{"result":[
			{"backupFilesExists":true,"date":"23.06.2023","files":18,"mysql":1,"warnQuota":false},
			{"backupFilesExists":true,"date":"01.06.2023","files":null,"mysql":0,"warnQuota":true}
		]}`))
	})
	list, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "getList" {
		t.Errorf("method = %q, want getList", gotMethod)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list[0].Date != "23.06.2023" || list[0].Files != 18 || list[0].Mysql != 1 || !list[0].BackupFilesExists {
		t.Errorf("entry[0] = %+v, want 23.06.2023/18/1/exists", list[0])
	}
	// null files/mysql decode to flex.Int 0 (the "still counting" state).
	if list[1].Files != 0 || list[1].Mysql != 0 || !list[1].WarnQuota {
		t.Errorf("entry[1] = %+v, want null files/mysql → 0 and warnQuota true", list[1])
	}
}

func TestBackupListFiles(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Date string `json:"date"`
		Dir  string `json:"dir"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = capture(r, &gotParams)
		_, _ = w.Write([]byte(`{"result":[
			{"dir":false,"name":".htaccess","size":"0 B"},
			{"dir":true,"name":"cgi-bin/","size":"4 KB"},
			{"dir":false,"name":"index.html","size":"309,89 KB"}
		]}`))
	})
	files, err := s.ListFiles(context.Background(), "2023-02-27", "/")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if gotMethod != "getListFiles" {
		t.Errorf("method = %q, want getListFiles", gotMethod)
	}
	if gotParams.Date != "2023-02-27" || gotParams.Dir != "/" {
		t.Errorf("params = %+v, want date 2023-02-27 / dir /", gotParams)
	}
	if len(files) != 3 {
		t.Fatalf("files len = %d, want 3", len(files))
	}
	if files[0].Name != ".htaccess" || files[0].Dir || files[0].Size != "0 B" {
		t.Errorf("files[0] = %+v, want .htaccess/file/0 B", files[0])
	}
	if !files[1].Dir || files[1].Name != "cgi-bin/" {
		t.Errorf("files[1] = %+v, want dir cgi-bin/", files[1])
	}
	// locale comma-decimal size stays a string, unparsed.
	if files[2].Size != "309,89 KB" {
		t.Errorf("files[2].Size = %q, want 309,89 KB", files[2].Size)
	}
}

func TestBackupListMysql(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Date string `json:"date"`
		Dir  string `json:"dir"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = capture(r, &gotParams)
		_, _ = w.Write([]byte(`{"result":[{"dir":true,"name":"test1234"}]}`))
	})
	dumps, err := s.ListMysql(context.Background(), "2023-02-27", "/")
	if err != nil {
		t.Fatalf("ListMysql: %v", err)
	}
	if gotMethod != "getListMysql" {
		t.Errorf("method = %q, want getListMysql", gotMethod)
	}
	if gotParams.Date != "2023-02-27" || gotParams.Dir != "/" {
		t.Errorf("params = %+v, want date 2023-02-27 / dir /", gotParams)
	}
	if len(dumps) != 1 || dumps[0].Name != "test1234" || !dumps[0].Dir {
		t.Errorf("dumps = %+v, want one dir test1234", dumps)
	}
}

func TestBackupMakeAccountCopy(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = methodOf(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	raw, err := s.MakeAccountCopy(context.Background())
	if err != nil {
		t.Fatalf("MakeAccountCopy: %v", err)
	}
	if gotMethod != "makeAccountCopy" {
		t.Errorf("method = %q, want makeAccountCopy", gotMethod)
	}
	if string(raw) != "1" {
		t.Errorf("raw = %s, want 1", raw)
	}
}

func TestBackupRestoreFiles(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Date  string              `json:"date"`
		Files [][]json.RawMessage `json:"files"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = capture(r, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	raw, err := s.RestoreFiles(context.Background(), "2023-02-27", []FileRef{
		{Dir: false, Path: "/test_mysql57_2023-06-07_16-00.sql.gz"},
		{Dir: true, Path: "/cgi-bin"},
	})
	if err != nil {
		t.Fatalf("RestoreFiles: %v", err)
	}
	if gotMethod != "restoreFiles" {
		t.Errorf("method = %q, want restoreFiles", gotMethod)
	}
	if gotParams.Date != "2023-02-27" {
		t.Errorf("date = %q, want 2023-02-27", gotParams.Date)
	}
	// Each FileRef marshals to the positional [type, path] pair.
	if len(gotParams.Files) != 2 {
		t.Fatalf("files len = %d, want 2", len(gotParams.Files))
	}
	if string(gotParams.Files[0][0]) != "0" || string(gotParams.Files[1][0]) != "1" {
		t.Errorf("file types = %s/%s, want 0/1 (file/folder)", gotParams.Files[0][0], gotParams.Files[1][0])
	}
	if string(gotParams.Files[0][1]) != `"/test_mysql57_2023-06-07_16-00.sql.gz"` {
		t.Errorf("file[0] path = %s, want the .sql.gz path", gotParams.Files[0][1])
	}
	if string(raw) != "1" {
		t.Errorf("raw = %s, want 1", raw)
	}
}

func TestBackupDownloadFile(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = methodOf(r)
		// Recorded example shape: [{"file":{mimetype,metadata,content,name}}].
		_, _ = w.Write([]byte(`{"result":[{"file":{
			"content":"H4sICA==","metadata":[],
			"mimetype":"application/gzip;base64","name":"dump.sql.gz"
		}}]}`))
	})
	raw, err := s.DownloadFile(context.Background(), "2023-02-27", []FileRef{
		{Dir: false, Path: "/dump.sql.gz"},
	})
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if gotMethod != "downloadFile" {
		t.Errorf("method = %q, want downloadFile", gotMethod)
	}
	// Returned raw so the caller can decode the base64 payload itself.
	var got []struct {
		File struct {
			Mimetype string `json:"mimetype"`
			Content  string `json:"content"`
			Name     string `json:"name"`
		} `json:"file"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	if len(got) != 1 || got[0].File.Name != "dump.sql.gz" || got[0].File.Mimetype != "application/gzip;base64" {
		t.Errorf("raw = %s, want one dump.sql.gz/gzip", raw)
	}
}

func TestBackupReceiveFiles(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Date  string              `json:"date"`
		Files [][]json.RawMessage `json:"files"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = capture(r, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	raw, err := s.ReceiveFiles(context.Background(), "2023-02-27", []FileRef{
		{Dir: false, Path: "/.authfile"},
	})
	if err != nil {
		t.Fatalf("ReceiveFiles: %v", err)
	}
	if gotMethod != "receiveFiles" {
		t.Errorf("method = %q, want receiveFiles", gotMethod)
	}
	if gotParams.Date != "2023-02-27" || len(gotParams.Files) != 1 {
		t.Errorf("params = %+v, want date + one file", gotParams)
	}
	if string(gotParams.Files[0][0]) != "0" || string(gotParams.Files[0][1]) != `"/.authfile"` {
		t.Errorf("file = %s/%s, want 0//.authfile", gotParams.Files[0][0], gotParams.Files[0][1])
	}
	if string(raw) != "1" {
		t.Errorf("raw = %s, want 1", raw)
	}
}

func TestBackupReceiveMysql(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Date      string   `json:"date"`
		Databases []string `json:"databases"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = capture(r, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	raw, err := s.ReceiveMysql(context.Background(), "2023-02-27", []string{"test123", "test456"})
	if err != nil {
		t.Fatalf("ReceiveMysql: %v", err)
	}
	if gotMethod != "receiveMysql" {
		t.Errorf("method = %q, want receiveMysql", gotMethod)
	}
	if gotParams.Date != "2023-02-27" || len(gotParams.Databases) != 2 || gotParams.Databases[0] != "test123" {
		t.Errorf("params = %+v, want date + two dbs", gotParams)
	}
	if string(raw) != "1" {
		t.Errorf("raw = %s, want 1", raw)
	}
}

func TestBackupRestoreMysql(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Date      string   `json:"date"`
		Databases []string `json:"databases"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = capture(r, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	raw, err := s.RestoreMysql(context.Background(), "2023-02-27", []string{"test123"})
	if err != nil {
		t.Fatalf("RestoreMysql: %v", err)
	}
	if gotMethod != "restoreMysql" {
		t.Errorf("method = %q, want restoreMysql", gotMethod)
	}
	// Doc types databases as a bare String; we forward a []string (one element here).
	if gotParams.Date != "2023-02-27" || len(gotParams.Databases) != 1 || gotParams.Databases[0] != "test123" {
		t.Errorf("params = %+v, want date + one db test123", gotParams)
	}
	if string(raw) != "1" {
		t.Errorf("raw = %s, want 1", raw)
	}
}

// rpcReq is the decoded JSON-RPC envelope of a request body. The body can only
// be read once, so methodOf/paramsOf each decode the whole envelope fresh — but
// no test calls both on the same request; those inspect params via paramsOf and
// assert the method from the same decode, so envelope is exposed via decodeReq.
type rpcReq struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// decodeReq reads and decodes the JSON-RPC envelope from r exactly once.
func decodeReq(r *http.Request) rpcReq {
	var req rpcReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	return req
}

// methodOf returns the JSON-RPC method name from a request body. Use only when
// the test does not also inspect params — the body reads once.
func methodOf(r *http.Request) string { return decodeReq(r).Method }

// capture decodes the request envelope once, unmarshals its params into v, and
// returns the method name — for tests that assert both (the body reads once, so
// methodOf + paramsOf on the same request would drain it).
func capture(r *http.Request, v any) string {
	req := decodeReq(r)
	_ = json.Unmarshal(req.Params, v)
	return req.Method
}

// serve spins up a mock JSON-RPC server for h and returns a backup.Service
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
