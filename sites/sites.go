// Package sites groups shared-hosting website operations (endpoint /sites):
// the read side (index/getSiteInfo/getBackEndsList) plus the add/edit/del and
// changeDomainSite/changeBackEnd mutations. All calls dispatch through the
// shared transport.
package sites

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const sitesEndpoint = "/sites"

// Service groups shared-hosting website operations (endpoint /sites):
// index/getSiteInfo/getBackEndsList reads and the add/edit/del,
// changeDomainSite, changeBackEnd mutations.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Site is one shared-hosting website as returned by List (method "index").
//
// Types are reconciled against the spec's recorded example: ID arrives quoted
// ("105394") and AntivirusPrice/AntivirusActive as bare numbers, so both decode
// through flex.Int. AntivirusExpired and DomainTech are nullable strings (null →
// "").
type Site struct {
	ID                   flex.Int `json:"id"`
	DocRoot              string   `json:"docRoot"`
	DocRootFull          string   `json:"docRootFull"`
	Alias                string   `json:"alias"`
	DomainTech           string   `json:"domainTech"`       // nullable
	AntivirusExpired     string   `json:"antivirusExpired"` // nullable date
	AntivirusAvailable   bool     `json:"antivirusAvailable"`
	AntivirusActive      flex.Int `json:"antivirusActive"` // 1 active, 0 inactive
	AntivirusPrice       flex.Int `json:"antivirusPrice"`
	RedisSessionSelected bool     `json:"redisSessionSelected"`
	RedisSessionEnabled  bool     `json:"redisSessionEnabled"`
}

// ListOptions are the optional paging/filter knobs for List. The zero value asks
// for the server default page (all sites).
type ListOptions struct {
	Page    int    // 1-based page number
	PerPage int    // records per page
	Filter  string // filter by site name or domain
}

