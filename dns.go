package sweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

const dnsEndpoint = "/domains/dns"

// DNSService groups DNS-zone operations (endpoint /domains/dns): read the zone
// (Records, ZoneFile) and edit records by type (Main/MX/SRV/NS/TXT).
type DNSService struct{ c *Client }

// DNSAction is the operation an edit method performs on a record. The API's
// "action" parameter ("тип операции с записью") is the add/edit/remove
// discriminator on every edit* method. Only "edit" is shown in the apidoc
// examples; "add"/"remove" are the documented parameter's other values and are
// not yet confirmed against a live response — probe before relying on them.
type DNSAction string

// The DNS edit "action" values. Only DNSActionEdit is confirmed against the
// apidoc examples; add/remove are the parameter's other documented values.
const (
	DNSActionAdd    DNSAction = "add"
	DNSActionEdit   DNSAction = "edit"
	DNSActionRemove DNSAction = "remove"
)

// DNSRecord is one record in a zone as returned by the "info" method. The API
// returns a heterogeneous list — a single struct carries every type's fields,
// with only the relevant ones populated per Type. Numeric fields (Priority,
// TTL, Weight, Port, Index, Main) arrive as quoted strings on some record types
// and bare numbers on others, so all decode through FlexInt.
type DNSRecord struct {
	Index    FlexInt `json:"index"`    // record id for edit/remove; per-category, not globally unique
	Type     string  `json:"type"`     // A, AAAA, CNAME, MX, SRV, TXT, NS
	Category string  `json:"category"` // zoneMain, subdom, mx, srv, mainTxt, …
	Name     string  `json:"name"`
	Value    string  `json:"value"`

	// MX, SRV
	Priority FlexInt `json:"priority"`

	// SRV
	Service  string  `json:"service"`
	Protocol string  `json:"protocol"`
	TTL      FlexInt `json:"ttl"`
	Weight   FlexInt `json:"weight"`
	Port     FlexInt `json:"port"`
	Target   string  `json:"target"`

	// A ("A"/… selector and a stringified bool for whether it may change)
	Sel       string `json:"sel"`
	CanChange string `json:"canChange"` // "true"/"false" (stringified bool)

	// TXT
	Domain string  `json:"domain"` // "@" for the apex
	Main   FlexInt `json:"main"`
}

// Records returns the DNS zone's records (method "info"). Read-only.
func (s *DNSService) Records(ctx context.Context, domain string) ([]DNSRecord, error) {
	var raw json.RawMessage
	if err := s.c.call(ctx, dnsEndpoint, "info", map[string]string{"domain": domain}, &raw); err != nil {
		return nil, err
	}
	return parseDNSInfo(raw)
}

// parseDNSInfo extracts the zone records from an "info" result. SpaceWeb returns
// them in one of two shapes: a bare array of records (the normal case), or — for
// a domain attached to a VPS — an object that wraps them in ips (alongside the
// same protected_ips/vps fields the /vps/ip index carries). Tolerate both, and
// the array-of-arrays the object's ips uses.
func parseDNSInfo(raw json.RawMessage) ([]DNSRecord, error) {
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
func flattenDNSRecords(raw json.RawMessage) ([]DNSRecord, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var flat []DNSRecord
	if err := json.Unmarshal(raw, &flat); err == nil {
		return flat, nil
	}
	var nested [][]DNSRecord
	if err := json.Unmarshal(raw, &nested); err != nil {
		return nil, err
	}
	var out []DNSRecord
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
func (s *DNSService) GetFile(ctx context.Context, domain string) (*ZoneFile, error) {
	var out ZoneFile
	if err := s.c.call(ctx, dnsEndpoint, "getFile", map[string]string{"domain": domain}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MainRecord addresses a general zone record (A/AAAA/CNAME/…) for editMain.
type MainRecord struct {
	Index  int    // record id for edit/remove
	Name   string // subdomain name, or "" for the apex
	Type   string // A, AAAA, CNAME, MX, TXT, …
	Value  string
	Prefix string // "префикс или TTL записи"; the apidoc example sends a bare int
}

// EditMain adds/edits/removes a general zone record (method "editMain",
// true=success). Covers the record types without a dedicated method (A, AAAA,
// CNAME, …).
func (s *DNSService) EditMain(ctx context.Context, domain string, action DNSAction, r MainRecord) error {
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
func (s *DNSService) EditMX(ctx context.Context, domain string, action DNSAction, r MXRecord) error {
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
func (s *DNSService) EditSRV(ctx context.Context, domain string, action DNSAction, r SRVRecord) error {
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
func (s *DNSService) EditNS(ctx context.Context, domain string, action DNSAction, index int, subDomain, value string) error {
	return s.editBool(ctx, "editNS", map[string]any{
		"domain":    domain,
		"action":    string(action),
		"index":     index,
		"subDomain": subDomain,
		"value":     value,
	})
}

// EditTXT adds/edits/removes a TXT record (method "editTxt", true=success).
func (s *DNSService) EditTXT(ctx context.Context, domain string, action DNSAction, index int, subDomain, value string) error {
	return s.editBool(ctx, "editTxt", map[string]any{
		"domain":    domain,
		"action":    string(action),
		"index":     index,
		"subDomain": subDomain,
		"value":     value,
	})
}

// editOne runs an edit method whose success sentinel is integer 1 (editMx).
func (s *DNSService) editOne(ctx context.Context, method string, params map[string]any) error {
	var out FlexInt
	if err := s.c.call(ctx, dnsEndpoint, method, params, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: %s returned %d, want 1", method, int64(out))
	}
	return nil
}

// editBool runs an edit method whose success sentinel is boolean true
// (editMain/editSrv/editNS/editTxt). A bad-parameters failure comes back as a
// JSON-RPC error (surfaced by call), so a decoded non-true is defensive.
func (s *DNSService) editBool(ctx context.Context, method string, params map[string]any) error {
	var out bool
	if err := s.c.call(ctx, dnsEndpoint, method, params, &out); err != nil {
		return err
	}
	if !out {
		return fmt.Errorf("sweb: %s returned false, want true", method)
	}
	return nil
}
