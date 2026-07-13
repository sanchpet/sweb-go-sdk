// Package partner groups partner-program operations (endpoint
// /vh/partnerProgram): the referral catalog (standard/VIP hosting plans, the
// VPS OS/panel configurator) and order placement, becoming a partner and filling
// requisites, advertising materials, the partner's client roster and per-client
// card / event / finance logs, reward withdrawal, and referral-site statistics.
// All calls dispatch through the shared transport.
package partner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const partnerEndpoint = "/vh/partnerProgram"

// Service groups partner-program operations (endpoint /vh/partnerProgram).
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// ---------------------------------------------------------------------------
// Referral catalog: hosting plans and the VPS OS/panel configurator.
// ---------------------------------------------------------------------------

// PlanPeriod is one billing period of a hosting plan: its length in months, the
// price for that length, whether a free SSL / domain is included, and (for the
// annual period) the eligible domain zones. Money and toggles arrive polymorphic
// across nodes, so numeric fields decode through flex.Int.
type PlanPeriod struct {
	Length     flex.Int `json:"length"`     // months
	Price      flex.Int `json:"price"`      // rubles
	SSL        flex.Int `json:"ssl"`        // 1 = included
	Domain     flex.Int `json:"domain"`     // free domains granted
	DomainZone string   `json:"domainZone"` // eligible zones, e.g. ".ru, .рф"
}

// Plan is one orderable hosting tariff (methods "standardPlans"/"vipPlans"). Id
// arrives quoted; the resource limits arrive as bare ints but decode through
// flex.Int for the usual node-to-node quoting drift.
type Plan struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Disk      flex.Int     `json:"disk"` // GB
	Sites     flex.Int     `json:"sites"`
	DBCount   flex.Int     `json:"dbCount"`
	FTPCount  flex.Int     `json:"ftpCount"`
	MailCount flex.Int     `json:"mailCount"`
	Period    []PlanPeriod `json:"period"`
}

