// Package vps groups VPS operations (endpoint /vps): the VPS lifecycle
// (create/remove/rename/changePlan/copy/reinstall), power control, the
// configurator lookups, the first-order flow, load graphs, logs, and the
// getAvailableConfig catalog. All calls dispatch through the shared transport.
package vps

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const vpsEndpoint = "/vps"

// Service groups VPS operations. All calls hit the /vps endpoint with a
// JSON-RPC method.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// SSHKeyRef is an SSH key attached to a VPS.
type SSHKeyRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Features are capability flags for a VPS / plan.
type Features struct {
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
	LicenseType  string     `json:"license_type"`
	IP           string     `json:"ip"`
	ActiveUntil  string     `json:"active_until"`
	Price        flex.Float `json:"price"`
	Link         string     `json:"link"`
	IsBlocked    flex.Int   `json:"is_blocked"`
	CreationTime string     `json:"creation_time"`
}

// VPS is a VPS instance as returned by List (method "index").
//
// Types are reconciled against a real API response: SpaceWeb returns many
// numeric fields either as numbers or quoted strings (flex.Int/flex.Float handle
// both), and nullable fields (parent_plan_id, local_*) as JSON null.
type VPS struct {
	BillingID      string            `json:"billingId"`
	Name           string            `json:"name"` // user-facing alias
	UID            string            `json:"uid"`  // stable unique id
	PlanID         flex.Int          `json:"plan_id"`
	PlanName       string            `json:"plan_name"`
	ParentPlanID   flex.Int          `json:"parent_plan_id"` // nullable
	PlanPrice      flex.Float        `json:"plan_price"`     // money, may be fractional
	CPU            flex.Int          `json:"cpu"`
	RAM            flex.Int          `json:"ram"`  // MB; API may quote it ("1024")
	Disk           string            `json:"disk"` // localized human size, e.g. "10 ГБ"
	BlockUI        flex.Int          `json:"blockUi"`
	Active         flex.Int          `json:"active"`
	OSDistribution string            `json:"os_distribution"`
	OSDistrID      flex.Int          `json:"os_distr_id"`
	Category       string            `json:"category"`
	TSCreate       string            `json:"ts_create"`
	MAC            string            `json:"mac"`
	IP             string            `json:"ip"`
	LocalIP        string            `json:"local_ip"`  // nullable
	LocalMAC       string            `json:"local_mac"` // nullable
	LocalMask      string            `json:"local_mask"`
	CurrentAction  string            `json:"current_action"`
	IsRunning      flex.Int          `json:"is_running"`
	ISP            []ISPInfo         `json:"isp"`
	ExtIPs         []json.RawMessage `json:"ext_ips"` // TODO: type once a populated example exists (doc: array of IP objects)
	IsTest         flex.Int          `json:"is_test"`
	IsNew          bool              `json:"is_new"`
	Datacenter     string            `json:"datacenter"`
	OrderedIPCount flex.Int          `json:"ordered_ip_count"`
	ProtectedIPs   []string          `json:"protected_ips"`

	// Not part of the documented index response — present in an earlier recorded
	// response and still consumed by the Terraform provider's computed mapping
	// (disk size, datacenter id). Kept until the provider reconcile (variant B);
	// index does not populate these, so they decode to zero here.
	DiskGB         int         `json:"diskGb"`
	DatacenterID   string      `json:"datacenter_id"`
	PasswordAccess bool        `json:"password_access"`
	SSHKeys        []SSHKeyRef `json:"ssh_keys"`
	Features       Features    `json:"features"`
}

// List returns all VPS instances (method "index").
func (s *Service) List(ctx context.Context) ([]VPS, error) {
	var out []VPS
	err := s.t.Call(ctx, vpsEndpoint, "index", nil, &out)
	return out, err
}

// CreateRequest holds the parameters for Create (method "create"). Use
// AvailableConfig to resolve the numeric IDs (plan, distributive, datacenter).
type CreateRequest struct {
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
func (s *Service) Create(ctx context.Context, req CreateRequest) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, vpsEndpoint, "create", req, &out)
	return out, err
}

