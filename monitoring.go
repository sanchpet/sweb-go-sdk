package sweb

import (
	"context"
	"fmt"
)

const monitoringEndpoint = "/monitoring"

// MonitoringService groups the monitoring-service subscription operations
// (endpoint /monitoring): enable, disable, and change the monitoring tariff, and
// list the available plans. The checks and contacts objects live in their own
// services (Client.MonitoringChecks, Client.MonitoringContacts).
type MonitoringService struct{ c *Client }

// MonitoringPlan is one available monitoring tariff plan (method "plans"): how
// many checks and SMS notifications it grants and its monthly price.
type MonitoringPlan struct {
	ID     FlexInt   `json:"id"`
	Name   string    `json:"name"`
	Checks FlexInt   `json:"checks"` // number of checks the plan allows
	SMS    FlexInt   `json:"sms"`    // number of SMS notifications per period
	Price  FlexFloat `json:"price"`  // money: the spec documents price as float
}

// Plans lists the available monitoring tariff plans (method "plans"). Read-only.
func (s *MonitoringService) Plans(ctx context.Context) ([]MonitoringPlan, error) {
	var out []MonitoringPlan
	if err := s.c.call(ctx, monitoringEndpoint, "plans", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Enable subscribes the account to the monitoring tariff with the given plan id
// (method "enable"). This BILLS. Integer 1/0 success sentinel.
func (s *MonitoringService) Enable(ctx context.Context, planID int) error {
	return s.tariffAction(ctx, "enable", planID)
}

// Disable cancels the monitoring tariff subscription (method "disable"). Integer
// 1/0 success sentinel.
func (s *MonitoringService) Disable(ctx context.Context, planID int) error {
	return s.tariffAction(ctx, "disable", planID)
}

// Change switches the monitoring subscription to a different tariff plan (method
// "change"). This may BILL. Integer 1/0 success sentinel.
func (s *MonitoringService) Change(ctx context.Context, planID int) error {
	return s.tariffAction(ctx, "change", planID)
}

// tariffAction runs a monitoring-tariff mutation (enable/disable/change) whose
// success sentinel is integer 1 (0 = failure), per the spec's resultEnable/
// resultDisable/resultChange descriptors.
func (s *MonitoringService) tariffAction(ctx context.Context, method string, planID int) error {
	var out FlexInt
	if err := s.c.call(ctx, monitoringEndpoint, method, map[string]any{"id": planID}, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
