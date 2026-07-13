// Package domains groups domain and subdomain operations (endpoint /domains):
// read the account's domains (List, Info, Subdomains), check/price registration
// and transfer, and mutate the domain lifecycle (register, move-in, prolong,
// remove, redirect, subdomain CRUD). All calls dispatch through the shared
// transport.
package domains

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const domainsEndpoint = "/domains"

// Service groups domain and subdomain operations (endpoint /domains): read the
// account's domains (List, Info, Subdomains), check/price registration and
// transfer, and mutate the domain lifecycle (register, move-in, prolong, remove,
// redirect, subdomain CRUD).
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Payment sources for paid operations (reg, prolong).
const (
	PayBalance = "balance" // charge the account's money balance
	PayBonus   = "bonus"   // charge bonus points
)

// Auto-prolongation modes. The API is inconsistent about the "do not prolong"
// token: reg/changeProlong/changeProlongList use "none", move/moveList use "no".
// Both spellings are provided; pick the one the target method documents.
const (
	ProlongNone       = "none"        // reg, changeProlong: do not auto-prolong
	ProlongNo         = "no"          // move, moveList: do not auto-prolong
	ProlongManual     = "manual"      // prolong manually (client-initiated)
	ProlongBonusMoney = "bonus_money" // auto-prolong from the bonus balance
)

// Domain is one entry from the account's domain list ("index"). A single struct
// carries both a plain domain and a domain package; the *List / is_available /
// order_package_id fields are populated only for packages (showPackages=true).
type Domain struct {
	FQDN         string `json:"fqdn"`          // encoded name
	FQDNReadable string `json:"fqdn_readable"` // human-readable name
	FQDNTech     string `json:"fqdn_tech"`     // technical domain
	Docroot      string `json:"docroot"`       // home directory
	SiteAlias    string `json:"siteAlias"`     // site name in the control panel

	// Package-only fields.
	FQDNList         []string `json:"fqdnList"`         // encoded member domains
	FQDNReadableList []string `json:"fqdnReadableList"` // readable member domains
	FQDNTechList     []string `json:"fqdnTechList"`     // technical domains of the package
	IsAvailable      flex.Int `json:"is_available"`     // 1 if the package can be registered
	InQueue          flex.Int `json:"in_queue"`         // 1 while an operation is running
	OrderPackageID   flex.Int `json:"order_package_id"` // order id for the package

	RegPrice       flex.Float  `json:"reg_price"`       // registration price
	BonusAvailable bool        `json:"bonus_available"` // registrable for bonus points
	Subdomains     []Subdomain `json:"subdomains"`
}

// Subdomain is a subdomain as nested under a Domain in the "index" result.
type Subdomain struct {
	Machine            string `json:"machine"`          // encoded name (e.g. "*")
	MachineReadable    string `json:"machine_readable"` // readable name
	FQDN               string `json:"fqdn"`             // full encoded name incl. domain
	FQDNReadable       string `json:"fqdn_readable"`    // full readable name
	ParentFQDN         string `json:"parent_fqdn"`      // parent domain (encoded)
	ParentFQDNReadable string `json:"parent_fqdn_readable"`
	Docroot            string `json:"docroot"`   // home directory
	SiteAlias          string `json:"siteAlias"` // site name in the control panel
}

// SubdomainRef is one entry from "getSubdomains": encoded value + readable name.
type SubdomainRef struct {
	Value string `json:"value"` // encoded name
	Name  string `json:"name"`  // readable name
}

// DomainInfo is the full per-domain record returned by "getDomainInfo".
type DomainInfo struct {
	IsActiveTask   flex.Int        `json:"is_active_task"`   // an operation is running
	Autoreg        flex.Int        `json:"autoreg"`          // auto-registration enabled
	IsTaken        flex.Int        `json:"is_taken"`         // domain is taken
	Registrar      string          `json:"registrar"`        // null → ""
	IsOur          flex.Int        `json:"is_our"`           // 1 if under our management
	Expired        string          `json:"expired"`          // registration expiry date, "" if unknown
	CanProlong     flex.Int        `json:"can_prolong"`      // 1 if prolongable now
	ProlongPrice   flex.Int        `json:"prolong_price"`    // prolongation price
	ProlongByBonus bool            `json:"prolong_by_bonus"` // prolongable for bonus points
	ProlongConfirm *ProlongConfirm `json:"prolong_confirm"`  // null when no confirmation dialog
	RegPrice       flex.Int        `json:"reg_price"`        // registration price
	TransferPrice  flex.Int        `json:"transfer_price"`   // -1 when transfer not offered
	Autoprolong    string          `json:"autoprolong"`      // "no", "manual", "bonus_money"
	DocRoot        string          `json:"docRoot"`          // doc says int; live is a path string
	SiteAlias      string          `json:"siteAlias"`        // site name in the control panel
	BonusAvailable bool            `json:"bonus_available"`  // registrable for bonus points
	TransferLink   string          `json:"transferLink"`     // null → ""
	RedirectURL    string          `json:"redirectUrl"`      // configured redirect, null → ""
}

