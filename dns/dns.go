// Package dns groups DNS-zone operations (endpoint /domains/dns): read the zone
// (Records, ZoneFile) and edit records by type (Main/MX/SRV/NS/TXT). All calls
// dispatch through the shared transport.
package dns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const dnsEndpoint = "/domains/dns"

// Service groups DNS-zone operations (endpoint /domains/dns): read the zone
// (Records, ZoneFile) and edit records by type (Main/MX/SRV/NS/TXT).
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Action is the operation an edit method performs on a record — the
// add/edit/remove discriminator. This is the SDK's logical token; the wire
// "action" value can differ (remove is sent as "del", see editDelete).
//
// add and edit are confirmed against live responses (add via a live editTxt
// probe, edit via the apidoc examples). remove is routed through editDelete,
// whose wire shape ("del" + a "type" discriminator, no subDomain/value) was
// observed live for editTxt; the same shape is applied to the other record
// types by inference.
type Action string

// The DNS edit action values (SDK-logical). ActionRemove maps to the wire
// verb "del" inside editDelete — it is not sent verbatim.
const (
	ActionAdd    Action = "add"
	ActionEdit   Action = "edit"
	ActionRemove Action = "remove"
)

// Record is one record in a zone as returned by the "info" method. The API
// returns a heterogeneous list — a single struct carries every type's fields,
// with only the relevant ones populated per Type. Numeric fields (Priority,
// TTL, Weight, Port, Index, Main) arrive as quoted strings on some record types
// and bare numbers on others, so all decode through flex.Int.
type Record struct {
	Index    flex.Int `json:"index"`    // record id for edit/remove; per-category, not globally unique
	Type     string   `json:"type"`     // A, AAAA, CNAME, MX, SRV, TXT, NS
	Category string   `json:"category"` // zoneMain, subdom, mx, srv, mainTxt, …
	Name     string   `json:"name"`
	Value    string   `json:"value"`

	// MX, SRV
	Priority flex.Int `json:"priority"`

	// SRV
	Service  string   `json:"service"`
	Protocol string   `json:"protocol"`
	TTL      flex.Int `json:"ttl"`
	Weight   flex.Int `json:"weight"`
	Port     flex.Int `json:"port"`
	Target   string   `json:"target"`

	// A ("A"/… selector and a stringified bool for whether it may change)
	Sel       string `json:"sel"`
	CanChange string `json:"canChange"` // "true"/"false" (stringified bool)

	// TXT
	Domain string   `json:"domain"` // "@" for the apex
	Main   flex.Int `json:"main"`
}

// Records returns the DNS zone's records (method "info"). Read-only.
func (s *Service) Records(ctx context.Context, domain string) ([]Record, error) {
	var raw json.RawMessage
	if err := s.t.Call(ctx, dnsEndpoint, "info", map[string]string{"domain": domain}, &raw); err != nil {
		return nil, err
	}
	return parseDNSInfo(raw)
}

