package sweb

import (
	"context"
	"encoding/json"
)

const sslEndpoint = "/vps/ssl"

// SSLService groups VPS SSL-certificate operations (endpoint /vps/ssl): list the
// account's certificates, browse and order the certificate catalogue, download an
// issued certificate archive, and manage a certificate's auto-prolongation and
// lifecycle.
//
// The read-only methods (List, OrderList, ProlongInfo, Download) are fully typed
// against the OpenRPC descriptor. The mutating methods (EditAutoprolong,
// RemoveCertificate, OrderSubmit) return json.RawMessage: their success sentinels
// are documented but not yet reconciled against recorded live responses, and
// OrderSubmit mutates and bills, so evidence-first typing leaves them raw (see the
// per-method notes) rather than guessing an error-mapping the SDK can't verify.
type SSLService struct{ c *Client }

// Certificate is one issued/ordered certificate from the account, as nested under
// "list" in the index result.
type Certificate struct {
	ID                 FlexInt `json:"id"`
	Status             string  `json:"status"`
	IP                 string  `json:"ip"`     // only populated for virtual hosting; null otherwise
	Domain             string  `json:"domain"` // fully-qualified domain the cert covers
	Name               string  `json:"name"`   // product name, e.g. "GlobalSign AlphaSSL"
	ValidTo            string  `json:"valid_to"`
	ProlongAvailable   FlexInt `json:"prolong_available"` // 1 if prolongation is offered
	Autoprolong        bool    `json:"autoprolong"`       // auto-prolongation enabled
	AutoprolongAllowed bool    `json:"autoprolongAllowed"`
	IsFree             bool    `json:"isFree"`
	// AutoprolongAddition carries the product/price the auto-prolongation would
	// order; null when unavailable.
	AutoprolongAddition *AutoprolongAddition `json:"autoprolongAddition"`
}

// AutoprolongAddition is the product an auto-prolongation would order for a
// Certificate (the "autoprolongAddition" object in the index result).
type AutoprolongAddition struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	FullName string    `json:"full_name"`
	Price    FlexFloat `json:"price"`
}

// CertFilterInfo is the pagination/sort echo returned alongside the certificate
// list ("filterInfo" in the index result).
type CertFilterInfo struct {
	OrderDirect string  `json:"orderDirect"`
	OrderField  string  `json:"orderField"`
	Page        FlexInt `json:"page"`
	PerPage     FlexInt `json:"perPage"`
	TotalCount  FlexInt `json:"totalCount"`
}

// CertificateList is the index result: the account's certificates plus the
// pagination/sort echo. The API wraps it in a one-element array; List unwraps it.
type CertificateList struct {
	List       []Certificate  `json:"list"`
	FilterInfo CertFilterInfo `json:"filterInfo"`
}

// ListOptions are the optional paging/sort knobs for List. The zero value asks
// for the server default page.
type ListOptions struct {
	Page        int    // 1-based page number
	PerPage     int    // records per page
	OrderField  string // "id", "valid_to", "fqdn", "status", "ip"
	OrderDirect string // "asc" or "desc"
}

type sslListParams struct {
	Page        int    `json:"page,omitempty"`
	PerPage     int    `json:"perPage,omitempty"`
	OrderField  string `json:"orderField,omitempty"`
	OrderDirect string `json:"orderDirect,omitempty"`
}

