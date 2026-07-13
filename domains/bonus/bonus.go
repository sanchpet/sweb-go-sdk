// Package bonus groups domain-bonus operations (endpoint /domains/bonus):
// read the account's domain bonuses (Index), list purchasable bonus packages
// (GetList), and buy a package (Buy). All calls dispatch through the shared
// transport. It is a sub-package of domains, kept separate so the bonus
// vocabulary does not collide with the domains package's own names.
package bonus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const bonusEndpoint = "/domains/bonus"

// Service groups domain-bonus operations (endpoint /domains/bonus):
// Index/GetList plus the mutating Buy.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Bonus is one domain bonus as returned inside Index's result. Every field
// arrives as a quoted string or null in the recorded example, so the numeric
// ids (bonus_id/id/payment_id/type) decode through flex.Int to tolerate both a
// bare number and a quoted one; the nullable strings stay "" when null.
type Bonus struct {
	ID         flex.Int `json:"id"`          // quoted "106067"
	BonusID    flex.Int `json:"bonus_id"`    // quoted "0"
	BonusTitle string   `json:"bonus_title"` // nullable
	CustomerID string   `json:"customer_id"`
	Domain     string   `json:"domain"` // nullable
	PaymentID  flex.Int `json:"payment_id"`
	TLD        string   `json:"tld"`
	TSClose    string   `json:"ts_close"` // nullable
	TSCreate   string   `json:"ts_create"`
	Type       flex.Int `json:"type"` // quoted "3"
	TypeTitle  string   `json:"type_title"`
	UseType    string   `json:"use_type"` // nullable
	Used       string   `json:"used"`     // "n" | "y"
	ValidTill  string   `json:"valid_till"`
}

// IndexResult is the result of the "index" method: the page of bonuses plus the
// total and unused counts.
//
// Doc-vs-reality: the spec types the result as a bare array, but its own example
// is a one-element array wrapping an object {bonuses,count,unusedCount}. The
// object is the real shape (the content descriptor documents count/unusedCount
// as top-level fields), so Index unwraps a leading single-element array to that
// object; a direct object is also accepted.
type IndexResult struct {
	Bonuses     []Bonus  `json:"bonuses"`
	Count       flex.Int `json:"count"`
	UnusedCount flex.Int `json:"unusedCount"`
}

// IndexOptions are the inputs to Index. Page is required; OrderBy, OrderType and
// Used are optional filters forwarded only when set.
type IndexOptions struct {
	Page      int    // required; page number (0-based)
	OrderBy   string // optional: "valid_till" | "date_used"
	OrderType string // optional: "ASC" | "DESC"
	Used      *bool  // optional: filter by used; nil = no filter
}

// Index returns the account's domain bonuses (method "index"). Read-only.
//
// The API quotes page as a string in the spec example ("0"); it is forwarded as
// given here. The result arrives as a single-element array wrapping the object
// (see IndexResult), which this unwraps.
func (s *Service) Index(ctx context.Context, o IndexOptions) (*IndexResult, error) {
	params := map[string]any{"page": o.Page}
	if o.OrderBy != "" {
		params["orderBy"] = o.OrderBy
	}
	if o.OrderType != "" {
		params["orderType"] = o.OrderType
	}
	if o.Used != nil {
		params["used"] = *o.Used
	}

	var raw json.RawMessage
	if err := s.t.Call(ctx, bonusEndpoint, "index", params, &raw); err != nil {
		return nil, err
	}
	out, err := unwrapIndex(raw)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// unwrapIndex decodes the index result, tolerating both the observed shape (a
// single-element array wrapping the object) and a bare object.
func unwrapIndex(raw json.RawMessage) (*IndexResult, error) {
	var arr []IndexResult
	if err := json.Unmarshal(raw, &arr); err == nil {
		if len(arr) == 0 {
			return &IndexResult{}, nil
		}
		return &arr[0], nil
	}
	var out IndexResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("sweb: bonus index returned unexpected result %s: %w", raw, err)
	}
	return &out, nil
}

// Package is one purchasable domain-bonus package as returned by GetList.
// Numeric fields (id/price/price_old/domains) arrive as bare ints in the
// example but decode through flex.Int per the SDK's polymorphic-number rule.
type Package struct {
	ID             flex.Int `json:"id"`
	Title          string   `json:"title"`
	Descr          string   `json:"descr"`
	Price          flex.Int `json:"price"`
	PriceOld       flex.Int `json:"price_old"`
	Domains        flex.Int `json:"domains"`
	PriceForDomain string   `json:"price_for_domain"` // e.g. "170 ₽ за домен"
}

// GetList returns the packages available to purchase (method "getList").
// Read-only. Takes no parameters.
func (s *Service) GetList(ctx context.Context) ([]Package, error) {
	var out []Package
	if err := s.t.Call(ctx, bonusEndpoint, "getList", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Buy purchases a bonus package by id (method "buy"). MUTATING and billable —
// never exercise against the live API in tests. Returns on the 1/0 sentinel
// (1 = success, 0 = failure per the spec's resultInt descriptor).
//
// The result is decoded via json.RawMessage first so a shape not yet observed
// live (should the API ever answer richer than a bare 1) does not silently
// pass — only a plain 1 is accepted as success.
func (s *Service) Buy(ctx context.Context, bonusID int) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, bonusEndpoint, "buy", map[string]any{"bonusId": bonusID}, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: bonus buy returned unexpected result %s: %w", raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: bonus buy returned %d, want 1 (0 = failure)", int64(out))
	}
	return nil
}
