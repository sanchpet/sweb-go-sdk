package mail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

// --- Read-only methods -----------------------------------------------------

func TestMailDomainsList(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":{
			"filterInfo":{"limit":20,"orderDirect":"asc","orderField":0,"page":1,"totalCount":1},
			"list":[{"autoDiscover":0,"dkim":"on","emailCollector":null,"fqdn_readable":"test.ru",
			"mailboxesCnt":4,"quota":0,"senderVerify":0,"spf":0}]
		}}`))
	})
	out, err := s.DomainsList(context.Background(), ListOptions{Page: 1, Limit: 20, OrderDirect: "asc"})
	if err != nil {
		t.Fatalf("DomainsList: %v", err)
	}
	if gotMethod != "getDomainsList" {
		t.Errorf("method = %q, want getDomainsList", gotMethod)
	}
	if gotParams["page"] != float64(1) || gotParams["orderDirect"] != "asc" {
		t.Errorf("params = %v, want page 1 / orderDirect asc", gotParams)
	}
	if len(out.List) != 1 || out.List[0].FQDN != "test.ru" || out.List[0].MailboxesCnt != 4 || out.List[0].DKIM != "on" {
		t.Errorf("list = %+v, want one test.ru/4/dkim on", out.List)
	}
	if out.List[0].EmailCollector != "" {
		t.Errorf("emailCollector = %q, want empty (null)", out.List[0].EmailCollector)
	}
	if out.FilterInfo.TotalCount != 1 || out.FilterInfo.OrderDirect != "asc" {
		t.Errorf("filterInfo = %+v, want totalCount 1 / asc", out.FilterInfo)
	}
}

func TestMailMailboxesList(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":{
			"filterInfo":{"limit":20,"orderDirect":"asc","orderField":0,"page":1,"totalCount":3},
			"list":[{"antispam":10,"comment":"c","mbox":"test","purpose":"mail","quota":0,"spf":1}]
		}}`))
	})
	out, err := s.MailboxesList(context.Background(), "test.ru", "test", ListOptions{Page: 1})
	if err != nil {
		t.Fatalf("MailboxesList: %v", err)
	}
	if gotMethod != "getMailboxesList" {
		t.Errorf("method = %q, want getMailboxesList", gotMethod)
	}
	if gotParams["domain"] != "test.ru" || gotParams["searchMbox"] != "test" {
		t.Errorf("params = %v, want domain/searchMbox", gotParams)
	}
	if len(out.List) != 1 || out.List[0].Mbox != "test" || out.List[0].Antispam != 10 || out.List[0].SPF != 1 {
		t.Errorf("list = %+v, want one test/antispam 10/spf 1", out.List)
	}
	if out.FilterInfo.TotalCount != 3 {
		t.Errorf("totalCount = %d, want 3", out.FilterInfo.TotalCount)
	}
}

func TestMailMailboxesListSearchOmittedWhenEmpty(t *testing.T) {
	var hasSearch bool
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, p := decode(r)
		_, hasSearch = p["searchMbox"]
		_, _ = w.Write([]byte(`{"result":{"list":[],"filterInfo":{}}}`))
	})
	if _, err := s.MailboxesList(context.Background(), "test.ru", "", ListOptions{}); err != nil {
		t.Fatalf("MailboxesList: %v", err)
	}
	if hasSearch {
		t.Error("params carried an empty searchMbox key, want it omitted")
	}
}

func TestMailMailQuota(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decode(r)
		_, _ = w.Write([]byte(`{"result":42}`))
	})
	got, err := s.MailQuota(context.Background())
	if err != nil {
		t.Fatalf("MailQuota: %v", err)
	}
	if gotMethod != "getMailQuota" {
		t.Errorf("method = %q, want getMailQuota", gotMethod)
	}
	if got != 42 {
		t.Errorf("MailQuota = %d, want 42", got)
	}
}

func TestMailAutoreply(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":"on vacation"}`))
	})
	got, err := s.Autoreply(context.Background(), "test.ru", "box")
	if err != nil {
		t.Fatalf("Autoreply: %v", err)
	}
	if gotMethod != "getAutoreply" || gotParams["domain"] != "test.ru" || gotParams["mbox"] != "box" {
		t.Errorf("method/params = %q/%v, want getAutoreply test.ru/box", gotMethod, gotParams)
	}
	if got != "on vacation" {
		t.Errorf("Autoreply = %q, want 'on vacation'", got)
	}
}

