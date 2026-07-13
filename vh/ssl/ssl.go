// Package ssl groups shared-hosting SSL-certificate operations (endpoint
// /vh/ssl): list the account's certificates, browse the certificate catalogue,
// download an issued certificate archive, manage a certificate's
// auto-prolongation and lifecycle, and install a free Let's Encrypt certificate.
// All calls dispatch through the shared transport.
//
// This is the shared-hosting (virtual hosting) SSL panel, distinct from the VPS
// SSL service at /vps/ssl: it exposes Let's Encrypt installation and its index
// carries a per-certificate IP, where the VPS service exposes a paid
// order/order-submit flow instead.
package ssl

import (
	"context"
	"encoding/json"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const sslEndpoint = "/vh/ssl"

// Service groups shared-hosting SSL-certificate operations (endpoint /vh/ssl):
// list the account's certificates, browse the certificate catalogue, download an
// issued certificate archive, manage a certificate's auto-prolongation and
// lifecycle, and install a free Let's Encrypt certificate.
//
// The read-only methods (List, OrderList, ProlongInfo, Download) are fully typed
// against the OpenRPC descriptor. The mutating methods (EditAutoprolong,
// RemoveCertificate, ProlongCertificate, InstallLetsEncrypt) return
// json.RawMessage: their success sentinels are documented (integer 1/0) but not
// yet reconciled against recorded live responses, so evidence-first typing leaves
// them raw for the caller to interpret rather than guessing an error-mapping the
// SDK can't verify (see the per-method notes).
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Certificate is one issued/ordered certificate from the account, as nested under
// "list" in the index result.
type Certificate struct {
	ID      flex.Int `json:"id"`
	Status  string   `json:"status"`
	IP      string   `json:"ip"`     // shared-hosting IP ("sni" for SNI); nullable
	Domain  string   `json:"domain"` // fully-qualified domain the cert covers
	Name    string   `json:"name"`   // product name, e.g. "Let's Encrypt"; nullable
	ValidTo string   `json:"valid_to"`
	// ProlongAvailable is 1 when prolongation is offered.
	ProlongAvailable   flex.Int `json:"prolong_available"`
	Autoprolong        bool     `json:"autoprolong"`        // auto-prolongation enabled
	AutoprolongAllowed bool     `json:"autoprolongAllowed"` // auto-prolongation offered
	// AutoprolongAddition carries the product/price the auto-prolongation would
	// order; null when unavailable.
	AutoprolongAddition *AutoprolongAddition `json:"autoprolongAddition"`
}

// AutoprolongAddition is the product an auto-prolongation would order for a
// Certificate (the "autoprolongAddition" object in the index result).
type AutoprolongAddition struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	FullName string     `json:"full_name"`
	Price    flex.Float `json:"price"`
}

// FilterInfo is the pagination/sort echo returned alongside the certificate
// list ("filterInfo" in the index result).
type FilterInfo struct {
	OrderDirect string   `json:"orderDirect"`
	OrderField  string   `json:"orderField"`
	Page        flex.Int `json:"page"`
	PerPage     flex.Int `json:"perPage"`
	TotalCount  flex.Int `json:"totalCount"`
}

// CertificateList is the index result: the account's certificates plus the
// pagination/sort echo. The API returns it as a bare object.
type CertificateList struct {
	List       []Certificate `json:"list"`
	FilterInfo FilterInfo    `json:"filterInfo"`
}

// ListOptions are the optional paging/sort knobs for List. The zero value asks
// for the server default page (perPage 20).
type ListOptions struct {
	Page        int    // 1-based page number
	PerPage     int    // records per page
	OrderField  string // "id", "valid_to", "fqdn", "status", "ip"
	OrderDirect string // "asc" or "desc"
}

type listParams struct {
	Page        int    `json:"page,omitempty"`
	PerPage     int    `json:"perPage,omitempty"`
	OrderField  string `json:"orderField,omitempty"`
	OrderDirect string `json:"orderDirect,omitempty"`
}

