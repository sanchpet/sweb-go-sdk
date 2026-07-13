package contacts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

// serve spins up a mock JSON-RPC server for h and returns a Service backed by a
// transport pointed at it.
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

// capture records the method and raw params of a single JSON-RPC call and
// replies with the given result JSON (the bare result value, wrapped here in the
// envelope). It returns pointers the test can assert against after the call.
func capture(t *testing.T, result string) (s *Service, method *string, params *json.RawMessage) {
	t.Helper()
	var gotMethod string
	var gotParams json.RawMessage
	s = serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		gotParams = req.Params
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + result + `}`))
	})
	return s, &gotMethod, &gotParams
}

func TestContactsIndex(t *testing.T) {
	s, method, params := capture(t, `{"filterInfo":{"orderDirect":"desc","orderField":"type","page":1,"perPage":2,"totalCount":2},`+
		`"list":[{"id":"4320","name":"тест","type":"phone","value":"+7 921 8990681","verified":true},`+
		`{"id":"3205","name":"Иванов","type":"email","value":"test@gmail.ru","verified":true}]}`)
	got, err := s.Index(context.Background(), &ListOptions{Page: 1, PerPage: 2, OrderField: "type", OrderDir: "desc"})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if *method != "index" {
		t.Errorf("method = %q, want index", *method)
	}
	var p struct {
		OrderField  string `json:"orderField"`
		OrderDirect string `json:"orderDirect"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.OrderField != "type" || p.OrderDirect != "desc" {
		t.Errorf("params = %s", *params)
	}
	if len(got.List) != 2 || got.List[0].ID != 4320 || got.List[0].Type != "phone" {
		t.Errorf("list = %+v", got.List)
	}
	if got.FilterInfo.OrderField != "type" || got.FilterInfo.TotalCount != 2 {
		t.Errorf("filterInfo = %+v", got.FilterInfo)
	}
}

func TestContactsGetAll(t *testing.T) {
	s, method, _ := capture(t, `[{"admin":true,"id":"3204","name":"Иванов","type":"phone","value":"+7 949 8469541","verified":false},`+
		`{"admin":false,"id":"4320","name":"тест","type":"phone","value":"+7 921 8990681","verified":true}]`)
	got, err := s.GetAllContacts(context.Background())
	if err != nil {
		t.Fatalf("GetAllContacts: %v", err)
	}
	if *method != "getAllContacts" {
		t.Errorf("method = %q", *method)
	}
	if len(got) != 2 || !got[0].Admin || got[1].Admin {
		t.Errorf("contacts = %+v", got)
	}
	if got[0].ID != 3204 || got[0].Verified {
		t.Errorf("contacts[0] = %+v", got[0])
	}
}

func TestContactsAddReturnsID(t *testing.T) {
	cases := []struct {
		name   string
		call   func(*Service) (int64, error)
		method string
	}{
		{"AddContact", func(s *Service) (int64, error) {
			return s.AddContact(context.Background(), ContactEmail, "a@b.ru", "n")
		}, "addContact"},
		{"AddEmail", func(s *Service) (int64, error) {
			return s.AddEmail(context.Background(), "a@b.ru", "n")
		}, "addEmail"},
		{"AddPhone", func(s *Service) (int64, error) {
			return s.AddPhone(context.Background(), "+70000000000", "n")
		}, "addPhone"},
		{"AddTelegram", func(s *Service) (int64, error) {
			return s.AddTelegram(context.Background(), "n")
		}, "addTelegram"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, method, _ := capture(t, `4321`)
			id, err := tc.call(s)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if *method != tc.method {
				t.Errorf("method = %q, want %q", *method, tc.method)
			}
			if id != 4321 {
				t.Errorf("id = %d, want 4321 (the new contact id, not a 1/0 sentinel)", id)
			}
		})
	}
}

func TestContactsAddFailure(t *testing.T) {
	s, _, _ := capture(t, `0`)
	if _, err := s.AddEmail(context.Background(), "a@b.ru", "n"); err == nil {
		t.Fatal("AddEmail: want error on result 0")
	}
}

func TestContactsEditFamily(t *testing.T) {
	cases := []struct {
		name   string
		call   func(*Service) error
		method string
	}{
		{"EditContact", func(s *Service) error {
			return s.EditContact(context.Background(), "1", "v", "n")
		}, "editContact"},
		{"EditEmail", func(s *Service) error {
			return s.EditEmail(context.Background(), "1", "a@b.ru", "n")
		}, "editEmail"},
		{"EditPhone", func(s *Service) error {
			return s.EditPhone(context.Background(), "1", "+70000000000", "n")
		}, "editPhone"},
		{"EditTelegram", func(s *Service) error {
			return s.EditTelegram(context.Background(), "1", "n")
		}, "editTelegram"},
		{"DeleteContact", func(s *Service) error {
			return s.DeleteContact(context.Background(), "1")
		}, "deleteContact"},
		{"VerifyContact", func(s *Service) error {
			return s.VerifyContact(context.Background(), "1", "611021")
		}, "verifyContact"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, method, params := capture(t, `1`)
			if err := tc.call(s); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if *method != tc.method {
				t.Errorf("method = %q, want %q", *method, tc.method)
			}
			var p struct {
				ContactID string `json:"contactId"`
			}
			_ = json.Unmarshal(*params, &p)
			if p.ContactID != "1" {
				t.Errorf("contactId = %s", *params)
			}
		})
	}
}

func TestContactsEditFailure(t *testing.T) {
	s, _, _ := capture(t, `0`)
	if err := s.DeleteContact(context.Background(), "1"); err == nil {
		t.Fatal("DeleteContact: want error on result 0")
	}
}

func TestContactsDeleteContacts(t *testing.T) {
	// The spec's result is inconsistent (array type vs integer-1 example); accept
	// both the documented 1 and an array/true as success.
	for _, result := range []string{`1`, `true`, `[1]`} {
		t.Run(result, func(t *testing.T) {
			s, method, params := capture(t, result)
			if err := s.DeleteContacts(context.Background(), "1", "2"); err != nil {
				t.Fatalf("DeleteContacts(%s): %v", result, err)
			}
			if *method != "deleteContacts" {
				t.Errorf("method = %q", *method)
			}
			var p struct {
				ContactIDs []string `json:"contactIds"`
			}
			_ = json.Unmarshal(*params, &p)
			if len(p.ContactIDs) != 2 {
				t.Errorf("contactIds = %s", *params)
			}
		})
	}
}

func TestContactsDeleteContactsFailure(t *testing.T) {
	s, _, _ := capture(t, `0`)
	if err := s.DeleteContacts(context.Background(), "1"); err == nil {
		t.Fatal("DeleteContacts: want error on result 0")
	}
}

func TestContactsTelegramFlow(t *testing.T) {
	t.Run("requestTelegramVerifyCode", func(t *testing.T) {
		s, method, params := capture(t, `"611021"`)
		code, err := s.RequestTelegramVerifyCode(context.Background(), "4321")
		if err != nil {
			t.Fatalf("RequestTelegramVerifyCode: %v", err)
		}
		if *method != "requestTelegramVerifyCode" || code != "611021" {
			t.Errorf("method=%q code=%q", *method, code)
		}
		var p struct {
			ContactID string `json:"contactId"`
		}
		_ = json.Unmarshal(*params, &p)
		if p.ContactID != "4321" {
			t.Errorf("contactId = %s", *params)
		}
	})
	t.Run("isVerified", func(t *testing.T) {
		s, method, _ := capture(t, `true`)
		ok, err := s.IsVerified(context.Background(), "4321")
		if err != nil {
			t.Fatalf("IsVerified: %v", err)
		}
		if *method != "isVerified" || !ok {
			t.Errorf("method=%q ok=%v", *method, ok)
		}
	})
	t.Run("isVerified-false", func(t *testing.T) {
		s, _, _ := capture(t, `false`)
		ok, err := s.IsVerified(context.Background(), "4321")
		if err != nil || ok {
			t.Errorf("IsVerified false: ok=%v err=%v", ok, err)
		}
	})
}
