// Package pay groups billing operations (endpoint /pay): the account balance
// and status (index/getBalance), autopayment/deferment state, payment
// recommendations and upcoming payments, the runway to blocking
// (getRemainsDate/getRemainsDays), and the reserves currently holding funds.
// All calls dispatch through the shared transport. Every method here is
// read-only except ChangeDeferment, which toggles the payment deferment.
package pay

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const payEndpoint = "/pay"

// Service groups billing operations (endpoint /pay).
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Balance is the account's money broken down by pocket (method "getBalance",
// and the balance portion of "index"). Every monetary field decodes through
// flex.Float: the API quotes these int-or-float ("1492.0000", 1492, or a bare
// 1544) inconsistently across accounts. VATBalance is a map keyed by VAT-type
// id to a (string-quoted) amount, present only in multiple-balance mode.
type Balance struct {
	RealBalance            flex.Float `json:"real_balance"`  // real money
	BonusBalance           flex.Float `json:"bonus_balance"` // bonus money
	CloudBalance           flex.Float `json:"cloud_balance"` // funds for cloud services
	OtherBalance           flex.Float `json:"other_balance"` // funds for other services
	CloudBalanceView       flex.Float `json:"cloud_balance_view"`
	OtherBalanceView       flex.Float `json:"other_balance_view"`
	CreditBalance          flex.Float `json:"credit_balance"`       // total debt
	CreditCloudBalance     flex.Float `json:"credit_cloud_balance"` // debt on cloud services
	CreditOtherBalance     flex.Float `json:"credit_other_balance"` // debt on other services
	Type                   flex.Int   `json:"type"`                 // billing type
	MultipleBalanceEnabled bool       `json:"multiple_balance_enabled"`
	// VATBalance maps VAT-type id → amount (quoted, e.g. "1492.0000"); the
	// getBalance example carries it though the descriptor omits it. Amounts stay
	// strings: they are per-VAT sub-balances the caller rarely arithmetics on.
	VATBalance map[string]string `json:"vat_balance,omitempty"`
}

// Deferment is the payment-deferment state carried in the index result:
// whether to surface the deferment offer and how many days it grants.
type Deferment struct {
	Show  bool     `json:"show"`
	Value flex.Int `json:"value"` // days of deferment
}

// BlockInfo is the countdown to account blocking carried in the index result.
//
// Doc-vs-reality: the spec's index descriptor types blockInfo as a string
// ("Причина блокировки"), but the recorded example returns an object with the
// days remaining. Typed against the observed object; fields are omitempty so an
// empty/absent value decodes cleanly.
type BlockInfo struct {
	Days     flex.Int `json:"days,omitempty"`
	DaysDate string   `json:"days_date,omitempty"`
	DaysWord string   `json:"days_word,omitempty"`
}

// Account is the full billing snapshot the "index" method returns: the nested
// balance plus autopayment/deferment state, block countdown, and account
// status.
//
// Doc-vs-reality: the spec's index descriptor lists the balance fields flat at
// the top level, but the recorded example nests them under "balance" and adds
// blockInfo/blockedMoney/domainBonuses/edgeDate. Typed against the example.
type Account struct {
	Balance           Balance  `json:"balance"`
	AutoPaymentEnable flex.Int `json:"auto_payment_enable"` // 1 = autopayment connected
	// IsAutopaymentEnable: 1 = autopayments are available on the account. Kept as
	// flex.Int here (the raw 1/0) rather than bool; IsAutopaymentEnable() exposes
	// the dedicated method's boolean answer.
	IsAutopaymentEnable flex.Int   `json:"isAutopaymentEnable"`
	DomainBonuses       flex.Int   `json:"domainBonuses"`
	Status              string     `json:"status"`    // account status, e.g. "active"
	BlockInfo           BlockInfo  `json:"blockInfo"` // countdown to blocking
	BlockedMoney        flex.Float `json:"blockedMoney"`
	Deferment           Deferment  `json:"deferment"`
	EdgeDate            string     `json:"edgeDate"` // date from which documents are available
}

