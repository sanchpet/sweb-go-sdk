package persons

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestPersonsList(t *testing.T) {
	// index wraps the contacts in a single-element array under {props_filled, persons};
	// resident/used/valid arrive as bare integers.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":[{"props_filled":1,"persons":[
			{"id":367684,"name":"Иванов Иван Иванович","resident":0,
			 "str":"[SWEB-FIZ-III-2168] Иванов Иван Иванович","sweb_handle":"SWEB-FIZ-III-2168",
			 "type":"f","used":0,"valid":1},
			{"id":368972,"name":"ООО Ромашка","resident":0,
			 "str":"[SWEB-ORG-R-1424] ООО Ромашка","sweb_handle":"SWEB-ORG-R-1424",
			 "type":"u","used":0,"valid":1}
		]}]}`))
	})
	list, propsFilled, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if !propsFilled {
		t.Errorf("propsFilled = false, want true")
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	p := list[0]
	if p.ID != 367684 || p.SwebHandle != "SWEB-FIZ-III-2168" || p.Type != TypeIndividual {
		t.Errorf("person[0] = %+v, want id 367684 / SWEB-FIZ-III-2168 / type f", p)
	}
	if p.Resident != 0 || p.Used != 0 || p.Valid != 1 {
		t.Errorf("person[0] flags = %+v, want resident 0 / used 0 / valid 1", p)
	}
	if list[1].Type != TypeLegal {
		t.Errorf("person[1].Type = %q, want u", list[1].Type)
	}
}

func TestPersonsListEmpty(t *testing.T) {
	// An empty array result must not panic and must report no contacts.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	list, propsFilled, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 || propsFilled {
		t.Errorf("empty result = (%d persons, propsFilled %v), want (0, false)", len(list), propsFilled)
	}
}

func TestPersonsInfo(t *testing.T) {
	// getinfo wraps the record in a single-element array; phones/emails arrive as
	// arrays despite the doc typing them string; inn is null → "".
	var gotMethod string
	var gotID int
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				ID int `json:"id"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotID = req.Method, req.Params.ID
		_, _ = w.Write([]byte(`{"result":[{
			"name":"Иванов Иван Иванович","nameTrans":"Ivanov Ivan Ivanovich",
			"resident":false,"phones":["+7 999 9999999"],"emails":["test@sweb.ru"],
			"inn":null,"type":"f","used":0,
			"postIndex":"197376","postCity":"Санкт-Петербург","postAddress":"наб. р. Карповки, д. 5, корп. 3",
			"birthdate":"1990-01-01","passSeries":"4502","passNum":"987432",
			"passDate":"2010-01-01","passOrg":"ОВД района Южное Бутово города Москвы"
		}]}`))
	})
	info, err := s.Info(context.Background(), 367684)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if gotMethod != "getinfo" {
		t.Errorf("method = %q, want getinfo", gotMethod)
	}
	if gotID != 367684 {
		t.Errorf("id param = %d, want 367684", gotID)
	}
	if info == nil {
		t.Fatal("Info = nil, want a record")
	}
	if info.Name != "Иванов Иван Иванович" || info.Type != TypeIndividual || info.Resident {
		t.Errorf("info = %+v, want name/type f/resident false", info)
	}
	if len(info.Phones) != 1 || info.Phones[0] != "+7 999 9999999" {
		t.Errorf("phones = %v, want [+7 999 9999999]", info.Phones)
	}
	if len(info.Emails) != 1 || info.Emails[0] != "test@sweb.ru" {
		t.Errorf("emails = %v, want [test@sweb.ru]", info.Emails)
	}
	if info.INN != "" {
		t.Errorf("inn = %q, want empty (was null)", info.INN)
	}
	if info.Birthdate != "1990-01-01" || info.PassSeries != "4502" {
		t.Errorf("individual fields = %+v, want birthdate/passSeries", info)
	}
}

func TestPersonsInfoScalarPhones(t *testing.T) {
	// The apidoc types phones/emails as scalar strings; StringOrList must also
	// accept that shape, wrapping it in a one-element slice.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"phones":"+7 999 9999999","emails":""}]}`))
	})
	info, err := s.Info(context.Background(), 1)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if len(info.Phones) != 1 || info.Phones[0] != "+7 999 9999999" {
		t.Errorf("scalar phones = %v, want one-element slice", info.Phones)
	}
	if len(info.Emails) != 0 {
		t.Errorf("empty-string emails = %v, want nil", info.Emails)
	}
}