// StandardPlans returns the standard hosting tariffs available to order for a
// referred client (method "standardPlans"). Read-only.
func (s *Service) StandardPlans(ctx context.Context) ([]Plan, error) {
	var out []Plan
	if err := s.t.Call(ctx, partnerEndpoint, "standardPlans", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// VipPlans returns the VIP hosting tariffs available to order (method
// "vipPlans"). Read-only.
func (s *Service) VipPlans(ctx context.Context) ([]Plan, error) {
	var out []Plan
	if err := s.t.Call(ctx, partnerEndpoint, "vipPlans", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OSConfig is the VPS ordering catalog (method "vpsOsConfig"): datacenters,
// category groupings, the OS/panel selection lists, and the OS-panel
// availability matrix. Despite the params documenting no result shape, the
// spec's recorded example is this object.
type OSConfig struct {
	Categories  []OSCategory   `json:"categories"`
	Datacenters []OSDatacenter `json:"datacenters"`
	OSPanel     []OSPanelRule  `json:"osPanel"`
	SelectOS    []OSDistro     `json:"selectOs"`
	SelectPanel []OSPanel      `json:"selectPanel"`
}

// OSCategory is one VPS product grouping (e.g. NVMe, configurator). All numeric
// ids arrive quoted.
type OSCategory struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Prior string `json:"prior"`
}

// OSDatacenter is one datacenter a VPS may be ordered in.
type OSDatacenter struct {
	ID       string `json:"id"`
	Name     string `json:"name"` // slug, e.g. "spb"
	Location string `json:"location"`
	SiteName string `json:"site_name"`
}

// OSPanelRule is one row of the OS/panel availability matrix: which plans a given
// distributive+os+panel combination is offered on, and its resource floor.
type OSPanelRule struct {
	Distributive     string     `json:"distributive"`
	OS               string     `json:"os"`
	Panel            string     `json:"panel"`
	MinRAM           flex.Int   `json:"minRam"`
	MinStorage       flex.Int   `json:"minStorage"`
	AvailablePlanIDs []flex.Int `json:"availablePlanIds"`
}

// OSDistro is one selectable operating system (method "vpsOsConfig", selectOs).
// FullDescription and URL are nullable; PanelType lists the compatible panels.
type OSDistro struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	FullDescription  string   `json:"full_description"` // nullable
	Order            string   `json:"order"`
	OSDistributionID string   `json:"os_distribution_id"`
	PlanID           string   `json:"plan_id"`
	PanelType        []string `json:"panel_type"`
	URL              string   `json:"url"` // nullable
}

// OSPanel is one selectable control panel (method "vpsOsConfig", selectPanel).
// Price arrives as a bare number here but decodes through flex.Float for
// consistency with the money-drift convention; CreationTime and the descriptions
// are nullable.
type OSPanel struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	FullDescription  string     `json:"full_description"` // nullable
	Order            string     `json:"order"`
	OSDistributionID string     `json:"os_distribution_id"`
	PlanID           string     `json:"plan_id"`
	Price            flex.Float `json:"price"`
	OldPrice         flex.Float `json:"old_price"`
	Action           flex.Int   `json:"action"`
	CreationTime     string     `json:"creation_time"` // nullable, e.g. "20-30"
	URL              string     `json:"url"`           // nullable
}

// OSConfig returns the VPS OS/panel/datacenter ordering catalog (method
// "vpsOsConfig"). Read-only.
func (s *Service) OSConfig(ctx context.Context) (*OSConfig, error) {
	var out OSConfig
	if err := s.t.Call(ctx, partnerEndpoint, "vpsOsConfig", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// New-client login check and order placement.
// ---------------------------------------------------------------------------

// CheckLogin reports whether login is free for a new referred user (method
// "checkLogin", true = available). Read-only.
func (s *Service) CheckLogin(ctx context.Context, login string) (bool, error) {
	var out bool
	if err := s.t.Call(ctx, partnerEndpoint, "checkLogin", map[string]any{"login": login}, &out); err != nil {
		return false, err
	}
	return out, nil
}

// OrderResult is the credentials the API returns for a placed referral order
// (methods "createOrderVh"/"createOrderVip"/"createOrderVps"): the new account's
// login and password. Per the spec these are the only documented fields.
type OrderResult struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// StandardOrder is the input to CreateStandardOrder / CreateVIPOrder: a shared
// hosting order for a referred client. TariffID is a Plan.ID; Period is the
// PlanPeriod.Length in months.
type StandardOrder struct {
	Email    string
	TariffID int
	Period   int
	Login    string
	Password string
}

// VPSOrder is the input to CreateVPSOrder. It extends the hosting order with the
// distributive and datacenter selected from OSConfig.
type VPSOrder struct {
	Email          string
	TariffID       int
	DistributiveID int
	Period         int
	Login          string
	Password       string
	Datacenter     int
}

// CreateStandardOrder places a standard-plan hosting order for a referred client
// (method "createOrderVh"). MUTATING and BILLING — never exercise against the
// live API. Returns the new account's login/password.
func (s *Service) CreateStandardOrder(ctx context.Context, o StandardOrder) (*OrderResult, error) {
	return s.createHostingOrder(ctx, "createOrderVh", o)
}

// CreateVIPOrder places a VIP-plan hosting order for a referred client (method
// "createOrderVip"). MUTATING and BILLING — never exercise against the live API.
func (s *Service) CreateVIPOrder(ctx context.Context, o StandardOrder) (*OrderResult, error) {
	return s.createHostingOrder(ctx, "createOrderVip", o)
}

// createHostingOrder is the shared body of the two hosting-order siblings, which
// take the same params and return the same {login,password} shape and differ
// only in the method name (standard vs VIP catalog).
func (s *Service) createHostingOrder(ctx context.Context, method string, o StandardOrder) (*OrderResult, error) {
	params := map[string]any{
		"email":    o.Email,
		"tariffId": o.TariffID,
		"period":   o.Period,
		"login":    o.Login,
		"password": o.Password,
	}
	var out OrderResult
	if err := s.t.Call(ctx, partnerEndpoint, method, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateVPSOrder places a VPS order for a referred client (method
// "createOrderVps"). MUTATING and BILLING — never exercise against the live API.
func (s *Service) CreateVPSOrder(ctx context.Context, o VPSOrder) (*OrderResult, error) {
	params := map[string]any{
		"email":          o.Email,
		"tariffId":       o.TariffID,
		"distributiveId": o.DistributiveID,
		"period":         o.Period,
		"login":          o.Login,
		"password":       o.Password,
		"datacenter":     o.Datacenter,
	}
	var out OrderResult
	if err := s.t.Call(ctx, partnerEndpoint, "createOrderVps", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Becoming a partner and filling requisites.
// ---------------------------------------------------------------------------

// StartPartnership enrolls the account in the partner program without requisites
// (method "startPartnership", the "Стать партнером" button). MUTATING. Returns
// on the 1/0 sentinel.
func (s *Service) StartPartnership(ctx context.Context) error {
	return s.actionOne(ctx, "startPartnership", nil)
}

// Requisites are the partner's legal identifiers (method "fillPartnerRequisites").
type Requisites struct {
	INN        string
	SNILS      string
	RegAddress string
}

// FillRequisites saves the partner's legal requisites (method
// "fillPartnerRequisites"). MUTATING. Returns on the 1/0 sentinel.
func (s *Service) FillRequisites(ctx context.Context, r Requisites) error {
	return s.actionOne(ctx, "fillPartnerRequisites", map[string]any{
		"inn":        r.INN,
		"snils":      r.SNILS,
		"regAddress": r.RegAddress,
	})
}

// ---------------------------------------------------------------------------
// Advertising materials.
// ---------------------------------------------------------------------------

// AdvertMaterialType is one selectable banner type (method
// "getTypesAdvertMaterials"): a display Name and the Value to pass to
// AdvertMaterials as its type filter ("all" selects every type).
type AdvertMaterialType struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AdvertMaterialTypes returns the banner types available to a partner (method
// "getTypesAdvertMaterials"). Read-only.
func (s *Service) AdvertMaterialTypes(ctx context.Context) ([]AdvertMaterialType, error) {
	var out []AdvertMaterialType
	if err := s.t.Call(ctx, partnerEndpoint, "getTypesAdvertMaterials", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AdvertMaterial is one advertising banner (method "getAdvertMaterials"): the
// ready-to-embed HTML Code, its display Sizes, and human-readable file size.
type AdvertMaterial struct {
	Code     string `json:"code"`
	Sizes    string `json:"sizes"`
	FileSize string `json:"filesize"` // human-readable, e.g. "29 КБ"
}

// AdvertMaterials returns the banners of the given type (method
// "getAdvertMaterials"; typ is an AdvertMaterialType.Value, "all" for every
// type). Read-only.
func (s *Service) AdvertMaterials(ctx context.Context, typ string) ([]AdvertMaterial, error) {
	var out []AdvertMaterial
	if err := s.t.Call(ctx, partnerEndpoint, "getAdvertMaterials", map[string]any{"type": typ}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Partner client roster, card, and per-client logs.
// ---------------------------------------------------------------------------

// ClientsList is the paginated partner-client roster (method
// "getPartnerClientsList").
type ClientsList struct {
	FilterInfo ClientsFilter `json:"filterInfo"`
	List       []Client      `json:"list"`
}

// ClientsFilter is the pagination/filter echo of a ClientsList response.
type ClientsFilter struct {
	FilterStatus flex.Int `json:"filterStatus"`
	Page         flex.Int `json:"page"`
	PerPage      flex.Int `json:"perPage"`
	TotalCount   flex.Int `json:"totalCount"`
}

// Client is one referred client in the roster. Login arrives masked; PaysAll /
// PaysMonth are accrued reward amounts and decode through flex.Int for the
// money-drift convention.
type Client struct {
	ID        string   `json:"id"`
	CustLogin string   `json:"cust_login"` // masked, e.g. "in****ly82"
	Plan      []string `json:"plan"`
	Status    flex.Int `json:"status"`
	Type      flex.Int `json:"type"`
	IsPromo   bool     `json:"is_promo"`
	PaysAll   flex.Int `json:"pays_all"`
	PaysMonth flex.Int `json:"pays_month"`
	TS        string   `json:"ts"` // registration date, e.g. "14.03.2023"
}

// ClientsList returns the partner's client roster, filtered by status and paged
// (method "getPartnerClientsList", filterStatus -1 = all). Read-only.
func (s *Service) ClientsList(ctx context.Context, filterStatus, page int) (*ClientsList, error) {
	var out ClientsList
	params := map[string]any{"filterStatus": filterStatus, "page": page}
	if err := s.t.Call(ctx, partnerEndpoint, "getPartnerClientsList", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClientCard is the detailed view of one referred client (method
// "getPartnerClientCard"). AmountsPeriod / AmountsLastMonth are accrued rewards.
type ClientCard struct {
	ID               string   `json:"id"`
	Login            string   `json:"login"` // masked
	PlanName         []string `json:"planName"`
	Status           flex.Int `json:"status"`
	Type             flex.Int `json:"type"`
	Attraction       string   `json:"attraction"` // referral code that attracted the client
	Comment          string   `json:"comment"`
	ContractNumber   string   `json:"contractNumber"`
	RegDate          string   `json:"regDate"`
	AmountsPeriod    flex.Int `json:"amountsPeriod"`
	AmountsLastMonth flex.Int `json:"amountsLastMonth"`
}

// ClientCard returns one client's detailed card (method "getPartnerClientCard").
// Read-only.
func (s *Service) ClientCard(ctx context.Context, clientID string) (*ClientCard, error) {
	var out ClientCard
	if err := s.t.Call(ctx, partnerEndpoint, "getPartnerClientCard", map[string]any{"clientId": clientID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveClientComment updates the partner's private comment on a client (method
// "savePartnerClientComment"). MUTATING. Returns on the 1/0 sentinel.
func (s *Service) SaveClientComment(ctx context.Context, clientID, comment string) error {
	return s.actionOne(ctx, "savePartnerClientComment", map[string]any{
		"clientId": clientID,
		"comment":  comment,
	})
}

// LogPage is the pagination echo shared by the event and finance logs.
type LogPage struct {
	Page       flex.Int `json:"page"`
	PerPage    flex.Int `json:"perPage"`
	TotalCount flex.Int `json:"totalCount"`
}

// EventLog is the paginated client-event log (method
// "getPartnerClientLogEvents").
type EventLog struct {
	FilterInfo LogPage    `json:"filterInfo"`
	List       []EventRow `json:"list"`
}

// EventRow is one client-event log entry.
type EventRow struct {
	EventName string `json:"eventName"`
	TS        string `json:"ts"`
}

// ClientLogEvents returns the paginated client-event log (method
// "getPartnerClientLogEvents"). Read-only.
func (s *Service) ClientLogEvents(ctx context.Context, page int) (*EventLog, error) {
	var out EventLog
	if err := s.t.Call(ctx, partnerEndpoint, "getPartnerClientLogEvents", map[string]any{"page": page}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FinanceLog is the paginated client-finance log (method
// "getPartnerClientLogFinance").
type FinanceLog struct {
	FilterInfo LogPage      `json:"filterInfo"`
	List       []FinanceRow `json:"list"`
}

// FinanceRow is one finance-log entry. Withdrawal / Payment / Lock are amounts
// that arrive EITHER as a number or as an empty string ("") when not applicable
// to the row, so they decode through flex.Int (empty string → 0).
type FinanceRow struct {
	EventName  string   `json:"eventName"`
	TS         string   `json:"ts"`
	Withdrawal flex.Int `json:"withdrawal"`
	Payment    flex.Int `json:"payment"`
	Lock       flex.Int `json:"lock"`
}

// ClientLogFinance returns the paginated client-finance log (method
// "getPartnerClientLogFinance"). Read-only.
func (s *Service) ClientLogFinance(ctx context.Context, page int) (*FinanceLog, error) {
	var out FinanceLog
	if err := s.t.Call(ctx, partnerEndpoint, "getPartnerClientLogFinance", map[string]any{"page": page}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Reward withdrawal.
// ---------------------------------------------------------------------------

// WithdrawalRequisites is the saved withdrawal setup (method
// "getRequisitesWithdrawal"): the current Balance, the available payout methods
// keyed by name, and the previously-entered bank requisites.
type WithdrawalRequisites struct {
	Balance         flex.Float               `json:"balance"`
	OrderTypes      map[string]WithdrawalWay `json:"orderTypes"`
	ReqUserName     string                   `json:"reqUserName"`
	ReqCheckAccount string                   `json:"reqCheckAccount"`
	ReqBankName     string                   `json:"reqBankName"`
	ReqBIC          string                   `json:"reqBIC"`
	ReqCorrAccount  string                   `json:"reqCorrAccount"`
}

// WithdrawalWay is one available payout method (an orderTypes value). Type is the
// numeric order type to pass to SendWithdrawalOrder; MaximumMonthAmount is absent
// for methods without a monthly cap (→ 0 via flex.Int).
type WithdrawalWay struct {
	Type               string   `json:"type"`
	TypeName           string   `json:"typeName"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Enable             bool     `json:"enable"`
	Error              string   `json:"error"`
	Sort               flex.Int `json:"sort"`
	MinimumAmount      flex.Int `json:"minimumAmount"`
	MaximumMonthAmount flex.Int `json:"maximumMonthAmount"`
}

// WithdrawalRequisites returns the saved payout methods and bank requisites
// (method "getRequisitesWithdrawal"; the result depends on the account type).
// Read-only.
func (s *Service) WithdrawalRequisites(ctx context.Context) (*WithdrawalRequisites, error) {
	var out WithdrawalRequisites
	if err := s.t.Call(ctx, partnerEndpoint, "getRequisitesWithdrawal", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WithdrawalOrder is the input to SendWithdrawalOrder. OrderType is a
// WithdrawalWay.Type; the Req* bank fields are required only for a bank-account
// payout (order type 1) and may be empty otherwise.
type WithdrawalOrder struct {
	OrderType       int
	CountMoney      float64
	ReqUserName     string
	ReqPayPurpose   string
	ReqCheckAccount string
	ReqBankName     string
	ReqBIC          string
	ReqCorrAccount  string
}

// SendWithdrawalOrder requests a payout of accrued partner reward (method
// "sendWithdrawalOrder"). MUTATING and moves real money — never exercise against
// the live API. Returns on the 1/0 sentinel.
func (s *Service) SendWithdrawalOrder(ctx context.Context, o WithdrawalOrder) error {
	return s.actionOne(ctx, "sendWithdrawalOrder", map[string]any{
		"orderType":       o.OrderType,
		"countMoney":      o.CountMoney,
		"reqUserName":     o.ReqUserName,
		"reqPayPurpose":   o.ReqPayPurpose,
		"reqCheckAccount": o.ReqCheckAccount,
		"reqBankName":     o.ReqBankName,
		"reqBIC":          o.ReqBIC,
		"reqCorrAccount":  o.ReqCorrAccount,
	})
}

// ---------------------------------------------------------------------------
// Referral statistics.
// ---------------------------------------------------------------------------

// StatFile is a base64-encoded export bundled with a statistics response: the
// CSV and PNG chart of the same series. Content is the base64 payload; Mimetype
// records its encoding (e.g. "application/csv;base64").
type StatFile struct {
	Name     string          `json:"name"`
	Mimetype string          `json:"mimetype"`
	Content  string          `json:"content"` // base64
	Metadata json.RawMessage `json:"metadata"`
}

// Statistic is the per-referral-site statistics (method "getStatistic"): a CSV
// export, a PNG chart, and the raw daily Data series. Each Data row is a
// positional [date, hits, orders] tuple (a mixed-type JSON array), kept as raw
// cells because the API returns it positionally rather than keyed.
type Statistic struct {
	CSV  StatFile            `json:"csv"`
	PNG  StatFile            `json:"png"`
	Data [][]json.RawMessage `json:"data"`
}

// GetStatistic returns the statistics for one referral site over the given month
// (method "getStatistic"; month is 1..12). Read-only.
func (s *Service) GetStatistic(ctx context.Context, site string, year, month int) (*Statistic, error) {
	var out Statistic
	params := map[string]any{"site": site, "year": year, "month": month}
	if err := s.t.Call(ctx, partnerEndpoint, "getStatistic", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// LinkStatistic is the referral-link statistics (method "getLinkStatistics"),
// the same shape as Statistic but for the partner's referral link rather than a
// site; each Data row is a positional [date, clicks, regs, orders] tuple.
type LinkStatistic struct {
	CSV  StatFile            `json:"csv"`
	PNG  StatFile            `json:"png"`
	Data [][]json.RawMessage `json:"data"`
}

// GetLinkStatistics returns the referral-link statistics over the given month
// (method "getLinkStatistics"; month is 1..12). Read-only.
func (s *Service) GetLinkStatistics(ctx context.Context, year, month int) (*LinkStatistic, error) {
	var out LinkStatistic
	params := map[string]any{"year": year, "month": month}
	if err := s.t.Call(ctx, partnerEndpoint, "getLinkStatistics", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Shared sentinel helper.
// ---------------------------------------------------------------------------

// actionOne runs a /vh/partnerProgram method whose success is the documented
// integer sentinel 1 (startPartnership, fillPartnerRequisites,
// savePartnerClientComment, sendWithdrawalOrder all answer 1 on success, 0 on
// failure per the spec's resultInt descriptor "1 - успешно, 0 - ошибка"). The
// result is decoded via json.RawMessage first so that a shape not yet observed
// live does not silently pass — only a plain 1 is accepted as success. A real
// failure usually surfaces as a JSON-RPC error (*apierr.Error) via the
// transport; the non-1 check is defensive.
func (s *Service) actionOne(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, partnerEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: partner %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: partner %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