// Remove deletes a VPS (method "remove"). billingID is the service identifier
// (format "login_vps_N"), as returned in VPS.BillingID by List.
//
// This is destructive — it cancels the VPS. The result shape is left raw
// pending a recorded response.
func (s *Service) Remove(ctx context.Context, billingID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, vpsEndpoint, "remove", map[string]string{"billingId": billingID}, &out)
	return out, err
}

// Rename changes a VPS's user-facing name/alias (method "rename"). billingID is
// the service identifier ("login_vps_N"); alias is the new name. This is an
// in-place label change — it does not reprovision or bill. The API returns 1 on
// success; a JSON-RPC error surfaces as *apierr.Error.
func (s *Service) Rename(ctx context.Context, billingID, alias string) error {
	var out json.RawMessage
	return s.t.Call(ctx, vpsEndpoint, "rename", map[string]string{
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
func (s *Service) ChangePlan(ctx context.Context, billingID string, vpsPlanID int) error {
	var out int
	if err := s.t.Call(ctx, vpsEndpoint, "changePlan", map[string]any{
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
func (s *Service) PowerOn(ctx context.Context, billingID string) error {
	return s.powerAction(ctx, "powerOn", billingID)
}

// PowerOff shuts a running VPS down (method "powerOff"). Asynchronous — see PowerOn.
func (s *Service) PowerOff(ctx context.Context, billingID string) error {
	return s.powerAction(ctx, "powerOff", billingID)
}

// Reboot restarts a VPS (method "reboot"). Asynchronous — see PowerOn.
func (s *Service) Reboot(ctx context.Context, billingID string) error {
	return s.powerAction(ctx, "reboot", billingID)
}

// powerAction issues a power method against the VPS endpoint. Like the other
// action methods, the API answers 1 on success and 0 on failure (quoted or bare
// — hence flex.Int).
func (s *Service) powerAction(ctx context.Context, method, billingID string) error {
	var out flex.Int
	if err := s.t.Call(ctx, vpsEndpoint, method, map[string]string{"billingId": billingID}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}

// Copy clones a VPS into a new one on the given plan (method "copy"). billingID
// is the source; vpsPlanID is the new VPS's plan (from AvailableConfig, or
// GetConstructorPlanID for a custom configuration). Like Create, this provisions
// a NEW, billed VPS and runs asynchronously — the result shape is left raw
// pending a recorded response; find the new node via List once it settles.
func (s *Service) Copy(ctx context.Context, billingID string, vpsPlanID int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, vpsEndpoint, "copy", map[string]any{
		"billingId": billingID,
		"vpsPlanId": vpsPlanID,
	}, &out)
	return out, err
}

// ReinstallOS reinstalls the VPS's operating system (method "reinstallOs") to the
// given distributive (see AvailableConfig.SelectOS for the ids). This is
// DESTRUCTIVE — it wipes the system disk unless keepDisk is set (the API's
// save_disk flag). It runs asynchronously; poll WaitForIdle to await the rebuild.
// The API answers 1 on acceptance (0 = failure).
func (s *Service) ReinstallOS(ctx context.Context, billingID string, distributiveID int, keepDisk bool) error {
	var out flex.Int
	if err := s.t.Call(ctx, vpsEndpoint, "reinstallOs", map[string]any{
		"billingId":      billingID,
		"distributiveId": distributiveID,
		"save_disk":      keepDisk,
	}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: reinstallOs returned %d, want 1 (0 = failure)", int64(out))
	}
	return nil
}

// IsRunning reports whether the VPS is powered on (method "isRunning"). Read-only.
// Note List already carries VPS.IsRunning for every node; this is the cheaper
// single-VPS query when only the power state is needed.
func (s *Service) IsRunning(ctx context.Context, billingID string) (bool, error) {
	var out flex.Int
	if err := s.t.Call(ctx, vpsEndpoint, "isRunning", map[string]string{"billingId": billingID}, &out); err != nil {
		return false, err
	}
	return out == 1, nil
}

// GetConstructorPlanID resolves a custom ("configurator") plan ID for the given
// resources via the "getConstructorPlanId" method. ram and disk are in GB;
// categoryID is a catalog category id (see AvailableConfig.Categories). This is
// read-only — it neither creates nor bills; feed the result to Create as the
// VPSPlanID.
func (s *Service) GetConstructorPlanID(ctx context.Context, cpuCores, ramGB, diskGB, categoryID int) (int, error) {
	var raw json.RawMessage
	err := s.t.Call(ctx, vpsEndpoint, "getConstructorPlanId", map[string]int{
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
	Plan                    string     `json:"plan"`
	OS                      string     `json:"os"`
	Panel                   string     `json:"panel"`
	CPUCores                flex.Int   `json:"cpu_cores"`
	RAM                     flex.Int   `json:"ram"`         // MB
	VolumeDisk              string     `json:"volume_disk"` // localized, e.g. "10 ГБ"
	PricePerMonth           flex.Float `json:"price_per_month"`
	PayPeriod               flex.Int   `json:"pay_period"` // months
	PriceForPeriodWithStock flex.Float `json:"price_for_period_with_stock"`
	PricePerMonthWithStock  flex.Float `json:"price_per_month_with_stock"`
	Promocode               string     `json:"promocode"` // empty when null
	ClearAvailable          bool       `json:"clearAvailable"`
	PlanIsConstructor       bool       `json:"plan_is_constructor"`
	IPCount                 flex.Int   `json:"ipCount"`
	ProtectedIPs            []flex.Int `json:"protectedIps"`
}

// GetFirstOrderInfo returns the account's first-order info (method
// "getFirstOrderInfo"), or nil if there is no first order.
//
// The API double-wraps the payload: the outer result is a one-element array
// whose element is itself a JSON-RPC envelope, so the object lives at
// result[0].result. This unwraps it.
func (s *Service) GetFirstOrderInfo(ctx context.Context) (*FirstOrderInfo, error) {
	var out []struct {
		Result FirstOrderInfo `json:"result"`
	}
	if err := s.t.Call(ctx, vpsEndpoint, "getFirstOrderInfo", nil, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0].Result, nil
}

// GetCurrentAction returns the VPS's current async operation (method
// "getCurrentAction") — the same value carried per-node as VPS.CurrentAction,
// but as a cheap single-VPS query useful for polling a resize/provision from a
// provider. Read-only. The result is a bare string ("" / "none" once idle, e.g.
// "start" / "Modify" while an action runs).
func (s *Service) GetCurrentAction(ctx context.Context, billingID string) (string, error) {
	var out string
	if err := s.t.Call(ctx, vpsEndpoint, "getCurrentAction", map[string]string{"billingId": billingID}, &out); err != nil {
		return "", err
	}
	return out, nil
}

// CreateEnable reports whether ordering a VPS is currently available for the
// account (method "createEnable") — the precondition check the panel runs before
// the first-order flow. Read-only: it neither creates nor bills. The API answers
// 1 (available) or 0 (not available); a JSON-RPC error surfaces as *apierr.Error.
func (s *Service) CreateEnable(ctx context.Context) (bool, error) {
	var out flex.Int
	if err := s.t.Call(ctx, vpsEndpoint, "createEnable", nil, &out); err != nil {
		return false, err
	}
	return out == 1, nil
}

// CreateFirstRequest holds the parameters for CreateFirst (method
// "createFirst"). Only DistributiveID and VPSPlanID are required; the rest are
// omitted when zero-valued. Resolve the numeric IDs via AvailableConfig.
type CreateFirstRequest struct {
	DistributiveID      int    `json:"distributiveId"`
	VPSPlanID           int    `json:"vpsPlanId"`
	Datacenter          int    `json:"datacenter,omitempty"`
	Alias               string `json:"alias,omitempty"`
	SSHKey              string `json:"sshKey,omitempty"`
	SSHKeyName          string `json:"sshKeyName,omitempty"`
	PrivateIP           bool   `json:"privateIp,omitempty"`
	Period              int    `json:"period,omitempty"` // 1 or 12 months
	StartTestPeriod     bool   `json:"startTestPeriod,omitempty"`
	MonitoringPlanID    int    `json:"monitoringPlanId,omitempty"`
	MonitoringContactID int    `json:"monitoringContactId,omitempty"`
	IPCount             int    `json:"ipCount,omitempty"` // first order: 0 or 1
	ProtectedIPs        []int  `json:"protectedIps,omitempty"`
}

// CreateFirst places the account's promotional first VPS order (method
// "createFirst"). Like Create it provisions a NEW, billed VPS (with the
// first-order pricing / trial-period options), so the result shape is left raw:
// it mutates and bills and was not exercised during the Evidence phase. The docs
// describe the result as the new service's billingId string ("login_vps_N");
// type it once a real response is recorded. Undo with RemoveFirst while the order
// is still unpaid.
func (s *Service) CreateFirst(ctx context.Context, req CreateFirstRequest) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, vpsEndpoint, "createFirst", req, &out)
	return out, err
}

// RemoveFirst cancels the account's first VPS order (method "removeFirst") —
// available only while that order is still unpaid and the service has not
// started. Takes no parameters (it targets the account's single first order).
// This is destructive; the result shape is left raw pending a recorded response.
// The docs describe a 1/0 result.
func (s *Service) RemoveFirst(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.t.Call(ctx, vpsEndpoint, "removeFirst", nil, &out)
	return out, err
}

// LoadGraph is the result of Load (method "load"): a rendered resource-usage
// chart returned inline as base64. Shape reconciled against the spec's recorded
// example — note the OpenRPC result schema nominally declares "array" but the
// example is this object, so the object shape is authoritative.
type LoadGraph struct {
	MIMEType string            `json:"mimetype"` // e.g. "image/png;base64"
	Metadata []json.RawMessage `json:"metadata"` // shape unobserved beyond empty []
	Content  string            `json:"content"`  // base64-encoded image
}

// LoadType selects which resource-usage series Load renders.
type LoadType string

// The load series accepted by Load (the API's "type" parameter).
const (
	LoadCPU    LoadType = "cpu"
	LoadHDDOps LoadType = "hdd_ops"
	LoadNet    LoadType = "net"
)

// Load renders a VPS resource-usage graph as a base64 image (method "load").
// billingID is the service id; loadType is the series (LoadCPU/LoadHDDOps/
// LoadNet); from and to bound the window (dd-mm-yyyy, e.g. "08-03-2023"); width
// is the graph width in pixels. Read-only.
func (s *Service) Load(ctx context.Context, billingID string, loadType LoadType, from, to string, width int) (*LoadGraph, error) {
	var out LoadGraph
	if err := s.t.Call(ctx, vpsEndpoint, "load", map[string]any{
		"billingId": billingID,
		"type":      string(loadType),
		"from":      from,
		"to":        to,
		"width":     width,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
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
func (s *Service) WaitForIdle(ctx context.Context, billingID string, poll time.Duration, onPhase func(action string)) (*VPS, error) {
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

// LogEntry is one entry of a VPS's operation log (method "logs"). Field names
// follow the documented shape (type/status/started_at/ended_at); the call is
// read-only, so the struct is reconciled from the docs pending a recorded
// response — unknown fields simply decode to zero.
type LogEntry struct {
	Type      string `json:"type"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at"`
}

// Logs returns the VPS's operation log (method "logs") — the record of lifecycle
// actions (create, reinstall, resize, …) run against the node. Read-only.
func (s *Service) Logs(ctx context.Context, billingID string) ([]LogEntry, error) {
	var out []LogEntry
	if err := s.t.Call(ctx, vpsEndpoint, "logs", map[string]string{"billingId": billingID}, &out); err != nil {
		return nil, err
	}
	return out, nil
}
