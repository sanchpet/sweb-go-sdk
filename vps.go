package sweb

import (
	"context"
	"encoding/json"
)

// VPSService groups VPS operations. All calls hit the /vps endpoint with a
// JSON-RPC method.
type VPSService struct{ c *Client }

const vpsEndpoint = "/vps"

// VPS is a VPS instance as returned by List (method "index").
//
// NOTE: field set is provisional — inferred from the reference CLI, not yet
// confirmed against a recorded real response (Evidence phase). The "index"
// result may also be wrapped in an object rather than a bare array; firm this
// up from fixtures before relying on it.
type VPS struct {
	ID     json.Number `json:"id,omitempty"`
	Alias  string      `json:"alias,omitempty"`
	Status string      `json:"status,omitempty"`
}

// List returns all VPS instances (method "index").
func (s *VPSService) List(ctx context.Context) ([]VPS, error) {
	var out []VPS
	err := s.c.call(ctx, vpsEndpoint, "index", nil, &out)
	return out, err
}

// CreateVPSRequest holds the parameters for Create (method "create").
type CreateVPSRequest struct {
	DistributiveID      int    `json:"distributiveId"`
	VPSPlanID           int    `json:"vpsPlanId"`
	Datacenter          int    `json:"datacenter"`
	Alias               string `json:"alias"`
	SSHKey              string `json:"sshKey"`
	MonitoringPlanID    int    `json:"monitoringPlanId"`
	MonitoringContactID int    `json:"monitoringContactId"`
	IPCount             int    `json:"ipCount"`
	ProtectedIPs        []int  `json:"protectedIps,omitempty"`
}

// Create provisions a new VPS (method "create"). The result shape is not yet
// confirmed, so the raw JSON result is returned for now (Evidence phase will
// type it).
func (s *VPSService) Create(ctx context.Context, req CreateVPSRequest) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, vpsEndpoint, "create", req, &out)
	return out, err
}

// AvailableConfig returns the catalog of selectable VPS options — plans,
// distributives/OS, datacenters (method "getAvailableConfig"). Raw for now,
// pending a recorded fixture.
func (s *VPSService) AvailableConfig(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, vpsEndpoint, "getAvailableConfig", nil, &out)
	return out, err
}
