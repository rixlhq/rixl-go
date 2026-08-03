package sdk

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

const defaultUploadContentType = "application/octet-stream"

type UploadOption func(*uploadConfig)

type uploadConfig struct {
	contentType string
}

// WithContentType sets the Content-Type header sent with the upload.
func WithContentType(contentType string) UploadOption {
	return func(c *uploadConfig) { c.contentType = contentType }
}

// Upload PUTs body to a presigned upload URL. size must be the exact number of
// bytes in body. If body implements io.Closer, http.Client.Do closes it; wrap
// with io.NopCloser to keep a caller-owned reader open.
func (c *Client) Upload(ctx context.Context, presignedURL string, body io.Reader, size int64, opts ...UploadOption) error {
	cfg := uploadConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedURL, body)
	if err != nil {
		return fmt.Errorf("build upload request: %w", err)
	}
	req.ContentLength = size

	contentType := cfg.contentType
	if contentType == "" {
		contentType = defaultUploadContentType
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload to presigned url: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("upload failed: status %d: %s", resp.StatusCode, snippet)
	}
	return nil
}

// UploadFile uploads the file at path to a presigned upload URL, deriving the
// Content-Type from the file extension unless overridden with WithContentType.
func (c *Client) UploadFile(ctx context.Context, presignedURL, path string, opts ...UploadOption) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open upload file: %w", err)
	}

	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat upload file: %w", err)
	}

	derived := []UploadOption{WithContentType(contentTypeForPath(path))}
	return c.Upload(ctx, presignedURL, f, info.Size(), append(derived, opts...)...)
}

func contentTypeForPath(path string) string {
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		return ct
	}
	return defaultUploadContentType
}