// ProlongConfirm carries the prolongation-dialog details from DomainInfo.
type ProlongConfirm struct {
	Domain  string   `json:"domain"`  // readable domain name
	Confirm bool     `json:"confirm"` // whether to show a confirmation dialog
	Price   flex.Int `json:"price"`   // prolongation price
	Link    string   `json:"link"`    // extra-info URL, null → ""
}

// Package is one discounted registration package from "getAvailablePackages".
type Package struct {
	ID             flex.Int        `json:"id"`
	NameReadable   string          `json:"name_readable"`
	Price          flex.Float      `json:"price"`  // promotional price
	Price2         flex.Float      `json:"price2"` // regular price
	Priority       flex.Int        `json:"priority"`
	Available      bool            `json:"available"`
	OrderPackageID flex.Int        `json:"order_package_id"`
	Domains        []PackageDomain `json:"domains"`
}

// PackageDomain is one member domain of a Package.
type PackageDomain struct {
	Name         string `json:"name"`          // encoded name
	NameReadable string `json:"name_readable"` // readable name
}

// ListOptions filters and paginates List ("index"). All fields are optional.
type ListOptions struct {
	OrderField   string // sort field (e.g. "fqdn_readable")
	OrderDirect  string // sort direction ("ASC"/"DESC")
	Type         string // "all", "sweb", "free", "other"
	Filter       string // substring filter on the domain name
	Page         int
	PerPage      int
	ShowPackages bool // include domain packages in the result
}

type listParams struct {
	OrderField   string `json:"orderField,omitempty"`
	OrderDirect  string `json:"orderDirect,omitempty"`
	Type         string `json:"type,omitempty"`
	Filter       string `json:"filter,omitempty"`
	Page         int    `json:"page,omitempty"`
	PerPage      int    `json:"perPage,omitempty"`
	ShowPackages bool   `json:"showPackages"`
}

