// Package balancer groups load-balancer operations (endpoint /balancer):
// list/isCreateEnable/getAvailableConfig plus the create/edit/remove lifecycle.
// All calls dispatch through the shared transport.
package balancer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const balancerEndpoint = "/balancer"

// Service groups load-balancer operations (endpoint /balancer):
// list/isCreateEnable/getAvailableConfig plus the create/edit/remove lifecycle.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Rule is one forwarding rule of a load balancer: the front-end
// (balancer) protocol/port and the back-end (server) protocol/port. The API
// quotes ports as strings and, per the spec, may quote protocolServer as a
// number ("float"); flex.Int/strings tolerate both.
type Rule struct {
	ProtocolBalancer string `json:"protocolBalancer"`
	PortBalancer     string `json:"portBalancer"`
	ProtocolServer   string `json:"protocolServer"`
	PortServer       string `json:"portServer"`
}

// Server is one back-end server behind a load balancer. Weight is only
// present for type "roundrobin" (1..5); VPSName is null for a bare IP target.
type Server struct {
	IP      string   `json:"ip"`
	Weight  flex.Int `json:"weight"`  // roundrobin only; 0 when absent
	VPSName string   `json:"vpsName"` // nullable
}

// Balancer is one load-balancer service as returned by List (method "index").
//
// Types are reconciled against the spec's recorded example: numeric fields
// arrive polymorphic (plan_id quoted "4298", price bare 375, datacenter bare 1)
// so they decode through flex.Int; CurrentAction is null when idle.
type Balancer struct {
	BillingID     string   `json:"billingId"`
	Name          string   `json:"name"`
	Type          string   `json:"type"` // "roundrobin" | "leastconn"
	PlanID        flex.Int `json:"plan_id"`
	PlanName      string   `json:"plan_name"`
	Price         flex.Int `json:"price"`
	Active        bool     `json:"active"`
	RemoveAllowed bool     `json:"removeAllowed"`
	BlockUI       bool     `json:"blockUi"`
	CurrentAction string   `json:"currentAction"` // nullable: ""|create|edit|remove
	TSCreate      string   `json:"tsCreate"`
	IPBalancer    string   `json:"ipBalancer"`
	Datacenter    flex.Int `json:"datacenter"`
	HealthCheck   bool     `json:"healthCheck"`
	ProxyProto    bool     `json:"proxyProto"`
	Keepalive     bool     `json:"keepalive"`
	SaveSession   bool     `json:"saveSession"`
	Rules         []Rule   `json:"rules"`
	Servers       []Server `json:"servers"`
}

// Config is the catalog of selectable options for ordering a balancer
// (method "getAvailableConfig"): plans, protocols, and per-plan descriptions.
type Config struct {
	Plans        []Plan        `json:"plans"`
	Protocols    []Protocol    `json:"protocols"`
	Descriptions []Description `json:"descriptions"`
}

// Plan is one orderable balancer tariff.
type Plan struct {
	ID    string     `json:"id"`
	Tag   string     `json:"tag"`
	Title string     `json:"title"`
	Price flex.Float `json:"price"`
}

// Protocol is one supported front-end protocol and the back-end
// protocols it may forward to (restrictions).
type Protocol struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Restrictions []string `json:"restrictions"`
}

// Description is a per-plan marketing description keyed by service id.
type Description struct {
	ServiceID     string `json:"service_id"`
	ServicePlanID string `json:"service_plan_id"`
	Description   string `json:"description"`
}

// balancerIndex is the keyed wrapper the "index" method returns: the balancers
// live under "ips" despite the spec typing the result as a bare array.
type balancerIndex struct {
	IPs []Balancer `json:"ips"`
}

// UnmarshalJSON tolerates the endpoint's dual shape reconciled against the live
// API: a populated index is the object {"ips":[…]}, but an account with no
// balancers answers with a bare (empty) array. A JSON array means no balancers.
func (b *balancerIndex) UnmarshalJSON(data []byte) error {
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '[' {
		b.IPs = nil
		return nil
	}
	type alias balancerIndex
	return json.Unmarshal(data, (*alias)(b))
}

// List returns the account's load balancers (method "index"). Read-only. The API
// wraps the list in {"ips":[…]}; this unwraps it.
func (s *Service) List(ctx context.Context) ([]Balancer, error) {
	var out balancerIndex
	if err := s.t.Call(ctx, balancerEndpoint, "index", nil, &out); err != nil {
		return nil, err
	}
	return out.IPs, nil
}

