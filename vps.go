package sweb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// VPSService groups VPS operations. All calls hit the /vps endpoint with a
// JSON-RPC method.
type VPSService struct{ c *Client }

const vpsEndpoint = "/vps"

// SSHKeyRef is an SSH key attached to a VPS.
type SSHKeyRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// VPSFeatures are capability flags for a VPS / plan.
type VPSFeatures struct {
	AllowBackups        bool `json:"allowBackups"`
	AllowLocalNetwork   bool `json:"allowLocalNetwork"`
	AllowCustomImage    bool `json:"allowCustomImage"`
	AllowDdosProtection bool `json:"allowDdosProtection"`
	MaxIPCount          int  `json:"maxIpCount"`
	AllowConfigurator   bool `json:"allowConfigurator"`
	AllowAccess         bool `json:"allowAccess"`
	AllowDiskConnection bool `json:"allowDiskConnection"`
	AllowAutoBackups    bool `json:"allowAutoBackups"`
	AllowClone          bool `json:"allowClone"`
}

// ISPInfo is control-panel (ISP license) info attached to a VPS ("isp" in the
// index response).
type ISPInfo struct {
	LicenseType  string    `json:"license_type"`
	IP           string    `json:"ip"`
	ActiveUntil  string    `json:"active_until"`
	Price        FlexFloat `json:"price"`
	Link         string    `json:"link"`
	IsBlocked    FlexInt   `json:"is_blocked"`
	CreationTime string    `json:"creation_time"`
}

// VPS is a VPS instance as returned by List (method "index").
//
// Types are reconciled against a real API response: SpaceWeb returns many
// numeric fields either as numbers or quoted strings (FlexInt/FlexFloat handle
// both), and nullable fields (parent_plan_id, local_*) as JSON null.
type VPS struct {
	BillingID      string            `json:"billingId"`
	Name           string            `json:"name"` // user-facing alias
	UID            string            `json:"uid"`  // stable unique id
	PlanID         FlexInt           `json:"plan_id"`
	PlanName       string            `json:"plan_name"`
	ParentPlanID   FlexInt           `json:"parent_plan_id"` // nullable
	PlanPrice      FlexFloat         `json:"plan_price"`     // money, may be fractional
	CPU            FlexInt           `json:"cpu"`
	RAM            FlexInt           `json:"ram"`  // MB; API may quote it ("1024")
	Disk           string            `json:"disk"` // localized human size, e.g. "10 ГБ"
	BlockUI        FlexInt           `json:"blockUi"`
	Active         FlexInt           `json:"active"`
	OSDistribution string            `json:"os_distribution"`
	OSDistrID      FlexInt           `json:"os_distr_id"`
	Category       string            `json:"category"`
	TSCreate       string            `json:"ts_create"`
	MAC            string            `json:"mac"`
	IP             string            `json:"ip"`
	LocalIP        string            `json:"local_ip"`  // nullable
	LocalMAC       string            `json:"local_mac"` // nullable
	LocalMask      string            `json:"local_mask"`
	CurrentAction  string            `json:"current_action"`
	IsRunning      FlexInt           `json:"is_running"`
	ISP            []ISPInfo         `json:"isp"`
	ExtIPs         []json.RawMessage `json:"ext_ips"` // TODO: type once a populated example exists (doc: array of IP objects)
	IsTest         FlexInt           `json:"is_test"`
	IsNew          bool              `json:"is_new"`
	Datacenter     string            `json:"datacenter"`
	OrderedIPCount FlexInt           `json:"ordered_ip_count"`
	ProtectedIPs   []string          `json:"protected_ips"`

	// Not part of the documented index response — present in an earlier recorded
	// response and still consumed by the Terraform provider's computed mapping
	// (disk size, datacenter id). Kept until the provider reconcile (variant B);
	// index does not populate these, so they decode to zero here.
	DiskGB         int         `json:"diskGb"`
	DatacenterID   string      `json:"datacenter_id"`
	PasswordAccess bool        `json:"password_access"`
	SSHKeys        []SSHKeyRef `json:"ssh_keys"`
	Features       VPSFeatures `json:"features"`
}

// List returns all VPS instances (method "index").
func (s *VPSService) List(ctx context.Context) ([]VPS, error) {
	var out []VPS
	err := s.c.call(ctx, vpsEndpoint, "index", nil, &out)
	return out, err
}

// CreateVPSRequest holds the parameters for Create (method "create"). Use
// AvailableConfig to resolve the numeric IDs (plan, distributive, datacenter).
type CreateVPSRequest struct {
	DistributiveID      int    `json:"distributiveId"`
	VPSPlanID           int    `json:"vpsPlanId"`
	Datacenter          int    `json:"datacenter"`
	Alias               string `json:"alias"`
	SSHKey              string `json:"sshKey"`
	MonitoringPlanID    int    `json:"monitoringPlanId,omitempty"`
	MonitoringContactID int    `json:"monitoringContactId,omitempty"`
	IPCount             int    `json:"ipCount,omitempty"`
	ProtectedIPs        []int  `json:"protectedIps,omitempty"`
}

