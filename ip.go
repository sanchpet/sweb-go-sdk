package sweb

import (
	"bytes"
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

// decodeArrayOrObject unmarshals a JSON value SpaceWeb returns as EITHER a list
// ([]) or a single bare object ({}) — the /vps/ip index does this for local_ip
// (empty → [], attached → a bare object) and may for the IP lists too.
func decodeArrayOrObject[T any](b []byte) ([]T, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil, nil
	}
	switch b[0] {
	case '[':
		var arr []T
		if err := json.Unmarshal(b, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	case '{':
		var one T
		if err := json.Unmarshal(b, &one); err != nil {
			return nil, err
		}
		return []T{one}, nil
	default:
		return nil, fmt.Errorf("sweb: expected a JSON array or object, got %s", b)
	}
}

// LocalIPList is []LocalIP that also decodes a bare object or null (SpaceWeb
// returns local_ip as [] when unattached, a single object when attached).
type LocalIPList []LocalIP

// UnmarshalJSON accepts an array, a single object, or null.
func (l *LocalIPList) UnmarshalJSON(b []byte) error {
	v, err := decodeArrayOrObject[LocalIP](b)
	*l = v
	return err
}

// IPAddressList is []IPAddress with the same array-or-object tolerance.
type IPAddressList []IPAddress

// UnmarshalJSON accepts an array, a single object, or null.
func (l *IPAddressList) UnmarshalJSON(b []byte) error {
	v, err := decodeArrayOrObject[IPAddress](b)
	*l = v
	return err
}

// IPInfo is the per-VPS IP inventory returned by the "index" method: public IPs,
// protected IPs, and the local-network attachment (if any).
type IPInfo struct {
	IPs          IPAddressList   `json:"ips"`
	ProtectedIPs json.RawMessage `json:"protected_ips"` // raw: shape varies; decode on demand
	LocalIP      LocalIPList     `json:"local_ip"`
	VPS          IPVPSInfo       `json:"vps"`
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

// Add orders number additional public IPs for a VPS (method "add"). This BILLS.
// Like Create, the result shape is left raw pending a recorded response — read
// the assigned addresses back via Info once they settle.
func (s *IPService) Add(ctx context.Context, billingID string, number int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, ipEndpoint, "add", map[string]any{
		"billingId": billingID,
		"number":    number,
	}, &out)
	return out, err
}

// Remove releases a public IP from a VPS (method "remove"). Action 1/0 result.
func (s *IPService) Remove(ctx context.Context, billingID, ip string) error {
	var out FlexInt
	if err := s.c.call(ctx, ipEndpoint, "remove", map[string]string{
		"billingId": billingID,
		"ip":        ip,
	}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: remove ip returned %d, want 1 (0 = failure)", int64(out))
	}
	return nil
}

// Move attaches an IP to a VPS, or detaches it when billingID is empty (method
// "move"; the API takes billingId=null to detach). Action 1/0 result.
func (s *IPService) Move(ctx context.Context, ip, billingID string) error {
	params := map[string]any{"ip": ip, "billingId": nil}
	if billingID != "" {
		params["billingId"] = billingID
	}
	var out FlexInt
	if err := s.c.call(ctx, ipEndpoint, "move", params, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: move ip returned %d, want 1 (0 = failure)", int64(out))
	}
	return nil
}

// GetPtr returns the PTR (reverse-DNS) record for an IP (method "getPtr").
// Read-only. Tolerates the record arriving as a bare string or a {"ptr": …}
// object.
func (s *IPService) GetPtr(ctx context.Context, ip string) (string, error) {
	var raw json.RawMessage
	if err := s.c.call(ctx, ipEndpoint, "getPtr", map[string]string{"ip": ip}, &raw); err != nil {
		return "", err
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		return str, nil
	}
	var obj struct {
		PTR string `json:"ptr"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.PTR, nil
	}
	return "", fmt.Errorf("sweb: unexpected getPtr result %s", raw)
}

// EditPtr sets the PTR (reverse-DNS) record for an IP (method "editPtr"). An empty
// ptr resets it to the provider default. Action 1/0 result.
func (s *IPService) EditPtr(ctx context.Context, ip, ptr string) error {
	var out FlexInt
	if err := s.c.call(ctx, ipEndpoint, "editPtr", map[string]string{
		"ip":  ip,
		"ptr": ptr,
	}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: editPtr returned %d, want 1 (0 = failure)", int64(out))
	}
	return nil
}