// List returns the account's SSL certificates ("index"). The API returns a bare
// CertificateList object; this decodes it directly. Read-only.
func (s *Service) List(ctx context.Context, opts *ListOptions) (*CertificateList, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	var out CertificateList
	if err := s.t.Call(ctx, sslEndpoint, "index", listParams{
		Page:        opts.Page,
		PerPage:     opts.PerPage,
		OrderField:  opts.OrderField,
		OrderDirect: opts.OrderDirect,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OrderOption is one certificate product available for order ("getOrderList").
type OrderOption struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`           // "dv", "ov", "ev"
	AdvantageText string   `json:"advantage_text"` // one-line summary
	Advantages    []string `json:"advantages"`     // per-line HTML descriptions
	Persons       []string `json:"persons"`        // eligible domain-person kinds ("u", "f", "ip")
	Periods       []string `json:"periods"`        // orderable periods in months
	// Prices/PricesOld map a period (months) to its price. The descriptor types
	// them as array|null, but populated they arrive as an object keyed by period
	// (e.g. {"12":"4100.00"}); null decodes to a nil map.
	Prices    map[string]flex.Float `json:"prices"`
	PricesOld map[string]flex.Float `json:"prices_old"`
	// AutoprolongAddition is null in observed responses; kept raw as its populated
	// shape is unrecorded.
	AutoprolongAddition json.RawMessage `json:"autoprolongAddition"`
}

// OrderList returns the certificate products available for order ("getOrderList").
// Read-only.
func (s *Service) OrderList(ctx context.Context) ([]OrderOption, error) {
	var out []OrderOption
	err := s.t.Call(ctx, sslEndpoint, "getOrderList", struct{}{}, &out)
	return out, err
}

// CertFile is one file from a downloaded certificate archive ("download"). Content
// is the file body encoded per Mimetype (e.g. base64 for "application/zip;base64").
type CertFile struct {
	Mimetype string          `json:"mimetype"`
	Name     string          `json:"name"`
	Content  string          `json:"content"`
	Metadata json.RawMessage `json:"metadata"` // shape unrecorded (empty array in observed responses)
}

// Download returns the archive files of an issued certificate ("download"). The
// password is the account password. Read-only (no mutation), but returns secret
// key material — handle CertFile.Content accordingly. id is a Certificate.ID.
func (s *Service) Download(ctx context.Context, id int, password string) ([]CertFile, error) {
	var out []CertFile
	err := s.t.Call(ctx, sslEndpoint, "download", map[string]any{
		"id":       id,
		"password": password,
	}, &out)
	return out, err
}

// ProlongOrderData is the pre-filled order form a prolongation would submit, as
// nested under "orderData" in the getProlongInfo result. Fields mirror the wire
// names; several are nullable strings ("N"/"Y" flags, ids as strings).
type ProlongOrderData struct {
	Domain      string `json:"domain"`
	SubDomain   string `json:"sub_domain"`
	Mailbox     string `json:"mailbox"`
	PersonID    string `json:"person_id"`
	CompanyLink string `json:"company_link"`
	AuthType    string `json:"auth_type"`
	IsMachine   string `json:"is_machine"` // "N"/"Y"
	NicCustomer string `json:"nic_customer_id"`
	NicOrderID  string `json:"nic_order_id"`
}

// ProlongInfo describes the prolongation options for a certificate
// ("getProlongInfo"): the target product, its per-period prices and product ids,
// and the pre-filled order form. The API wraps it in a one-element array;
// ProlongInfo unwraps it.
type ProlongInfo struct {
	CurrentCertificateID flex.Int              `json:"currentCertificateId"`
	Title                string                `json:"title"`
	IsFreeCertificate    bool                  `json:"isFreeCertificate"`
	OrderData            ProlongOrderData      `json:"orderData"`
	Prices               map[string]flex.Float `json:"prices"` // period (months) → price
	IDs                  map[string]string     `json:"ids"`    // period (months) → product id
}

// ProlongInfo returns the prolongation options for a certificate
// ("getProlongInfo"). The API wraps the result in a one-element array; this
// unwraps it (nil if empty). certificateID is a Certificate.ID. Read-only.
func (s *Service) ProlongInfo(ctx context.Context, certificateID int) (*ProlongInfo, error) {
	var out []ProlongInfo
	if err := s.t.Call(ctx, sslEndpoint, "getProlongInfo", map[string]any{
		"certificateId": certificateID,
	}, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
}

// EditAutoprolong toggles a certificate's auto-prolongation ("editAutoprolong").
// certificateID is a Certificate.ID; enabled maps to the boolean autoprolong flag.
//
// The descriptor documents a 1/0 (success/failure) integer result, but that
// sentinel has not been reconciled against a recorded live response, so the raw
// result is returned verbatim rather than mapped to an error — the caller decides
// (doc-vs-reality: treat a decoded 1 as success once confirmed live).
func (s *Service) EditAutoprolong(ctx context.Context, certificateID int, enabled bool) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, sslEndpoint, "editAutoprolong", map[string]any{
		"certificateId": certificateID,
		"autoprolong":   enabled,
	}, &out)
	return out, err
}

// RemoveCertificate deletes a certificate ("removeCertificate"). certificateID is
// a Certificate.ID.
//
// The descriptor documents a 1/0 (success/failure) integer result; as it is
// unreconciled against a recorded live response, the raw result is returned
// verbatim rather than mapped to an error (doc-vs-reality: treat 1 as success once
// confirmed live).
func (s *Service) RemoveCertificate(ctx context.Context, certificateID int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, sslEndpoint, "removeCertificate", map[string]any{
		"certificateId": certificateID,
	}, &out)
	return out, err
}

// ProlongCertificate prolongs a certificate ("prolongCertificate"): it swaps the
// certificate identified by currentCertificateID (a Certificate.ID) for the
// product certificateProlongID (a ProlongInfo.IDs value). MUTATING.
//
// The descriptor documents a 1/0 (success/failure) integer result; as it is
// unreconciled against a recorded live response, the raw result is returned
// verbatim rather than mapped to an error (doc-vs-reality: treat 1 as success once
// confirmed live).
func (s *Service) ProlongCertificate(ctx context.Context, currentCertificateID, certificateProlongID int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, sslEndpoint, "prolongCertificate", map[string]any{
		"currentCertificateId": currentCertificateID,
		"certificateProlongId": certificateProlongID,
	}, &out)
	return out, err
}

// InstallLetsEncryptOptions carries the optional fields of an InstallLetsEncrypt
// request. The zero value omits them.
type InstallLetsEncryptOptions struct {
	Virtdom   string // subdomain to cover, e.g. "sub.mysite.ru"
	IP        string // target IP, or "sni" for SNI
	Challenge string // validation type: "acme" or "dns"
}

type installLetsEncryptParams struct {
	Domain    string `json:"domain"`
	Wildcard  int    `json:"wildcard"`
	Virtdom   string `json:"virtdom,omitempty"`
	IP        string `json:"ip,omitempty"`
	Challenge string `json:"challenge,omitempty"`
}

// InstallLetsEncrypt installs a free Let's Encrypt certificate for domain
// ("installLetsEncrypt"). wildcard requests a wildcard certificate; opts carries
// the optional subdomain, IP, and challenge type. MUTATING (issues a certificate).
//
// The descriptor documents a 1/0 (success/failure) integer result; as it is
// unreconciled against a recorded live response, the raw result is returned
// verbatim rather than mapped to an error (doc-vs-reality: treat 1 as success once
// confirmed live).
func (s *Service) InstallLetsEncrypt(ctx context.Context, domain string, wildcard bool, opts *InstallLetsEncryptOptions) (json.RawMessage, error) {
	if opts == nil {
		opts = &InstallLetsEncryptOptions{}
	}
	p := installLetsEncryptParams{
		Domain:    domain,
		Wildcard:  boolToInt(wildcard),
		Virtdom:   opts.Virtdom,
		IP:        opts.IP,
		Challenge: opts.Challenge,
	}
	var out json.RawMessage
	err := s.t.Call(ctx, sslEndpoint, "installLetsEncrypt", p, &out)
	return out, err
}

// boolToInt maps a Go bool to the API's 1/0 integer flag (wildcard).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
