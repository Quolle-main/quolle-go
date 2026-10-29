package quolle

import (
	"context"
	"net/url"
)

// APIKeys manages API keys (client.APIKeys). Needs a full-access key; a
// sending-only key gets 403 restricted_api_key.
type APIKeys struct{ client *Client }

// Key scopes.
const (
	ScopeFull    = "full"    // everything an API key can do
	ScopeSending = "sending" // send email and read its status
)

// APIKey is an existing key. The key itself is never returned, only Preview.
type APIKey struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Preview    *string  `json:"preview"`
	Scope      string   `json:"scope"`
	Domains    []string `json:"domains"`
	ExpiresAt  *string  `json:"expiresAt"`
	Expired    bool     `json:"expired"`
	CreatedBy  *string  `json:"createdBy"`
	LastUsedAt *string  `json:"lastUsedAt"`
	CreatedAt  string   `json:"createdAt"`
}

// CreateAPIKeyParams describes a new key.
type CreateAPIKeyParams struct {
	Name          string   `json:"name"`
	Scope         string   `json:"scope,omitempty"`         // ScopeFull (default) or ScopeSending
	Domains       []string `json:"domains,omitempty"`       // limit to these sending domains
	ExpiresInDays int      `json:"expiresInDays,omitempty"` // 0 = no expiry
}

// CreatedAPIKey holds the new key. Key is shown once; store it now.
type CreatedAPIKey struct {
	Key       string   `json:"key"`
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Scope     string   `json:"scope"`
	Domains   []string `json:"domains"`
	ExpiresAt *string  `json:"expiresAt"`
}

// List returns the active keys.
func (k *APIKeys) List(ctx context.Context) ([]APIKey, error) {
	var out struct {
		Keys []APIKey `json:"keys"`
	}
	if err := k.client.do(ctx, "GET", "/v1/keys", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Keys, nil
}

// Create makes a key. The returned Key is shown once; store it now.
func (k *APIKeys) Create(ctx context.Context, params CreateAPIKeyParams) (*CreatedAPIKey, error) {
	var out CreatedAPIKey
	if err := k.client.do(ctx, "POST", "/v1/keys", params, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove revokes a key. Requests made with it fail from now on.
func (k *APIKeys) Remove(ctx context.Context, id string) error {
	return k.client.do(ctx, "DELETE", "/v1/keys/"+url.PathEscape(id), nil, nil, nil)
}
