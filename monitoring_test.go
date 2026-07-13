package sweb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// capture records the method and raw params of a single JSON-RPC call and
// replies with the given result JSON (the bare result value, wrapped here in the
// envelope). It returns pointers the test can assert against after the call.
func capture(t *testing.T, result string) (c *Client, method *string, params *json.RawMessage) {
	t.Helper()
	var gotMethod string
	var gotParams json.RawMessage
	c = serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		gotParams = req.Params
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + result + `}`))
	})
	return c, &gotMethod, &gotParams
}

// ---- /monitoring ----------------------------------------------------------

func TestMonitoringPlans(t *testing.T) {
	c, method, _ := capture(t, `[{"id":1,"name":"Базовый","checks":1,"sms":6,"price":30},`+
		`{"id":2,"name":"Стандартный","checks":10,"sms":30,"price":150.5}]`)
	plans, err := c.Monitoring.Plans(context.Background())
	if err != nil {
		t.Fatalf("Plans: %v", err)
	}
	if *method != "plans" {
		t.Errorf("method = %q, want plans", *method)
	}
	if len(plans) != 2 {
		t.Fatalf("got %d plans, want 2", len(plans))
	}
	if plans[0].ID != 1 || plans[0].Name != "Базовый" || plans[0].Checks != 1 || plans[0].SMS != 6 || plans[0].Price != 30 {
		t.Errorf("plans[0] = %+v", plans[0])
	}
	if plans[1].Price != 150.5 {
		t.Errorf("plans[1].Price = %v, want 150.5 (FlexFloat)", plans[1].Price)
	}
}

func TestMonitoringTariffActions(t *testing.T) {
	cases := []struct {
		name   string
		call   func(*Client) error
		method string
	}{
		{"Enable", func(c *Client) error { return c.Monitoring.Enable(context.Background(), 2) }, "enable"},
		{"Disable", func(c *Client) error { return c.Monitoring.Disable(context.Background(), 2) }, "disable"},
		{"Change", func(c *Client) error { return c.Monitoring.Change(context.Background(), 3) }, "change"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, method, params := capture(t, `1`)
			if err := tc.call(c); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if *method != tc.method {
				t.Errorf("method = %q, want %q", *method, tc.method)
			}
			var p struct {
				ID int `json:"id"`
			}
			_ = json.Unmarshal(*params, &p)
			if p.ID == 0 {
				t.Errorf("id param not sent: %s", *params)
			}
		})
	}
}

func TestMonitoringTariffActionFailure(t *testing.T) {
	c, _, _ := capture(t, `0`)
	if err := c.Monitoring.Enable(context.Background(), 1); err == nil {
		t.Fatal("Enable: want error on result 0, got nil")
	}
}

// ---- /monitoring/checks ---------------------------------------------------