// Create provisions a new VPS (method "create").
//
// The result shape is intentionally left raw: "create" mutates (and bills), so
// it was not exercised during the Evidence phase. Type it once a real create
// response is recorded.
func (s *VPSService) Create(ctx context.Context, req CreateVPSRequest) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, vpsEndpoint, "create", req, &out)
	return out, err
}

// Remove deletes a VPS (method "remove"). billingID is the service identifier
// (format "login_vps_N"), as returned in VPS.BillingID by List.
//
// This is destructive — it cancels the VPS. The result shape is left raw
// pending a recorded response.
func (s *VPSService) Remove(ctx context.Context, billingID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, vpsEndpoint, "remove", map[string]string{"billingId": billingID}, &out)
	return out, err
}

// Rename changes a VPS's user-facing name/alias (method "rename"). billingID is
// the service identifier ("login_vps_N"); alias is the new name. This is an
// in-place label change — it does not reprovision or bill. The API returns 1 on
// success; a JSON-RPC error surfaces as *Error.
func (s *VPSService) Rename(ctx context.Context, billingID, alias string) error {
	var out json.RawMessage
	return s.c.call(ctx, vpsEndpoint, "rename", map[string]string{
		"billingId": billingID,
		"alias":     alias,
	}, &out)
}

// ChangePlan changes a VPS's tariff plan in place (method "changePlan") — a
// resize without reprovisioning. billingID is the service id ("login_vps_N");
// vpsPlanID is a plan id (from AvailableConfig, or GetConstructorPlanID for a
// custom configuration). The API returns 1 on success, 0 on failure (surfaced
// here as an error).
//
// NOTE: the parameter is "planId" (per the docs' parameter table). The docs'
// EXAMPLE instead shows "vpsPlanId" (as in Create), but the API rejects that with
// -32602 "Invalid method parameter(s)" — confirmed live. NOTE: the resize is
// asynchronous — poll List / current_action until it settles.
func (s *VPSService) ChangePlan(ctx context.Context, billingID string, vpsPlanID int) error {
	var out int
	if err := s.c.call(ctx, vpsEndpoint, "changePlan", map[string]any{
		"billingId": billingID,
		"planId":    vpsPlanID,
	}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: changePlan returned %d, want 1 (0 = failure)", out)
	}
	return nil
}

// PowerOn boots a stopped VPS (method "powerOn"). The power change is
// asynchronous: the API accepts the request (returns 1) and the machine settles
// over the following seconds — poll IsRunning, or WaitForIdle for the
// current_action to clear. A JSON-RPC error (or a 0 result) surfaces as an error.
func (s *VPSService) PowerOn(ctx context.Context, billingID string) error {
	return s.powerAction(ctx, "powerOn", billingID)
}

// PowerOff shuts a running VPS down (method "powerOff"). Asynchronous — see PowerOn.
func (s *VPSService) PowerOff(ctx context.Context, billingID string) error {
	return s.powerAction(ctx, "powerOff", billingID)
}

// Reboot restarts a VPS (method "reboot"). Asynchronous — see PowerOn.
func (s *VPSService) Reboot(ctx context.Context, billingID string) error {
	return s.powerAction(ctx, "reboot", billingID)
}

