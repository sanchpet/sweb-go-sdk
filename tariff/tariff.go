// Package tariff groups the hosting-account tariff and server-info reads
// (endpoint /tariff): index (current plan and real resource usage) and
// serverInfo (the node the account lives on). Both are read-only; all calls
// dispatch through the shared transport.
package tariff

import (
	"context"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const tariffEndpoint = "/tariff"

// Service groups the tariff reads (endpoint /tariff): index and serverInfo.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Tariff is the current-tariff record returned by Index (method "index"): the
// billed plan (Info) alongside the resources really in use (Usage). The API
// wraps a single such record in a one-element array; Index unwraps it.
type Tariff struct {
	Info  Info  `json:"info"`
	Usage Usage `json:"real"`
}

// Info is the billed plan ("info"): plan identity, included quotas and the
// price ladder. Numeric fields arrive polymorphic (bare or quoted) so they
// decode through flex.Int.
type Info struct {
	PlanID     flex.Int `json:"plan_id"`
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Quota      flex.Int `json:"quota"`      // included disk quota
	MailQuota  flex.Int `json:"mail_quota"` // included mail quota
	MySQL      flex.Int `json:"mysql"`      // number of MySQL databases
	Site       flex.Int `json:"site"`       // number of sites
	PostgreSQL flex.Int `json:"postgresql"` // number of PostgreSQL databases
	Price      flex.Int `json:"price"`      // monthly price
	Price6     flex.Int `json:"price_6"`    // half-year price
	Price12    flex.Int `json:"price_12"`   // yearly price
	Duration   flex.Int `json:"duration"`   // auto-renewal period (months)
}

// Usage is the resources really in use ("real"). Two fields drift from the
// spec: the doc types Quota/MailQuota as float, but the API returns them as
// locale comma-decimal strings ("0,00") that no numeric parser accepts, so they
// stay string (unparsed), mirroring vh/backup's treatment of "309,89 KB". The
// remaining counters arrive quoted ("0") and decode through flex.Int.
type Usage struct {
	Quota                string   `json:"quota"`      // doc "float"; real "0,00" (locale comma) — kept as string
	MailQuota            string   `json:"mail_quota"` // doc "float"; real "0,00" (locale comma) — kept as string
	MySQL                flex.Int `json:"mysql"`
	Site                 flex.Int `json:"site"`
	PostgreSQL           flex.Int `json:"postgresql"`
	Firebird             flex.Int `json:"firebird"`
	Mailbox              flex.Int `json:"mailbox"`
	RealPrice            flex.Int `json:"realPrice"`
	RealDuration         flex.Int `json:"realDuration"`
	ProlongPrice         flex.Int `json:"prolongPrice"`
	ProlongDuration      flex.Int `json:"prolongDuration"`
	NoHosting            flex.Int `json:"noHosting"` // 1 when not a hosting plan
	ProlongChangeDisable bool     `json:"prolongChangeDisable"`
}

// ServerInfo describes the node the account is hosted on (method "serverInfo").
// Software versions are free-form strings (empty when the stack is absent, e.g.
// Python/Ruby). Backend drifts from the spec: it is documented as a string but
// the API returns an array of available Apache+PHP backends.
type ServerInfo struct {
	Name    string    `json:"name"`
	IP      string    `json:"ip"`
	OS      string    `json:"os"`
	Apache  string    `json:"apache"`
	MySQL   string    `json:"mysql"`
	Perl    string    `json:"perl"`
	Python  string    `json:"python"`
	Ruby    string    `json:"ruby"`
	Backend []Backend `json:"backend"` // doc "string"; real: array of available backends
}

// Backend is one selectable Apache+PHP backend on the node. Port arrives quoted
// ("8094") and decodes through flex.Int; ReleaseVersion is absent on older
// (legacy) backends.
type Backend struct {
	ID             flex.Int `json:"id"`
	Type           string   `json:"type"`
	Descr          string   `json:"descr"`
	Port           flex.Int `json:"port"`
	PHPInfo        string   `json:"php_info"`
	ReleaseVersion string   `json:"release_version"` // absent on legacy backends
}

// Index returns the account's current tariff and real resource usage (method
// "index"). Read-only. The API wraps the single record in a one-element array;
// this unwraps it, returning nil when the array is empty.
func (s *Service) Index(ctx context.Context) (*Tariff, error) {
	var out []Tariff
	if err := s.t.Call(ctx, tariffEndpoint, "index", nil, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
}

// ServerInfo returns information about the node the account is hosted on (method
// "serverInfo"). Read-only. The API wraps the single record in a one-element
// array; this unwraps it, returning nil when the array is empty.
func (s *Service) ServerInfo(ctx context.Context) (*ServerInfo, error) {
	var out []ServerInfo
	if err := s.t.Call(ctx, tariffEndpoint, "serverInfo", nil, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
}