// parseDNSInfo extracts the zone records from an "info" result. SpaceWeb returns
// them in one of two shapes: a bare array of records (the normal case), or — for
// a domain attached to a VPS — an object that wraps them in ips (alongside the
// same protected_ips/vps fields the /vps/ip index carries). Tolerate both, and
// the array-of-arrays the object's ips uses.
func parseDNSInfo(raw json.RawMessage) ([]Record, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	if raw[0] == '{' {
		var env struct {
			IPs json.RawMessage `json:"ips"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, err
		}
		return flattenDNSRecords(env.IPs)
	}
	return flattenDNSRecords(raw)
}

// flattenDNSRecords decodes a records array that is either flat ([{…}]) or
// nested one level ([[{…}]]).
func flattenDNSRecords(raw json.RawMessage) ([]Record, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var flat []Record
	if err := json.Unmarshal(raw, &flat); err == nil {
		return flat, nil
	}
	var nested [][]Record
	if err := json.Unmarshal(raw, &nested); err != nil {
		return nil, err
	}
	var out []Record
	for _, g := range nested {
		out = append(out, g...)
	}
	return out, nil
}

// ZoneFile is the raw BIND-style zone file returned by "getFile".
type ZoneFile struct {
	Mimetype string          `json:"mimetype"`
	Metadata json.RawMessage `json:"metadata"` // [] in observed responses; shape not yet pinned
	Content  string          `json:"content"`
	Name     string          `json:"name"`
}

// GetFile returns the raw zone-file contents for a domain (method "getFile").
// Read-only.
func (s *Service) GetFile(ctx context.Context, domain string) (*ZoneFile, error) {
	var out ZoneFile
	if err := s.t.Call(ctx, dnsEndpoint, "getFile", map[string]string{"domain": domain}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MainRecord addresses a general zone record (A/AAAA/CNAME/…) for editMain.
type MainRecord struct {
	Index int    // record id for edit/remove
	Name  string // subdomain name, or "" for the apex
	Type  string // A, AAAA, CNAME, MX, TXT, …
	Value string
	// Prefix is a name label prepended to Name (e.g. Prefix "600" + Name
	// "probe" → "600.probe"). The apidoc calls it "префикс или TTL", but a live
	// probe showed it is NOT a TTL: A/CNAME records carry no per-record TTL
	// (they inherit the zone default), so this only ever shifts the host name.
	Prefix string
}

// EditMain adds/edits/removes a general zone record (method "editMain",
// true=success). Covers the record types without a dedicated method (A, AAAA,
// CNAME, …).
func (s *Service) EditMain(ctx context.Context, domain string, action Action, r MainRecord) error {
	if action == ActionRemove {
		return s.editDelete(ctx, "editMain", r.Type, domain, r.Index)
	}
	return s.editBool(ctx, "editMain", map[string]any{
		"domain": domain,
		"action": string(action),
		"index":  r.Index,
		"name":   r.Name,
		"type":   r.Type,
		"value":  r.Value,
		"prefix": r.Prefix,
	})
}

// MXRecord addresses an MX record for editMx.
type MXRecord struct {
	Index     int
	Priority  int
	Value     string // mail server
	SubDomain string // subdomain if not for the main domain
}

// EditMX adds/edits/removes an MX record (method "editMx", 1=success).
func (s *Service) EditMX(ctx context.Context, domain string, action Action, r MXRecord) error {
	if action == ActionRemove {
		return s.editDelete(ctx, "editMx", "MX", domain, r.Index)
	}
	// editMx is the one edit method answering with integer 1 rather than boolean true.
	return s.editOne(ctx, "editMx", map[string]any{
		"domain":    domain,
		"action":    string(action),
		"index":     r.Index,
		"priority":  r.Priority,
		"value":     r.Value,
		"subDomain": r.SubDomain,
	})
}

// SRVRecord addresses an SRV record for editSrv.
type SRVRecord struct {
	Index     int
	Priority  int
	TTL       int
	Weight    int
	Target    string
	Service   string
	Protocol  string
	Port      int
	SubDomain string
}

// EditSRV adds/edits/removes an SRV record (method "editSrv", true=success).
func (s *Service) EditSRV(ctx context.Context, domain string, action Action, r SRVRecord) error {
	if action == ActionRemove {
		return s.editDelete(ctx, "editSrv", "SRV", domain, r.Index)
	}
	return s.editBool(ctx, "editSrv", map[string]any{
		"domain":    domain,
		"action":    string(action),
		"index":     r.Index,
		"priority":  r.Priority,
		"ttl":       r.TTL,
		"weight":    r.Weight,
		"target":    r.Target,
		"service":   r.Service,
		"protocol":  r.Protocol,
		"port":      r.Port,
		"subDomain": r.SubDomain,
	})
}

// EditNS adds/edits/removes an NS record (method "editNS", true=success).
func (s *Service) EditNS(ctx context.Context, domain string, action Action, index int, subDomain, value string) error {
	if action == ActionRemove {
		return s.editDelete(ctx, "editNS", "NS", domain, index)
	}
	return s.editBool(ctx, "editNS", map[string]any{
		"domain":    domain,
		"action":    string(action),
		"index":     index,
		"subDomain": subDomain,
		"value":     value,
	})
}

// EditTXT adds/edits/removes a TXT record (method "editTxt", true=success).
func (s *Service) EditTXT(ctx context.Context, domain string, action Action, index int, subDomain, value string) error {
	if action == ActionRemove {
		return s.editDelete(ctx, "editTxt", "TXT", domain, index)
	}
	return s.editBool(ctx, "editTxt", map[string]any{
		"domain":    domain,
		"action":    string(action),
		"index":     index,
		"subDomain": subDomain,
		"value":     value,
	})
}

// editOne runs an edit method whose success sentinel is integer 1 (editMx).
func (s *Service) editOne(ctx context.Context, method string, params map[string]any) error {
	var out flex.Int
	if err := s.t.Call(ctx, dnsEndpoint, method, params, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: %s returned %d, want 1", method, int64(out))
	}
	return nil
}

// editDelete removes a record via its per-type edit method. Deletion uses a
// param shape distinct from add/edit: the wire action "del" plus a "type"
// discriminator, addressing the record by index — no subDomain/value. This was
// observed live for editTxt (a TXT delete sends {domain, action:"del", index,
// type:"TXT"}); the other record types are deleted the same way by inference.
// The del success sentinel is not observed per method, so accept either the
// integer 1 or boolean true.
func (s *Service) editDelete(ctx context.Context, method, recordType, domain string, index int) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, dnsEndpoint, method, map[string]any{
		"domain": domain,
		"action": "del",
		"index":  index,
		"type":   recordType,
	}, &raw); err != nil {
		return err
	}
	switch b := bytes.TrimSpace(raw); {
	case bytes.Equal(b, []byte("1")), bytes.Equal(b, []byte("true")):
		return nil
	default:
		return fmt.Errorf("sweb: %s del returned %s, want 1 or true", method, b)
	}
}

// editBool runs an edit method whose success sentinel is boolean true
// (editMain/editSrv/editNS/editTxt). A bad-parameters failure comes back as a
// JSON-RPC error (surfaced by call), so a decoded non-true is defensive.
func (s *Service) editBool(ctx context.Context, method string, params map[string]any) error {
	var out bool
	if err := s.t.Call(ctx, dnsEndpoint, method, params, &out); err != nil {
		return err
	}
	if !out {
		return fmt.Errorf("sweb: %s returned false, want true", method)
	}
	return nil
}
