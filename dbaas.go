package sweb

import (
	"context"
	"encoding/json"
	"fmt"
)

const dbaasEndpoint = "/dbaas"

// DBaaSService groups managed-database (DBaaS) operations (endpoint /dbaas):
// listing clusters, the create-page config/constructor lookups, the promotional
// first-order flow, and cluster/database lifecycle (create/edit/remove/delete).
type DBaaSService struct{ c *Client }

// DBaaSIndex is the object returned by List (method "index"): the account's
// clusters plus beta-quota metadata.
type DBaaSIndex struct {
	Instances    []DBaaSInstance `json:"instances"`
	TotalCount   FlexInt         `json:"total_count"` // arrives as a quoted string
	MaxCount     FlexInt         `json:"max_count"`
	CanCreate    bool            `json:"can_create"`
	UpgradeAgree *bool           `json:"upgrade_agree"` // null until the user chooses
}

// DBaaSInstance is one managed-database cluster in DBaaSIndex.Instances.
type DBaaSInstance struct {
	ID            FlexInt         `json:"id"`
	BillingID     string          `json:"billing_id"`
	Price         FlexFloat       `json:"price"`
	Plan          DBaaSPlan       `json:"plan"`
	Active        bool            `json:"active"`
	TsWillDelete  string          `json:"ts_will_be_deleted"` // "" when null (beta only)
	BlockUI       bool            `json:"blockUi"`
	CurrentAction string          `json:"currentAction"` // "" when idle
	InstanceUUID  string          `json:"instance_uuid"`
	Name          string          `json:"name"`
	DisplayName   string          `json:"display_name"`
	Status        string          `json:"status"`
	Engine        string          `json:"engine"`
	Instances     FlexInt         `json:"instances"`
	SyncReplicas  FlexInt         `json:"sync_replicas"`
	ReadReplicas  FlexInt         `json:"read_replicas"`
	Replicas      FlexInt         `json:"replicas"`
	IsEnabled     bool            `json:"is_enabled"`
	IP            string          `json:"ip"` // "ip:port"
	Endpoints     []DBaaSEndpoint `json:"endpoints"`
	Users         []DBaaSUser     `json:"users"`
	Databases     []DBaaSDatabase `json:"databases"`
}

// DBaaSPlan is a DBaaS tariff (shared by index and getAvailableConfig). Numeric
// fields arrive as quoted strings in observed responses.
type DBaaSPlan struct {
	ID      FlexInt `json:"id"`
	Name    string  `json:"name"`
	CPU     FlexInt `json:"cpu"`
	Memory  FlexInt `json:"memory"`  // GB
	Storage FlexInt `json:"storage"` // GB
}

// DBaaSEndpoint is a cluster connection endpoint. Type is "rw" (always present)
// or "ro" (only when read_replicas > 0).
type DBaaSEndpoint struct {
	Port FlexInt `json:"port"`
	Type string  `json:"type"`
	IP   string  `json:"ip"`
}

// DBaaSUser is a cluster or per-database user (only the name is exposed).
type DBaaSUser struct {
	Name string `json:"name"`
}

// DBaaSDatabase is one database inside a cluster.
type DBaaSDatabase struct {
	Name        string      `json:"name"` // technical name
	Size        FlexInt     `json:"size"`
	Users       []DBaaSUser `json:"users"`
	DisplayName string      `json:"display_name"`
}

