// Package ddg groups shared-hosting DDoS-Guard operations (endpoint /vh/ddg):
// list the domains and their protection state, read the enable page's catalogue,
// count the account's domains for pagination, quote the service price (per current
// plan and for the tariff-change widget), and enable/disable protection per domain.
// All calls dispatch through the shared transport.
//
// The service is available only to VH clients on non-hosting plans (per the
// descriptor). Every method is fully typed against the OpenRPC descriptor's field
// docs and examples: unlike balancer/dbaas, the mutating methods here (Enable,
// Disable) do NOT answer a 1/0-or-boolean sentinel — the descriptor documents each
// as a concrete object (the affected domain plus, for enable, its new IP; for
// disable, its paid-through date), so there is no sentinelAction helper. Those
// shapes come from the descriptor example and are not yet reconciled against a
// recorded live response (see the per-type notes); numeric fields decode through
// flex to tolerate the API's polymorphic number quoting.
package ddg

import (
	"context"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const ddgEndpoint = "/vh/ddg"

// Service groups shared-hosting DDoS-Guard operations (endpoint /vh/ddg):
// list/enableInfo/countAllDomains/getPrice/priceWidget plus per-domain enable/disable.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Domain is one domain and its DDoS-Guard state, as returned by List (method
// "index"). IP/Expired/Blocked are null when the service is not connected; the
// zero value ("") represents that null here. Status is one of: "disabled" (not
// connected), "active" (connected), "active_blocked" (blocked by the client),
// "blocked" (blocked for non-payment).
type Domain struct {
	FQDN         string `json:"fqdn"`         // Punycode
	FQDNReadable string `json:"fqdnReadable"` // human-readable
	IP           string `json:"ip"`           // "" when null (service not connected)
	Expired      string `json:"expired"`      // "yyyy-mm-dd"; "" when null
	Blocked      string `json:"blocked"`      // "yyyy-mm-dd"; "" when null
	Status       string `json:"status"`
}

// ListOptions are the paging/sort knobs for List. Page and PerPage are required by
// the descriptor; the zero value sends them as 0 (let the caller set them). OrderField
// is "status" (default) or "fqdn"; OrderDirect is "ASC" (default) or "DESC".
type ListOptions struct {
	Page        int    // 1-based page number
	PerPage     int    // domains per page
	OrderField  string // "status" (default) or "fqdn"
	OrderDirect string // "ASC" (default) or "DESC"
}

type listParams struct {
	Page        int    `json:"page"`
	PerPage     int    `json:"perPage"`
	OrderField  string `json:"orderField,omitempty"`
	OrderDirect string `json:"orderDirect,omitempty"`
}

// List returns the account's domains with their DDoS-Guard state (method "index").
// Read-only. page and perPage are required; the descriptor returns the domains as a
// bare array.
func (s *Service) List(ctx context.Context, opts *ListOptions) ([]Domain, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	var out []Domain
	err := s.t.Call(ctx, ddgEndpoint, "index", listParams{
		Page:        opts.Page,
		PerPage:     opts.PerPage,
		OrderField:  opts.OrderField,
		OrderDirect: opts.OrderDirect,
	}, &out)
	return out, err
}

// CountAllDomains returns the number of the client's domains, excluding technical
// ones (method "countAllDomains"). Used to drive pagination on the main page.
// Read-only.
func (s *Service) CountAllDomains(ctx context.Context) (int64, error) {
	var out flex.Int
	err := s.t.Call(ctx, ddgEndpoint, "countAllDomains", nil, &out)
	return int64(out), err
}

// EnableInfo is the data for the enable page (method "enableInfo"): the domains
// eligible to connect the service, and its price.
type EnableInfo struct {
	Domains []EligibleDomain `json:"domains"`
	Price   flex.Float       `json:"price"` // documented float|int
}

// EligibleDomain is one domain eligible to connect DDoS-Guard, as nested under
// "domains" in the EnableInfo result. SSL carries the domain's certificate info,
// or is nil when the domain has no certificate.
type EligibleDomain struct {
	FQDN         string   `json:"fqdn"`         // Punycode
	FQDNReadable string   `json:"fqdnReadable"` // human-readable
	IsOnOurNS    bool     `json:"isOnOurNs"`    // domain on SpaceWeb NS
	SSL          *SSLInfo `json:"ssl"`          // nil when the domain has no certificate
}

// SSLInfo is the per-domain SSL summary in EligibleDomain.SSL. The descriptor types
// it only as array|null; the example returns an object with these flags, so it is
// typed from the example (doc-vs-reality: the descriptor's "array" is the null case,
// the populated form is this object).
type SSLInfo struct {
	IsFilled bool `json:"isFilled"`
	IsOur    bool `json:"isOur"`
}

// EnableInfo returns the enable-page data: the domains eligible to connect the
// service and its price (method "enableInfo"). Read-only.
func (s *Service) EnableInfo(ctx context.Context) (*EnableInfo, error) {
	var out EnableInfo
	if err := s.t.Call(ctx, ddgEndpoint, "enableInfo", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPrice returns the service price for the user's current tariff plan (method
// "getPrice"). Read-only. Money arrives polymorphic (int or float), so it decodes
// through flex.Float.
func (s *Service) GetPrice(ctx context.Context) (float64, error) {
	var out flex.Float
	err := s.t.Call(ctx, ddgEndpoint, "getPrice", nil, &out)
	return float64(out), err
}

// Price is the tariff-change-widget pricing (method "priceWidget"): the service
// price on the current tariff and on a tariff from another line. The descriptor
// documents both as float|int, and the example quotes them as strings ("290"), so
// both decode through flex.Float.
type Price struct {
	Current flex.Float `json:"current"` // price on the current tariff
	New     flex.Float `json:"new"`     // price when switching to another tariff line
}

// PriceWidget returns the service prices for the "Change tariff" widget (method
// "priceWidget"). Read-only.
func (s *Service) PriceWidget(ctx context.Context) (*Price, error) {
	var out Price
	if err := s.t.Call(ctx, ddgEndpoint, "priceWidget", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Enable is the result of connecting/unblocking the service for a domain (method
// "enable"): the affected domain and the new IP it now resolves to.
//
// Doc-vs-reality: the descriptor types the result only as "object"; these fields
// come from its field docs and example and are not yet reconciled against a
// recorded live response. Unlike balancer/dbaas, the descriptor documents no 1/0
// sentinel for this mutating method, so it is typed rather than run through a
// sentinelAction helper.
type Enable struct {
	FQDN         string `json:"fqdn"`         // Punycode
	FQDNReadable string `json:"fqdnReadable"` // human-readable
	IsOnOurNS    bool   `json:"isOnOurNs"`    // domain on SpaceWeb NS
	IP           string `json:"ip"`           // new IP address
}

// Enable connects (or unblocks) DDoS-Guard for domain (method "enable"). MUTATING.
// Returns the affected domain and its new IP.
func (s *Service) Enable(ctx context.Context, domain string) (*Enable, error) {
	var out Enable
	if err := s.t.Call(ctx, ddgEndpoint, "enable", map[string]any{"domain": domain}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Disable is the result of disconnecting/blocking the service for a domain (method
// "disable"): the affected domain and its paid-through date.
//
// Doc-vs-reality: the descriptor types the result only as "object"; these fields
// come from its field docs and example and are not yet reconciled against a
// recorded live response. Expire is a "dd.mm.yy" string as documented (kept as a
// string — no consistent parseable format is guaranteed across the API). As with
// Enable, no 1/0 sentinel is documented, so the result is typed.
type Disable struct {
	FQDN         string `json:"fqdn"`         // Punycode
	FQDNReadable string `json:"fqdnReadable"` // human-readable
	IsOnOurNS    bool   `json:"isOnOurNs"`    // domain on SpaceWeb NS
	Expire       string `json:"expire"`       // paid-through date, "dd.mm.yy"
}

// Disable disconnects (or blocks) DDoS-Guard for domain (method "disable").
// MUTATING. Returns the affected domain and its paid-through date.
func (s *Service) Disable(ctx context.Context, domain string) (*Disable, error) {
	var out Disable
	if err := s.t.Call(ctx, ddgEndpoint, "disable", map[string]any{"domain": domain}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
