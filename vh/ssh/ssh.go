// Package ssh groups the shared-hosting SSH-access toggle (endpoint /vh/utils):
// turning SSH access on for a fixed period and turning it off. Both calls
// dispatch through the shared transport.
//
// The endpoint is /vh/utils (SpaceWeb's "virtual hosting utilities" server, which
// currently exposes only SSH on/off); the package is named ssh after what it does
// rather than utils, which would name a grab-bag.
package ssh

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const sshEndpoint = "/vh/utils"

// Service groups the shared-hosting SSH-access toggle (endpoint /vh/utils):
// On (with a lease period) and Off.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// On enables SSH access to the shared-hosting account for period hours (method
// "sshOn"). MUTATING. period is 3, 8, or 24 (the values the API documents); it is
// forwarded as-is and the API validates it. Success is the 1/0 sentinel.
func (s *Service) On(ctx context.Context, period int) error {
	return s.sentinelAction(ctx, "sshOn", map[string]any{"period": period})
}

// Off disables SSH access to the shared-hosting account (method "sshOff").
// MUTATING. Takes no parameters. Success is the 1/0 sentinel.
func (s *Service) Off(ctx context.Context) error {
	return s.sentinelAction(ctx, "sshOff", nil)
}

// sentinelAction runs a /vh/utils mutating method whose success is the integer
// sentinel 1 (per the spec both sshOn and sshOff answer resultInt: 1 = success,
// 0 = failure). A real failure usually surfaces as a JSON-RPC error via Call; the
// non-1 check is defensive. The result is decoded via json.RawMessage first so
// that a shape not yet observed live — the 1/0 sentinel is documented but not
// reconciled against a recorded response — does not silently pass: only a plain 1
// is accepted as success.
func (s *Service) sentinelAction(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, sshEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: ssh %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: ssh %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