// Index returns the account's billing snapshot (method "index"). Read-only.
//
// Doc-vs-reality: the spec types the result as an array, but the live API
// returns a bare account object; this decodes it directly.
func (s *Service) Index(ctx context.Context) (*Account, error) {
	var out Account
	if err := s.t.Call(ctx, payEndpoint, "index", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// IsAutopaymentEnable reports whether autopayment is enabled on the account
// (method "isAutopaymentEnable"). Read-only. The spec documents a boolean
// result; boolAnswer also tolerates the 1/0 form the sibling index field uses.
func (s *Service) IsAutopaymentEnable(ctx context.Context) (bool, error) {
	return s.boolAnswer(ctx, "isAutopaymentEnable")
}

// Recommendation is one service recommended for payment inside a
// GetPayRecommendations bucket. Cost decodes through flex.Float (int-or-float).
type Recommendation struct {
	ID   flex.Int   `json:"id"`
	Name string     `json:"name"`
	Date string     `json:"date"` // e.g. "не зарегистрирован" or a date string
	Cost flex.Float `json:"cost"`
}

// Recommendations is the payment-recommendation bundle (method
// "getPayRecommendations"). RecommendedForPay lists services to pay for;
// RecommendedForPayBalance lists top-up recommendations (present only when the
// addBalanceRecommendations param is true). The domain-bonus counters are
// carried alongside.
type Recommendations struct {
	RecommendedForPay        []Recommendation `json:"recommended_for_pay"`
	RecommendedForPayBalance []Recommendation `json:"recommended_for_pay_balance"`
	ExistDomainBonus         flex.Int         `json:"exist_domain_bonus"`
	TotalFRPBalance          flex.Float       `json:"total_frp_balance"` // real balance
	TariffDomainBonus        flex.Int         `json:"tariff_domain_bonus"`
	TariffDomainBonusTLD     flex.Int         `json:"tariff_domain_bonus_tld"`
	// DomainBonusesByTLD maps a TLD → bonus count; the example carries {"any":0}.
	DomainBonusesByTLD map[string]flex.Int `json:"domain_bonuses_by_tld,omitempty"`
}

// GetPayRecommendations returns the payment recommendations (method
// "getPayRecommendations"). Read-only. addBalanceRecommendations toggles
// whether balance top-up recommendations are included.
//
// Doc-vs-reality: the spec types the result as an array, but the live API
// returns a bare recommendation-bundle object; this decodes it directly.
func (s *Service) GetPayRecommendations(ctx context.Context, addBalanceRecommendations bool) (*Recommendations, error) {
	params := map[string]any{"addBalanceRecommendations": addBalanceRecommendations}
	var out Recommendations
	if err := s.t.Call(ctx, payEndpoint, "getPayRecommendations", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRecommendationTotalCost returns the full amount recommended for payment
// (method "getRecommendationTotalCost"). Read-only. flex.Float tolerates the
// int-or-float money form.
func (s *Service) GetRecommendationTotalCost(ctx context.Context) (flex.Float, error) {
	var out flex.Float
	if err := s.t.Call(ctx, payEndpoint, "getRecommendationTotalCost", nil, &out); err != nil {
		return 0, err
	}
	return out, nil
}

// UpcomingPayment is one upcoming payment / recommendation for a VH user
// (method "getUpcomingPaymentsVh"). The recorded example is heterogeneous: some
// entries carry base_cost/bonus_cost/tld (domain regs), others carry a plain
// type (antivirus). Every monetary field decodes through flex.Float, and
// action/checkbox_available/vat_value through flex.Int, since the API quotes
// them as int-or-string across entry kinds. Fields not present on a given entry
// stay at their zero value (omitempty on marshal).
type UpcomingPayment struct {
	ID                flex.Int   `json:"id"`
	Name              string     `json:"name"`
	Date              string     `json:"date"`
	Cost              flex.Float `json:"cost"`
	CostStr           string     `json:"cost_str"`
	BaseCost          flex.Float `json:"base_cost,omitempty"`
	BaseCostStr       string     `json:"base_cost_str,omitempty"`
	BonusCost         flex.Float `json:"bonus_cost,omitempty"`
	BonusCostStr      string     `json:"bonus_cost_str,omitempty"`
	Action            flex.Int   `json:"action"`
	CheckboxAvailable flex.Int   `json:"checkbox_available"`
	ReadyForBonus     flex.Int   `json:"ready_for_bonus,omitempty"`
	EntityType        string     `json:"entity_type,omitempty"`
	Type              string     `json:"type,omitempty"`
	ServiceID         string     `json:"service_id"`
	TLD               string     `json:"tld,omitempty"`
	VATType           string     `json:"vat_type"`
	VATName           string     `json:"vat_name"`
	VATValue          flex.Int   `json:"vat_value"`
}

// GetUpcomingPaymentsVh returns the upcoming payments for a VH user (method
// "getUpcomingPaymentsVh"). Read-only. The result is a genuine array.
func (s *Service) GetUpcomingPaymentsVh(ctx context.Context) ([]UpcomingPayment, error) {
	var out []UpcomingPayment
	if err := s.t.Call(ctx, payEndpoint, "getUpcomingPaymentsVh", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ChangeDeferment turns the payment deferment on or off (method
// "changeDeferment"). MUTATING — the only write on this endpoint.
//
// Doc-vs-reality: the spec documents the result via the integer 1/0 sentinel
// (1 = success), and the example returns 1. Decoded via json.RawMessage first
// so a shape not yet observed live does not silently pass — only a bare 1 (or
// its boolean twin true, defensively) is accepted as success.
func (s *Service) ChangeDeferment(ctx context.Context, turnOn bool) error {
	params := map[string]any{"turnOn": turnOn}
	var raw json.RawMessage
	if err := s.t.Call(ctx, payEndpoint, "changeDeferment", params, &raw); err != nil {
		return err
	}
	switch string(raw) {
	case "1", "true":
		return nil
	case "0", "false":
		return fmt.Errorf("sweb: pay changeDeferment returned 0, want 1 (0 = failure)")
	default:
		var out flex.Int
		if err := json.Unmarshal(raw, &out); err != nil {
			return fmt.Errorf("sweb: pay changeDeferment returned unexpected result %s: %w", raw, err)
		}
		if out != 1 {
			return fmt.Errorf("sweb: pay changeDeferment returned %d, want 1 (0 = failure)", int64(out))
		}
		return nil
	}
}

// GetRemainsDate returns the date the account's funds run out (method
// "getRemainsDate"), formatted "d.m.Y" (e.g. "01.10.2023"). Read-only.
func (s *Service) GetRemainsDate(ctx context.Context) (string, error) {
	var out string
	if err := s.t.Call(ctx, payEndpoint, "getRemainsDate", nil, &out); err != nil {
		return "", err
	}
	return out, nil
}

// GetRemainsDays returns the number of days until the account is blocked
// (method "getRemainsDays"). Read-only. flex.Int tolerates the int-or-string
// form.
func (s *Service) GetRemainsDays(ctx context.Context) (flex.Int, error) {
	var out flex.Int
	if err := s.t.Call(ctx, payEndpoint, "getRemainsDays", nil, &out); err != nil {
		return 0, err
	}
	return out, nil
}

// GetBalance returns the account balance broken down by pocket (method
// "getBalance"). Read-only.
//
// Doc-vs-reality: the spec types the result as an array, but the live API
// returns a bare balance object; this decodes it directly.
func (s *Service) GetBalance(ctx context.Context) (*Balance, error) {
	var out Balance
	if err := s.t.Call(ctx, payEndpoint, "getBalance", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReserveInfo is the detail block of a blocked-funds reserve: the title and,
// when present, the end date of the block (nullable).
type ReserveInfo struct {
	Title   string `json:"title"`
	EndDate string `json:"endDate"` // nullable → ""
}

// Reserve is one active reserve currently holding funds (method
// "getActiveReserves"). Charge (the blocked amount) decodes through flex.Float:
// the example returns it both as a bare float (2368.74) and quoted ("3120.00").
type Reserve struct {
	Charge      flex.Float  `json:"charge"`       // blocked amount
	Type        string      `json:"type"`         // service type, e.g. "tariff"
	BalanceType string      `json:"balance_type"` // tax/balance type, e.g. "cloud"
	Info        ReserveInfo `json:"info"`
}

// GetActiveReserves returns the reserves currently holding funds (method
// "getActiveReserves"). Read-only. The result is a genuine array.
func (s *Service) GetActiveReserves(ctx context.Context) ([]Reserve, error) {
	var out []Reserve
	if err := s.t.Call(ctx, payEndpoint, "getActiveReserves", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// boolAnswer runs a read-only /pay method whose answer is a boolean, tolerating
// both the JSON boolean the spec documents and the 1/0 integer form the API
// uses for the sibling autopayment fields (0 is a valid "no", not a failure).
func (s *Service) boolAnswer(ctx context.Context, method string) (bool, error) {
	var raw json.RawMessage
	if err := s.t.Call(ctx, payEndpoint, method, nil, &raw); err != nil {
		return false, err
	}
	switch string(raw) {
	case "true", "1", `"1"`:
		return true, nil
	case "false", "0", `"0"`, "null", "":
		return false, nil
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return false, fmt.Errorf("sweb: pay %s returned unexpected result %s: %w", method, raw, err)
	}
	return out == 1, nil
}
