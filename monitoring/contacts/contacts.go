// Package contacts groups the monitoring-contact operations (endpoint
// /monitoring/contacts): list contacts, add/edit/delete email, phone, and
// Telegram contacts, and drive the Telegram verification flow.
package contacts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanchpet/sweb-go-sdk/flex"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

const contactsEndpoint = "/monitoring/contacts"

// Service groups the monitoring-contact operations (endpoint
// /monitoring/contacts): list contacts, add/edit/delete email, phone, and
// Telegram contacts, and drive the Telegram verification flow.
type Service struct{ t *transport.Client }

// New builds a Service over the shared transport.
func New(t *transport.Client) *Service { return &Service{t: t} }

// Contact types accepted by AddContact.
const (
	ContactEmail    = "email"
	ContactPhone    = "phone"
	ContactTelegram = "telegram"
)

// Contact is one monitoring contact as returned by the list methods. IDs arrive
// as quoted strings on this endpoint, so ID decodes through flex.Int.
type Contact struct {
	ID       flex.Int `json:"id"`
	Type     string   `json:"type"` // "email", "phone", "telegram"
	Name     string   `json:"name"`
	Value    string   `json:"value"`
	Verified bool     `json:"verified"`
	Admin    bool     `json:"admin"` // only set by getAllContacts (is an account admin contact)
}

// FilterInfo is the pagination envelope shared by the list-style results: the
// current page/size and the total row count. Fields arrive as bare numbers here
// but decode through flex.Int for safety.
type FilterInfo struct {
	Page       flex.Int `json:"page"`
	PerPage    flex.Int `json:"perPage"`
	TotalCount flex.Int `json:"totalCount"`
	OrderField string   `json:"orderField"` // only present on the contacts index
	OrderDir   string   `json:"orderDirect"`
}

// ContactList is the paginated result of the "index" method.
type ContactList struct {
	List       []Contact  `json:"list"`
	FilterInfo FilterInfo `json:"filterInfo"`
}

// ListOptions carries optional pagination and ordering for Index.
type ListOptions struct {
	Page       int
	PerPage    int
	OrderField string // e.g. "type"
	OrderDir   string // "asc" | "desc"
}

