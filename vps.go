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

// VPS is a VPS instance as returned by List (method "index").
// Field set confirmed against a real API response (Evidence phase). Some rarely
// used / always-null fields (local_*, isp, protected_ips, parent_plan_id) are
// omitted; add them when a use arises.
type VPS struct {
	BillingID      string      `json:"billingId"`
	Name           string      `json:"name"` // user-facing alias
	UID            string      `json:"uid"`  // stable unique id
	PlanID         string      `json:"plan_id"`
	PlanName       string      `json:"plan_name"`
	PlanPrice      int         `json:"plan_price"`
	CPU            int         `json:"cpu"`
	RAM            int         `json:"ram"`
	Disk           string      `json:"disk"`
	DiskGB         int         `json:"diskGb"`
	Active         int         `json:"active"`
	IsRunning      int         `json:"is_running"`
	CurrentAction  string      `json:"current_action"`
	OSDistribution string      `json:"os_distribution"`
	OSDistrID      int         `json:"os_distr_id"`
	Category       string      `json:"category"`
	MAC            string      `json:"mac"`
	IP             string      `json:"ip"`
	ExtIPs         []string    `json:"ext_ips"`
	OrderedIPCount int         `json:"ordered_ip_count"`
	Datacenter     string      `json:"datacenter"`
	DatacenterID   string      `json:"datacenter_id"`
	PasswordAccess bool        `json:"password_access"`
	SSHKeys        []SSHKeyRef `json:"ssh_keys"`
	Features       VPSFeatures `json:"features"`
	TSCreate       string      `json:"ts_create"`
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