func TestChecksIndex(t *testing.T) {
	c, method, params := capture(t, `{"filterInfo":{"page":1,"perPage":2,"totalCount":1},`+
		`"list":[{"disabled":false,"id":"339","lastResult":true,"name":"sweb.ru","status":true,`+
		`"tsDeltaResult":null,"tsLastResult":null,"type":"1"}]}`)
	got, err := c.MonitoringChecks.Index(context.Background(), &ListOptions{Page: 1, PerPage: 2})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if *method != "index" {
		t.Errorf("method = %q, want index", *method)
	}
	var p struct {
		Page    int `json:"page"`
		PerPage int `json:"perPage"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.Page != 1 || p.PerPage != 2 {
		t.Errorf("params = %s, want page=1 perPage=2", *params)
	}
	if got.FilterInfo.TotalCount != 1 || len(got.List) != 1 {
		t.Fatalf("filterInfo/list = %+v", got)
	}
	if got.List[0].ID != "339" || got.List[0].Type != "1" || !got.List[0].Status || got.List[0].Disabled {
		t.Errorf("list[0] = %+v", got.List[0])
	}
}

func TestChecksReferenceLists(t *testing.T) {
	t.Run("getTypes", func(t *testing.T) {
		c, method, _ := capture(t, `[{"code":"ping","id":"1","name":"Ping"},{"code":"http","id":"2","name":"HTTP"}]`)
		got, err := c.MonitoringChecks.GetTypes(context.Background())
		if err != nil || *method != "getTypes" {
			t.Fatalf("GetTypes: err=%v method=%q", err, *method)
		}
		if len(got) != 2 || got[0].Code != "ping" || got[1].Name != "HTTP" {
			t.Errorf("types = %+v", got)
		}
	})
	t.Run("getIntervals", func(t *testing.T) {
		c, method, _ := capture(t, `[{"id":"1","name":"1 мин","time":"1"},{"id":"7","name":"1 час","time":"60"}]`)
		got, err := c.MonitoringChecks.GetIntervals(context.Background())
		if err != nil || *method != "getIntervals" {
			t.Fatalf("GetIntervals: err=%v method=%q", err, *method)
		}
		if len(got) != 2 || got[1].Time != "60" {
			t.Errorf("intervals = %+v", got)
		}
	})
	t.Run("getPorts", func(t *testing.T) {
		c, method, _ := capture(t, `[{"name":"HTTPS","nameFull":"Hypertext Transfer Protocol Secure","value":"443"}]`)
		got, err := c.MonitoringChecks.GetPorts(context.Background())
		if err != nil || *method != "getPorts" {
			t.Fatalf("GetPorts: err=%v method=%q", err, *method)
		}
		if len(got) != 1 || got[0].Value != "443" {
			t.Errorf("ports = %+v", got)
		}
	})
	t.Run("getKeywordModes", func(t *testing.T) {
		c, method, _ := capture(t, `[{"id":"1","name":"на странице должны быть все слова"}]`)
		got, err := c.MonitoringChecks.GetKeywordModes(context.Background())
		if err != nil || *method != "getKeywordModes" {
			t.Fatalf("GetKeywordModes: err=%v method=%q", err, *method)
		}
		if len(got) != 1 || got[0].ID != "1" {
			t.Errorf("modes = %+v", got)
		}
	})
}

func TestChecksGetInfo(t *testing.T) {
	c, method, _ := capture(t, `{"active":true,"availableChecks":0,"availableSms":6,"currentChecks":1,`+
		`"currentSms":0,"expired":"29.10.2025","totalChecks":1,"totalSms":6,`+
		`"types":[{"code":"ping","id":"1","name":"Ping"}],`+
		`"intervals":[{"id":"1","name":"1 мин","time":"1"}],`+
		`"keywordModes":[{"id":"1","name":"m"}],`+
		`"ports":[{"name":"WWW","nameFull":"Web","value":"80"}],"tariff":null}`)
	got, err := c.MonitoringChecks.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if *method != "getInfo" {
		t.Errorf("method = %q, want getInfo", *method)
	}
	if !got.Active || got.AvailableSMS != 6 || got.CurrentChecks != 1 {
		t.Errorf("counters = %+v", got)
	}
	if len(got.Types) != 1 || len(got.Intervals) != 1 || len(got.Ports) != 1 || len(got.KeywordModes) != 1 {
		t.Errorf("nested lists = %+v", got)
	}
}

func TestChecksGetFullCheckInfo(t *testing.T) {
	c, method, params := capture(t, `{"contacts":[{"id":3205,"name":"Тест","type":"email","value":"test@sweb.ru","verified":true}],`+
		`"id":1,"lastResult":true,"name":"sweb.ru","settings":[{"type":"target","value":"192.168.122.75"},`+
		`{"type":"interval","value":"4"}],"status":true,"type":1}`)
	got, err := c.MonitoringChecks.GetFullCheckInfo(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetFullCheckInfo: %v", err)
	}
	if *method != "getFullCheckInfo" {
		t.Errorf("method = %q", *method)
	}
	var p struct {
		ID int `json:"id"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.ID != 1 {
		t.Errorf("id param = %s", *params)
	}
	if got.ID != 1 || got.Type != 1 || len(got.Settings) != 2 || len(got.Contacts) != 1 {
		t.Errorf("full info = %+v", got)
	}
	if got.Settings[0].Type != "target" || got.Contacts[0].Value != "test@sweb.ru" {
		t.Errorf("nested = %+v / %+v", got.Settings, got.Contacts)
	}
}

func TestChecksCreate(t *testing.T) {
	c, method, params := capture(t, `1`)
	err := c.MonitoringChecks.Create(context.Background(), CheckSpec{
		Type: 2, Target: "https://example.com", Name: "web", Interval: 3,
		ContactIDs: []int{3205}, SSL: true, Keywords: []string{"ok"}, KeywordMode: 1,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if *method != "create" {
		t.Errorf("method = %q, want create", *method)
	}
	var p struct {
		Type        int      `json:"type"`
		Target      string   `json:"target"`
		ContactIDs  []int    `json:"contactIds"`
		SSL         bool     `json:"ssl"`
		Keywords    []string `json:"keywords"`
		KeywordMode int      `json:"keywordMode"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.Type != 2 || p.Target != "https://example.com" || len(p.ContactIDs) != 1 || !p.SSL || len(p.Keywords) != 1 || p.KeywordMode != 1 {
		t.Errorf("params = %s", *params)
	}
}

func TestChecksCreateFailure(t *testing.T) {
	c, _, _ := capture(t, `0`)
	if err := c.MonitoringChecks.Create(context.Background(), CheckSpec{Type: 1, Target: "x", Name: "n", Interval: 1, ContactIDs: []int{1}}); err == nil {
		t.Fatal("Create: want error on result 0")
	}
}

func TestChecksEdit(t *testing.T) {
	c, method, params := capture(t, `1`)
	err := c.MonitoringChecks.Edit(context.Background(), 42, CheckSpec{
		Type: 3, Target: "1.2.3.4", Name: "port", Interval: 2, ContactIDs: []int{1}, Port: 443,
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if *method != "edit" {
		t.Errorf("method = %q, want edit", *method)
	}
	var p struct {
		ID   int `json:"id"`
		Port int `json:"port"`
		Type int `json:"type"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.ID != 42 || p.Port != 443 {
		t.Errorf("params = %s, want id=42 port=443", *params)
	}
	if p.Type != 0 {
		t.Errorf("edit must not send type, got %s", *params)
	}
}

func TestChecksToggleAndRemove(t *testing.T) {
	cases := []struct {
		name   string
		call   func(*Client) error
		method string
		bulk   bool
	}{
		{"Activate", func(c *Client) error { return c.MonitoringChecks.Activate(context.Background(), 1) }, "activate", false},
		{"ActivateList", func(c *Client) error { return c.MonitoringChecks.ActivateList(context.Background(), 1, 2) }, "activateList", true},
		{"Deactivate", func(c *Client) error { return c.MonitoringChecks.Deactivate(context.Background(), 1) }, "deactivate", false},
		{"DeactivateList", func(c *Client) error { return c.MonitoringChecks.DeactivateList(context.Background(), 1, 2) }, "deactivateList", true},
		{"Remove", func(c *Client) error { return c.MonitoringChecks.Remove(context.Background(), 1) }, "remove", false},
		{"RemoveList", func(c *Client) error { return c.MonitoringChecks.RemoveList(context.Background(), 1, 2) }, "removeList", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, method, params := capture(t, `1`)
			if err := tc.call(c); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if *method != tc.method {
				t.Errorf("method = %q, want %q", *method, tc.method)
			}
			var p struct {
				ID  int   `json:"id"`
				IDs []int `json:"ids"`
			}
			_ = json.Unmarshal(*params, &p)
			if tc.bulk && len(p.IDs) != 2 {
				t.Errorf("bulk ids = %s, want 2", *params)
			}
			if !tc.bulk && p.ID != 1 {
				t.Errorf("id = %s, want 1", *params)
			}
		})
	}
}

func TestChecksActionFailure(t *testing.T) {
	c, _, _ := capture(t, `0`)
	if err := c.MonitoringChecks.Activate(context.Background(), 1); err == nil {
		t.Fatal("Activate: want error on result 0")
	}
}

func TestChecksHistory(t *testing.T) {
	c, method, params := capture(t, `{"filterInfo":{"page":1,"perPage":20,"totalCount":1},`+
		`"list":[{"id":"7","check_id":"339","ts":"2025-01-01 00:00:00","success":"y"}]}`)
	got, err := c.MonitoringChecks.History(context.Background(), 339, &HistoryOptions{StartDate: "2025-01-01", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if *method != "history" {
		t.Errorf("method = %q, want history", *method)
	}
	var p struct {
		ID        int    `json:"id"`
		StartDate string `json:"startDate"`
	}
	_ = json.Unmarshal(*params, &p)
	if p.ID != 339 || p.StartDate != "2025-01-01" {
		t.Errorf("params = %s", *params)
	}
	if len(got.List) != 1 || got.List[0].CheckID != "339" || got.List[0].Success != "y" {
		t.Errorf("history = %+v", got)
	}
}

// ---- /monitoring/contacts -------------------------------------------------

func TestContactsIndex(t *testing.T) {
	c, method, params := capture(t, `{"filterInfo":{"orderDirect":"desc","orderField":"type","page":1,"perPage":2,"totalCount":2},`+
		`"list":[{"id":"4320","name":"тест","type":"phone","value":"+7 921 8990681","verified":true},`+
		`{"id":"3205","name":"Иванов","type":"email","value":"test@gmail.ru","verified":true}]}`)
	got, err := c.MonitoringContacts.Index(context.Background(), &ContactListOptions{Page: 1, PerPage: 2, OrderField: "type", OrderDir: "desc"})
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
	c, method, _ := capture(t, `[{"admin":true,"id":"3204","name":"Иванов","type":"phone","value":"+7 949 8469541","verified":false},`+
		`{"admin":false,"id":"4320","name":"тест","type":"phone","value":"+7 921 8990681","verified":true}]`)
	got, err := c.MonitoringContacts.GetAllContacts(context.Background())
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
		call   func(*Client) (int64, error)
		method string
	}{
		{"AddContact", func(c *Client) (int64, error) {
			return c.MonitoringContacts.AddContact(context.Background(), ContactEmail, "a@b.ru", "n")
		}, "addContact"},
		{"AddEmail", func(c *Client) (int64, error) {
			return c.MonitoringContacts.AddEmail(context.Background(), "a@b.ru", "n")
		}, "addEmail"},
		{"AddPhone", func(c *Client) (int64, error) {
			return c.MonitoringContacts.AddPhone(context.Background(), "+70000000000", "n")
		}, "addPhone"},
		{"AddTelegram", func(c *Client) (int64, error) {
			return c.MonitoringContacts.AddTelegram(context.Background(), "n")
		}, "addTelegram"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, method, _ := capture(t, `4321`)
			id, err := tc.call(c)
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
	c, _, _ := capture(t, `0`)
	if _, err := c.MonitoringContacts.AddEmail(context.Background(), "a@b.ru", "n"); err == nil {
		t.Fatal("AddEmail: want error on result 0")
	}
}

func TestContactsEditFamily(t *testing.T) {
	cases := []struct {
		name   string
		call   func(*Client) error
		method string
	}{
		{"EditContact", func(c *Client) error {
			return c.MonitoringContacts.EditContact(context.Background(), "1", "v", "n")
		}, "editContact"},
		{"EditEmail", func(c *Client) error {
			return c.MonitoringContacts.EditEmail(context.Background(), "1", "a@b.ru", "n")
		}, "editEmail"},
		{"EditPhone", func(c *Client) error {
			return c.MonitoringContacts.EditPhone(context.Background(), "1", "+70000000000", "n")
		}, "editPhone"},
		{"EditTelegram", func(c *Client) error {
			return c.MonitoringContacts.EditTelegram(context.Background(), "1", "n")
		}, "editTelegram"},
		{"DeleteContact", func(c *Client) error {
			return c.MonitoringContacts.DeleteContact(context.Background(), "1")
		}, "deleteContact"},
		{"VerifyContact", func(c *Client) error {
			return c.MonitoringContacts.VerifyContact(context.Background(), "1", "611021")
		}, "verifyContact"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, method, params := capture(t, `1`)
			if err := tc.call(c); err != nil {
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
	c, _, _ := capture(t, `0`)
	if err := c.MonitoringContacts.DeleteContact(context.Background(), "1"); err == nil {
		t.Fatal("DeleteContact: want error on result 0")
	}
}

func TestContactsDeleteContacts(t *testing.T) {
	// The spec's result is inconsistent (array type vs integer-1 example); accept
	// both the documented 1 and an array/true as success.
	for _, result := range []string{`1`, `true`, `[1]`} {
		t.Run(result, func(t *testing.T) {
			c, method, params := capture(t, result)
			if err := c.MonitoringContacts.DeleteContacts(context.Background(), "1", "2"); err != nil {
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
	c, _, _ := capture(t, `0`)
	if err := c.MonitoringContacts.DeleteContacts(context.Background(), "1"); err == nil {
		t.Fatal("DeleteContacts: want error on result 0")
	}
}

func TestContactsTelegramFlow(t *testing.T) {
	t.Run("requestTelegramVerifyCode", func(t *testing.T) {
		c, method, params := capture(t, `"611021"`)
		code, err := c.MonitoringContacts.RequestTelegramVerifyCode(context.Background(), "4321")
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
		c, method, _ := capture(t, `true`)
		ok, err := c.MonitoringContacts.IsVerified(context.Background(), "4321")
		if err != nil {
			t.Fatalf("IsVerified: %v", err)
		}
		if *method != "isVerified" || !ok {
			t.Errorf("method=%q ok=%v", *method, ok)
		}
	})
	t.Run("isVerified-false", func(t *testing.T) {
		c, _, _ := capture(t, `false`)
		ok, err := c.MonitoringContacts.IsVerified(context.Background(), "4321")
		if err != nil || ok {
			t.Errorf("IsVerified false: ok=%v err=%v", ok, err)
		}
	})
}