func TestMailForwardingEmailsList(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decode(r)
		_, _ = w.Write([]byte(`{"result":["a@test.ru","b@test.ru"]}`))
	})
	got, err := s.ForwardingEmailsList(context.Background(), "test.ru", "box")
	if err != nil {
		t.Fatalf("ForwardingEmailsList: %v", err)
	}
	if gotMethod != "getForwardingEmailsList" {
		t.Errorf("method = %q, want getForwardingEmailsList", gotMethod)
	}
	if len(got) != 2 || got[0] != "a@test.ru" {
		t.Errorf("list = %v, want [a@test.ru b@test.ru]", got)
	}
}

func TestMailIsDeletingAfterForwarding(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{`{"result":1}`, true},
		{`{"result":0}`, false},
	} {
		var gotMethod string
		s := serve(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod, _ = decode(r)
			_, _ = w.Write([]byte(tc.body))
		})
		got, err := s.IsDeletingAfterForwarding(context.Background(), "test.ru", "box")
		if err != nil {
			t.Fatalf("IsDeletingAfterForwarding(%s): %v", tc.body, err)
		}
		if gotMethod != "isEnabledDeletingAfterForwarding" {
			t.Errorf("method = %q, want isEnabledDeletingAfterForwarding", gotMethod)
		}
		if got != tc.want {
			t.Errorf("IsDeletingAfterForwarding(%s) = %v, want %v", tc.body, got, tc.want)
		}
	}
}

func TestMailDeliveryAddressesList(t *testing.T) {
	// Doc-vs-reality: the spec types this as an array but the API returns the
	// {list, filterInfo} envelope with a quoted totalCount.
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decode(r)
		_, _ = w.Write([]byte(`{"result":{"filterInfo":{"limit":20,"page":1,"totalCount":"1"},"list":["x@test.ru"]}}`))
	})
	out, err := s.DeliveryAddressesList(context.Background(), "test.ru", "box", ListOptions{Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("DeliveryAddressesList: %v", err)
	}
	if gotMethod != "getDeliveryAddressesList" {
		t.Errorf("method = %q, want getDeliveryAddressesList", gotMethod)
	}
	if len(out.List) != 1 || out.List[0] != "x@test.ru" || out.FilterInfo.TotalCount != 1 {
		t.Errorf("out = %+v, want one x@test.ru / totalCount 1 (from \"1\")", out)
	}
}

func TestMailDeliveryInfo(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decode(r)
		_, _ = w.Write([]byte(`{"result":{"addresses":{"current":0,"max":100},"groups":{"current":2,"max":10}}}`))
	})
	out, err := s.DeliveryInfo(context.Background(), "test.ru", "box")
	if err != nil {
		t.Fatalf("DeliveryInfo: %v", err)
	}
	if gotMethod != "getDeliveryInfo" {
		t.Errorf("method = %q, want getDeliveryInfo", gotMethod)
	}
	if out.Groups.Current != 2 || out.Groups.Max != 10 || out.Addresses.Max != 100 {
		t.Errorf("out = %+v, want groups 2/10 addresses .../100", out)
	}
}

func TestMailMailsCollector(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":"collector@gmail.com"}`))
	})
	got, err := s.MailsCollector(context.Background(), "test.ru")
	if err != nil {
		t.Fatalf("MailsCollector: %v", err)
	}
	if gotMethod != "getMailsCollector" || gotParams["domain"] != "test.ru" {
		t.Errorf("method/params = %q/%v, want getMailsCollector test.ru", gotMethod, gotParams)
	}
	if got != "collector@gmail.com" {
		t.Errorf("MailsCollector = %q, want collector@gmail.com", got)
	}
}

func TestMailWhitelist(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":{"filterInfo":{"limit":20,"page":1,"totalCount":1},"list":["w@test.ru"]}}`))
	})
	out, err := s.Whitelist(context.Background(), "test.ru", "box", ListOptions{Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("Whitelist: %v", err)
	}
	if gotMethod != "getWhitelist" || gotParams["page"] != float64(1) {
		t.Errorf("method/params = %q/%v, want getWhitelist page 1", gotMethod, gotParams)
	}
	if len(out.List) != 1 || out.List[0] != "w@test.ru" {
		t.Errorf("list = %v, want [w@test.ru]", out.List)
	}
}