// List returns the account's domains ("index"). Read-only.
func (s *Service) List(ctx context.Context, opts *ListOptions) ([]Domain, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	var out []Domain
	if err := s.t.Call(ctx, domainsEndpoint, "index", listParams{
		OrderField:   opts.OrderField,
		OrderDirect:  opts.OrderDirect,
		Type:         opts.Type,
		Filter:       opts.Filter,
		Page:         opts.Page,
		PerPage:      opts.PerPage,
		ShowPackages: opts.ShowPackages,
	}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Subdomains lists a domain's subdomains ("getSubdomains"). Read-only.
func (s *Service) Subdomains(ctx context.Context, domain string) ([]SubdomainRef, error) {
	var out []SubdomainRef
	if err := s.t.Call(ctx, domainsEndpoint, "getSubdomains", map[string]any{"domain": domain}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Info returns the full record for one domain ("getDomainInfo"). Read-only.
func (s *Service) Info(ctx context.Context, domain string) (*DomainInfo, error) {
	var out DomainInfo
	if err := s.t.Call(ctx, domainsEndpoint, "getDomainInfo", map[string]any{"domain": domain}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RegAvailable reports whether a domain can be registered ("regAvailable",
// 1=available). A preliminary check — read-only.
func (s *Service) RegAvailable(ctx context.Context, domain, payType string) (bool, error) {
	return s.queryFlag(ctx, "regAvailable", map[string]any{"domain": domain, "payType": payType})
}

// TransferAvailable reports whether a domain can be transferred in
// ("priceForTrasfer" — the API misspells "transfer"; the wire method keeps the
// typo, 1=available). Read-only.
func (s *Service) TransferAvailable(ctx context.Context, domain string) (bool, error) {
	return s.queryFlag(ctx, "priceForTrasfer", map[string]any{"domain": domain})
}

// RegistrationPrice returns a domain's registration price ("priceForRegistration").
// Read-only.
func (s *Service) RegistrationPrice(ctx context.Context, domain string) (float64, error) {
	var out flex.Float
	if err := s.t.Call(ctx, domainsEndpoint, "priceForRegistration", map[string]any{"domain": domain}, &out); err != nil {
		return 0, err
	}
	return float64(out), nil
}

// Redirect returns a domain's configured redirect URL ("getRedirectVh"). Read-only.
func (s *Service) Redirect(ctx context.Context, domain string) (string, error) {
	var out string
	if err := s.t.Call(ctx, domainsEndpoint, "getRedirectVh", map[string]any{"domain": domain}, &out); err != nil {
		return "", err
	}
	return out, nil
}

// AvailablePackages checks whether the given domains form a discounted package
// and returns the matching packages with an order_package_id ("getAvailablePackages").
//
// SIDE EFFECT: despite the "get" name, on a hit the API ADDS the package to the
// account (per the apidoc). Not a pure read — do not call it speculatively.
func (s *Service) AvailablePackages(ctx context.Context, domains ...string) ([]Package, error) {
	var out []Package
	if err := s.t.Call(ctx, domainsEndpoint, "getAvailablePackages", map[string]any{"domains": strings.Join(domains, ", ")}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RegisterOptions parameterizes Register ("reg").
type RegisterOptions struct {
	Domain      string
	PayType     string // PayBalance or PayBonus
	DomPerson   int    // domain-person (registrant contact) id
	ProlongType string // ProlongNone, ProlongBonusMoney, ProlongManual
	AutoReg     int    // auto-registration flag
	Dir         string // relative site directory
	IDShield    bool   // hide WHOIS
}

// Register registers a domain on the account ("reg", 1=success). Bills the account.
func (s *Service) Register(ctx context.Context, o RegisterOptions) error {
	return s.actionOne(ctx, "reg", map[string]any{
		"domain":      o.Domain,
		"payType":     o.PayType,
		"domPerson":   o.DomPerson,
		"prolongType": o.ProlongType,
		"autoReg":     o.AutoReg,
		"dir":         o.Dir,
		"idShield":    o.IDShield,
	})
}

// RegisterList registers several domains at once ("regList", 1=success). Bills
// the account.
func (s *Service) RegisterList(ctx context.Context, domains ...string) error {
	return s.actionOne(ctx, "regList", map[string]any{"domains": strings.Join(domains, ", ")})
}

// Move adds an existing domain to the account ("move"). On success the API
// answers 1; on failure it answers an ExtendedResult carrying per-domain errors,
// surfaced here as an error.
func (s *Service) Move(ctx context.Context, domain, prolongType, dir string) error {
	return s.callExtended(ctx, "move", map[string]any{
		"domain":      domain,
		"prolongType": prolongType,
		"dir":         dir,
	})
}

// MoveItem is one domain for MoveList.
type MoveItem struct {
	FQDN        string `json:"fqdn"`
	ProlongType string `json:"prolongType"` // ProlongNo, ProlongManual, ProlongBonusMoney
	Dir         string `json:"dir,omitempty"`
}

// MoveList adds several existing domains to the account ("moveList"). Same
// success/ExtendedResult contract as Move.
func (s *Service) MoveList(ctx context.Context, items ...MoveItem) error {
	return s.callExtended(ctx, "moveList", map[string]any{"domains": items})
}

// ChangeProlong changes a domain's auto-prolongation setting ("changeProlong",
// 1=success).
func (s *Service) ChangeProlong(ctx context.Context, domain, prolongType string) error {
	return s.actionOne(ctx, "changeProlong", map[string]any{"domain": domain, "prolongType": prolongType})
}

// ProlongItem is one domain for ChangeProlongList.
type ProlongItem struct {
	Domain      string `json:"domain"`
	ProlongType string `json:"prolongType"` // ProlongNo, ProlongManual, ProlongBonusMoney
}

// ChangeProlongList changes auto-prolongation for several domains at once
// ("changeProlongList", 1=success).
func (s *Service) ChangeProlongList(ctx context.Context, items ...ProlongItem) error {
	return s.actionOne(ctx, "changeProlongList", map[string]any{"domains": items})
}

// Remove deletes a domain from the account ("remove", 1=success).
func (s *Service) Remove(ctx context.Context, domain string) error {
	return s.actionOne(ctx, "remove", map[string]any{"domain": domain})
}

// RemoveList deletes several domains at once ("removeList", 1=success).
func (s *Service) RemoveList(ctx context.Context, domains ...string) error {
	return s.actionOne(ctx, "removeList", map[string]any{"domains": strings.Join(domains, ", ")})
}

// Prolong prolongs a domain's registration ("prolong", 1=success). Bills the
// account.
func (s *Service) Prolong(ctx context.Context, domain, payType string) error {
	return s.actionOne(ctx, "prolong", map[string]any{"domain": domain, "payType": payType})
}

// ProlongList prolongs several domains at once ("prolongList"). Unlike the other
// batch methods it always answers an ExtendedResult envelope; it is returned to
// the caller (with the success message) and, on a non-1 code, also as an error.
func (s *Service) ProlongList(ctx context.Context, domains ...string) (*ExtendedResult, error) {
	var raw json.RawMessage
	if err := s.t.Call(ctx, domainsEndpoint, "prolongList", map[string]any{"domains": strings.Join(domains, ", ")}, &raw); err != nil {
		return nil, err
	}
	er, err := parseExtended(raw)
	if err != nil {
		return nil, fmt.Errorf("sweb: prolongList: decode result: %w", err)
	}
	return er, er.err("prolongList")
}

// CreateSubdomain adds a subdomain ("createSubdomain", 1=success).
func (s *Service) CreateSubdomain(ctx context.Context, domain, machine, dir string) error {
	return s.actionOne(ctx, "createSubdomain", map[string]any{"domain": domain, "machine": machine, "dir": dir})
}

// RemoveSubdomain deletes a subdomain ("removeSubdomain", 1=success).
func (s *Service) RemoveSubdomain(ctx context.Context, domain, machine string) error {
	return s.actionOne(ctx, "removeSubdomain", map[string]any{"domain": domain, "machine": machine})
}

// SetRedirect sets a domain's redirect URL ("setRedirectVh", 1=success).
//
// NOTE: the apidoc's request example names the URL param "setRedirectVh", but the
// documented parameter is "redirect"; this sends "redirect" per the parameter
// table. Confirm against a live response before relying on it.
func (s *Service) SetRedirect(ctx context.Context, domain, redirect string) error {
	return s.actionOne(ctx, "setRedirectVh", map[string]any{"domain": domain, "redirect": redirect})
}

// ExtendedResult is SpaceWeb's batch-operation envelope. move/moveList answer a
// bare 1 on success or this on failure (Data holds per-domain [fqdn, error]
// pairs); prolongList always wraps it as {"extendedResult":{…}}. Code 1 == success.
type ExtendedResult struct {
	Code    flex.Int        `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"` // [] on success; per-domain errors otherwise
}

// err converts a non-success ExtendedResult (Code != 1) into a Go error, folding
// in the message and any per-domain error data.
func (er *ExtendedResult) err(method string) error {
	if er == nil || er.Code == 1 {
		return nil
	}
	msg := er.Message
	if data := bytes.TrimSpace(er.Data); len(data) > 0 &&
		!bytes.Equal(data, []byte("[]")) && !bytes.Equal(data, []byte("null")) {
		if msg != "" {
			msg += ": "
		}
		msg += string(data)
	}
	if msg == "" {
		msg = fmt.Sprintf("code %d", int64(er.Code))
	}
	return fmt.Errorf("sweb: %s failed: %s", method, msg)
}

// parseExtended decodes a batch-method result that may be a bare success sentinel
// (1/true), a bare ExtendedResult object, or one wrapped as {"extendedResult":{…}}.
func parseExtended(raw json.RawMessage) (*ExtendedResult, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	if bytes.Equal(raw, []byte("1")) || bytes.Equal(raw, []byte("true")) {
		return &ExtendedResult{Code: 1}, nil
	}
	if bytes.Equal(raw, []byte("0")) || bytes.Equal(raw, []byte("false")) {
		return &ExtendedResult{Code: 0}, nil
	}
	var wrap struct {
		ExtendedResult *ExtendedResult `json:"extendedResult"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && wrap.ExtendedResult != nil {
		return wrap.ExtendedResult, nil
	}
	var er ExtendedResult
	if err := json.Unmarshal(raw, &er); err != nil {
		return nil, err
	}
	return &er, nil
}

// callExtended runs a batch method whose success is a bare 1 and whose failure is
// an ExtendedResult (move/moveList).
func (s *Service) callExtended(ctx context.Context, method string, params any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, domainsEndpoint, method, params, &raw); err != nil {
		return err
	}
	er, err := parseExtended(raw)
	if err != nil {
		return fmt.Errorf("sweb: %s: decode result: %w", method, err)
	}
	return er.err(method)
}

// actionOne runs a mutating method whose success sentinel is integer 1.
func (s *Service) actionOne(ctx context.Context, method string, params any) error {
	var out flex.Int
	if err := s.t.Call(ctx, domainsEndpoint, method, params, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: %s returned %d, want 1", method, int64(out))
	}
	return nil
}

// queryFlag runs a read-only check whose 1/0 result is a boolean answer, not a
// success/failure sentinel (regAvailable, priceForTrasfer): 0 is a valid "no".
func (s *Service) queryFlag(ctx context.Context, method string, params any) (bool, error) {
	var out flex.Int
	if err := s.t.Call(ctx, domainsEndpoint, method, params, &out); err != nil {
		return false, err
	}
	return out == 1, nil
}
