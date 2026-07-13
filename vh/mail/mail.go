// Package mail groups SpaceWeb shared-hosting email operations (endpoint
// /vh/mail): the account's mail domains and mailboxes, mailbox lifecycle
// (create/drop/password/comment/antispam), autoreply and SPF toggles,
// forwarding and delivery (mailing) lists, the domain-level mail collector,
// per-mailbox white/black lists, and domain DKIM/SenderVerify/AutoDiscover.
// All calls dispatch through the shared transport.
package mail

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const mailEndpoint = "/vh/mail"

// Service groups shared-hosting email operations (endpoint /vh/mail):
// domains/mailboxes, mailbox lifecycle, autoreply/SPF, forwarding and delivery
// lists, the mail collector, white/black lists, and domain DKIM/SenderVerify.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// FilterInfo is the pagination footer the list methods return alongside their
// "list" array. Numeric fields arrive polymorphic (TotalCount comes bare 1 on
// most lists but quoted "1" on getDeliveryAddressesList), so all decode through
// flex.Int. OrderField/OrderDirect are only present on the sortable lists
// (getDomainsList/getMailboxesList).
type FilterInfo struct {
	Page        flex.Int `json:"page"`
	Limit       flex.Int `json:"limit"`
	TotalCount  flex.Int `json:"totalCount"`
	OrderField  flex.Int `json:"orderField"`  // sortable lists only
	OrderDirect string   `json:"orderDirect"` // "asc"|"desc"; sortable lists only
}

// ListOptions carries the optional pagination/sort inputs shared by the list
// methods. A zero field is omitted from the request so the server default
// applies; Page is 1-based. Not every list honours every field — getDomainsList
// and getMailboxesList sort (OrderBy/OrderDirect), the address lists only page.
type ListOptions struct {
	Page        int    // page number (1-based); 0 = server default
	Limit       int    // rows per page; 0 = server default
	OrderBy     int    // sort field index; sortable lists only
	OrderDirect string // "asc"|"desc"; sortable lists only
}

// apply writes the non-zero ListOptions fields onto a params map.
func (o ListOptions) apply(p map[string]any) {
	if o.Page != 0 {
		p["page"] = o.Page
	}
	if o.Limit != 0 {
		p["limit"] = o.Limit
	}
	if o.OrderBy != 0 {
		p["orderBy"] = o.OrderBy
	}
	if o.OrderDirect != "" {
		p["orderDirect"] = o.OrderDirect
	}
}

// Domain is one mail domain in DomainsList. Numeric flags arrive as 0/1 ints
// (SPF, SenderVerify, AutoDiscover) and decode through flex.Int; DKIM is the
// string "on"/"off" (not an int). EmailCollector is null when no collector is
// set (→ "").
type Domain struct {
	FQDN           string   `json:"fqdn_readable"`
	MailboxesCnt   flex.Int `json:"mailboxesCnt"`
	SPF            flex.Int `json:"spf"`   // 1 on, 0 off
	Quota          flex.Int `json:"quota"` // MB
	SenderVerify   flex.Int `json:"senderVerify"`
	AutoDiscover   flex.Int `json:"autoDiscover"`
	EmailCollector string   `json:"emailCollector"` // "" when null
	DKIM           string   `json:"dkim"`           // "on"|"off"
}

// DomainsList is the result of DomainsList (method "getDomainsList").
type DomainsList struct {
	List       []Domain   `json:"list"`
	FilterInfo FilterInfo `json:"filterInfo"`
}

// Mailbox is one mailbox in MailboxesList. Antispam is the filter level (5 hard,
// 8 medium, 10 soft, 0 off); numeric fields decode through flex.Int.
type Mailbox struct {
	Mbox     string   `json:"mbox"`
	SPF      flex.Int `json:"spf"`   // 1 on, 0 off
	Quota    flex.Int `json:"quota"` // MB
	Purpose  string   `json:"purpose"`
	Antispam flex.Int `json:"antispam"` // 5|8|10|0
	Comment  string   `json:"comment"`
}

