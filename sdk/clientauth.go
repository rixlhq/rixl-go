package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const mintPath = "/platform/clientauth/v1/token"

const refreshLeeway = 60 * time.Second

// ClientCredentials is a client ID/secret pair that mints short-lived tokens
// scoped to one of your end users.
type ClientCredentials struct {
	ClientID     string
	ClientSecret string
	// Subject is your own identifier for the end user the token acts for.
	Subject   string
	ProjectID string
	// TTL is the requested token lifetime, 1-15 minutes. Defaults to 15.
	TTL time.Duration
	// Scopes are the permissions the credential is expected to hold. They are
	// validated locally; the API grants whatever the credential's policies allow.
	Scopes     []Scope
	BaseURL    string
	HTTPClient *http.Client
}

func (cc ClientCredentials) validate() error {
	switch {
	case cc.ClientID == "":
		return errors.New("client credentials: ClientID is required")
	case cc.ClientSecret == "":
		return errors.New("client credentials: ClientSecret is required")
	case cc.Subject == "":
		return errors.New("client credentials: Subject is required")
	}
	if cc.TTL < 0 || cc.TTL > 15*time.Minute {
		return fmt.Errorf("client credentials: TTL must be between 1 and 15 minutes, got %s", cc.TTL)
	}
	if err := validateScopes(cc.Scopes); err != nil {
		return fmt.Errorf("client credentials: %w", err)
	}
	return nil
}

// TokenSource mints and caches client tokens. It is safe for concurrent use.
type TokenSource struct {
	creds ClientCredentials

	mu      sync.Mutex
	token   string
	expires time.Time
}

// NewTokenSource validates creds and returns a token source that mints lazily.
func NewTokenSource(creds ClientCredentials) (*TokenSource, error) {
	if err := creds.validate(); err != nil {
		return nil, err
	}
	if creds.BaseURL == "" {
		creds.BaseURL = baseURL
	}
	if creds.HTTPClient == nil {
		creds.HTTPClient = http.DefaultClient
	}
	return &TokenSource{creds: creds}, nil
}

// Scopes returns the scopes the credential is expected to hold.
func (ts *TokenSource) Scopes() []Scope {
	out := make([]Scope, len(ts.creds.Scopes))
	copy(out, ts.creds.Scopes)
	return out
}

// Token returns a cached token, re-minting when it nears expiry.
func (ts *TokenSource) Token(ctx context.Context) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.token != "" && time.Until(ts.expires) > refreshLeeway {
		return ts.token, nil
	}

	tok, expires, err := ts.mint(ctx)
	if err != nil {
		return "", err
	}
	ts.token, ts.expires = tok, expires
	return tok, nil
}

type mintRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Subject      string `json:"subject"`
	ProjectID    string `json:"project_id,omitempty"`
	TTLMinutes   *int32 `json:"ttl_minutes,omitempty"`
}

type mintResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int64     `json:"expires_in"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (ts *TokenSource) mint(ctx context.Context) (string, time.Time, error) {
	body := mintRequest{
		ClientID:     ts.creds.ClientID,
		ClientSecret: ts.creds.ClientSecret,
		Subject:      ts.creds.Subject,
		ProjectID:    ts.creds.ProjectID,
	}
	if ts.creds.TTL > 0 {
		minutes := int32((ts.creds.TTL + time.Minute - 1) / time.Minute)
		body.TTLMinutes = &minutes
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("encode mint request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.creds.BaseURL+mintPath, bytes.NewReader(payload))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("build mint request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := ts.creds.HTTPClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("mint client token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("read mint response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("mint client token: status %d: %s", resp.StatusCode, bytes.TrimSpace(raw))
	}

	var out mintResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", time.Time{}, fmt.Errorf("decode mint response: %w", err)
	}
	if out.AccessToken == "" {
		return "", time.Time{}, errors.New("mint client token: response contained no access_token")
	}

	expires := out.ExpiresAt
	if expires.IsZero() {
		secs := out.ExpiresIn
		if secs <= 0 {
			secs = 900
		}
		expires = time.Now().Add(time.Duration(secs) * time.Second)
	}
	return out.AccessToken, expires, nil
}

// WithClientCredentials authenticates requests with tokens minted from creds,
// replacing any API key passed to New. Invalid credentials are reported by New.
func WithClientCredentials(creds ClientCredentials) Option {
	return func(c *config) { c.creds = &creds }
}

func resolveCredentials(cfg *config) error {
	if cfg.creds == nil {
		return nil
	}
	creds := *cfg.creds
	if creds.HTTPClient == nil {
		creds.HTTPClient = cfg.httpClient
	}
	ts, err := NewTokenSource(creds)
	if err != nil {
		return err
	}
	cfg.editors = []editorFn{tokenSourceEditor(ts)}
	return nil
}

// WithTokenSource authenticates requests with tokens from ts, letting several
// clients share one cached token.
func WithTokenSource(ts *TokenSource) Option {
	return func(c *config) {
		c.editors = []editorFn{tokenSourceEditor(ts)}
	}
}

func tokenSourceEditor(ts *TokenSource) editorFn {
	return func(ctx context.Context, req *http.Request) error {
		token, err := ts.Token(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
}
