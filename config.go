package sweb

import "context"

// AvailableConfig is the catalog of selectable options for creating a VPS
// (method "getAvailableConfig"). Shapes confirmed against a real response.
// selectPanel and the kit{} configurator ranges are omitted for now — add them
// when the CLI/provider need the custom-configurator flow.
type AvailableConfig struct {
	VPSPlans    []VPSPlan    `json:"vpsPlans"`
	SelectOS    []OSOption   `json:"selectOs"`
	OSPanel     []OSPanel    `json:"osPanel"`
	Datacenters []Datacenter `json:"datacenters"`
	Categories  []Category   `json:"categories"`
}

// VPSPlan is a purchasable VPS plan. Note SpaceWeb returns several numeric-ish
// fields (cpu_cores, ram, volume_disk) as strings.
type VPSPlan struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	PricePerMonth float64 `json:"price_per_month"` // money: API returns fractional prices
	Category      string  `json:"category"`
	CPUCores      string  `json:"cpu_cores"`
	RAM           string  `json:"ram"`
	DiskType      string  `json:"disk_type"`
	VolumeDisk    string  `json:"volume_disk"`
	Datacenters   []int   `json:"datacenters"`
	SoldOut       bool    `json:"sold_out"`
}

// OSOption is a selectable OS image.
type OSOption struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	OSDistributionID string `json:"os_distribution_id"`
	PlanID           string `json:"plan_id"`
}

// OSPanel maps a distributive/OS to the plans and minimum resources it needs.
type OSPanel struct {
	Distributive     string `json:"distributive"`
	OS               string `json:"os"`
	Panel            string `json:"panel"`
	AvailablePlanIDs []int  `json:"availablePlanIds"`
	MinRAM           int    `json:"minRam"`
	MinStorage       int    `json:"minStorage"`
}

// Datacenter is a location a VPS can be placed in.
type Datacenter struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
	SiteName string `json:"site_name"`
}

// Category groups plans.
type Category struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// AvailableConfig returns the catalog of selectable VPS options (method
// "getAvailableConfig"): plans, OS images, datacenters, categories.
func (s *VPSService) AvailableConfig(ctx context.Context) (*AvailableConfig, error) {
	var out AvailableConfig
	if err := s.c.call(ctx, vpsEndpoint, "getAvailableConfig", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
