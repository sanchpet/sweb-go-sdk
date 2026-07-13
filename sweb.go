// Package sweb is a Go client for the SpaceWeb (sweb.ru) hosting API.
//
// The API speaks JSON-RPC 2.0 over HTTPS POST. Client is a facade: New wires the
// per-service clients (vps, ip, dns, …) over one shared transport and exposes
// them as fields (e.g. Client.VPS), preserving every call site. The transport
// (envelope, Bearer auth with transparent token refresh, error handling) lives
// in internal/transport, so it is unimportable by external consumers.
//
// SpaceWeb issues short-lived session tokens and has no refresh-token flow. Use
// WithCredentials so the client transparently re-exchanges login+password for a
// fresh token when the session expires.
package sweb

import (
	"context"

	"github.com/sanchpet/sweb-go-sdk/backup"
	"github.com/sanchpet/sweb-go-sdk/balancer"
	"github.com/sanchpet/sweb-go-sdk/dbaas"
	"github.com/sanchpet/sweb-go-sdk/dns"
	"github.com/sanchpet/sweb-go-sdk/domains"
	"github.com/sanchpet/sweb-go-sdk/internal/transport"
	"github.com/sanchpet/sweb-go-sdk/ip"
	"github.com/sanchpet/sweb-go-sdk/monitoring"
	"github.com/sanchpet/sweb-go-sdk/monitoring/checks"
	"github.com/sanchpet/sweb-go-sdk/monitoring/contacts"
	"github.com/sanchpet/sweb-go-sdk/remotebackup"
	"github.com/sanchpet/sweb-go-sdk/ssl"
	"github.com/sanchpet/sweb-go-sdk/vps"
)

// DefaultBaseURL is the production SpaceWeb API root.
const DefaultBaseURL = transport.DefaultBaseURL

// Option configures a Client. Options are defined in the transport and
// re-exported here so callers keep using sweb.WithToken(…) etc.
type Option = transport.Option

// The functional options, re-exported from the transport.
var (
	// WithBaseURL overrides the API root (useful for tests / staging).
	WithBaseURL = transport.WithBaseURL
	// WithToken sets the Bearer token used for authenticated endpoints.
	WithToken = transport.WithToken
	// WithHTTPClient injects a custom *http.Client (timeouts, transport, test server).
	WithHTTPClient = transport.WithHTTPClient
	// WithCredentials enables transparent token refresh: when a call fails because
	// the session token expired, the client exchanges login+password for a fresh
	// token (getToken) and retries once. Pair with WithOnTokenRefresh to persist it.
	WithCredentials = transport.WithCredentials
	// WithOnTokenRefresh registers a callback invoked with the new token whenever
	// the client refreshes it — e.g. to cache it in an OS keyring.
	WithOnTokenRefresh = transport.WithOnTokenRefresh
)

// Client talks to the SpaceWeb JSON-RPC API. Construct it with New. It is a
// facade: each field is a service client sharing one transport.
type Client struct {
	t *transport.Client

	// VPS groups VPS operations (endpoint /vps).
	VPS *vps.Service
	// IP groups IP operations (endpoint /vps/ip): local network + public IPs.
	IP *ip.Service
	// Backup groups local backup operations (endpoint /vps/backup).
	Backup *backup.Service
	// RemoteBackup groups cloud backup operations (endpoint /vps/remoteBackup).
	RemoteBackup *remotebackup.Service
	// DNS groups DNS-zone operations (endpoint /domains/dns).
	DNS *dns.Service
	// Domains groups domain/subdomain operations (endpoint /domains).
	Domains *domains.Service
	// Balancer groups load-balancer operations (endpoint /balancer).
	Balancer *balancer.Service
	// DBaaS groups managed-database operations (endpoint /dbaas).
	DBaaS *dbaas.Service
	// SSL groups VPS SSL-certificate operations (endpoint /vps/ssl).
	SSL *ssl.Service
	// Monitoring groups monitoring-tariff operations (endpoint /monitoring).
	Monitoring *monitoring.Service
	// MonitoringChecks groups monitoring-check operations (endpoint /monitoring/checks).
	MonitoringChecks *checks.Service
	// MonitoringContacts groups monitoring-contact operations (endpoint /monitoring/contacts).
	MonitoringContacts *contacts.Service
}

// New builds a Client. A token (WithToken) and/or credentials (WithCredentials)
// are optional but required for authenticated endpoints.
func New(opts ...Option) *Client {
	t := transport.New(opts...)
	return &Client{
		t:                  t,
		VPS:                vps.New(t),
		IP:                 ip.New(t),
		Backup:             backup.New(t),
		RemoteBackup:       remotebackup.New(t),
		DNS:                dns.New(t),
		Domains:            domains.New(t),
		Balancer:           balancer.New(t),
		DBaaS:              dbaas.New(t),
		SSL:                ssl.New(t),
		Monitoring:         monitoring.New(t),
		MonitoringChecks:   checks.New(t),
		MonitoringContacts: contacts.New(t),
	}
}

// Token returns the current Bearer token (which may have been refreshed).
func (c *Client) Token() string { return c.t.Token() }

// CreateToken exchanges a login + password for a personal access token via the
// unauthenticated endpoint (/notAuthorized/, method getToken). The returned
// token is then supplied via WithToken for authenticated calls.
func (c *Client) CreateToken(ctx context.Context, login, password string) (string, error) {
	return c.t.GetToken(ctx, login, password)
}
