package sweb

import (
	"context"
	"encoding/json"
	"fmt"
)

const monitoringChecksEndpoint = "/monitoring/checks"

// MonitoringChecksService groups the monitoring-check operations (endpoint
// /monitoring/checks): list and inspect checks, read the reference dictionaries
// (types, intervals, ports, keyword modes), create/edit checks, toggle them on
// and off individually or in bulk, remove them, and read check history.
type MonitoringChecksService struct{ c *Client }

// Check is one monitoring check as returned by the "index" list method. IDs and
// the type discriminator arrive as quoted strings on this endpoint, so numeric
// fields decode through FlexInt.
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
// numbers here but decode through FlexInt for safety.
type FilterInfo struct {
	Page       FlexInt `json:"page"`
	PerPage    FlexInt `json:"perPage"`
	TotalCount FlexInt `json:"totalCount"`
	OrderField string  `json:"orderField"` // only present on the contacts index
	OrderDir   string  `json:"orderDirect"`
}

// CheckList is the paginated result of the "index" method.
type CheckList struct {
	List       []Check    `json:"list"`
	FilterInfo FilterInfo `json:"filterInfo"`
}

// CheckListOptions carries optional pagination for the list methods.
type CheckListOptions struct {
	Page    int
	PerPage int
}

func (o *CheckListOptions) params() map[string]any {
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
func (s *MonitoringChecksService) Index(ctx context.Context, opts *CheckListOptions) (*CheckList, error) {
	var out CheckList
	if err := s.c.call(ctx, monitoringChecksEndpoint, "index", opts.params(), &out); err != nil {
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
func (s *MonitoringChecksService) GetTypes(ctx context.Context) ([]CheckType, error) {
	var out []CheckType
	if err := s.c.call(ctx, monitoringChecksEndpoint, "getTypes", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CheckInterval is an available check interval (method "getIntervals").
type CheckInterval struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Time string `json:"time"` // interval length in minutes (arrives as a string)
}

// GetIntervals lists the available check intervals (method "getIntervals").
// Read-only.
func (s *MonitoringChecksService) GetIntervals(ctx context.Context) ([]CheckInterval, error) {
	var out []CheckInterval
	if err := s.c.call(ctx, monitoringChecksEndpoint, "getIntervals", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CheckPort is a recommended port for a Port-type check (method "getPorts").
type CheckPort struct {
	Name     string `json:"name"`     // short name, e.g. "HTTPS"
	NameFull string `json:"nameFull"` // full name
	Value    string `json:"value"`    // port number (arrives as a string)
}

// GetPorts lists the recommended ports for Port checks (method "getPorts").
// Read-only.
func (s *MonitoringChecksService) GetPorts(ctx context.Context) ([]CheckPort, error) {
	var out []CheckPort
	if err := s.c.call(ctx, monitoringChecksEndpoint, "getPorts", nil, &out); err != nil {
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
func (s *MonitoringChecksService) GetKeywordModes(ctx context.Context) ([]KeywordMode, error) {
	var out []KeywordMode
	if err := s.c.call(ctx, monitoringChecksEndpoint, "getKeywordModes", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CheckSettingsInfo is the combined settings reference plus subscription usage
// returned by "getInfo": the reference dictionaries and the tariff counters.
type CheckSettingsInfo struct {
	Types        []CheckType     `json:"types"`
	Intervals    []CheckInterval `json:"intervals"`
	KeywordModes []KeywordMode   `json:"keywordModes"`
	Ports        []CheckPort     `json:"ports"`

	AvailableSMS    FlexInt `json:"availableSms"`
	CurrentSMS      FlexInt `json:"currentSms"`
	TotalSMS        FlexInt `json:"totalSms"`
	AvailableChecks FlexInt `json:"availableChecks"`
	CurrentChecks   FlexInt `json:"currentChecks"`
	TotalChecks     FlexInt `json:"totalChecks"`
	Active          bool    `json:"active"`
	// Expired is the service end date (string) or null.
	Expired json.RawMessage `json:"expired"`
	// Tariff is the tariff detail (array|null); shape undocumented, left raw.
	Tariff json.RawMessage `json:"tariff"`
}

// GetInfo returns the combined settings reference and subscription usage for the
// checks UI (method "getInfo"). Read-only.
func (s *MonitoringChecksService) GetInfo(ctx context.Context) (*CheckSettingsInfo, error) {
	var out CheckSettingsInfo
	if err := s.c.call(ctx, monitoringChecksEndpoint, "getInfo", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckSetting is one entry of a check's settings array (getFullCheckInfo): a
// typed key/value such as target, interval, keyword, keyword_mode, port, ssl.
type CheckSetting struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// CheckContact is a contact attached to a check (getFullCheckInfo); see the
// /monitoring/contacts index for the full contact shape.
type CheckContact struct {
	ID       FlexInt `json:"id"`
	Type     string  `json:"type"` // "email", "phone", "telegram"
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Verified bool    `json:"verified"`
}

// FullCheckInfo is the detailed view of a single check (method
// "getFullCheckInfo"): its settings and attached contacts.
type FullCheckInfo struct {
	ID     FlexInt `json:"id"`
	Type   FlexInt `json:"type"` // type id (bare number here, unlike the quoted index)
	Name   string  `json:"name"`
	Status bool    `json:"status"`
	// LastResult is the last check outcome (null|true|false), kept raw.
	LastResult json.RawMessage `json:"lastResult"`
	Settings   []CheckSetting  `json:"settings"`
	Contacts   []CheckContact  `json:"contacts"`
}

// GetFullCheckInfo returns the detailed configuration of one check (method
// "getFullCheckInfo"). Read-only.
func (s *MonitoringChecksService) GetFullCheckInfo(ctx context.Context, id int) (*FullCheckInfo, error) {
	var out FullCheckInfo
	if err := s.c.call(ctx, monitoringChecksEndpoint, "getFullCheckInfo", map[string]any{"id": id}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckSpec describes a check to create or edit. Port, SSL, Keywords, and
// KeywordMode are optional and only meaningful for the matching type (Port/Http).
type CheckSpec struct {
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

func (spec CheckSpec) params() map[string]any {
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
func (s *MonitoringChecksService) Create(ctx context.Context, spec CheckSpec) error {
	return s.actionOne(ctx, "create", spec.params())
}

// Edit updates an existing check (method "edit"). The spec reuses the create
// parameters plus the check id. Integer 1/0 success sentinel.
func (s *MonitoringChecksService) Edit(ctx context.Context, id int, spec CheckSpec) error {
	p := spec.params()
	delete(p, "type") // edit is keyed by id, not type
	p["id"] = id
	return s.actionOne(ctx, "edit", p)
}

// Activate enables one check (method "activate"). Integer 1 success sentinel.
func (s *MonitoringChecksService) Activate(ctx context.Context, id int) error {
	return s.actionOne(ctx, "activate", map[string]any{"id": id})
}

// ActivateList enables several checks (method "activateList"). Integer 1
// success sentinel.
func (s *MonitoringChecksService) ActivateList(ctx context.Context, ids ...int) error {
	return s.actionOne(ctx, "activateList", map[string]any{"ids": ids})
}

// Deactivate disables one check (method "deactivate"). Integer 1 success
// sentinel.
func (s *MonitoringChecksService) Deactivate(ctx context.Context, id int) error {
	return s.actionOne(ctx, "deactivate", map[string]any{"id": id})
}

// DeactivateList disables several checks (method "deactivateList"). Integer 1
// success sentinel.
func (s *MonitoringChecksService) DeactivateList(ctx context.Context, ids ...int) error {
	return s.actionOne(ctx, "deactivateList", map[string]any{"ids": ids})
}

// Remove deletes one check (method "remove"). Integer 1 success sentinel.
func (s *MonitoringChecksService) Remove(ctx context.Context, id int) error {
	return s.actionOne(ctx, "remove", map[string]any{"id": id})
}

// RemoveList deletes several checks (method "removeList"). Integer 1 success
// sentinel.
func (s *MonitoringChecksService) RemoveList(ctx context.Context, ids ...int) error {
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
func (s *MonitoringChecksService) History(ctx context.Context, id int, opts *HistoryOptions) (*CheckHistory, error) {
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
	if err := s.c.call(ctx, monitoringChecksEndpoint, "history", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// actionOne runs a checks mutation whose success sentinel is integer 1 (0 =
// failure), shared by create/edit and the activate/deactivate/remove family.
func (s *MonitoringChecksService) actionOne(ctx context.Context, method string, params any) error {
	var out FlexInt
	if err := s.c.call(ctx, monitoringChecksEndpoint, method, params, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
