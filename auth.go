package sweb

import "context"

// CreateToken exchanges a login + password for a personal access token via the
// unauthenticated endpoint (/notAuthorized/, method getToken). The returned
// token is then supplied via WithToken for authenticated calls.
func (c *Client) CreateToken(ctx context.Context, login, password string) (string, error) {
	var token string
	err := c.call(ctx, "/notAuthorized/", "getToken", map[string]string{
		"login":    login,
		"password": password,
	}, &token)
	return token, err
}