// List returns the account's SSL certificates ("index"). The API wraps the result
// in a one-element array; this unwraps it (nil if empty). Read-only.
func (s *SSLService) List(ctx context.Context, opts *ListOptions) (*CertificateList, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	var out []CertificateList
	if err := s.c.call(ctx, sslEndpoint, "index", sslListParams{
		Page:        opts.Page,
		PerPage:     opts.PerPage,
		OrderField:  opts.OrderField,
		OrderDirect: opts.OrderDirect,
	}, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
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
	// (e.g. {"12":"1900.00"}); null decodes to a nil map.
	Prices    map[string]FlexFloat `json:"prices"`
	PricesOld map[string]FlexFloat `json:"prices_old"`
	// AutoprolongAddition is null in observed responses; kept raw as its populated
	// shape is unrecorded.
	AutoprolongAddition json.RawMessage `json:"autoprolongAddition"`
}

// OrderList returns the certificate products available for order ("getOrderList").
// Read-only.
func (s *SSLService) OrderList(ctx context.Context) ([]OrderOption, error) {
	var out []OrderOption
	err := s.c.call(ctx, sslEndpoint, "getOrderList", struct{}{}, &out)
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
func (s *SSLService) Download(ctx context.Context, id int, password string) ([]CertFile, error) {
	var out []CertFile
	err := s.c.call(ctx, sslEndpoint, "download", map[string]any{
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
	CurrentCertificateID FlexInt              `json:"currentCertificateId"`
	Title                string               `json:"title"`
	IsFreeCertificate    bool                 `json:"isFreeCertificate"`
	OrderData            ProlongOrderData     `json:"orderData"`
	Prices               map[string]FlexFloat `json:"prices"` // period (months) → price
	IDs                  map[string]string    `json:"ids"`    // period (months) → product id
}

// ProlongInfo returns the prolongation options for a certificate
// ("getProlongInfo"). The API wraps the result in a one-element array; this
// unwraps it (nil if empty). certificateID is a Certificate.ID. Read-only.
func (s *SSLService) ProlongInfo(ctx context.Context, certificateID int) (*ProlongInfo, error) {
	var out []ProlongInfo
	if err := s.c.call(ctx, sslEndpoint, "getProlongInfo", map[string]any{
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
// enabled true sends 1, false sends 0.
//
// The descriptor documents a 1/0 (success/failure) integer result, but that
// sentinel has not been reconciled against a recorded live response, so the raw
// result is returned verbatim rather than mapped to an error — the caller decides
// (doc-vs-reality: treat a decoded 1 as success once confirmed live).
func (s *SSLService) EditAutoprolong(ctx context.Context, certificateID int, enabled bool) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, sslEndpoint, "editAutoprolong", map[string]any{
		"certificateId": certificateID,
		"autoprolong":   boolToInt(enabled),
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
func (s *SSLService) RemoveCertificate(ctx context.Context, certificateID int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, sslEndpoint, "removeCertificate", map[string]any{
		"certificateId": certificateID,
	}, &out)
	return out, err
}

// OrderSubmitOptions carries the optional fields of an OrderSubmit request. The
// zero value omits them; PersonID/OldCertificateID <= 0 are omitted.
type OrderSubmitOptions struct {
	PersonID         int    // domain-person id
	CompanyPageLink  string // "about the company" URL (EV/OV)
	Subdomain        string // subdomain the cert covers
	Autoprolong      bool   // enable auto-prolongation on the new cert
	OldCertificateID int    // prior domain-person id (prolongation)
	FromProlongation bool   // order originates from a prolongation confirmation
}

type orderSubmitParams struct {
	Domain                 string `json:"domain"`
	CertificateID          int    `json:"certificateId"`
	CertificateConfirmMail string `json:"certificateConfirmMail"`
	PersonID               int    `json:"personId,omitempty"`
	CompanyPageLink        string `json:"companyPageLink,omitempty"`
	Subdomain              string `json:"subdomain,omitempty"`
	Autoprolong            int    `json:"autoprolong"`
	OldCertificateID       int    `json:"oldCertificateId,omitempty"`
	FromProlongation       bool   `json:"fromProlongation,omitempty"`
}

// OrderSubmit places a certificate order ("orderSubmit"). MUTATING AND BILLED —
// this orders a paid certificate. domain and confirmMail are required, as is
// certificateID (an OrderOption.ID); opts carries the optional fields.
//
// The descriptor documents a tri-state integer result — 1 (success), 0 (failure),
// 2 (queued for manual processing). That sentinel is not reconciled against a
// recorded live response (the method bills, so it is never exercised in recon), so
// the raw result is returned verbatim for the caller to interpret against the
// documented tri-state rather than mapped to a single error.
func (s *SSLService) OrderSubmit(ctx context.Context, domain string, certificateID int, confirmMail string, opts *OrderSubmitOptions) (json.RawMessage, error) {
	if opts == nil {
		opts = &OrderSubmitOptions{}
	}
	p := orderSubmitParams{
		Domain:                 domain,
		CertificateID:          certificateID,
		CertificateConfirmMail: confirmMail,
		PersonID:               opts.PersonID,
		CompanyPageLink:        opts.CompanyPageLink,
		Subdomain:              opts.Subdomain,
		Autoprolong:            boolToInt(opts.Autoprolong),
		OldCertificateID:       opts.OldCertificateID,
		FromProlongation:       opts.FromProlongation,
	}
	var out json.RawMessage
	err := s.c.call(ctx, sslEndpoint, "orderSubmit", p, &out)
	return out, err
}

// boolToInt maps a Go bool to the API's 1/0 integer flag (autoprolong).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