// powerAction issues a power method against the VPS endpoint. Like the other
// action methods, the API answers 1 on success and 0 on failure (quoted or bare
// — hence FlexInt).
func (s *VPSService) powerAction(ctx context.Context, method, billingID string) error {
	var out FlexInt
	if err := s.c.call(ctx, vpsEndpoint, method, map[string]string{"billingId": billingID}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}

// IsRunning reports whether the VPS is powered on (method "isRunning"). Read-only.
// Note List already carries VPS.IsRunning for every node; this is the cheaper
// single-VPS query when only the power state is needed.
func (s *VPSService) IsRunning(ctx context.Context, billingID string) (bool, error) {
	var out FlexInt
	if err := s.c.call(ctx, vpsEndpoint, "isRunning", map[string]string{"billingId": billingID}, &out); err != nil {
		return false, err
	}
	return out == 1, nil
}

// GetConstructorPlanID resolves a custom ("configurator") plan ID for the given
// resources via the "getConstructorPlanId" method. ram and disk are in GB;
// categoryID is a catalog category id (see AvailableConfig.Categories). This is
// read-only — it neither creates nor bills; feed the result to Create as the
// VPSPlanID.
func (s *VPSService) GetConstructorPlanID(ctx context.Context, cpuCores, ramGB, diskGB, categoryID int) (int, error) {
	var raw json.RawMessage
	err := s.c.call(ctx, vpsEndpoint, "getConstructorPlanId", map[string]int{
		"cpu_cores":   cpuCores,
		"ram":         ramGB,
		"volume_disk": diskGB,
		"category_id": categoryID,
	}, &raw)
	if err != nil {
		return 0, err
	}
	// The API may return the id as a bare number or a quoted string.
	id, convErr := strconv.Atoi(strings.Trim(string(raw), `" `))
	if convErr != nil {
		return 0, fmt.Errorf("sweb: unexpected getConstructorPlanId result %s: %w", raw, convErr)
	}
	// Guard: SpaceWeb's resolver can map an out-of-range configuration onto a
	// sold-out / archived plan (e.g. 1cpu/1GB/10GB → the "Промо" plan), which then
	// fails create with a cryptic "-32500 Тариф распродан". If the resolved id is a
	// KNOWN sold-out plan, surface it here. Best-effort: the catalog lists stock
	// plans, so a genuine configurator plan is absent from it and left alone, and a
	// catalog fetch error doesn't block resolution (create would still surface it).
	if cfg, cerr := s.AvailableConfig(ctx); cerr == nil {
		for _, p := range cfg.VPSPlans {
			if p.ID == id && p.SoldOut {
				return 0, fmt.Errorf("sweb: configurator %dcpu/%dGB/%dGB resolved to plan %d (%q), which is sold out — the requested resources are likely below the orderable configurator range", cpuCores, ramGB, diskGB, id, p.Name)
			}
		}
	}
	return id, nil
}

// FirstOrderInfo describes the account's promotional first VPS order (method
// "getFirstOrderInfo"), used by the onboarding / clear-first-order flow.
//
// Doc caveats reconciled against a real response: cpu_cores/ram come as quoted
// strings; pay_period is a month COUNT (not a price, despite the doc); and the
// two *_with_stock descriptions are swapped in the docs — field names + values
// are authoritative.
type FirstOrderInfo struct {
	Plan                    string    `json:"plan"`
	OS                      string    `json:"os"`
	Panel                   string    `json:"panel"`
	CPUCores                FlexInt   `json:"cpu_cores"`
	RAM                     FlexInt   `json:"ram"`         // MB
	VolumeDisk              string    `json:"volume_disk"` // localized, e.g. "10 ГБ"
	PricePerMonth           FlexFloat `json:"price_per_month"`
	PayPeriod               FlexInt   `json:"pay_period"` // months
	PriceForPeriodWithStock FlexFloat `json:"price_for_period_with_stock"`
	PricePerMonthWithStock  FlexFloat `json:"price_per_month_with_stock"`
	Promocode               string    `json:"promocode"` // empty when null
	ClearAvailable          bool      `json:"clearAvailable"`
	PlanIsConstructor       bool      `json:"plan_is_constructor"`
	IPCount                 FlexInt   `json:"ipCount"`
	ProtectedIPs            []FlexInt `json:"protectedIps"`
}

// GetFirstOrderInfo returns the account's first-order info (method
// "getFirstOrderInfo"), or nil if there is no first order.
//
// The API double-wraps the payload: the outer result is a one-element array
// whose element is itself a JSON-RPC envelope, so the object lives at
// result[0].result. This unwraps it.
func (s *VPSService) GetFirstOrderInfo(ctx context.Context) (*FirstOrderInfo, error) {
	var out []struct {
		Result FirstOrderInfo `json:"result"`
	}
	if err := s.c.call(ctx, vpsEndpoint, "getFirstOrderInfo", nil, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0].Result, nil
}

// WaitForIdle polls the VPS until its current_action is empty. SpaceWeb runs a
// resize / provisioning as a SEQUENCE of async actions (e.g. Modify → ExtIpAdd —
// an IP re-issue runs even when no extra IP was ordered), and is_running stays 1
// throughout — so "settled" means current_action is idle, NOT is_running == 1.
//
// poll is the interval between checks (default 10s). onPhase, if non-nil, is
// called each poll with the current action (trimmed; "" once idle) — a CLI can
// render it as progress. Honors ctx for cancellation / timeout; wrap ctx with a
// deadline to bound the wait.
func (s *VPSService) WaitForIdle(ctx context.Context, billingID string, poll time.Duration, onPhase func(action string)) (*VPS, error) {
	if poll <= 0 {
		poll = 10 * time.Second
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		if list, err := s.List(ctx); err == nil {
			var node *VPS
			for i := range list {
				if list[i].BillingID == billingID {
					node = &list[i]
					break
				}
			}
			if node == nil {
				return nil, fmt.Errorf("sweb: VPS %q not found while waiting", billingID)
			}
			if onPhase != nil {
				onPhase(strings.TrimSpace(node.CurrentAction))
			}
			if isIdle(node.CurrentAction) {
				return node, nil
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// isIdle reports whether a current_action value means "no operation in flight".
func isIdle(action string) bool {
	a := strings.TrimSpace(action)
	return a == "" || strings.EqualFold(a, "none")
}
