// Package sweb is a Go client for the SpaceWeb (sweb.ru) hosting API.
//
// The API speaks JSON-RPC 2.0 over HTTPS POST. A Client wraps the transport
// (envelope, auth, error handling); typed operations are grouped into services
// (e.g. Client.VPS) in the spirit of the kubectl/yc clients.
package sweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// DefaultBaseURL is the production SpaceWeb API root.
const DefaultBaseURL = "https://api.sweb.ru"

// Client talks to the SpaceWeb JSON-RPC API. Construct it with New.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	idSeq      atomic.Int64

	// VPS groups VPS operations (endpoint /vps).
	VPS *VPSService
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API root (useful for tests / staging).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithToken sets the Bearer token used for authenticated endpoints.
func WithToken(t string) Option { return func(c *Client) { c.token = t } }

// WithHTTPClient injects a custom *http.Client (timeouts, transport, test server).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// New builds a Client. The token is optional — it is required for authenticated
// endpoints but not for CreateToken.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	c.VPS = &VPSService{c: c}
	return c
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

// call performs one JSON-RPC call against endpoint (e.g. "/vps") with the given
// method and params, decoding the result into out (out may be nil to discard it).
func (c *Client) call(ctx context.Context, endpoint, method string, params, out any) error {
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
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
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
