package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func mintServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, req mintRequest)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != mintPath {
			t.Errorf("mint path = %s, want %s", r.URL.Path, mintPath)
		}
		var req mintRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode mint request: %v", err)
		}
		handler(w, r, req)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeToken(w http.ResponseWriter, token string, ttl time.Duration) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(mintResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(ttl.Seconds()),
		ExpiresAt:   time.Now().Add(ttl),
	})
}

func testCreds(srv *httptest.Server) ClientCredentials {
	return ClientCredentials{
		ClientID:     "XyZ9aB8cD1",
		ClientSecret: "rxl_sk_secret",
		Subject:      "QwErTy1234",
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
	}
}

func TestTokenSourceMintsWithExpectedBody(t *testing.T) {
	t.Parallel()

	var got mintRequest
	srv := mintServer(t, func(w http.ResponseWriter, _ *http.Request, req mintRequest) {
		got = req
		writeToken(w, "tok-1", 15*time.Minute)
	})

	creds := testCreds(srv)
	creds.ProjectID = "Bq4y3QB38S"
	creds.TTL = 5 * time.Minute
	creds.Scopes = MediaReadScopes

	ts, err := NewTokenSource(creds)
	if err != nil {
		t.Fatalf("NewTokenSource: %v", err)
	}
	tok, err := ts.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}

	if tok != "tok-1" {
		t.Errorf("token = %q, want tok-1", tok)
	}
	if got.ClientID != creds.ClientID || got.ClientSecret != creds.ClientSecret {
		t.Errorf("credentials not sent: %+v", got)
	}
	if got.Subject != "QwErTy1234" || got.ProjectID != "Bq4y3QB38S" {
		t.Errorf("subject/project = %q/%q", got.Subject, got.ProjectID)
	}
	if got.TTLMinutes == nil || *got.TTLMinutes != 5 {
		t.Errorf("ttl_minutes = %v, want 5", got.TTLMinutes)
	}
}

func TestTokenSourceOmitsTTLWhenUnset(t *testing.T) {
	t.Parallel()

	var got mintRequest
	srv := mintServer(t, func(w http.ResponseWriter, _ *http.Request, req mintRequest) {
		got = req
		writeToken(w, "tok", time.Minute)
	})

	ts, err := NewTokenSource(testCreds(srv))
	if err != nil {
		t.Fatalf("NewTokenSource: %v", err)
	}
	if _, err := ts.Token(context.Background()); err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got.TTLMinutes != nil {
		t.Errorf("ttl_minutes = %v, want omitted", *got.TTLMinutes)
	}
	if got.ProjectID != "" {
		t.Errorf("project_id = %q, want omitted", got.ProjectID)
	}
}

func TestTokenSourceCachesUntilNearExpiry(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	calls := 0
	srv := mintServer(t, func(w http.ResponseWriter, _ *http.Request, _ mintRequest) {
		mu.Lock()
		calls++
		mu.Unlock()
		writeToken(w, "tok", 15*time.Minute)
	})

	ts, err := NewTokenSource(testCreds(srv))
	if err != nil {
		t.Fatalf("NewTokenSource: %v", err)
	}
	for range 3 {
		if _, err := ts.Token(context.Background()); err != nil {
			t.Fatalf("Token: %v", err)
		}
	}
	if calls != 1 {
		t.Errorf("mint calls = %d, want 1 (cached)", calls)
	}
}

func TestTokenSourceRemintsWhenExpiryWithinLeeway(t *testing.T) {
	t.Parallel()

	calls := 0
	srv := mintServer(t, func(w http.ResponseWriter, _ *http.Request, _ mintRequest) {
		calls++
		writeToken(w, "tok", refreshLeeway/2)
	})

	ts, err := NewTokenSource(testCreds(srv))
	if err != nil {
		t.Fatalf("NewTokenSource: %v", err)
	}
	for range 2 {
		if _, err := ts.Token(context.Background()); err != nil {
			t.Fatalf("Token: %v", err)
		}
	}
	if calls != 2 {
		t.Errorf("mint calls = %d, want 2", calls)
	}
}

func TestTokenSourceSurfacesMintError(t *testing.T) {
	t.Parallel()

	srv := mintServer(t, func(w http.ResponseWriter, _ *http.Request, _ mintRequest) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid client secret"}`))
	})

	ts, err := NewTokenSource(testCreds(srv))
	if err != nil {
		t.Fatalf("NewTokenSource: %v", err)
	}
	_, err = ts.Token(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid client secret") {
		t.Errorf("error = %v, want status + body", err)
	}
}

func TestNewTokenSourceValidation(t *testing.T) {
	t.Parallel()

	base := ClientCredentials{ClientID: "id", ClientSecret: "secret", Subject: "sub"}

	cases := map[string]struct {
		mutate func(*ClientCredentials)
		want   string
	}{
		"missing client id":     {func(c *ClientCredentials) { c.ClientID = "" }, "ClientID is required"},
		"missing client secret": {func(c *ClientCredentials) { c.ClientSecret = "" }, "ClientSecret is required"},
		"missing subject":       {func(c *ClientCredentials) { c.Subject = "" }, "Subject is required"},
		"ttl too long":          {func(c *ClientCredentials) { c.TTL = time.Hour }, "between 1 and 15 minutes"},
		"unknown scope":         {func(c *ClientCredentials) { c.Scopes = []Scope{"images:read"} }, `unknown scope "images:read"`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			creds := base
			tc.mutate(&creds)
			_, err := NewTokenSource(creds)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestWithClientCredentialsSetsBearerAndReplacesAPIKey(t *testing.T) {
	t.Parallel()

	var gotAuth, gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == mintPath {
			writeToken(w, "minted-token", 15*time.Minute)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	creds := ClientCredentials{
		ClientID:     "id",
		ClientSecret: "secret",
		Subject:      "user-1",
		Scopes:       []Scope{ScopeImagesRead},
		BaseURL:      srv.URL,
	}

	cfg := config{editors: []editorFn{headerEditor("X-API-Key", "api-key")}}
	for _, opt := range []Option{WithClientCredentials(creds), WithHTTPClient(srv.Client())} {
		opt(&cfg)
	}
	if err := resolveCredentials(&cfg); err != nil {
		t.Fatalf("resolveCredentials: %v", err)
	}
	if len(cfg.editors) != 1 {
		t.Fatalf("editors = %d, want 1 (api key replaced)", len(cfg.editors))
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/media/images/v1/images", nil)
	for _, e := range cfg.editors {
		if err := e(context.Background(), req); err != nil {
			t.Fatalf("editor: %v", err)
		}
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if gotAuth != "Bearer minted-token" {
		t.Errorf("Authorization = %q, want Bearer minted-token", gotAuth)
	}
	if gotAPIKey != "" {
		t.Errorf("X-API-Key = %q, want empty (replaced by client auth)", gotAPIKey)
	}
}

func TestNewRejectsInvalidClientCredentials(t *testing.T) {
	t.Parallel()

	_, err := New("", WithClientCredentials(ClientCredentials{ClientID: "id"}))
	if err == nil {
		t.Fatal("expected New to reject incomplete credentials")
	}
	if !strings.Contains(err.Error(), "ClientSecret is required") {
		t.Errorf("error = %v", err)
	}
}

func TestScopeValid(t *testing.T) {
	t.Parallel()

	for _, s := range AllScopes {
		if !s.Valid() {
			t.Errorf("%s should be valid", s)
		}
	}
	for _, s := range []Scope{"", "images:read", "media:images:admin", "media:images:*"} {
		if s.Valid() {
			t.Errorf("%q should be invalid", s)
		}
	}
}