// List returns the account's DBaaS clusters and beta-quota metadata (method
// "index"). Read-only.
func (s *DBaaSService) List(ctx context.Context) (*DBaaSIndex, error) {
	var out DBaaSIndex
	if err := s.c.call(ctx, dbaasEndpoint, "index", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetUpgradeAgree records the user's choice to move (or not) to the paid tier
// after the beta (method "setUpgradeAgree"). Sentinel 1/0 result (1 = saved).
func (s *DBaaSService) SetUpgradeAgree(ctx context.Context, agree bool) error {
	return s.actionOne(ctx, "setUpgradeAgree", map[string]any{"upgradeAgree": agree})
}

// DBaaSConfig is the create-page config (method "getAvailableConfig"): available
// plans, engine versions, and the constructor kit.
//
// Engines maps an engine type ("PostgreSQL"/"MySQL") to its selectable versions.
// Kit (the constructor pricing model — nested price brackets with nullable bounds)
// is left raw: its shape is variable and only needed to render the constructor UI.
type DBaaSConfig struct {
	Plans   []DBaaSPlan              `json:"plans"`
	Engines map[string][]DBaaSEngine `json:"engines"`
	Kit     json.RawMessage          `json:"kit"`
}

// DBaaSEngine is one selectable DBMS version in DBaaSConfig.Engines.
type DBaaSEngine struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// AvailableConfig returns the data for the cluster-creation page: available
// plans and engine versions (method "getAvailableConfig"). Read-only.
func (s *DBaaSService) AvailableConfig(ctx context.Context) (*DBaaSConfig, error) {
	var out DBaaSConfig
	if err := s.c.call(ctx, dbaasEndpoint, "getAvailableConfig", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ConstructorPlanID resolves a constructor tariff ID from resource sizing (method
// "getConstructorPlanId"): CPU cores, memory (GB), storage (GB), and replica
// count (0 = master only). The returned ID feeds CreateInstance/EditInstance.
// Read-only.
func (s *DBaaSService) ConstructorPlanID(ctx context.Context, cpu, memory, storage, replicas int) (int64, error) {
	var out FlexInt
	err := s.c.call(ctx, dbaasEndpoint, "getConstructorPlanId", map[string]any{
		"cpu":      cpu,
		"memory":   memory,
		"storage":  storage,
		"replicas": replicas,
	}, &out)
	return int64(out), err
}

// DBaaSFirstOrder describes the account's promotional first DBaaS order (method
// "getFirstOrderInfo"), used by the onboarding / clear-first-order flow.
//
// Doc-vs-reality: several numeric fields arrive as quoted strings in the recorded
// example (cpu, memory, storage, sync_replicas, read_replicas) while others come
// bare (replicas, instances, pay_period) — all decoded through FlexInt. Nullable
// per the doc; PricePerMonth is FlexFloat (doc says int but the example returns a
// bare int, and money fields drift to float elsewhere in the API).
type DBaaSFirstOrder struct {
	Plan              string    `json:"plan"`
	Engine            string    `json:"engine"`
	EngineType        string    `json:"engine_type"`
	EngineVersion     string    `json:"engine_version"`
	CPU               FlexInt   `json:"cpu"`
	Memory            FlexInt   `json:"memory"`  // GB
	Storage           FlexInt   `json:"storage"` // GB
	SyncReplicas      FlexInt   `json:"sync_replicas"`
	ReadReplicas      FlexInt   `json:"read_replicas"`
	Replicas          FlexInt   `json:"replicas"`
	Instances         FlexInt   `json:"instances"`
	PricePerMonth     FlexFloat `json:"price_per_month"`
	PayPeriod         FlexInt   `json:"pay_period"` // months
	PlanIsConstructor bool      `json:"plan_is_constructor"`
	ClearAvailable    bool      `json:"clearAvailable"`
	Promocode         string    `json:"promocode"` // "" when null
}

// GetFirstOrderInfo returns the account's DBaaS first-order info (method
// "getFirstOrderInfo"). Read-only.
func (s *DBaaSService) GetFirstOrderInfo(ctx context.Context) (*DBaaSFirstOrder, error) {
	var out DBaaSFirstOrder
	if err := s.c.call(ctx, dbaasEndpoint, "getFirstOrderInfo", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveFirst clears the promotional first order (method "removeFirst"), deleting
// an unstarted cluster created during it. The API answers a bare 1 on success and
// errors with "Доступ запрещен" when there is no first order.
//
// Evidence-first: this mutates and its result shape has not been reconciled
// against a recorded response, so the raw result is returned rather than a guessed
// struct — the transport already surfaces a JSON-RPC error object as *Error.
func (s *DBaaSService) RemoveFirst(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, dbaasEndpoint, "removeFirst", nil, &out)
	return out, err
}

// DBaaSUserCredentials is a cluster user in a create/edit/validate request. Omit
// Password to leave an existing user untouched on edit (see EditInstance).
type DBaaSUserCredentials struct {
	Name     string `json:"name"`
	Password string `json:"password,omitempty"`
}

// CreateInstanceRequest is the createInstance payload. Only EngineType,
// EngineVersion, Users, and PlanID are required; the rest default server-side.
// InstanceOptions/DBOptions/Databases are passed through as-is (their nested
// shapes are loosely specified) — use map[string]any / the documented fields.
type CreateInstanceRequest struct {
	EngineType      string                 `json:"engineType"`
	EngineVersion   string                 `json:"engineVersion"`
	Users           []DBaaSUserCredentials `json:"users"`
	PlanID          int                    `json:"planId"`
	DisplayName     string                 `json:"displayName,omitempty"`
	InstanceOptions any                    `json:"instanceOptions,omitempty"`
	DBDisplayName   string                 `json:"dbDisplayName,omitempty"`
	DBOptions       any                    `json:"dbOptions,omitempty"`
	Databases       any                    `json:"databases,omitempty"`
}

// CreateInstance provisions a new managed-database cluster (method
// "createInstance"). MUTATING and BILLING.
//
// Evidence-first: the documented result is a Success message wrapped as
// {"extendedResult":{code,message,data}}, but this has not been reconciled against
// a recorded live response, so the raw result is returned rather than a guessed
// struct. A JSON-RPC error surfaces as *Error from the transport.
func (s *DBaaSService) CreateInstance(ctx context.Context, req CreateInstanceRequest) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, dbaasEndpoint, "createInstance", req, &out)
	return out, err
}

// EditInstanceRequest is the editInstance payload. Only BillingID is required; a
// nil/omitted field leaves that facet unchanged. Users follows the edit
// semantics: a user with a Password is created, one without is kept, and an
// existing user absent from the list is removed.
type EditInstanceRequest struct {
	BillingID       string                 `json:"billingId"`
	Users           []DBaaSUserCredentials `json:"users,omitempty"`
	PlanID          int                    `json:"planId,omitempty"`
	DisplayName     string                 `json:"displayName,omitempty"`
	InstanceOptions any                    `json:"instanceOptions,omitempty"`
	Databases       any                    `json:"databases,omitempty"`
}

// EditInstance changes a cluster's users/plan/name/replicas/databases (method
// "editInstance"). Sentinel 1/0 result (1 = saved). MUTATING.
func (s *DBaaSService) EditInstance(ctx context.Context, req EditInstanceRequest) error {
	return s.actionOne(ctx, "editInstance", req)
}

// RemoveInstance deletes a managed-database cluster (method "removeInstance").
// DESTRUCTIVE — removes the cluster and its databases. The API documents a bare 1
// on success.
//
// Evidence-first: this mutates and its result shape has not been reconciled
// against a recorded response, so the raw result is returned rather than treated
// as a guessed sentinel. A JSON-RPC error surfaces as *Error from the transport.
func (s *DBaaSService) RemoveInstance(ctx context.Context, billingID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, dbaasEndpoint, "removeInstance", map[string]string{"billingId": billingID}, &out)
	return out, err
}

// DeleteDatabase removes a single database from a cluster (method
// "deleteDatabase"). DESTRUCTIVE. dbName is a DBaaSDatabase.Name from List.
//
// Evidence-first: the documented result is a Success message wrapped as
// {"extendedResult":{…}}, but this has not been reconciled against a recorded
// response, so the raw result is returned rather than a guessed struct. A JSON-RPC
// error surfaces as *Error from the transport.
func (s *DBaaSService) DeleteDatabase(ctx context.Context, billingID, dbName string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.call(ctx, dbaasEndpoint, "deleteDatabase", map[string]string{
		"billingId": billingID,
		"dbName":    dbName,
	}, &out)
	return out, err
}

// ValidateUsers checks a proposed cluster user list (method "validateUsers"),
// returning nil if the list is valid. Read-only: the API answers boolean true on
// success or a validation error message otherwise (surfaced as *Error).
func (s *DBaaSService) ValidateUsers(ctx context.Context, users []DBaaSUserCredentials) error {
	var out bool
	if err := s.c.call(ctx, dbaasEndpoint, "validateUsers", map[string]any{"users": users}, &out); err != nil {
		return err
	}
	if !out {
		return fmt.Errorf("sweb: dbaas validateUsers returned false, want true")
	}
	return nil
}

// actionOne runs a mutating DBaaS method whose success sentinel is integer 1
// (setUpgradeAgree, editInstance): non-1 is an error.
func (s *DBaaSService) actionOne(ctx context.Context, method string, params any) error {
	var out FlexInt
	if err := s.c.call(ctx, dbaasEndpoint, method, params, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: dbaas %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
