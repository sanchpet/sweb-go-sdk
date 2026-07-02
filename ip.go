package sweb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const ipEndpoint = "/vps/ip"

// IPService groups IP operations (endpoint /vps/ip): the account private (local)
// network and public/additional IP management.
type IPService struct{ c *Client }

// LocalIP is a VPS's attachment to the account private (local) network.
type LocalIP struct {
	IP   string `json:"ip"`
	MAC  string `json:"mac"`
	Mask string `json:"mask"`
}

// IPAddress is a public IP bound to (or orderable for) a VPS.
type IPAddress struct {
	IP         string    `json:"ip"`
	Gateway    string    `json:"gateway"`
	Netmask    string    `json:"netmask"`
	Datacenter FlexInt   `json:"datacenter"`
	PTR        string    `json:"ptr"`
	Price      FlexFloat `json:"price"` // money: the API returns fractional prices (e.g. 142.06)
}

// IPInfo is the per-VPS IP inventory returned by the "index" method: public IPs,
// protected IPs, and the local-network attachment (if any).
type IPInfo struct {
	IPs          []IPAddress       `json:"ips"`
	ProtectedIPs []json.RawMessage `json:"protected_ips"` // typed on demand
	LocalIP      []LocalIP         `json:"local_ip"`
	VPS          IPVPSInfo         `json:"vps"`
}

// IPVPSInfo is the VPS summary embedded in the IP index.
type IPVPSInfo struct {
	BillingID      string  `json:"billingId"`
	CurrentAction  string  `json:"currentAction"` // string|null
	IsEmpty        string  `json:"isEmpty"`       // "0" once the OS is installed
	OrderedIPCount FlexInt `json:"ordered_ip_count"`
}

// Info returns the IP inventory for a VPS (method "index"). Read-only.
func (s *IPService) Info(ctx context.Context, billingID string) (*IPInfo, error) {
	var out IPInfo
	if err := s.c.call(ctx, ipEndpoint, "index", map[string]string{"billingId": billingID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddLocal attaches the VPS to the account private (local) network. The local IP
// is assigned by SpaceWeb — read it back via Info or WaitForLocalIP. This is the
// declarative way to put an EXISTING VPS on the private network (no re-create).
func (s *IPService) AddLocal(ctx context.Context, billingID string) error {
	return s.localAction(ctx, "addLocal", billingID)
}

// RemoveLocal detaches the VPS from the private (local) network.
func (s *IPService) RemoveLocal(ctx context.Context, billingID string) error {
	return s.localAction(ctx, "removeLocal", billingID)
}

func (s *IPService) localAction(ctx context.Context, method, billingID string) error {
	var out FlexInt
	if err := s.c.call(ctx, ipEndpoint, method, map[string]string{"billingId": billingID}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}

// WaitForLocalIP polls Info until the VPS reports a local IP (attachment can be
// asynchronous), returning the first one, or until ctx is done.
func (s *IPService) WaitForLocalIP(ctx context.Context, billingID string, interval time.Duration) (LocalIP, error) {
	for {
		info, err := s.Info(ctx, billingID)
		if err == nil && len(info.LocalIP) > 0 {
			return info.LocalIP[0], nil
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return LocalIP{}, err
			}
			return LocalIP{}, fmt.Errorf("sweb: timed out waiting for local IP on %s", billingID)
		case <-time.After(interval):
		}
	}
}
