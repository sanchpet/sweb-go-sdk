// Package sweb is a Go client for the SpaceWeb (sweb.ru) hosting API.
//
// The API speaks JSON-RPC 2.0 over HTTPS POST. A Client wraps the transport
// (envelope, auth, error handling); typed operations are grouped into services
// (e.g. Client.VPS) in the spirit of the kubectl/yc clients.
//
// SpaceWeb issues short-lived session tokens and has no refresh-token flow. Use
// WithCredentials so the client transparently re-exchanges login+password for a
// fresh token when the session expires.
package sweb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultBaseURL is the production SpaceWeb API root.
const DefaultBaseURL = "https://api.sweb.ru"

// sessionExpiredCode is the JSON-RPC error code SpaceWeb returns when the
// session token has expired ("Время сеанса истекло").
const sessionExpiredCode = -32603

// Client talks to the SpaceWeb JSON-RPC API. Construct it with New.
type Client struct {
	baseURL    string
	httpClient *http.Client
	idSeq      atomic.Int64

	mu        sync.Mutex
	token     string
	login     string
	password  string
	onRefresh func(string)

	// VPS groups VPS operations (endpoint /vps).
	VPS *VPSService
	// IP groups IP operations (endpoint /vps/ip): local network + public IPs.
	IP *IPService
	// Backup groups local backup operations (endpoint /vps/backup).
	Backup *BackupService
	// RemoteBackup groups cloud backup operations (endpoint /vps/remoteBackup).
	RemoteBackup *RemoteBackupService
	// DNS groups DNS-zone operations (endpoint /domains/dns).
	DNS *DNSService
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API root (useful for tests / staging).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithToken sets the Bearer token used for authenticated endpoints.
func WithToken(t string) Option { return func(c *Client) { c.token = t } }

// WithHTTPClient injects a custom *http.Client (timeouts, transport, test server).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithCredentials enables transparent token refresh: when a call fails because
// the session token expired, the client exchanges login+password for a fresh
// token (getToken) and retries once. Pair with WithOnTokenRefresh to persist it.
func WithCredentials(login, password string) Option {
	return func(c *Client) { c.login, c.password = login, password }
}

// WithOnTokenRefresh registers a callback invoked with the new token whenever
// the client refreshes it — e.g. to cache it in an OS keyring.
func WithOnTokenRefresh(fn func(string)) Option {
	return func(c *Client) { c.onRefresh = fn }
}

// New builds a Client. A token (WithToken) and/or credentials (WithCredentials)
// are optional but required for authenticated endpoints.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	c.VPS = &VPSService{c: c}
	c.IP = &IPService{c: c}
	c.Backup = &BackupService{c: c}
	c.RemoteBackup = &RemoteBackupService{c: c}
	c.DNS = &DNSService{c: c}
	return c
}

// Token returns the current Bearer token (which may have been refreshed).
func (c *Client) Token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

func (c *Client) canRefresh() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.login != "" && c.password != ""
}

// rpcRequest is the JSON-RPC 2.0 request envelope.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// rpcResponse is the JSON-RPC 2.0 response envelope. SpaceWeb is not strict about
// echoing jsonrpc/id (e.g. getToken returns a bare {"result":...}), so we only
// model result + error.
type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *Error          `json:"error"`
}

// call performs a JSON-RPC call with transparent re-auth: if no token is cached
// but credentials are set it fetches one first; if the call fails because the
// session expired it refreshes the token and retries once.
func (c *Client) call(ctx context.Context, endpoint, method string, params, out any) error {
	if c.Token() == "" && c.canRefresh() {
		if err := c.refreshToken(ctx); err != nil {
			return err
		}
	}

	err := c.doCall(ctx, endpoint, method, params, out)

	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.Code == sessionExpiredCode && c.canRefresh() {
		if rerr := c.refreshToken(ctx); rerr != nil {
			return err // keep the original error if re-auth fails
		}
		return c.doCall(ctx, endpoint, method, params, out)
	}
	return err
}

// refreshToken exchanges the stored credentials for a fresh token and notifies
// the onRefresh callback.
func (c *Client) refreshToken(ctx context.Context) error {
	c.mu.Lock()
	login, password := c.login, c.password
	c.mu.Unlock()

	tok, err := c.getToken(ctx, login, password)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.token = tok
	cb := c.onRefresh
	c.mu.Unlock()
	if cb != nil {
		cb(tok)
	}
	return nil
}

// getToken performs the raw login+password → token exchange (no auto-refresh).
func (c *Client) getToken(ctx context.Context, login, password string) (string, error) {
	var token string
	err := c.doCall(ctx, "/notAuthorized/", "getToken", map[string]string{
		"login":    login,
		"password": password,
	}, &token)
	return token, err
}

// doCall performs exactly one JSON-RPC call against endpoint (e.g. "/vps") with
// the given method and params, decoding the result into out (nil to discard).
func (c *Client) doCall(ctx context.Context, endpoint, method string, params, out any) error {
	if params == nil {
		params = struct{}{}
	}
	reqBody, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      strconv.FormatInt(c.idSeq.Add(1), 10),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("sweb: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("sweb: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := c.Token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sweb: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("sweb: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &Error{Code: resp.StatusCode, Message: fmt.Sprintf("http %s: %s", resp.Status, truncate(body, 256))}
	}

	var rpc rpcResponse
	if err := json.Unmarshal(body, &rpc); err != nil {
		return fmt.Errorf("sweb: decode response envelope: %w", err)
	}
	if rpc.Error != nil {
		return rpc.Error
	}
	if out != nil && len(rpc.Result) > 0 {
		if err := json.Unmarshal(rpc.Result, out); err != nil {
			return fmt.Errorf("sweb: decode result: %w", err)
		}
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
