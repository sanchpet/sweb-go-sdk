// Package checks groups the monitoring-check operations (endpoint
// /monitoring/checks): list and inspect checks, read the reference dictionaries
// (types, intervals, ports, keyword modes), create/edit checks, toggle them on
// and off individually or in bulk, remove them, and read check history.
package checks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const checksEndpoint = "/monitoring/checks"

// Service groups the monitoring-check operations (endpoint /monitoring/checks):
// list and inspect checks, read the reference dictionaries (types, intervals,
// ports, keyword modes), create/edit checks, toggle them on and off individually
// or in bulk, remove them, and read check history.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Check is one monitoring check as returned by the "index" list method. IDs and
// the type discriminator arrive as quoted strings on this endpoint, so numeric
// fields decode through flex.Int.
type Check struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`     // "1" Ping, "2" Http, "3" Port (arrives quoted)
	Status   bool   `json:"status"`   // true = active, false = disabled
	Disabled bool   `json:"disabled"` // true = blocked
	// LastResult is the last check outcome: null (never run yet), true (OK), or
	// false (failed). Kept raw because the tri-state includes null.
	LastResult    json.RawMessage `json:"lastResult"`
	TSLastResult  json.RawMessage `json:"tsLastResult"`  // string|null: time since last result
	TSDeltaResult json.RawMessage `json:"tsDeltaResult"` // shape undocumented beyond "array", left raw
}

// FilterInfo is the pagination envelope shared by the list-style results (index,
// history): the current page/size and the total row count. Fields arrive as bare
// numbers here but decode through flex.Int for safety.
type FilterInfo struct {
	Page       flex.Int `json:"page"`
	PerPage    flex.Int `json:"perPage"`
	TotalCount flex.Int `json:"totalCount"`
	OrderField string   `json:"orderField"` // only present on the contacts index
	OrderDir   string   `json:"orderDirect"`
}

// CheckList is the paginated result of the "index" method.
type CheckList struct {
	List       []Check    `json:"list"`
	FilterInfo FilterInfo `json:"filterInfo"`
}

// ListOptions carries optional pagination for the list methods.
type ListOptions struct {
	Page    int
	PerPage int
}

func (o *ListOptions) params() map[string]any {
	p := map[string]any{}
	if o == nil {
		return p
	}
	if o.Page > 0 {
		p["page"] = o.Page
	}
	if o.PerPage > 0 {
		p["perPage"] = o.PerPage
	}
	return p
}

// Index lists the account's monitoring checks (method "index"). Read-only.
func (s *Service) Index(ctx context.Context, opts *ListOptions) (*CheckList, error) {
	var out CheckList
	if err := s.t.Call(ctx, checksEndpoint, "index", opts.params(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckType is an available check type (method "getTypes"): Ping, HTTP, Port.
type CheckType struct {
	ID   string `json:"id"`
	Code string `json:"code"` // "ping", "http", "port"
	Name string `json:"name"`
}

// GetTypes lists the available check types (method "getTypes"). Read-only.
func (s *Service) GetTypes(ctx context.Context) ([]CheckType, error) {
	var out []CheckType
	if err := s.t.Call(ctx, checksEndpoint, "getTypes", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Interval is an available check interval (method "getIntervals").
type Interval struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Time string `json:"time"` // interval length in minutes (arrives as a string)
}

// GetIntervals lists the available check intervals (method "getIntervals").
// Read-only.
func (s *Service) GetIntervals(ctx context.Context) ([]Interval, error) {
	var out []Interval
	if err := s.t.Call(ctx, checksEndpoint, "getIntervals", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Port is a recommended port for a Port-type check (method "getPorts").
type Port struct {
	Name     string `json:"name"`     // short name, e.g. "HTTPS"
	NameFull string `json:"nameFull"` // full name
	Value    string `json:"value"`    // port number (arrives as a string)
}

// GetPorts lists the recommended ports for Port checks (method "getPorts").
// Read-only.
func (s *Service) GetPorts(ctx context.Context) ([]Port, error) {
	var out []Port
	if err := s.t.Call(ctx, checksEndpoint, "getPorts", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// KeywordMode is an available keyword-check mode (method "getKeywordModes").
type KeywordMode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetKeywordModes lists the available keyword-check modes (method
// "getKeywordModes"). Read-only.
func (s *Service) GetKeywordModes(ctx context.Context) ([]KeywordMode, error) {
	var out []KeywordMode
	if err := s.t.Call(ctx, checksEndpoint, "getKeywordModes", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SettingsInfo is the combined settings reference plus subscription usage
// returned by "getInfo": the reference dictionaries and the tariff counters.
type SettingsInfo struct {
	Types        []CheckType   `json:"types"`
	Intervals    []Interval    `json:"intervals"`
	KeywordModes []KeywordMode `json:"keywordModes"`
	Ports        []Port        `json:"ports"`

	AvailableSMS    flex.Int `json:"availableSms"`
	CurrentSMS      flex.Int `json:"currentSms"`
	TotalSMS        flex.Int `json:"totalSms"`
	AvailableChecks flex.Int `json:"availableChecks"`
	CurrentChecks   flex.Int `json:"currentChecks"`
	TotalChecks     flex.Int `json:"totalChecks"`
	Active          bool     `json:"active"`
	// Expired is the service end date (string) or null.
	Expired json.RawMessage `json:"expired"`
	// Tariff is the tariff detail (array|null); shape undocumented, left raw.
	Tariff json.RawMessage `json:"tariff"`
}

// GetInfo returns the combined settings reference and subscription usage for the
// checks UI (method "getInfo"). Read-only.
func (s *Service) GetInfo(ctx context.Context) (*SettingsInfo, error) {
	var out SettingsInfo
	if err := s.t.Call(ctx, checksEndpoint, "getInfo", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Setting is one entry of a check's settings array (getFullCheckInfo): a
// typed key/value such as target, interval, keyword, keyword_mode, port, ssl.
type Setting struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Contact is a contact attached to a check (getFullCheckInfo); see the
// /monitoring/contacts index for the full contact shape.
type Contact struct {
	ID       flex.Int `json:"id"`
	Type     string   `json:"type"` // "email", "phone", "telegram"
	Name     string   `json:"name"`
	Value    string   `json:"value"`
	Verified bool     `json:"verified"`
}

// FullInfo is the detailed view of a single check (method
// "getFullCheckInfo"): its settings and attached contacts.
type FullInfo struct {
	ID     flex.Int `json:"id"`
	Type   flex.Int `json:"type"` // type id (bare number here, unlike the quoted index)
	Name   string   `json:"name"`
	Status bool     `json:"status"`
	// LastResult is the last check outcome (null|true|false), kept raw.
	LastResult json.RawMessage `json:"lastResult"`
	Settings   []Setting       `json:"settings"`
	Contacts   []Contact       `json:"contacts"`
}

// GetFullCheckInfo returns the detailed configuration of one check (method
// "getFullCheckInfo"). Read-only.
func (s *Service) GetFullCheckInfo(ctx context.Context, id int) (*FullInfo, error) {
	var out FullInfo
	if err := s.t.Call(ctx, checksEndpoint, "getFullCheckInfo", map[string]any{"id": id}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Spec describes a check to create or edit. Port, SSL, Keywords, and
// KeywordMode are optional and only meaningful for the matching type (Port/Http).
type Spec struct {
	Type        int      // check type id (1 Ping, 2 Http, 3 Port)
	Target      string   // URL or IP to check
	Name        string   // display name
	Interval    int      // interval id (see GetIntervals)
	ContactIDs  []int    // contacts to notify
	Port        int      // Port checks only (0 = omit)
	SSL         bool     // Http checks: use SSL
	Keywords    []string // Http checks: keywords to match (nil = omit)
	KeywordMode int      // Http checks: keyword mode id (0 = omit)
}

func (spec Spec) params() map[string]any {
	p := map[string]any{
		"type":       spec.Type,
		"target":     spec.Target,
		"name":       spec.Name,
		"interval":   spec.Interval,
		"contactIds": spec.ContactIDs,
	}
	if spec.Port > 0 {
		p["port"] = spec.Port
	}
	if spec.SSL {
		p["ssl"] = true
	}
	if spec.Keywords != nil {
		p["keywords"] = spec.Keywords
	}
	if spec.KeywordMode > 0 {
		p["keywordMode"] = spec.KeywordMode
	}
	return p
}

// Create creates a monitoring check (method "create"). This BILLS against the
// check quota. Integer 1/0 success sentinel per the spec's resultCreate
// descriptor (the result shape is doc-only, not verified against a live
// response). Read the new check back via Index/GetFullCheckInfo.
func (s *Service) Create(ctx context.Context, spec Spec) error {
	return s.actionOne(ctx, "create", spec.params())
}

// Edit updates an existing check (method "edit"). The spec reuses the create
// parameters plus the check id. Integer 1/0 success sentinel.
func (s *Service) Edit(ctx context.Context, id int, spec Spec) error {
	p := spec.params()
	delete(p, "type") // edit is keyed by id, not type
	p["id"] = id
	return s.actionOne(ctx, "edit", p)
}

// Activate enables one check (method "activate"). Integer 1 success sentinel.
func (s *Service) Activate(ctx context.Context, id int) error {
	return s.actionOne(ctx, "activate", map[string]any{"id": id})
}

// ActivateList enables several checks (method "activateList"). Integer 1
// success sentinel.
func (s *Service) ActivateList(ctx context.Context, ids ...int) error {
	return s.actionOne(ctx, "activateList", map[string]any{"ids": ids})
}

// Deactivate disables one check (method "deactivate"). Integer 1 success
// sentinel.
func (s *Service) Deactivate(ctx context.Context, id int) error {
	return s.actionOne(ctx, "deactivate", map[string]any{"id": id})
}

// DeactivateList disables several checks (method "deactivateList"). Integer 1
// success sentinel.
func (s *Service) DeactivateList(ctx context.Context, ids ...int) error {
	return s.actionOne(ctx, "deactivateList", map[string]any{"ids": ids})
}

// Remove deletes one check (method "remove"). Integer 1 success sentinel.
func (s *Service) Remove(ctx context.Context, id int) error {
	return s.actionOne(ctx, "remove", map[string]any{"id": id})
}

// RemoveList deletes several checks (method "removeList"). Integer 1 success
// sentinel.
func (s *Service) RemoveList(ctx context.Context, ids ...int) error {
	return s.actionOne(ctx, "removeList", map[string]any{"ids": ids})
}

// HistoryOptions carries the optional date window and pagination for History.
type HistoryOptions struct {
	StartDate  string // inclusive lower bound (spec is opaque about the format)
	FinishDate string
	Page       int
	PerPage    int
}

// HistoryEvent is one row of a check's history (method "history").
type HistoryEvent struct {
	ID      string `json:"id"`
	CheckID string `json:"check_id"`
	TS      string `json:"ts"`      // event timestamp
	Success string `json:"success"` // "y" ok, "n" failed
}

// CheckHistory is the paginated result of the "history" method.
type CheckHistory struct {
	List       []HistoryEvent `json:"list"`
	FilterInfo FilterInfo     `json:"filterInfo"`
}

// History returns the event history for one check (method "history"). Read-only.
func (s *Service) History(ctx context.Context, id int, opts *HistoryOptions) (*CheckHistory, error) {
	p := map[string]any{"id": id}
	if opts != nil {
		if opts.StartDate != "" {
			p["startDate"] = opts.StartDate
		}
		if opts.FinishDate != "" {
			p["finishDate"] = opts.FinishDate
		}
		if opts.Page > 0 {
			p["page"] = opts.Page
		}
		if opts.PerPage > 0 {
			p["perPage"] = opts.PerPage
		}
	}
	var out CheckHistory
	if err := s.t.Call(ctx, checksEndpoint, "history", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// actionOne runs a checks mutation whose success sentinel is integer 1 (0 =
// failure), shared by create/edit and the activate/deactivate/remove family.
func (s *Service) actionOne(ctx context.Context, method string, params any) error {
	var out flex.Int
	if err := s.t.Call(ctx, checksEndpoint, method, params, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
