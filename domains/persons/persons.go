// Package persons groups domain-registrant-person operations (endpoint
// /domains/persons): list the account's registrant contacts (List), read one in
// full (Info), and create an individual/sole-proprietor (CreateFizIP) or a legal
// entity (CreateJur). All calls dispatch through the shared transport.
//
// A "domain person" is the registrant contact attached to a domain (WHOIS
// contact): a natural person / individual entrepreneur (type "f"/"ip") or an
// organization (type "u"). It is a sub-package of domains, distinct from the
// domains package itself.
package persons

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const personsEndpoint = "/domains/persons"

// Service groups domain-registrant-person operations (endpoint /domains/persons):
// list the account's registrant contacts (List), read one in full (Info), and
// create an individual/sole-proprietor (CreateFizIP) or a legal entity (CreateJur).
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Person types as reported in the "type" field of List/Info.
const (
	TypeIndividual   = "f"  // natural person (физ. лицо)
	TypeEntrepreneur = "ip" // sole proprietor (ИП)
	TypeLegal        = "u"  // legal entity (юр. лицо)
)

// Person is one registrant contact from the account's list ("index").
//
// Types are reconciled against the spec's recorded example: resident/used/valid
// arrive as bare integers (0/1) and decode through flex.Int for tolerance to the
// API's polymorphic numerics; SwebHandle/Str are the string identifiers.
type Person struct {
	ID         flex.Int `json:"id"`          // numeric person id
	Name       string   `json:"name"`        // display name / organization name
	SwebHandle string   `json:"sweb_handle"` // string identifier, e.g. "SWEB-FIZ-III-2168"
	Str        string   `json:"str"`         // identifier as a display string
	Type       string   `json:"type"`        // TypeIndividual | TypeEntrepreneur | TypeLegal
	Resident   flex.Int `json:"resident"`    // 1 if a resident of the RF
	Used       flex.Int `json:"used"`        // 1 if attached to a domain
	Valid      flex.Int `json:"valid"`       // 1 if the contact's details are valid
}

// personsIndex is the keyed wrapper the "index" method returns: the contacts
// live under "persons" (with a sibling "props_filled") in a bare object,
// despite the spec typing the result as a bare array.
type personsIndex struct {
	PropsFilled flex.Int `json:"props_filled"` // 1 if the account's requisites are filled
	Persons     []Person `json:"persons"`
}

// List returns the account's registrant contacts ("index"). Read-only. Also
// reports props_filled (whether the account's own requisites are complete).
//
// The API returns a bare object {"props_filled":…, "persons":[…]}; this decodes
// it directly. propsFilled is true when props_filled == 1.
func (s *Service) List(ctx context.Context) (persons []Person, propsFilled bool, err error) {
	var out personsIndex
	if err := s.t.Call(ctx, personsEndpoint, "index", nil, &out); err != nil {
		return nil, false, err
	}
	return out.Persons, out.PropsFilled == 1, nil
}

// Info is the full record for one registrant contact ("getinfo"). One struct
// carries both an individual (type "f"/"ip") and a legal entity (type "u"); the
// entity-only fields (Faxes, Jur*, KPP, PersName*) are populated only for "u",
// and the individual-only fields (Birthdate, Pass*) only for "f"/"ip".
//
// DOC-VS-REALITY: the apidoc's field list types phones/emails as string, but the
// recorded example returns them as arrays; StringOrList tolerates both. inn/kpp
// are string-or-null (kept as string, "" when null). resident is a bool in the
// example (the List "resident" is instead 0/1). used arrives as a bare int.
type Info struct {
	Name      string       `json:"name"`      // display name / organization name
	NameTrans string       `json:"nameTrans"` // Latin transliteration (chiefly "u")
	Resident  bool         `json:"resident"`  // resident of the RF
	Phones    StringOrList `json:"phones"`    // doc: string; live: array
	Emails    StringOrList `json:"emails"`    // doc: string; live: array
	INN       string       `json:"inn"`       // string|null → "" when null
	Type      string       `json:"type"`      // TypeIndividual | TypeEntrepreneur | TypeLegal
	Used      flex.Int     `json:"used"`      // 1 if attached to a domain

	// Postal address (both individual and legal entity).
	PostIndex   string `json:"postIndex"`
	PostCity    string `json:"postCity"`
	PostAddress string `json:"postAddress"`

	// Individual / sole proprietor only ("f"/"ip").
	Birthdate  string `json:"birthdate"`
	PassSeries string `json:"passSeries"`
	PassNum    string `json:"passNum"`
	PassDate   string `json:"passDate"`
	PassOrg    string `json:"passOrg"`

	// Legal entity only ("u").
	Faxes         StringOrList `json:"faxes"`      // doc: string; treated like phones/emails
	JurIndex      string       `json:"jurIndex"`   // legal address
	JurCity       string       `json:"jurCity"`    // legal address
	JurAddress    string       `json:"jurAddress"` // legal address
	KPP           string       `json:"kpp"`        // string|null → "" when null
	PersName      string       `json:"persName"`   // contact representative
	PersNameTrans string       `json:"persNameTrans"`
}