// IsCreateEnable reports whether ordering a new balancer is currently available
// (method "isCreateEnable", 1 = available, 0 = not). Read-only.
func (s *Service) IsCreateEnable(ctx context.Context) (bool, error) {
	var out flex.Int
	if err := s.t.Call(ctx, balancerEndpoint, "isCreateEnable", nil, &out); err != nil {
		return false, err
	}
	return out == 1, nil
}

// AvailableConfig returns the balancer order catalog (method "getAvailableConfig"):
// plans, protocols, and descriptions. Read-only. Despite the spec typing the
// result as an array, it is an object (confirmed against the spec example).
func (s *Service) AvailableConfig(ctx context.Context) (*Config, error) {
	var out Config
	if err := s.t.Call(ctx, balancerEndpoint, "getAvailableConfig", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateOptions are the inputs to Create. Datacenter, Type, Servers,
// Rules, PlanID and IsFirstOrder are required by the API; the boolean toggles
// and Alias are optional and forwarded as-is.
type CreateOptions struct {
	Datacenter   int      // id of the datacenter
	Type         string   // "roundrobin" | "leastconn"
	Servers      []Server // back-end servers (max 20)
	Rules        []Rule   // forwarding rules
	PlanID       int      // tariff plan id
	HealthCheck  bool
	ProxyProto   bool
	Keepalive    bool
	SaveSession  bool
	Alias        string // service name (optional)
	IsFirstOrder bool
}

// Create orders a new load balancer (method "create"). MUTATING and billable —
// never exercise against the live API in tests. Returns on the 1/0 sentinel
// (1 = the create procedure started; non-1 is an error).
func (s *Service) Create(ctx context.Context, o CreateOptions) error {
	params := map[string]any{
		"datacenter":   o.Datacenter,
		"type":         o.Type,
		"servers":      o.Servers,
		"rules":        o.Rules,
		"planId":       o.PlanID,
		"healthCheck":  o.HealthCheck,
		"proxyProto":   o.ProxyProto,
		"keepalive":    o.Keepalive,
		"saveSession":  o.SaveSession,
		"isFirstOrder": o.IsFirstOrder,
	}
	if o.Alias != "" {
		params["alias"] = o.Alias
	}
	return s.sentinelAction(ctx, "create", params)
}

// EditOptions are the inputs to Edit. BillingID identifies the balancer;
// Type, Servers and Rules are required; the booleans and Alias are optional.
type EditOptions struct {
	BillingID   string // balancer identifier (Balancer.BillingID)
	Type        string // "roundrobin" | "leastconn"
	Servers     []Server
	Rules       []Rule
	HealthCheck bool
	ProxyProto  bool
	Keepalive   bool
	SaveSession bool
	Alias       string // optional
}

// Edit changes a load balancer's settings (method "edit"). MUTATING. Returns on
// the 1/0 sentinel (1 = success).
func (s *Service) Edit(ctx context.Context, o EditOptions) error {
	params := map[string]any{
		"billingId":   o.BillingID,
		"type":        o.Type,
		"servers":     o.Servers,
		"rules":       o.Rules,
		"healthCheck": o.HealthCheck,
		"proxyProto":  o.ProxyProto,
		"keepalive":   o.Keepalive,
		"saveSession": o.SaveSession,
	}
	if o.Alias != "" {
		params["alias"] = o.Alias
	}
	return s.sentinelAction(ctx, "edit", params)
}

// Remove deletes a load balancer (method "remove"). MUTATING. billingID is a
// Balancer.BillingID. Returns on the 1/0 sentinel (1 = success).
func (s *Service) Remove(ctx context.Context, billingID string) error {
	return s.sentinelAction(ctx, "remove", map[string]any{"billingId": billingID})
}

// sentinelAction runs a /balancer method whose success is the integer sentinel
// 1 (create/edit/remove all answer 1 on success, 0 on failure per the spec). A
// real failure usually surfaces as a JSON-RPC error via call; the non-1 check is
// defensive. The result is decoded via json.RawMessage first so that a shape not
// yet observed live (should the API ever answer richer than a bare 1) does not
// silently pass — only a plain 1 is accepted as success.
func (s *Service) sentinelAction(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, balancerEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: balancer %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: balancer %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