// MailboxesList is the result of MailboxesList (method "getMailboxesList").
type MailboxesList struct {
	List       []Mailbox  `json:"list"`
	FilterInfo FilterInfo `json:"filterInfo"`
}

// AddressList is the {list, filterInfo} envelope shared by the delivery-address,
// whitelist and blacklist reads — each a paginated list of bare email strings.
type AddressList struct {
	List       []string   `json:"list"`
	FilterInfo FilterInfo `json:"filterInfo"`
}

// DeliveryInfo is the quota summary from DeliveryInfo (method "getDeliveryInfo"):
// how many mailing groups and delivery addresses exist versus the allowed max.
type DeliveryInfo struct {
	Groups    DeliveryCount `json:"groups"`
	Addresses DeliveryCount `json:"addresses"`
}

// DeliveryCount is a current/max pair inside DeliveryInfo.
type DeliveryCount struct {
	Current flex.Int `json:"current"`
	Max     flex.Int `json:"max"`
}

// Mailbox creation ----------------------------------------------------------

// NewMailbox is the result of CreateMbox (method "createMbox"): the created
// address plus everything needed to hand the user their credentials — the
// webmail URL, mail-client server settings, and a base64 PDF of the requisites.
type NewMailbox struct {
	Login               string           `json:"login"`
	Password            string           `json:"password"`
	WebMail             string           `json:"webMail"`
	MailProgramSettings []ProgramSetting `json:"mailProgramSettings"`
	Detailed            string           `json:"detailed"` // help-page URL
	PDF                 PDFFile          `json:"pdf"`
}

// ProgramSetting is one server row (IMAP/POP3/SMTP) in NewMailbox's mail-client
// setup guidance.
type ProgramSetting struct {
	Name   string `json:"name"`
	Server string `json:"server"`
	Port   string `json:"port"`
}

// PDFFile is the base64 requisites document attached to a NewMailbox. Metadata
// is left raw: the recorded example returns an empty array and its populated
// shape has not been observed.
type PDFFile struct {
	Name     string          `json:"name"`
	Mimetype string          `json:"mimetype"`
	Content  string          `json:"content"` // base64
	Metadata json.RawMessage `json:"metadata"`
}

// Read-only methods ---------------------------------------------------------