// Index lists the account's monitoring contacts, paginated, excluding deleted
// and unconfirmed system contacts (method "index"). Read-only.
func (s *Service) Index(ctx context.Context, opts *ListOptions) (*ContactList, error) {
	p := map[string]any{}
	if opts != nil {
		if opts.Page > 0 {
			p["page"] = opts.Page
		}
		if opts.PerPage > 0 {
			p["perPage"] = opts.PerPage
		}
		if opts.OrderField != "" {
			p["orderField"] = opts.OrderField
		}
		if opts.OrderDir != "" {
			p["orderDirect"] = opts.OrderDir
		}
	}
	var out ContactList
	if err := s.t.Call(ctx, contactsEndpoint, "index", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAllContacts lists every monitoring contact on the account, including admin
// contacts (method "getAllContacts"). Read-only. Unlike Index, this returns a
// bare array with the extra "admin" flag.
func (s *Service) GetAllContacts(ctx context.Context) ([]Contact, error) {
	var out []Contact
	if err := s.t.Call(ctx, contactsEndpoint, "getAllContacts", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddContact adds a contact of the given type ("email"/"phone"/"telegram") and
// returns the new contact id (method "addContact"). The spec documents the
// result as "the id of the added contact"; its own example shows 1, but the
// prose is authoritative, so the id is returned as-is.
func (s *Service) AddContact(ctx context.Context, contactType, value, name string) (int64, error) {
	return s.addAction(ctx, "addContact", map[string]any{
		"type":  contactType,
		"value": value,
		"name":  name,
	})
}

// AddEmail adds an email contact and returns its id (method "addEmail").
func (s *Service) AddEmail(ctx context.Context, email, name string) (int64, error) {
	return s.addAction(ctx, "addEmail", map[string]any{"email": email, "name": name})
}

// AddPhone adds a phone contact and returns its id (method "addPhone").
func (s *Service) AddPhone(ctx context.Context, phone, name string) (int64, error) {
	return s.addAction(ctx, "addPhone", map[string]any{"phone": phone, "name": name})
}

// AddTelegram adds a Telegram contact and returns its id (method "addTelegram").
// The contact must then be verified via RequestTelegramVerifyCode +
// VerifyContact before it can receive notifications.
func (s *Service) AddTelegram(ctx context.Context, name string) (int64, error) {
	return s.addAction(ctx, "addTelegram", map[string]any{"name": name})
}

// EditContact updates a contact's value and name (method "editContact").
// Integer 1 success sentinel.
func (s *Service) EditContact(ctx context.Context, contactID, value, name string) error {
	return s.editAction(ctx, "editContact", map[string]any{
		"contactId": contactID,
		"value":     value,
		"name":      name,
	})
}

// EditEmail updates an email contact (method "editEmail"). Integer 1 sentinel.
func (s *Service) EditEmail(ctx context.Context, contactID, email, name string) error {
	return s.editAction(ctx, "editEmail", map[string]any{
		"contactId": contactID,
		"email":     email,
		"name":      name,
	})
}

// EditPhone updates a phone contact (method "editPhone"). Integer 1 sentinel.
func (s *Service) EditPhone(ctx context.Context, contactID, phone, name string) error {
	return s.editAction(ctx, "editPhone", map[string]any{
		"contactId": contactID,
		"phone":     phone,
		"name":      name,
	})
}

// EditTelegram updates a Telegram contact's name (method "editTelegram").
// Integer 1 sentinel.
func (s *Service) EditTelegram(ctx context.Context, contactID, name string) error {
	return s.editAction(ctx, "editTelegram", map[string]any{
		"contactId": contactID,
		"name":      name,
	})
}

// DeleteContact removes one contact (method "deleteContact"). Integer 1 sentinel.
func (s *Service) DeleteContact(ctx context.Context, contactID string) error {
	return s.editAction(ctx, "deleteContact", map[string]any{"contactId": contactID})
}

// DeleteContacts removes several contacts (method "deleteContacts"). The spec is
// internally inconsistent — it types the result as an array yet documents and
// exemplifies integer 1 — so this accepts either integer 1 or a JSON array/true
// as success, a doc-vs-reality gap left tolerant until observed live.
func (s *Service) DeleteContacts(ctx context.Context, contactIDs ...string) error {
	var raw json.RawMessage
	if err := s.t.Call(ctx, contactsEndpoint, "deleteContacts", map[string]any{"contactIds": contactIDs}, &raw); err != nil {
		return err
	}
	switch b := bytes.TrimSpace(raw); {
	case bytes.Equal(b, []byte("1")), bytes.Equal(b, []byte("true")), len(b) > 0 && b[0] == '[':
		return nil
	default:
		return fmt.Errorf("sweb: deleteContacts returned %s, want 1", b)
	}
}

// RequestTelegramVerifyCode requests a verification code for a Telegram contact
// (method "requestTelegramVerifyCode"). The user sends the returned code to the
// bot; then confirm with VerifyContact. Returns the code string.
func (s *Service) RequestTelegramVerifyCode(ctx context.Context, contactID string) (string, error) {
	var out string
	if err := s.t.Call(ctx, contactsEndpoint, "requestTelegramVerifyCode", map[string]any{"contactId": contactID}, &out); err != nil {
		return "", err
	}
	return out, nil
}

// IsVerified reports whether a contact is confirmed (method "isVerified").
// Read-only.
func (s *Service) IsVerified(ctx context.Context, contactID string) (bool, error) {
	var out bool
	if err := s.t.Call(ctx, contactsEndpoint, "isVerified", map[string]any{"contactId": contactID}, &out); err != nil {
		return false, err
	}
	return out, nil
}

// VerifyContact confirms a contact with the verification code (method
// "verifyContact"). Integer 1 success sentinel.
func (s *Service) VerifyContact(ctx context.Context, contactID, code string) error {
	return s.editAction(ctx, "verifyContact", map[string]any{
		"contactId": contactID,
		"code":      code,
	})
}

// addAction runs an add-contact mutation whose result is the new contact id
// (addContact/addEmail/addPhone/addTelegram). A JSON-RPC error surfaces via
// Call; a decoded 0 is treated as failure.
func (s *Service) addAction(ctx context.Context, method string, params any) (int64, error) {
	var out flex.Int
	if err := s.t.Call(ctx, contactsEndpoint, method, params, &out); err != nil {
		return 0, err
	}
	if out == 0 {
		return 0, fmt.Errorf("sweb: %s returned 0 (contact not added)", method)
	}
	return int64(out), nil
}

// editAction runs a contact mutation whose success sentinel is integer 1 (the
// edit/delete/verify family).
func (s *Service) editAction(ctx context.Context, method string, params any) error {
	var out flex.Int
	if err := s.t.Call(ctx, contactsEndpoint, method, params, &out); err != nil {
		return err
	}
	if out != 1 {
		return fmt.Errorf("sweb: %s returned %d, want 1", method, int64(out))
	}
	return nil
}
