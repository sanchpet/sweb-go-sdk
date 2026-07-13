// Package load groups shared-hosting server-load statistics (endpoint /vh/load):
// listing the periods for which load data exists and fetching a period's load
// table (per-day CPU/MySQL usage plus the plan's level thresholds and a CSV
// export). Both methods are read-only. All calls dispatch through the shared
// transport.
package load

import (
	"context"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const loadEndpoint = "/vh/load"

// Service groups shared-hosting server-load statistics (endpoint /vh/load):
// the available periods (Periods) and a period's load table (LoadTable).
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Period is one month for which load statistics exist, as returned by Periods
// (method "index"). Both fields arrive as quoted strings ("11", "2022") in the
// recorded response, so they are kept as strings rather than parsed to numbers.
type Period struct {
	Month string `json:"month"` // "1".."12"
	Year  string `json:"year"`  // e.g. "2023"
}

// DayLoad is one day's server-load sample inside a Table (the "list" entries of
// method "getLoadTable"): the CPU load and MySQL load recorded for Date.
//
// The API quotes cpu as a string ("0.00") but returns mysql as a bare int (0),
// so cpu decodes through flex.Float and mysql through flex.Int.
type DayLoad struct {
	Date  string     `json:"date"`  // "2023-06-01"
	CPU   flex.Float `json:"cpu"`   // quoted "0.00"
	Mysql flex.Int   `json:"mysql"` // bare 0
}

// CSV is the base64-encoded CSV export bundled with a Table (the "csv" field of
// method "getLoadTable").
//
// Doc-vs-reality gap: the spec's content descriptor types csv as string[], but
// the recorded example returns an OBJECT with the fields below. Metadata is an
// empty array in the recorded response and its element shape has not been
// observed live, so it is left as json.RawMessage-free []any to tolerate either
// (it is unused by callers).
type CSV struct {
	Content  string `json:"content"`  // base64 of the CSV body (mimetype "…;base64")
	Metadata []any  `json:"metadata"` // empty in the recorded response
	Mimetype string `json:"mimetype"` // e.g. "application/csv;base64"
	Name     string `json:"name"`     // e.g. "loading_lina199302_6.csv"
}

// Table is a period's load table as returned by LoadTable (method
// "getLoadTable"): the per-day samples (List), the plan's level thresholds, and
// the CSV export.
//
// HostingLevels and DBLevels are the plan's escalation thresholds (int[] per the
// spec) and decode through flex.Int for the API's usual numeric polymorphism.
type Table struct {
	List          []DayLoad  `json:"list"`
	HostingLevels []flex.Int `json:"hostingLevels"`
	DBLevels      []flex.Int `json:"dbLevels"`
	CSV           CSV        `json:"csv"`
}

// Periods returns the months for which load statistics exist (method "index").
// Read-only. No parameters.
func (s *Service) Periods(ctx context.Context) ([]Period, error) {
	var out []Period
	err := s.t.Call(ctx, loadEndpoint, "index", nil, &out)
	return out, err
}

// LoadTable returns the load table for a period (method "getLoadTable").
// Read-only.
//
// year and month select the period (0 for either omits it, letting the API pick
// its default). loadType filters by kind ("cpu" or "mysql"); "" omits it (the
// spec's example passes the string "null"), returning all kinds.
//
// Doc-vs-reality gap: the spec types the result as a bare array, but the live
// API returns a bare Table object. LoadTable decodes it directly.
func (s *Service) LoadTable(ctx context.Context, year, month int, loadType string) (Table, error) {
	params := map[string]any{}
	if year != 0 {
		params["year"] = year
	}
	if month != 0 {
		params["month"] = month
	}
	if loadType != "" {
		params["type"] = loadType
	}

	var out Table
	if err := s.t.Call(ctx, loadEndpoint, "getLoadTable", params, &out); err != nil {
		return Table{}, err
	}
	return out, nil
}