func TestMailBlacklist(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, _ = decode(r)
		_, _ = w.Write([]byte(`{"result":{"filterInfo":{"limit":20,"page":1,"totalCount":1},"list":["b@test.ru"]}}`))
	})
	out, err := s.Blacklist(context.Background(), "test.ru", "box", ListOptions{Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("Blacklist: %v", err)
	}
	if gotMethod != "getBlacklist" {
		t.Errorf("method = %q, want getBlacklist", gotMethod)
	}
	if len(out.List) != 1 || out.List[0] != "b@test.ru" {
		t.Errorf("list = %v, want [b@test.ru]", out.List)
	}
}

// --- Mailbox lifecycle -----------------------------------------------------

func TestMailCreateMbox(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":{
			"detailed":"https://help.sweb.ru/29/","login":"box@test.ru",
			"mailProgramSettings":[{"name":"IMAP","port":"143","server":"imap.spaceweb.ru"}],
			"password":"Pw123!","webMail":"https://webmail.sweb.ru",
			"pdf":{"content":"JVBERi0=","metadata":[],"mimetype":"application/pdf;base64","name":"requisites.pdf"}
		}}`))
	})
	out, err := s.CreateMbox(context.Background(), "test.ru", "box", "Pw123!", "note")
	if err != nil {
		t.Fatalf("CreateMbox: %v", err)
	}
	if gotMethod != "createMbox" {
		t.Errorf("method = %q, want createMbox", gotMethod)
	}
	if gotParams["domain"] != "test.ru" || gotParams["mbox"] != "box" || gotParams["password"] != "Pw123!" || gotParams["comment"] != "note" {
		t.Errorf("params = %v, want domain/mbox/password/comment", gotParams)
	}
	if out.Login != "box@test.ru" || out.WebMail != "https://webmail.sweb.ru" {
		t.Errorf("out = %+v, want login/webmail set", out)
	}
	if len(out.MailProgramSettings) != 1 || out.MailProgramSettings[0].Name != "IMAP" || out.MailProgramSettings[0].Port != "143" {
		t.Errorf("settings = %+v, want one IMAP:143", out.MailProgramSettings)
	}
	if out.PDF.Name != "requisites.pdf" || out.PDF.Content != "JVBERi0=" {
		t.Errorf("pdf = %+v, want requisites.pdf", out.PDF)
	}
}

func TestMailSendRequisitesToEmail(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.SendRequisitesToEmail(context.Background(), "to@gmail.com", "box@test.ru", "Pw123!"); err != nil {
		t.Fatalf("SendRequisitesToEmail: %v", err)
	}
	if gotMethod != "sendRequisitesToEmail" || gotParams["email"] != "to@gmail.com" || gotParams["login"] != "box@test.ru" {
		t.Errorf("method/params = %q/%v, want sendRequisitesToEmail to/login", gotMethod, gotParams)
	}
}

func TestMailDropMbox(t *testing.T) {
	assertSentinel(t, "dropMbox", map[string]any{"domain": "test.ru", "mbox": "box"}, func(s *Service) error {
		return s.DropMbox(context.Background(), "test.ru", "box")
	})
}

func TestMailUpdateAntispamState(t *testing.T) {
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.UpdateAntispamState(context.Background(), "test.ru", "box", 5); err != nil {
		t.Fatalf("UpdateAntispamState: %v", err)
	}
	if gotParams["value"] != float64(5) {
		t.Errorf("value = %v, want 5", gotParams["value"])
	}
}

func TestMailUpdateComment(t *testing.T) {
	assertSentinel(t, "updateComment", map[string]any{"domain": "test.ru", "mbox": "box", "comment": "hi"}, func(s *Service) error {
		return s.UpdateComment(context.Background(), "test.ru", "box", "hi")
	})
}

func TestMailChangeMailboxPassword(t *testing.T) {
	assertSentinel(t, "changeMailboxPassword", map[string]any{"domain": "test.ru", "mbox": "box", "password": "New1!"}, func(s *Service) error {
		return s.ChangeMailboxPassword(context.Background(), "test.ru", "box", "New1!")
	})
}

func TestMailDeleteMails(t *testing.T) {
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.DeleteMails(context.Background(), "test.ru", "box", 7); err != nil {
		t.Fatalf("DeleteMails: %v", err)
	}
	if gotParams["days"] != float64(7) {
		t.Errorf("days = %v, want 7", gotParams["days"])
	}
}

// --- Autoreply / SPF -------------------------------------------------------

func TestMailChangeAutoreply(t *testing.T) {
	assertSentinel(t, "changeAutoreply", map[string]any{"domain": "test.ru", "mbox": "box", "text": "away"}, func(s *Service) error {
		return s.ChangeAutoreply(context.Background(), "test.ru", "box", "away")
	})
}

func TestMailChangeMailboxSpf(t *testing.T) {
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.ChangeMailboxSpf(context.Background(), "test.ru", "box", true); err != nil {
		t.Fatalf("ChangeMailboxSpf: %v", err)
	}
	if gotParams["turnOn"] != true {
		t.Errorf("turnOn = %v, want true", gotParams["turnOn"])
	}
}

func TestMailChangeDomainSpf(t *testing.T) {
	assertSentinel(t, "changeDomainSpf", map[string]any{"domain": "test.ru", "turnOn": false}, func(s *Service) error {
		return s.ChangeDomainSpf(context.Background(), "test.ru", false)
	})
}

// --- Forwarding ------------------------------------------------------------

func TestMailAddForwardingEmail(t *testing.T) {
	assertSentinel(t, "addForwardingEmail", map[string]any{"domain": "test.ru", "mbox": "box", "email": "f@test.ru"}, func(s *Service) error {
		return s.AddForwardingEmail(context.Background(), "test.ru", "box", "f@test.ru")
	})
}

func TestMailRemoveForwardingEmail(t *testing.T) {
	assertSentinel(t, "removeForwardingEmail", map[string]any{"domain": "test.ru", "mbox": "box", "email": "f@test.ru"}, func(s *Service) error {
		return s.RemoveForwardingEmail(context.Background(), "test.ru", "box", "f@test.ru")
	})
}

func TestMailChangeDeletingAfterForwarding(t *testing.T) {
	assertSentinel(t, "changeDeletingAfterForwarding", map[string]any{"domain": "test.ru", "mbox": "box", "turnOn": true}, func(s *Service) error {
		return s.ChangeDeletingAfterForwarding(context.Background(), "test.ru", "box", true)
	})
}

// --- Delivery (mailing) lists ----------------------------------------------

func TestMailAddDeliveryAddress(t *testing.T) {
	assertSentinel(t, "addDeliveryAddress", map[string]any{"domain": "test.ru", "mbox": "box", "email": "d@test.ru"}, func(s *Service) error {
		return s.AddDeliveryAddress(context.Background(), "test.ru", "box", "d@test.ru")
	})
}

func TestMailDropDeliveryAddress(t *testing.T) {
	assertSentinel(t, "dropDeliveryAddress", map[string]any{"domain": "test.ru", "mbox": "box", "email": "d@test.ru"}, func(s *Service) error {
		return s.DropDeliveryAddress(context.Background(), "test.ru", "box", "d@test.ru")
	})
}

// --- Mail collector --------------------------------------------------------

func TestMailChangeMailsCollector(t *testing.T) {
	for _, tc := range []struct {
		body    string
		want    int64
		wantErr bool
	}{
		{`{"result":1}`, 1, false},
		{`{"result":2}`, 2, false},
		{`{"result":0}`, 0, true},
	} {
		var gotMethod string
		var gotParams map[string]any
		s := serve(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotParams = decode(r)
			_, _ = w.Write([]byte(tc.body))
		})
		got, err := s.ChangeMailsCollector(context.Background(), "test.ru", "c@gmail.com")
		if (err != nil) != tc.wantErr {
			t.Fatalf("ChangeMailsCollector(%s) err = %v, wantErr %v", tc.body, err, tc.wantErr)
		}
		if got != tc.want {
			t.Errorf("ChangeMailsCollector(%s) = %d, want %d", tc.body, got, tc.want)
		}
		if !tc.wantErr && (gotMethod != "changeMailsCollector" || gotParams["email"] != "c@gmail.com") {
			t.Errorf("method/params = %q/%v, want changeMailsCollector c@gmail.com", gotMethod, gotParams)
		}
	}
}

func TestMailRemoveMailsCollector(t *testing.T) {
	assertSentinel(t, "removeMailsCollector", map[string]any{"domain": "test.ru"}, func(s *Service) error {
		return s.RemoveMailsCollector(context.Background(), "test.ru")
	})
}

func TestMailConfirmMailsCollectorEmail(t *testing.T) {
	assertSentinel(t, "confirmMailsCollectorEmail", map[string]any{"domain": "test.ru", "token": "tok"}, func(s *Service) error {
		return s.ConfirmMailsCollectorEmail(context.Background(), "test.ru", "tok")
	})
}

// --- White / black lists ---------------------------------------------------

func TestMailAddToWhitelist(t *testing.T) {
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.AddToWhitelist(context.Background(), "test.ru", "box", "a@test.ru", true); err != nil {
		t.Fatalf("AddToWhitelist: %v", err)
	}
	if gotParams["address"] != "a@test.ru" || gotParams["all"] != true {
		t.Errorf("params = %v, want address/all", gotParams)
	}
}

func TestMailAddToBlacklist(t *testing.T) {
	// Doc-vs-reality: the address is carried under "email", not "address".
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := s.AddToBlacklist(context.Background(), "test.ru", "box", "a@test.ru", false); err != nil {
		t.Fatalf("AddToBlacklist: %v", err)
	}
	if gotParams["email"] != "a@test.ru" {
		t.Errorf("params = %v, want email key carrying the address", gotParams)
	}
	if _, hasAddr := gotParams["address"]; hasAddr {
		t.Error("blacklist add carried an 'address' key, want 'email'")
	}
}

func TestMailDropFromWhitelist(t *testing.T) {
	assertSentinel(t, "dropFromWhitelist", map[string]any{"domain": "test.ru", "mbox": "box", "address": "a@test.ru"}, func(s *Service) error {
		return s.DropFromWhitelist(context.Background(), "test.ru", "box", "a@test.ru")
	})
}

func TestMailDropFromBlacklist(t *testing.T) {
	assertSentinel(t, "dropFromBlacklist", map[string]any{"domain": "test.ru", "mbox": "box", "email": "a@test.ru"}, func(s *Service) error {
		return s.DropFromBlacklist(context.Background(), "test.ru", "box", "a@test.ru")
	})
}

// --- Domain-level toggles --------------------------------------------------

func TestMailChangeSenderVerify(t *testing.T) {
	assertSentinel(t, "changeSenderVerify", map[string]any{"domain": "test.ru", "turnOn": true}, func(s *Service) error {
		return s.ChangeSenderVerify(context.Background(), "test.ru", true)
	})
}

func TestMailChangeAutoDiscover(t *testing.T) {
	assertSentinel(t, "changeAutoDiscover", map[string]any{"domain": "test.ru", "turnOn": false}, func(s *Service) error {
		return s.ChangeAutoDiscover(context.Background(), "test.ru", false)
	})
}

func TestMailEnableDkim(t *testing.T) {
	assertSentinel(t, "enableDkim", map[string]any{"domain": "test.ru"}, func(s *Service) error {
		return s.EnableDkim(context.Background(), "test.ru")
	})
}

func TestMailDisableDkim(t *testing.T) {
	assertSentinel(t, "disableDkim", map[string]any{"domain": "test.ru"}, func(s *Service) error {
		return s.DisableDkim(context.Background(), "test.ru")
	})
}

// --- Sentinel failure path -------------------------------------------------

func TestMailSentinelFailure(t *testing.T) {
	// A 0 sentinel (non-error envelope) must surface as an error.
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := s.EnableDkim(context.Background(), "test.ru"); err == nil {
		t.Error("EnableDkim with result 0: got nil error, want failure")
	}
}

// --- Helpers ---------------------------------------------------------------

// decode reads the JSON-RPC method and params from a request body. Params are
// decoded into map[string]any (numbers land as float64, matching encoding/json).
func decode(r *http.Request) (method string, params map[string]any) {
	var req struct {
		Method string         `json:"method"`
		Params map[string]any `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	return req.Method, req.Params
}

// assertSentinel exercises a mutating method that answers the integer-1 sentinel:
// it checks the wire method name and every expected param, and asserts success.
func assertSentinel(t *testing.T, wantMethod string, wantParams map[string]any, call func(*Service) error) {
	t.Helper()
	var gotMethod string
	var gotParams map[string]any
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotParams = decode(r)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := call(s); err != nil {
		t.Fatalf("%s: %v", wantMethod, err)
	}
	if gotMethod != wantMethod {
		t.Errorf("method = %q, want %q", gotMethod, wantMethod)
	}
	for k, want := range wantParams {
		got := gotParams[k]
		if b, ok := want.(bool); ok {
			if got != b {
				t.Errorf("param %q = %v, want %v", k, got, b)
			}
			continue
		}
		if got != want {
			t.Errorf("param %q = %v, want %v", k, got, want)
		}
	}
}

// serve spins up a mock JSON-RPC server for h and returns a mail.Service backed
// by a transport pointed at it.
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
