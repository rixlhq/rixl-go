package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newCredentialsClient(t *testing.T, h http.Handler) *CredentialsClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &CredentialsClient{
		baseURL:    srv.URL,
		httpClient: srv.Client(),
		editors:    []editorFn{headerEditor("X-API-Key", "key")},
	}
}

func TestCredentialsListSendsPaginationAndAuth(t *testing.T) {
	t.Parallel()

	var gotQuery, gotAPIKey, gotPath string
	c := newCredentialsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotAPIKey = r.URL.Path, r.URL.RawQuery, r.Header.Get("X-API-Key")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"credentials": []map[string]any{{"id": "Z9Y8X7W6V5", "client_id": "XyZ9aB8cD1", "status": CredentialStatusActive}},
			"total":       7,
		})
	}))

	creds, total, err := c.List(context.Background(), ListCredentialsParams{OrgID: "OR98234j23", Limit: 25, Offset: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if gotPath != credentialsPath {
		t.Errorf("path = %s, want %s", gotPath, credentialsPath)
	}
	for _, want := range []string{"org_id=OR98234j23", "pagination.limit=25", "pagination.offset=50"} {
		if !strings.Contains(gotQuery, strings.ReplaceAll(want, ".", "%2E")) && !strings.Contains(gotQuery, want) {
			t.Errorf("query = %s, want containing %s", gotQuery, want)
		}
	}
	if gotAPIKey != "key" {
		t.Errorf("X-API-Key = %q, want key", gotAPIKey)
	}
	if total != 7 || len(creds) != 1 || creds[0].ID != "Z9Y8X7W6V5" {
		t.Errorf("creds = %+v, total = %d", creds, total)
	}
	if creds[0].Status != CredentialStatusActive {
		t.Errorf("status = %s", creds[0].Status)
	}
}

func TestCredentialsListOmitsUnsetParams(t *testing.T) {
	t.Parallel()

	var gotQuery string
	c := newCredentialsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"credentials":[],"total":0}`))
	}))

	if _, _, err := c.List(context.Background(), ListCredentialsParams{}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}
}

func TestCredentialsListRejectsBadPagination(t *testing.T) {
	t.Parallel()

	c := &CredentialsClient{baseURL: "http://unused", httpClient: http.DefaultClient}
	if _, _, err := c.List(context.Background(), ListCredentialsParams{Limit: 500}); err == nil {
		t.Error("expected error for Limit 500")
	}
	if _, _, err := c.List(context.Background(), ListCredentialsParams{Offset: -1}); err == nil {
		t.Error("expected error for negative Offset")
	}
}

func TestCredentialsCreateReturnsSecret(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	c := newCredentialsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"credential":    map[string]any{"id": "Z9Y8X7W6V5", "name": "Server credential"},
			"client_secret": "rxl_sk_Bq4y3QB38SFpzVPNx1cG",
		})
	}))

	out, err := c.Create(context.Background(), CreateCredentialParams{OrgID: "OR98234j23", Name: "Server credential", Alg: "EdDSA"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotBody["name"] != "Server credential" || gotBody["alg"] != "EdDSA" || gotBody["org_id"] != "OR98234j23" {
		t.Errorf("body = %+v", gotBody)
	}
	if out.ClientSecret != "rxl_sk_Bq4y3QB38SFpzVPNx1cG" || out.Credential.ID != "Z9Y8X7W6V5" {
		t.Errorf("out = %+v", out)
	}
}

func TestCredentialsCreateValidation(t *testing.T) {
	t.Parallel()

	c := &CredentialsClient{baseURL: "http://unused", httpClient: http.DefaultClient}
	cases := map[string]CreateCredentialParams{
		"empty name": {Name: ""},
		"long name":  {Name: strings.Repeat("x", 65)},
		"bad alg":    {Name: "ok", Alg: "RS256"},
	}
	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := c.Create(context.Background(), params); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestCredentialsRevoke(t *testing.T) {
	t.Parallel()

	var gotPath, gotQuery string
	c := newCredentialsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))

	if err := c.Revoke(context.Background(), "Z9Y8X7W6V5", "OR98234j23"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if want := credentialsPath + "/Z9Y8X7W6V5/revoke"; gotPath != want {
		t.Errorf("path = %s, want %s", gotPath, want)
	}
	if gotQuery != "org_id=OR98234j23" {
		t.Errorf("query = %s", gotQuery)
	}
	if err := c.Revoke(context.Background(), "", ""); err == nil {
		t.Error("expected error for empty credential id")
	}
}

func TestCredentialsSurfacesAPIError(t *testing.T) {
	t.Parallel()

	c := newCredentialsClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"missing credentials:clientauth:write"}`))
	}))

	_, err := c.Create(context.Background(), CreateCredentialParams{Name: "x"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "clientauth:write") {
		t.Errorf("error = %v", err)
	}
}
