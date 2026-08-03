package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const credentialsPath = "/platform/clientauth/v1/credentials"

// CredentialStatus is the lifecycle state of a client credential.
type CredentialStatus string

const (
	CredentialStatusUnspecified CredentialStatus = "CLIENT_CREDENTIAL_STATUS_UNSPECIFIED"
	CredentialStatusActive      CredentialStatus = "CLIENT_CREDENTIAL_STATUS_ACTIVE"
	CredentialStatusRevoked     CredentialStatus = "CLIENT_CREDENTIAL_STATUS_REVOKED"
)

// Credential is a client-auth credential belonging to an organisation. The
// client secret is returned only by CreateCredential.
type Credential struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	ClientID   string           `json:"client_id"`
	KID        string           `json:"kid"`
	Alg        string           `json:"alg"`
	Status     CredentialStatus `json:"status"`
	CreatedAt  time.Time        `json:"created_at"`
	LastUsedAt time.Time        `json:"last_used_at"`
}

// CredentialsClient manages client-auth credentials. It requires
// ScopeClientAuthRead for reads and ScopeClientAuthWrite for writes.
type CredentialsClient struct {
	baseURL    string
	httpClient *http.Client
	editors    []editorFn
}

// ListCredentialsParams filters and paginates a credential listing.
type ListCredentialsParams struct {
	OrgID  string
	Limit  int32
	Offset int32
}

// CreateCredentialParams describes a credential to create. Alg defaults to
// EdDSA when empty; it is the only algorithm the API accepts.
type CreateCredentialParams struct {
	OrgID string
	Name  string
	Alg   string
}

// CreatedCredential pairs a new credential with its secret, which the API
// returns once and never again.
type CreatedCredential struct {
	Credential   Credential `json:"credential"`
	ClientSecret string     `json:"client_secret"`
}

// List returns the organisation's credentials and the total available.
func (c *CredentialsClient) List(ctx context.Context, params ListCredentialsParams) ([]Credential, int64, error) {
	if params.Limit < 0 || params.Limit > 100 {
		return nil, 0, fmt.Errorf("credentials: Limit must be between 1 and 100, got %d", params.Limit)
	}
	if params.Offset < 0 {
		return nil, 0, fmt.Errorf("credentials: Offset must not be negative, got %d", params.Offset)
	}

	query := url.Values{}
	if params.OrgID != "" {
		query.Set("org_id", params.OrgID)
	}
	if params.Limit > 0 {
		query.Set("pagination.limit", strconv.Itoa(int(params.Limit)))
	}
	if params.Offset > 0 {
		query.Set("pagination.offset", strconv.Itoa(int(params.Offset)))
	}

	path := credentialsPath
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var out struct {
		Credentials []Credential `json:"credentials"`
		Total       int64        `json:"total"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, 0, err
	}
	return out.Credentials, out.Total, nil
}

// Create issues a new credential and returns it together with its one-time secret.
func (c *CredentialsClient) Create(ctx context.Context, params CreateCredentialParams) (*CreatedCredential, error) {
	if params.Name == "" {
		return nil, errors.New("credentials: Name is required")
	}
	if len(params.Name) > 64 {
		return nil, fmt.Errorf("credentials: Name must be at most 64 characters, got %d", len(params.Name))
	}
	if params.Alg != "" && params.Alg != "EdDSA" {
		return nil, fmt.Errorf("credentials: Alg must be EdDSA, got %q", params.Alg)
	}

	body := struct {
		OrgID string `json:"org_id,omitempty"`
		Name  string `json:"name"`
		Alg   string `json:"alg,omitempty"`
	}{OrgID: params.OrgID, Name: params.Name, Alg: params.Alg}

	var out CreatedCredential
	if err := c.do(ctx, http.MethodPost, credentialsPath, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Revoke permanently disables a credential. Tokens already minted from it stay
// valid until they expire.
func (c *CredentialsClient) Revoke(ctx context.Context, credentialID, orgID string) error {
	if credentialID == "" {
		return errors.New("credentials: credentialID is required")
	}

	path := credentialsPath + "/" + url.PathEscape(credentialID) + "/revoke"
	if orgID != "" {
		path += "?" + url.Values{"org_id": {orgID}}.Encode()
	}
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

func (c *CredentialsClient) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, edit := range c.editors {
		if err := edit(ctx, req); err != nil {
			return err
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, bytes.TrimSpace(raw))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
