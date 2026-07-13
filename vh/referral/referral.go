// Package referral groups referral-program operations (endpoint
// /vh/referralProgram): listing the account's referral sites plus the
// add/confirm/remove lifecycle. All calls dispatch through the shared transport.
package referral

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const referralEndpoint = "/vh/referralProgram"

// Service groups referral-program operations (endpoint /vh/referralProgram):
// the referral-site list plus the add/confirm/remove lifecycle.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Site is one referral site as returned by List (method "index").
//
// Doc-vs-reality: the spec's result descriptor documents id/partner_id as
// strings and clientsCount as int, but SpaceWeb quotes numbers inconsistently;
// the recorded example returns id/partner_id quoted ("911"/"3523"). Numeric
// fields decode through flex.Int to tolerate either shape. ConfirmationFile is
// documented "array" yet the example returns a bare object — hence a struct, not
// a slice (see the tolerant-shape convention in CLAUDE.md).
type Site struct {
	ID               string           `json:"id"`
	PartnerID        string           `json:"partner_id"`
	Domain           string           `json:"domain"`
	VerificationCode string           `json:"verification_code"`
	Confirmed        bool             `json:"confirmed"`
	Created          string           `json:"created"` // "YYYY-MM-DD HH:MM:SS"
	ClientsCount     flex.Int         `json:"clientsCount"`
	ConfirmationFile ConfirmationFile `json:"confirmationFile"`
}

// ConfirmationFile is the domain-ownership verification file the client uploads
// to prove control of the site. Content is base64-encoded; Metadata is a free
// nested shape left raw (only needed to render the download).
type ConfirmationFile struct {
	Mimetype string          `json:"mimetype"`
	Metadata json.RawMessage `json:"metadata"`
	Content  string          `json:"content"` // base64
	Name     string          `json:"name"`
}

// FilterInfo is the pagination metadata that accompanies a referral-site list.
// Documented int throughout; decoded through flex.Int for the API's usual
// polymorphic numbers.
type FilterInfo struct {
	Page       flex.Int `json:"page"`
	Limit      flex.Int `json:"limit"`
	TotalCount flex.Int `json:"totalCount"`
}

// List is the object returned by List (method "index"): the referral sites
// under List and the pagination metadata under FilterInfo.
type List struct {
	List       []Site     `json:"list"`
	FilterInfo FilterInfo `json:"filterInfo"`
}

// ListOptions are the optional inputs to List. Page starts at 1; Limit is the
// page size. Both are omitted from the request when zero so the API applies its
// defaults.
type ListOptions struct {
	Page  int
	Limit int
}

type listParams struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}

// List returns the account's referral sites (method "index"). Read-only. The
// result wraps the sites in {"list":[…],"filterInfo":{…}}.
func (s *Service) List(ctx context.Context, opts *ListOptions) (*List, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	var out List
	if err := s.t.Call(ctx, referralEndpoint, "index", listParams{
		Page:  opts.Page,
		Limit: opts.Limit,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Add registers a new referral site for the given domain (method
// "addReferralSite"). MUTATING. Returns on the 1/0 sentinel (1 = success).
func (s *Service) Add(ctx context.Context, domain string) error {
	return s.sentinelAction(ctx, "addReferralSite", map[string]any{"domain": domain})
}

// Confirm attempts to confirm ownership of a referral site (method
// "confirmReferralSite"). MUTATING. id is a Site.ID. Returns on the 1/0 sentinel
// (1 = success).
func (s *Service) Confirm(ctx context.Context, id int) error {
	return s.sentinelAction(ctx, "confirmReferralSite", map[string]any{"id": id})
}

// Remove deletes a referral site (method "removeReferralSite"). MUTATING. id is
// a Site.ID. Returns on the 1/0 sentinel (1 = success).
func (s *Service) Remove(ctx context.Context, id int) error {
	return s.sentinelAction(ctx, "removeReferralSite", map[string]any{"id": id})
}

// sentinelAction runs a /vh/referralProgram method whose success is the integer
// sentinel 1 (addReferralSite/confirmReferralSite/removeReferralSite all answer
// 1 on success, 0 on failure per the spec). A real failure usually surfaces as a
// JSON-RPC error via Call; the non-1 check is defensive. The result is decoded
// via json.RawMessage first so that a shape not yet observed live does not
// silently pass — only a plain 1 is accepted as success.
func (s *Service) sentinelAction(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, referralEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: referral %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: referral %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
