package sdk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New("", WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, srv
}

func TestUploadPutsBodyWithContentLengthAndType(t *testing.T) {
	t.Parallel()

	var (
		gotMethod, gotType, gotBody string
		gotLen                      int64
	)
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotType = r.Header.Get("Content-Type")
		gotLen = r.ContentLength
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))

	body := "hello-bytes"
	err := c.Upload(context.Background(), srv.URL, strings.NewReader(body), int64(len(body)), WithContentType("image/png"))
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotType != "image/png" {
		t.Errorf("content-type = %s, want image/png", gotType)
	}
	if gotLen != int64(len(body)) {
		t.Errorf("content-length = %d, want %d", gotLen, len(body))
	}
	if gotBody != body {
		t.Errorf("body = %q, want %q", gotBody, body)
	}
}

func TestUploadReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("EntityTooLarge"))
	}))

	err := c.Upload(context.Background(), srv.URL, strings.NewReader("x"), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "EntityTooLarge") {
		t.Errorf("error = %v, want status 403 + body", err)
	}
}

func TestUploadFileDerivesContentTypeFromExtension(t *testing.T) {
	t.Parallel()

	var gotType string
	var gotLen int64
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotType = r.Header.Get("Content-Type")
		gotLen = r.ContentLength
		w.WriteHeader(http.StatusOK)
	}))

	dir := t.TempDir()
	path := filepath.Join(dir, "avatar.png")
	content := []byte("\x89PNG fake data")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := c.UploadFile(context.Background(), srv.URL, path); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if !strings.HasPrefix(gotType, "image/png") {
		t.Errorf("content-type = %s, want image/png", gotType)
	}
	if gotLen != int64(len(content)) {
		t.Errorf("content-length = %d, want %d", gotLen, len(content))
	}
}

func TestUploadFileExplicitContentTypeWins(t *testing.T) {
	t.Parallel()

	var gotType string
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))

	dir := t.TempDir()
	path := filepath.Join(dir, "blob.png")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := c.UploadFile(context.Background(), srv.URL, path, WithContentType("application/custom")); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if gotType != "application/custom" {
		t.Errorf("content-type = %s, want application/custom", gotType)
	}
}