type listParams struct {
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"perPage,omitempty"`
	Filter  string `json:"filter,omitempty"`
}

// List returns the account's websites (method "index"). Read-only.
func (s *Service) List(ctx context.Context, opts *ListOptions) ([]Site, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	var out []Site
	if err := s.t.Call(ctx, sitesEndpoint, "index", listParams{
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Filter:  opts.Filter,
	}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SiteInfo is the detailed view of one website (method "getSiteInfo").
//
// BackEndID arrives quoted ("8") so it decodes through flex.Int. Domains and
// Program are string arrays; the Redis* booleans expose the tariff/server-derived
// availability of Redis for the site (see the spec for each flag's exact meaning).
type SiteInfo struct {
	BackEnd               string   `json:"backEnd"`   // back-end type description
	BackEndID             flex.Int `json:"backEndId"` // quoted in observed responses
	ViewFiles             bool     `json:"viewFiles"`
	RunScripts            bool     `json:"runScripts"`
	RedisAvailable        bool     `json:"redisAvailable"`
	RedisNeedTransfer     bool     `json:"redisNeedTransfer"`
	RedisEnabled          bool     `json:"redisEnabled"`
	RedisBackendAvailable bool     `json:"redisBackendAvailable"`
	RedisSessionEnabled   bool     `json:"redisSessionEnabled"`
	RedisCanEnableSession bool     `json:"redisCanEnableSession"`
	RedisSessionSelected  bool     `json:"redisSessionSelected"`
	Encoding              string   `json:"encoding"`
	Domains               []string `json:"domains"`
	Program               []string `json:"program"` // installed programs (empty in observed responses)
}

// GetSiteInfo returns the detailed view of a website (method "getSiteInfo").
// docRoot is a Site.DocRoot. Read-only.
func (s *Service) GetSiteInfo(ctx context.Context, docRoot string) (*SiteInfo, error) {
	var out SiteInfo
	if err := s.t.Call(ctx, sitesEndpoint, "getSiteInfo", map[string]any{
		"docRoot": docRoot,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Backend is one selectable web back-end (method "getBackEndsList"): a numeric id
// and its human description (e.g. "Apache 2.4 + PHP 8.1 opcache").
type Backend struct {
	ID   flex.Int `json:"id"`
	Name string   `json:"name"`
}

// BackEndsList returns the back-ends available to assign to a site (method
// "getBackEndsList"). Read-only.
func (s *Service) BackEndsList(ctx context.Context) ([]Backend, error) {
	var out []Backend
	if err := s.t.Call(ctx, sitesEndpoint, "getBackEndsList", struct{}{}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddOptions are the inputs to Add. Alias, DocRoot and Domain are required;
// Machine (subdomain) and EnableRedisSession are optional and omitted when unset.
type AddOptions struct {
	Alias              string // site name
	DocRoot            string // home directory
	Domain             string // domain
	Machine            string // subdomain (optional)
	EnableRedisSession bool   // store sessions in Redis (optional)
}

// Add creates a website (method "add"). MUTATING. Returns on the 1/0 sentinel
// (1 = success).
func (s *Service) Add(ctx context.Context, o AddOptions) error {
	params := map[string]any{
		"alias":              o.Alias,
		"docRoot":            o.DocRoot,
		"domain":             o.Domain,
		"enableRedisSession": o.EnableRedisSession,
	}
	if o.Machine != "" {
		params["machine"] = o.Machine
	}
	return s.sentinelAction(ctx, "add", params)
}

// Edit renames a website and/or moves its docroot (method "edit"). MUTATING.
// docRoot is the current Site.DocRoot; alias is the new name; docRootNew is the
// new directory (empty to keep the current one). Returns on the 1/0 sentinel.
func (s *Service) Edit(ctx context.Context, docRoot, alias, docRootNew string) error {
	params := map[string]any{
		"docRoot": docRoot,
		"alias":   alias,
	}
	if docRootNew != "" {
		params["docRootNew"] = docRootNew
	}
	return s.sentinelAction(ctx, "edit", params)
}

// Del deletes a website (method "del"). MUTATING. docRoot is a Site.DocRoot.
// Returns on the 1/0 sentinel.
func (s *Service) Del(ctx context.Context, docRoot string) error {
	return s.sentinelAction(ctx, "del", map[string]any{"docRoot": docRoot})
}

// ChangeDomainSite repoints a domain at a different website (method
// "changeDomainSite"). MUTATING. domain is the domain to move; docRoot is the
// target Site.DocRoot; machine (subdomain) is optional. Returns on the 1/0
// sentinel.
func (s *Service) ChangeDomainSite(ctx context.Context, domain, docRoot, machine string) error {
	params := map[string]any{
		"domain":  domain,
		"docRoot": docRoot,
	}
	if machine != "" {
		params["machine"] = machine
	}
	return s.sentinelAction(ctx, "changeDomainSite", params)
}

// ChangeBackEnd switches a website's web back-end (method "changeBackEnd").
// MUTATING. docRoot is a Site.DocRoot; backEndID is a Backend.ID from
// BackEndsList. Returns on the 1/0 sentinel.
func (s *Service) ChangeBackEnd(ctx context.Context, docRoot string, backEndID int) error {
	return s.sentinelAction(ctx, "changeBackEnd", map[string]any{
		"docRoot":   docRoot,
		"backEndId": backEndID,
	})
}

// sentinelAction runs a /sites mutation whose success is the integer sentinel 1
// (add/edit/del/changeDomainSite/changeBackEnd all document 1 = success, 0 =
// failure). A real failure usually surfaces as a JSON-RPC error (*apierr.Error)
// via Call; the non-1 check is defensive. The result is decoded via
// json.RawMessage first because this 1/0 sentinel is documented but not yet
// reconciled against a recorded live response — a shape richer than a bare 1
// must not silently pass; only a plain 1 is accepted as success.
func (s *Service) sentinelAction(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, sitesEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: sites %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: sites %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