// DomainsList returns the account's mail domains with mailbox counts and
// per-domain mail settings (method "getDomainsList"). Read-only. All params are
// optional (pagination/sort); pass a zero ListOptions for the server defaults.
func (s *Service) DomainsList(ctx context.Context, o ListOptions) (*DomainsList, error) {
	p := map[string]any{}
	o.apply(p)
	var out DomainsList
	if err := s.t.Call(ctx, mailEndpoint, "getDomainsList", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MailboxesList returns the mailboxes of one domain (method "getMailboxesList").
// Read-only. domain is required; ListOptions and searchMbox (a name substring
// filter, "" to omit) are optional.
func (s *Service) MailboxesList(ctx context.Context, domain, searchMbox string, o ListOptions) (*MailboxesList, error) {
	p := map[string]any{"domain": domain}
	o.apply(p)
	if searchMbox != "" {
		p["searchMbox"] = searchMbox
	}
	var out MailboxesList
	if err := s.t.Call(ctx, mailEndpoint, "getMailboxesList", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MailQuota returns the total size (MB) of all the client's mailboxes (method
// "getMailQuota"). Read-only.
func (s *Service) MailQuota(ctx context.Context) (int64, error) {
	var out flex.Int
	if err := s.t.Call(ctx, mailEndpoint, "getMailQuota", nil, &out); err != nil {
		return 0, err
	}
	return int64(out), nil
}

// Autoreply returns the autoresponder text for a mailbox (method
// "getAutoreply"). Read-only.
func (s *Service) Autoreply(ctx context.Context, domain, mbox string) (string, error) {
	var out string
	err := s.t.Call(ctx, mailEndpoint, "getAutoreply", map[string]any{"domain": domain, "mbox": mbox}, &out)
	return out, err
}

// ForwardingEmailsList returns the forwarding addresses configured for a mailbox
// (method "getForwardingEmailsList") as a bare list of email strings. Read-only.
func (s *Service) ForwardingEmailsList(ctx context.Context, domain, mbox string) ([]string, error) {
	var out []string
	err := s.t.Call(ctx, mailEndpoint, "getForwardingEmailsList", map[string]any{"domain": domain, "mbox": mbox}, &out)
	return out, err
}

// IsDeletingAfterForwarding reports whether messages are deleted from the source
// mailbox after being forwarded (method "isEnabledDeletingAfterForwarding",
// 1 = on, 0 = off). Read-only.
func (s *Service) IsDeletingAfterForwarding(ctx context.Context, domain, mbox string) (bool, error) {
	var out flex.Int
	if err := s.t.Call(ctx, mailEndpoint, "isEnabledDeletingAfterForwarding", map[string]any{"domain": domain, "mbox": mbox}, &out); err != nil {
		return false, err
	}
	return out == 1, nil
}

// DeliveryAddressesList returns a mailbox's mailing (delivery) addresses (method
// "getDeliveryAddressesList"). Read-only. domain and mbox are required; page/limit
// (from ListOptions) are optional.
//
// Doc-vs-reality: the spec types the result as a bare array, but the recorded
// example returns the {list, filterInfo} envelope (and quotes totalCount as "1"),
// so it is decoded as AddressList like the white/black lists.
func (s *Service) DeliveryAddressesList(ctx context.Context, domain, mbox string, o ListOptions) (*AddressList, error) {
	p := map[string]any{"domain": domain, "mbox": mbox}
	o.apply(p)
	var out AddressList
	if err := s.t.Call(ctx, mailEndpoint, "getDeliveryAddressesList", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeliveryInfo returns the mailing-list and delivery-address quota usage for a
// mailbox (method "getDeliveryInfo"). Read-only.
func (s *Service) DeliveryInfo(ctx context.Context, domain, mbox string) (*DeliveryInfo, error) {
	var out DeliveryInfo
	if err := s.t.Call(ctx, mailEndpoint, "getDeliveryInfo", map[string]any{"domain": domain, "mbox": mbox}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MailsCollector returns the domain's configured mail-collector address, or ""
// if none is set (method "getMailsCollector"). Read-only.
func (s *Service) MailsCollector(ctx context.Context, domain string) (string, error) {
	var out string
	err := s.t.Call(ctx, mailEndpoint, "getMailsCollector", map[string]any{"domain": domain}, &out)
	return out, err
}

// Whitelist returns a mailbox's antispam whitelist (method "getWhitelist").
// Read-only. All four params (domain, mbox, page, limit) are required by the API,
// so callers must pass a non-zero Page/Limit in ListOptions.
func (s *Service) Whitelist(ctx context.Context, domain, mbox string, o ListOptions) (*AddressList, error) {
	return s.addressList(ctx, "getWhitelist", domain, mbox, o)
}

// Blacklist returns a mailbox's antispam blacklist (method "getBlacklist").
// Read-only. domain, mbox, page and limit are all required by the API.
func (s *Service) Blacklist(ctx context.Context, domain, mbox string, o ListOptions) (*AddressList, error) {
	return s.addressList(ctx, "getBlacklist", domain, mbox, o)
}

// addressList runs a paginated white/black-list read and decodes the shared
// {list, filterInfo} envelope.
func (s *Service) addressList(ctx context.Context, method, domain, mbox string, o ListOptions) (*AddressList, error) {
	p := map[string]any{"domain": domain, "mbox": mbox}
	o.apply(p)
	var out AddressList
	if err := s.t.Call(ctx, mailEndpoint, method, p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Mailbox lifecycle (mutating) ----------------------------------------------

// CreateMbox creates a mailbox and returns its credentials, mail-client settings,
// and requisites PDF (method "createMbox"). MUTATING and billable — never exercise
// against the live API in tests. domain, mbox, password and comment are all
// required by the API (pass "" for an empty comment). Unlike the other mutating
// methods this answers a rich object, which is typed as NewMailbox.
func (s *Service) CreateMbox(ctx context.Context, domain, mbox, password, comment string) (*NewMailbox, error) {
	var out NewMailbox
	if err := s.t.Call(ctx, mailEndpoint, "createMbox", map[string]any{
		"domain":   domain,
		"mbox":     mbox,
		"password": password,
		"comment":  comment,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendRequisitesToEmail emails the requisites of an existing mailbox (method
// "sendRequisitesToEmail"). MUTATING (sends mail). email is the recipient, login
// the mailbox whose requisites are sent, password its password. Sentinel 1 result.
func (s *Service) SendRequisitesToEmail(ctx context.Context, email, login, password string) error {
	return s.sentinelAction(ctx, "sendRequisitesToEmail", map[string]any{
		"email":    email,
		"login":    login,
		"password": password,
	})
}

// DropMbox deletes a mailbox (method "dropMbox"). DESTRUCTIVE. Sentinel 1 result.
func (s *Service) DropMbox(ctx context.Context, domain, mbox string) error {
	return s.sentinelAction(ctx, "dropMbox", map[string]any{"domain": domain, "mbox": mbox})
}

// UpdateAntispamState sets a mailbox's antispam filter level (method
// "updateAntispamState"): 5 hard, 8 medium, 10 soft, 0 off. MUTATING. Sentinel 1.
func (s *Service) UpdateAntispamState(ctx context.Context, domain, mbox string, value int) error {
	return s.sentinelAction(ctx, "updateAntispamState", map[string]any{"domain": domain, "mbox": mbox, "value": value})
}

// UpdateComment sets a mailbox's comment (method "updateComment"). MUTATING.
// Sentinel 1 result.
func (s *Service) UpdateComment(ctx context.Context, domain, mbox, comment string) error {
	return s.sentinelAction(ctx, "updateComment", map[string]any{"domain": domain, "mbox": mbox, "comment": comment})
}

// ChangeMailboxPassword sets a new password for a mailbox (method
// "changeMailboxPassword"). MUTATING. Sentinel 1 result.
func (s *Service) ChangeMailboxPassword(ctx context.Context, domain, mbox, password string) error {
	return s.sentinelAction(ctx, "changeMailboxPassword", map[string]any{"domain": domain, "mbox": mbox, "password": password})
}

// DeleteMails deletes a mailbox's messages older than days (method "deleteMails").
// DESTRUCTIVE. Sentinel 1 result.
func (s *Service) DeleteMails(ctx context.Context, domain, mbox string, days int) error {
	return s.sentinelAction(ctx, "deleteMails", map[string]any{"domain": domain, "mbox": mbox, "days": days})
}

// Autoreply / SPF (mutating) ------------------------------------------------

// ChangeAutoreply sets a mailbox's autoresponder text (method "changeAutoreply").
// MUTATING. Sentinel 1 result.
func (s *Service) ChangeAutoreply(ctx context.Context, domain, mbox, text string) error {
	return s.sentinelAction(ctx, "changeAutoreply", map[string]any{"domain": domain, "mbox": mbox, "text": text})
}

// ChangeMailboxSpf toggles SPF filtering for one mailbox (method
// "changeMailboxSpf"). MUTATING. Sentinel 1 result.
func (s *Service) ChangeMailboxSpf(ctx context.Context, domain, mbox string, on bool) error {
	return s.sentinelAction(ctx, "changeMailboxSpf", map[string]any{"domain": domain, "mbox": mbox, "turnOn": on})
}

// ChangeDomainSpf toggles SPF filtering for every mailbox of a domain (method
// "changeDomainSpf"). MUTATING. Sentinel 1 result.
func (s *Service) ChangeDomainSpf(ctx context.Context, domain string, on bool) error {
	return s.sentinelAction(ctx, "changeDomainSpf", map[string]any{"domain": domain, "turnOn": on})
}

// Forwarding (mutating) -----------------------------------------------------

// AddForwardingEmail adds a forwarding address to a mailbox (method
// "addForwardingEmail"). MUTATING. Sentinel 1 result.
func (s *Service) AddForwardingEmail(ctx context.Context, domain, mbox, email string) error {
	return s.sentinelAction(ctx, "addForwardingEmail", map[string]any{"domain": domain, "mbox": mbox, "email": email})
}

// RemoveForwardingEmail removes a forwarding address from a mailbox (method
// "removeForwardingEmail"). MUTATING. Sentinel 1 result.
func (s *Service) RemoveForwardingEmail(ctx context.Context, domain, mbox, email string) error {
	return s.sentinelAction(ctx, "removeForwardingEmail", map[string]any{"domain": domain, "mbox": mbox, "email": email})
}

// ChangeDeletingAfterForwarding toggles whether messages are deleted from the
// source mailbox after forwarding (method "changeDeletingAfterForwarding").
// MUTATING. Sentinel 1 result.
func (s *Service) ChangeDeletingAfterForwarding(ctx context.Context, domain, mbox string, on bool) error {
	return s.sentinelAction(ctx, "changeDeletingAfterForwarding", map[string]any{"domain": domain, "mbox": mbox, "turnOn": on})
}

// Delivery (mailing) lists (mutating) ---------------------------------------

// AddDeliveryAddress adds an address to a mailbox's mailing list (method
// "addDeliveryAddress"). MUTATING. Sentinel 1 result.
func (s *Service) AddDeliveryAddress(ctx context.Context, domain, mbox, email string) error {
	return s.sentinelAction(ctx, "addDeliveryAddress", map[string]any{"domain": domain, "mbox": mbox, "email": email})
}

// DropDeliveryAddress removes an address from a mailbox's mailing list (method
// "dropDeliveryAddress"). MUTATING. Sentinel 1 result.
func (s *Service) DropDeliveryAddress(ctx context.Context, domain, mbox, email string) error {
	return s.sentinelAction(ctx, "dropDeliveryAddress", map[string]any{"domain": domain, "mbox": mbox, "email": email})
}

// Mail collector (mutating) -------------------------------------------------

// ChangeMailsCollector sets the domain's mail-collector address (method
// "changeMailsCollector"). MUTATING. Returns the raw sentinel: 1 = done, 2 = the
// collector targets an address on a domain the client does not own and awaits
// email confirmation (see ConfirmMailsCollectorEmail). Because success is not a
// single value it is surfaced rather than folded into a bool; non-{1,2} is an error.
func (s *Service) ChangeMailsCollector(ctx context.Context, domain, email string) (int64, error) {
	var out flex.Int
	if err := s.t.Call(ctx, mailEndpoint, "changeMailsCollector", map[string]any{"domain": domain, "email": email}, &out); err != nil {
		return 0, err
	}
	if out != 1 && out != 2 {
		return int64(out), fmt.Errorf("sweb: mail changeMailsCollector returned %d, want 1 (done) or 2 (needs confirmation)", int64(out))
	}
	return int64(out), nil
}

// RemoveMailsCollector removes the domain's mail-collector address (method
// "removeMailsCollector"). MUTATING. Sentinel 1 result.
func (s *Service) RemoveMailsCollector(ctx context.Context, domain string) error {
	return s.sentinelAction(ctx, "removeMailsCollector", map[string]any{"domain": domain})
}

// ConfirmMailsCollectorEmail confirms a mail-collector target on a domain not on
// the client's account, using the token emailed to it (method
// "confirmMailsCollectorEmail", 1 = confirmed, 0 = failed). MUTATING. Sentinel 1.
func (s *Service) ConfirmMailsCollectorEmail(ctx context.Context, domain, token string) error {
	return s.sentinelAction(ctx, "confirmMailsCollectorEmail", map[string]any{"domain": domain, "token": token})
}

// White / black lists (mutating) -------------------------------------------

// AddToWhitelist adds an address to a mailbox's antispam whitelist (method
// "addToWhitelist"). MUTATING. When all is true the rule applies to every mailbox
// of the domain. Sentinel 1 result.
func (s *Service) AddToWhitelist(ctx context.Context, domain, mbox, address string, all bool) error {
	return s.sentinelAction(ctx, "addToWhitelist", map[string]any{"domain": domain, "mbox": mbox, "address": address, "all": all})
}

// AddToBlacklist adds an address to a mailbox's antispam blacklist (method
// "addToBlacklist"). MUTATING. When all is true the rule applies to every mailbox
// of the domain. Sentinel 1 result.
//
// Doc-vs-reality: the added address is passed under the param name "email" here
// (not "address" as in AddToWhitelist) — the spec names the params asymmetrically.
func (s *Service) AddToBlacklist(ctx context.Context, domain, mbox, email string, all bool) error {
	return s.sentinelAction(ctx, "addToBlacklist", map[string]any{"domain": domain, "mbox": mbox, "email": email, "all": all})
}

// DropFromWhitelist removes an address from a mailbox's whitelist (method
// "dropFromWhitelist"). MUTATING. Sentinel 1 result.
func (s *Service) DropFromWhitelist(ctx context.Context, domain, mbox, address string) error {
	return s.sentinelAction(ctx, "dropFromWhitelist", map[string]any{"domain": domain, "mbox": mbox, "address": address})
}

// DropFromBlacklist removes an address from a mailbox's blacklist (method
// "dropFromBlacklist"). MUTATING. Sentinel 1 result.
//
// Doc-vs-reality: as with AddToBlacklist the address is passed under "email".
func (s *Service) DropFromBlacklist(ctx context.Context, domain, mbox, email string) error {
	return s.sentinelAction(ctx, "dropFromBlacklist", map[string]any{"domain": domain, "mbox": mbox, "email": email})
}

// Domain-level toggles (mutating) -------------------------------------------

// ChangeSenderVerify toggles sender-address verification for a domain (method
// "changeSenderVerify"). MUTATING. Sentinel 1 result.
func (s *Service) ChangeSenderVerify(ctx context.Context, domain string, on bool) error {
	return s.sentinelAction(ctx, "changeSenderVerify", map[string]any{"domain": domain, "turnOn": on})
}

// ChangeAutoDiscover toggles mail-client auto-configuration for a domain (method
// "changeAutoDiscover"). MUTATING. Sentinel 1 result.
func (s *Service) ChangeAutoDiscover(ctx context.Context, domain string, on bool) error {
	return s.sentinelAction(ctx, "changeAutoDiscover", map[string]any{"domain": domain, "turnOn": on})
}

// EnableDkim enables DKIM signing for a domain (method "enableDkim"). MUTATING.
// Sentinel 1 result.
func (s *Service) EnableDkim(ctx context.Context, domain string) error {
	return s.sentinelAction(ctx, "enableDkim", map[string]any{"domain": domain})
}

// DisableDkim disables DKIM signing for a domain (method "disableDkim").
// MUTATING. Sentinel 1 result.
func (s *Service) DisableDkim(ctx context.Context, domain string) error {
	return s.sentinelAction(ctx, "disableDkim", map[string]any{"domain": domain})
}

// sentinelAction runs a /vh/mail method whose documented success result is the
// integer 1 (the create/change/add/drop/remove family). The result is decoded via
// json.RawMessage first so a shape not yet observed live does not silently pass —
// only a plain 1 is accepted; anything else (0 = failure, or a richer shape) is an
// error. A genuine API failure usually surfaces as a JSON-RPC *apierr.Error via Call.
func (s *Service) sentinelAction(ctx context.Context, method string, params map[string]any) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, mailEndpoint, method, params, &raw); err != nil {
		return err
	}
	var out flex.Int
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("sweb: mail %s returned unexpected result %s: %w", method, raw, err)
	}
	if out != 1 {
		return fmt.Errorf("sweb: mail %s returned %d, want 1 (0 = failure)", method, int64(out))
	}
	return nil
}
