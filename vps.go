package sweb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
	return id, nil
}