func TestPersonsInfoEmpty(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	info, err := s.Info(context.Background(), 1)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info != nil {
		t.Errorf("Info on empty result = %+v, want nil", info)
	}
}

func TestPersonsCreateFizIP(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Name        string `json:"name"`
		Resident    bool   `json:"resident"`
		Phones      string `json:"phones"`
		Emails      string `json:"emails"`
		PostIndex   string `json:"postIndex"`
		PostCity    string `json:"postCity"`
		PostAddress string `json:"postAddress"`
		Birthdate   string `json:"birthdate"`
		INN         string `json:"inn"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	err := s.CreateFizIP(context.Background(), FizIPOptions{
		Name:        "ИП Иванов Иван Иванович",
		Resident:    true,
		Phones:      "+7 999 9999999",
		Emails:      "test@sweb.ru",
		PostIndex:   "197376",
		PostCity:    "Санкт-Петербург",
		PostAddress: "наб. р. Карповки, д. 5, корп. 3",
		Birthdate:   "1990-01-01",
		INN:         "123456789123",
	})
	if err != nil {
		t.Fatalf("CreateFizIP: %v", err)
	}
	if gotMethod != "createFizIp" {
		t.Errorf("method = %q, want createFizIp", gotMethod)
	}
	if gotParams.Name != "ИП Иванов Иван Иванович" || !gotParams.Resident || gotParams.Birthdate != "1990-01-01" {
		t.Errorf("params = %+v, want name/resident/birthdate", gotParams)
	}
	if gotParams.INN != "123456789123" {
		t.Errorf("inn = %q, want 123456789123", gotParams.INN)
	}
}

func TestPersonsCreateFizIPOmitsEmptyOptionals(t *testing.T) {
	// Unset optional string fields (passSeries, inn, id, …) must be omitted from
	// the request rather than sent blank.
	var params map[string]json.RawMessage
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params map[string]json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		params = req.Params
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.CreateFizIP(context.Background(), FizIPOptions{Name: "X"}); err != nil {
		t.Fatalf("CreateFizIP: %v", err)
	}
	for _, k := range []string{"passSeries", "passNum", "passDate", "passOrg", "inn", "id"} {
		if _, ok := params[k]; ok {
			t.Errorf("params carried empty optional %q, want it omitted", k)
		}
	}
}

func TestPersonsCreateJur(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Name       string `json:"name"`
		NameTrans  string `json:"nameTrans"`
		Resident   bool   `json:"resident"`
		Phones1    string `json:"phones1"`
		JurAddress string `json:"jurAddress"`
		INN        string `json:"inn"`
		KPP        string `json:"kpp"`
		PersName   string `json:"persName"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	err := s.CreateJur(context.Background(), JurOptions{
		Name:       "ООО Ромашка",
		NameTrans:  "OOO Romashka",
		Resident:   true,
		Phones1:    "+7 930 7654323",
		JurAddress: "наб. р. Карповки, д.5, корп.3",
		INN:        "3664069397",
		KPP:        "3664069397",
		PersName:   "Иванов Иван Иванович",
	})
	if err != nil {
		t.Fatalf("CreateJur: %v", err)
	}
	if gotMethod != "createJur" {
		t.Errorf("method = %q, want createJur", gotMethod)
	}
	if gotParams.Name != "ООО Ромашка" || gotParams.NameTrans != "OOO Romashka" || !gotParams.Resident {
		t.Errorf("params = %+v, want name/nameTrans/resident", gotParams)
	}
	if gotParams.INN != "3664069397" || gotParams.KPP != "3664069397" || gotParams.PersName != "Иванов Иван Иванович" {
		t.Errorf("legal fields = %+v, want inn/kpp/persName", gotParams)
	}
}

func TestPersonsCreateSentinelFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.CreateJur(context.Background(), JurOptions{Name: "X"}); err == nil {
		t.Error("CreateJur with result 0: got nil error, want failure")
	}
}

// serve spins up a mock JSON-RPC server for h and returns a persons.Service
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