// StringOrList tolerates a field the apidoc types as a scalar string but the live
// API returns as an array of strings (phones/emails/faxes). A bare string decodes
// to a single-element slice; a null decodes to nil.
type StringOrList []string

// UnmarshalJSON accepts a JSON string, an array of strings, or null.
func (s *StringOrList) UnmarshalJSON(b []byte) error {
	var arr []string
	if err := json.Unmarshal(b, &arr); err == nil {
		*s = arr
		return nil
	}
	var one string
	if err := json.Unmarshal(b, &one); err != nil {
		return err
	}
	if one == "" {
		*s = nil
	} else {
		*s = []string{one}
	}
	return nil
}

// Info returns the full record for one registrant contact ("getinfo"). Read-only.
//
// The API wraps the record in a single-element array [{…}]; this unwraps it and
// returns nil for an empty result.
func (s *Service) Info(ctx context.Context, id int) (*Info, error) {
	var out []Info
	if err := s.t.Call(ctx, personsEndpoint, "getinfo", map[string]any{"id": id}, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
}

// FizIPOptions parameterizes CreateFizIP (an individual or sole proprietor).
// Name, Resident, Phones, Emails, PostIndex, PostCity, PostAddress and Birthdate
// are required by the API; the passport fields, INN and ID are optional. A
// non-empty ID edits the existing person rather than creating a new one.
type FizIPOptions struct {
	Name        string // full name / "ИП …"
	Resident    bool   // resident of the RF
	Phones      string
	Emails      string
	PostIndex   string
	PostCity    string
	PostAddress string
	Birthdate   string // "YYYY-MM-DD"
	PassSeries  string // optional
	PassNum     string // optional
	PassDate    string // optional, "YYYY-MM-DD"
	PassOrg     string // optional
	INN         string // optional
	ID          string // optional: id of the person to edit
}

// CreateFizIP creates (or, with ID set, edits) an individual / sole-proprietor
// registrant contact ("createFizIp"). MUTATING — never exercise against the live
// API in tests. Returns on the 1/0 sentinel (1 = success).
func (s *Service) CreateFizIP(ctx context.Context, o FizIPOptions) error {
	params := map[string]any{
		"name":        o.Name,
		"resident":    o.Resident,
		"phones":      o.Phones,
		"emails":      o.Emails,
		"postIndex":   o.PostIndex,
		"postCity":    o.PostCity,
		"postAddress": o.PostAddress,
		"birthdate":   o.Birthdate,
	}
	putIfSet(params, "passSeries", o.PassSeries)
	putIfSet(params, "passNum", o.PassNum)
	putIfSet(params, "passDate", o.PassDate)
	putIfSet(params, "passOrg", o.PassOrg)
	putIfSet(params, "inn", o.INN)
	putIfSet(params, "id", o.ID)
	return s.sentinelAction(ctx, "createFizIp", params)
}

// JurOptions parameterizes CreateJur (a legal entity). All fields are required by
// the API. Phones1 is the notification phone, Phones2 a secondary phone.
type JurOptions struct {
	Name        string // organization name
	NameTrans   string // Latin transliteration
	Resident    bool   // resident of the RF
	Phones1     string // notification phone
	Phones2     string // phone
	Faxes       string
	Emails      string
	PostIndex   string // postal address
	PostCity    string // postal address
	PostAddress string // postal address
	JurIndex    string // legal address
	JurCity     string // legal address
	JurAddress  string // legal address
	INN         string
	KPP         string
	PersName    string // contact representative
}

// CreateJur creates a legal-entity registrant contact ("createJur"). MUTATING —
// never exercise against the live API in tests. Returns on the 1/0 sentinel
// (1 = success).
func (s *Service) CreateJur(ctx context.Context, o JurOptions) error {
	return s.sentinelAction(ctx, "createJur", map[string]any{
		"name":        o.Name,
		"nameTrans":   o.NameTrans,
		"resident":    o.Resident,
		"phones1":     o.Phones1,
		"phones2":     o.Phones2,
		"faxes":       o.Faxes,
		"emails":      o.Emails,
		"postIndex":   o.PostIndex,
		"postCity":    o.PostCity,
		"postAddress": o.PostAddress,
		"jurIndex":    o.JurIndex,
		"jurCity":     o.JurCity,
		"jurAddress":  o.JurAddress,
		"inn":         o.INN,
		"kpp":         o.KPP,
		"persName":    o.PersName,
	})
}

// putIfSet adds key=val to params only when val is non-empty, so optional string
// fields are omitted from the request rather than sent blank.
func putIfSet(params map[string]any, key, val string) {
	if val != "" {
		params[key] = val
	}
}

// sentinelAction runs a /domains/persons method whose success is the integer
// sentinel 1 (createFizIp/createJur both answer 1 on success, 0 on failure per
// the spec's resultInt). A real failure usually surfaces as a JSON-RPC error via
// Call; the non-1 check is defensive. The result is decoded via json.RawMessage
// first so that a shape not yet observed live (should the API ever answer richer
// than a bare 1) does not silently pass — only a plain 1 is accepted as success.
func (s *Service) sentinelAction(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, personsEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: persons %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: persons %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
