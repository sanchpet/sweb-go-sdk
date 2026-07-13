package sweb

import (
	"context"
	"encoding/json"
	"fmt"
)

const balancerEndpoint = "/balancer"

// BalancerService groups load-balancer operations (endpoint /balancer):
// list/isCreateEnable/getAvailableConfig plus the create/edit/remove lifecycle.
type BalancerService struct{ c *Client }

// BalancerRule is one forwarding rule of a load balancer: the front-end
// (balancer) protocol/port and the back-end (server) protocol/port. The API
// quotes ports as strings and, per the spec, may quote protocolServer as a
// number ("float"); FlexInt/strings tolerate both.
type BalancerRule struct {
	ProtocolBalancer string `json:"protocolBalancer"`
	PortBalancer     string `json:"portBalancer"`
	ProtocolServer   string `json:"protocolServer"`
	PortServer       string `json:"portServer"`
}

// BalancerServer is one back-end server behind a load balancer. Weight is only
// present for type "roundrobin" (1..5); VPSName is null for a bare IP target.
type BalancerServer struct {
	IP      string  `json:"ip"`
	Weight  FlexInt `json:"weight"`  // roundrobin only; 0 when absent
	VPSName string  `json:"vpsName"` // nullable
}

// Balancer is one load-balancer service as returned by List (method "index").
//
// Types are reconciled against the spec's recorded example: numeric fields
// arrive polymorphic (plan_id quoted "4298", price bare 375, datacenter bare 1)
// so they decode through FlexInt; CurrentAction is null when idle.
type Balancer struct {
	BillingID     string           `json:"billingId"`
	Name          string           `json:"name"`
	Type          string           `json:"type"` // "roundrobin" | "leastconn"
	PlanID        FlexInt          `json:"plan_id"`
	PlanName      string           `json:"plan_name"`
	Price         FlexInt          `json:"price"`
	Active        bool             `json:"active"`
	RemoveAllowed bool             `json:"removeAllowed"`
	BlockUI       bool             `json:"blockUi"`
	CurrentAction string           `json:"currentAction"` // nullable: ""|create|edit|remove
	TSCreate      string           `json:"tsCreate"`
	IPBalancer    string           `json:"ipBalancer"`
	Datacenter    FlexInt          `json:"datacenter"`
	HealthCheck   bool             `json:"healthCheck"`
	ProxyProto    bool             `json:"proxyProto"`
	Keepalive     bool             `json:"keepalive"`
	SaveSession   bool             `json:"saveSession"`
	Rules         []BalancerRule   `json:"rules"`
	Servers       []BalancerServer `json:"servers"`
}

// BalancerConfig is the catalog of selectable options for ordering a balancer
// (method "getAvailableConfig"): plans, protocols, and per-plan descriptions.
type BalancerConfig struct {
	Plans        []BalancerPlan        `json:"plans"`
	Protocols    []BalancerProtocol    `json:"protocols"`
	Descriptions []BalancerDescription `json:"descriptions"`
}

// BalancerPlan is one orderable balancer tariff.
type BalancerPlan struct {
	ID    string    `json:"id"`
	Tag   string    `json:"tag"`
	Title string    `json:"title"`
	Price FlexFloat `json:"price"`
}

// BalancerProtocol is one supported front-end protocol and the back-end
// protocols it may forward to (restrictions).
type BalancerProtocol struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Restrictions []string `json:"restrictions"`
}

// BalancerDescription is a per-plan marketing description keyed by service id.
type BalancerDescription struct {
	ServiceID     string `json:"service_id"`
	ServicePlanID string `json:"service_plan_id"`
	Description   string `json:"description"`
}

// balancerIndex is the keyed wrapper the "index" method returns: the balancers
// live under "ips" despite the spec typing the result as a bare array.
type balancerIndex struct {
	IPs []Balancer `json:"ips"`
}

// List returns the account's load balancers (method "index"). Read-only. The API
// wraps the list in {"ips":[…]}; this unwraps it.
func (s *BalancerService) List(ctx context.Context) ([]Balancer, error) {
	var out balancerIndex
	if err := s.c.call(ctx, balancerEndpoint, "index", nil, &out); err != nil {
		return nil, err
	}
	return out.IPs, nil
}

// IsCreateEnable reports whether ordering a new balancer is currently available
// (method "isCreateEnable", 1 = available, 0 = not). Read-only.
func (s *BalancerService) IsCreateEnable(ctx context.Context) (bool, error) {
	var out FlexInt
	if err := s.c.call(ctx, balancerEndpoint, "isCreateEnable", nil, &out); err != nil {
		return false, err
	}
	return out == 1, nil
}

// AvailableConfig returns the balancer order catalog (method "getAvailableConfig"):
// plans, protocols, and descriptions. Read-only. Despite the spec typing the
// result as an array, it is an object (confirmed against the spec example).
func (s *BalancerService) AvailableConfig(ctx context.Context) (*BalancerConfig, error) {
	var out BalancerConfig
	if err := s.c.call(ctx, balancerEndpoint, "getAvailableConfig", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BalancerCreateOptions are the inputs to Create. Datacenter, Type, Servers,
// Rules, PlanID and IsFirstOrder are required by the API; the boolean toggles
// and Alias are optional and forwarded as-is.
type BalancerCreateOptions struct {
	Datacenter   int              // id of the datacenter
	Type         string           // "roundrobin" | "leastconn"
	Servers      []BalancerServer // back-end servers (max 20)
	Rules        []BalancerRule   // forwarding rules
	PlanID       int              // tariff plan id
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
func (s *BalancerService) Create(ctx context.Context, o BalancerCreateOptions) error {
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

// BalancerEditOptions are the inputs to Edit. BillingID identifies the balancer;
// Type, Servers and Rules are required; the booleans and Alias are optional.
type BalancerEditOptions struct {
	BillingID   string // balancer identifier (Balancer.BillingID)
	Type        string // "roundrobin" | "leastconn"
	Servers     []BalancerServer
	Rules       []BalancerRule
	HealthCheck bool
	ProxyProto  bool
	Keepalive   bool
	SaveSession bool
	Alias       string // optional
}

// Edit changes a load balancer's settings (method "edit"). MUTATING. Returns on
// the 1/0 sentinel (1 = success).
func (s *BalancerService) Edit(ctx context.Context, o BalancerEditOptions) error {
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
func (s *BalancerService) Remove(ctx context.Context, billingID string) error {
	return s.sentinelAction(ctx, "remove", map[string]any{"billingId": billingID})
}

// sentinelAction runs a /balancer method whose success is the integer sentinel
// 1 (create/edit/remove all answer 1 on success, 0 on failure per the spec). A
// real failure usually surfaces as a JSON-RPC error via call; the non-1 check is
// defensive. The result is decoded via json.RawMessage first so that a shape not
// yet observed live (should the API ever answer richer than a bare 1) does not
// silently pass — only a plain 1 is accepted as success.
func (s *BalancerService) sentinelAction(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.c.call(ctx, balancerEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out FlexInt
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: balancer %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: balancer %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
