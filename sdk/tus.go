package sdk

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"maps"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultTusEndpoint  = "https://uploads.rixl.com/media/v1/uploads/"
	tusVersion          = "1.0.0"
	tusOffsetType       = "application/offset+octet-stream"
	defaultChunkSize    = 50 << 20
	defaultMaxRetries   = 5
	defaultRetryBackoff = 500 * time.Millisecond
	maxRetryBackoff     = 30 * time.Second
)

// UploadKind selects the record the upload service allocates for a resumable
// upload.
type UploadKind string

const (
	UploadKindImage UploadKind = "image"
	UploadKindVideo UploadKind = "video"
)

// ResumableUploadParams describes the media a resumable upload creates.
type ResumableUploadParams struct {
	ProjectID string
	Kind      UploadKind
	FileName  string
}

// ResumableUpload is a finished resumable upload. FileID is the image or video
// id the upload service allocated for it.
type ResumableUpload struct {
	URL    string
	FileID string
	Size   int64
}

// TusError is a tus request the upload service rejected.
type TusError struct {
	Op         string
	StatusCode int
	Body       string
}

func (e *TusError) Error() string {
	return fmt.Sprintf("tus %s: status %d: %s", e.Op, e.StatusCode, e.Body)
}

func (e *TusError) retryable() bool {
	switch e.StatusCode {
	case http.StatusLocked, http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// ResumeStore remembers the upload URL for a fingerprint so an interrupted
// upload can continue, including from another process when the store is
// durable.
type ResumeStore interface {
	Get(ctx context.Context, fingerprint string) (string, bool, error)
	Set(ctx context.Context, fingerprint, uploadURL string) error
	Delete(ctx context.Context, fingerprint string) error
}

// MemoryResumeStore is a ResumeStore that lives as long as the process.
type MemoryResumeStore struct {
	mu   sync.Mutex
	urls map[string]string
}

// NewMemoryResumeStore returns an empty MemoryResumeStore.
func NewMemoryResumeStore() *MemoryResumeStore {
	return &MemoryResumeStore{urls: map[string]string{}}
}

func (s *MemoryResumeStore) Get(_ context.Context, fingerprint string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.urls[fingerprint]
	return u, ok, nil
}

func (s *MemoryResumeStore) Set(_ context.Context, fingerprint, uploadURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urls[fingerprint] = uploadURL
	return nil
}

func (s *MemoryResumeStore) Delete(_ context.Context, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.urls, fingerprint)
	return nil
}

type ResumableOption func(*resumableConfig)

type resumableConfig struct {
	chunkSize    int64
	maxRetries   int
	retryBackoff time.Duration
	progress     func(uploaded, total int64)
	store        ResumeStore
	fingerprint  string
}

// WithChunkSize sets how many bytes each PATCH carries. It defaults to 50 MiB,
// the upload service's S3 part size.
func WithChunkSize(n int64) ResumableOption {
	return func(c *resumableConfig) { c.chunkSize = n }
}

// WithRetries sets how many consecutive failed requests are retried and the
// initial backoff, which doubles per attempt up to 30s. Zero retries disables
// retrying.
func WithRetries(maxRetries int, backoff time.Duration) ResumableOption {
	return func(c *resumableConfig) {
		c.maxRetries = maxRetries
		c.retryBackoff = backoff
	}
}

// WithProgress calls fn with the bytes the server has confirmed after every
// chunk.
func WithProgress(fn func(uploaded, total int64)) ResumableOption {
	return func(c *resumableConfig) { c.progress = fn }
}

// WithResumeStore replaces the client's in-memory store, for example with one
// backed by disk so uploads resume across restarts.
func WithResumeStore(store ResumeStore) ResumableOption {
	return func(c *resumableConfig) { c.store = store }
}

// WithFingerprint sets the key an upload is remembered under. It defaults to a
// key derived from the params and size, plus path and mtime for files.
func WithFingerprint(fingerprint string) ResumableOption {
	return func(c *resumableConfig) { c.fingerprint = fingerprint }
}

// WithUploadEndpoint points resumable uploads at a different tus endpoint.
func WithUploadEndpoint(endpoint string) Option {
	return func(c *config) { c.tusEndpoint = endpoint }
}

// UploadResumable sends body to the RIXL upload service over tus, creating
// the image or video record described by params. An upload interrupted by a
// network error, a server error or a cancelled context resumes from the
// server's offset on the next call with the same fingerprint.
func (c *Client) UploadResumable(ctx context.Context, params ResumableUploadParams, body io.ReaderAt, size int64, opts ...ResumableOption) (*ResumableUpload, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, fmt.Errorf("resumable upload: size must not be negative, got %d", size)
	}

	cfg := resumableConfig{
		chunkSize:    defaultChunkSize,
		maxRetries:   defaultMaxRetries,
		retryBackoff: defaultRetryBackoff,
		store:        c.resumeStore,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.chunkSize <= 0 {
		return nil, fmt.Errorf("resumable upload: chunk size must be positive, got %d", cfg.chunkSize)
	}
	if cfg.fingerprint == "" {
		cfg.fingerprint = params.fingerprint(size)
	}

	u := &tusUpload{client: c, cfg: cfg, params: params, body: body, size: size}
	return u.run(ctx)
}

// UploadFileResumable uploads the file at path with UploadResumable.
func (c *Client) UploadFileResumable(ctx context.Context, params ResumableUploadParams, path string, opts ...ResumableOption) (*ResumableUpload, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open upload file: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat upload file: %w", err)
	}
	if params.FileName == "" {
		params.FileName = filepath.Base(path)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	derived := make([]ResumableOption, 0, 1+len(opts))
	derived = append(derived, WithFingerprint(params.fingerprint(info.Size())+"|"+abs+"|"+strconv.FormatInt(info.ModTime().UnixNano(), 10)))
	derived = append(derived, opts...)
	return c.UploadResumable(ctx, params, f, info.Size(), derived...)
}

// TerminateUpload deletes an unfinished resumable upload from the server.
func (c *Client) TerminateUpload(ctx context.Context, uploadURL string) error {
	resp, err := c.tusDo(ctx, http.MethodDelete, uploadURL, nil, 0, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		return tusErrorFrom("terminate", resp)
	}
	return nil
}

func (p ResumableUploadParams) validate() error {
	if p.ProjectID == "" {
		return errors.New("resumable upload: ProjectID is required")
	}
	if p.Kind != UploadKindImage && p.Kind != UploadKindVideo {
		return fmt.Errorf("resumable upload: unsupported Kind %q", p.Kind)
	}
	return nil
}

func (p ResumableUploadParams) fingerprint(size int64) string {
	return strings.Join([]string{p.ProjectID, string(p.Kind), p.FileName, strconv.FormatInt(size, 10)}, "|")
}

func (p ResumableUploadParams) metadata() string {
	pairs := []string{
		"projectId " + base64.StdEncoding.EncodeToString([]byte(p.ProjectID)),
		"kind " + base64.StdEncoding.EncodeToString([]byte(p.Kind)),
	}
	if p.FileName != "" {
		pairs = append(pairs, "filename "+base64.StdEncoding.EncodeToString([]byte(p.FileName)))
	}
	return strings.Join(pairs, ",")
}

type tusUpload struct {
	client *Client
	cfg    resumableConfig
	params ResumableUploadParams
	body   io.ReaderAt
	size   int64
	url    string
	fileID string
}

func (u *tusUpload) run(ctx context.Context) (*ResumableUpload, error) {
	offset, err := u.open(ctx)
	if err != nil {
		return nil, err
	}

	failures := 0
	for offset < u.size {
		next, err := u.patch(ctx, offset)
		if err == nil {
			offset = next
			failures = 0
			u.report(offset)
			continue
		}

		conflict := isConflict(err)
		if !conflict && !isTransient(err) {
			return nil, err
		}
		if failures >= u.cfg.maxRetries {
			return nil, fmt.Errorf("resumable upload: giving up after %d retries: %w", failures, err)
		}
		failures++
		if !conflict {
			if err := u.wait(ctx, failures); err != nil {
				return nil, err
			}
		}

		head, err := u.retry(ctx, u.head)
		if err != nil {
			return nil, err
		}
		offset = head.offset
		u.report(offset)
	}

	if err := u.cfg.store.Delete(ctx, u.cfg.fingerprint); err != nil {
		return nil, fmt.Errorf("resumable upload: forget finished upload: %w", err)
	}
	return &ResumableUpload{URL: u.url, FileID: u.fileID, Size: u.size}, nil
}

func (u *tusUpload) open(ctx context.Context) (int64, error) {
	stored, ok, err := u.cfg.store.Get(ctx, u.cfg.fingerprint)
	if err != nil {
		return 0, fmt.Errorf("resumable upload: read resume store: %w", err)
	}
	if ok {
		u.url = stored
		head, err := u.retry(ctx, u.head)
		if err == nil {
			u.report(head.offset)
			return head.offset, nil
		}
		if tusErr, ok := errors.AsType[*TusError](err); !ok || !gone(tusErr.StatusCode) {
			return 0, err
		}
		if err := u.cfg.store.Delete(ctx, u.cfg.fingerprint); err != nil {
			return 0, fmt.Errorf("resumable upload: forget stale upload: %w", err)
		}
	}

	created, err := u.retry(ctx, u.create)
	if err != nil {
		return 0, err
	}
	u.url = created.location
	if err := u.cfg.store.Set(ctx, u.cfg.fingerprint, u.url); err != nil {
		return 0, fmt.Errorf("resumable upload: write resume store: %w", err)
	}

	head, err := u.retry(ctx, u.head)
	if err != nil {
		return 0, err
	}
	u.report(head.offset)
	return head.offset, nil
}

type tusState struct {
	location string
	offset   int64
}

func (u *tusUpload) retry(ctx context.Context, fn func(context.Context) (tusState, error)) (tusState, error) {
	for attempt := 0; ; attempt++ {
		state, err := fn(ctx)
		if err == nil || !isTransient(err) || attempt >= u.cfg.maxRetries {
			return state, err
		}
		if err := u.wait(ctx, attempt+1); err != nil {
			return tusState{}, err
		}
	}
}

func (u *tusUpload) create(ctx context.Context) (tusState, error) {
	header := http.Header{}
	header.Set("Upload-Length", strconv.FormatInt(u.size, 10))
	header.Set("Upload-Metadata", u.params.metadata())

	endpoint := u.client.tusEndpoint
	resp, err := u.client.tusDo(ctx, http.MethodPost, endpoint, header, 0, nil)
	if err != nil {
		return tusState{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		return tusState{}, tusErrorFrom("create", resp)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return tusState{}, errors.New("tus create: response has no Location")
	}
	base, err := url.Parse(endpoint)
	if err != nil {
		return tusState{}, fmt.Errorf("tus create: parse endpoint: %w", err)
	}
	ref, err := url.Parse(loc)
	if err != nil {
		return tusState{}, fmt.Errorf("tus create: parse Location: %w", err)
	}
	return tusState{location: base.ResolveReference(ref).String()}, nil
}

func (u *tusUpload) head(ctx context.Context) (tusState, error) {
	resp, err := u.client.tusDo(ctx, http.MethodHead, u.url, nil, 0, nil)
	if err != nil {
		return tusState{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return tusState{}, tusErrorFrom("head", resp)
	}

	offset, err := offsetFrom(resp)
	if err != nil {
		return tusState{}, err
	}
	if offset > u.size {
		return tusState{}, fmt.Errorf("tus head: server offset %d exceeds size %d", offset, u.size)
	}
	if id := metadataValue(resp.Header.Get("Upload-Metadata"), "fileId"); id != "" {
		u.fileID = id
	}
	return tusState{offset: offset}, nil
}

func (u *tusUpload) patch(ctx context.Context, offset int64) (int64, error) {
	n := min(u.cfg.chunkSize, u.size-offset)
	header := http.Header{}
	header.Set("Upload-Offset", strconv.FormatInt(offset, 10))
	header.Set("Content-Type", tusOffsetType)

	resp, err := u.client.tusDo(ctx, http.MethodPatch, u.url, header, n, io.NewSectionReader(u.body, offset, n))
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return 0, tusErrorFrom("patch", resp)
	}

	next, err := offsetFrom(resp)
	if err != nil {
		return 0, err
	}
	if next <= offset || next > u.size {
		return 0, fmt.Errorf("%w: server offset %d after sending from %d", errOffsetMismatch, next, offset)
	}
	return next, nil
}

func (u *tusUpload) report(offset int64) {
	if u.cfg.progress != nil {
		u.cfg.progress(offset, u.size)
	}
}

func (u *tusUpload) wait(ctx context.Context, attempt int) error {
	var d time.Duration
	if u.cfg.retryBackoff > 0 {
		d = u.cfg.retryBackoff << min(attempt-1, 30)
		if d <= 0 || d > maxRetryBackoff {
			d = maxRetryBackoff
		}
		d = d/2 + rand.N(d/2+1)
	}

	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("resumable upload: %w", ctx.Err())
	case <-t.C:
		return nil
	}
}

func (c *Client) tusDo(ctx context.Context, method, target string, header http.Header, length int64, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("build tus %s request: %w", method, err)
	}
	maps.Copy(req.Header, header)
	req.Header.Set("Tus-Resumable", tusVersion)
	if body != nil {
		req.ContentLength = length
	}
	for _, edit := range c.editors {
		if err := edit(ctx, req); err != nil {
			return nil, fmt.Errorf("tus %s: %w", method, err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &transientError{err: fmt.Errorf("tus %s: %w", method, err)}
	}
	return resp, nil
}

var errOffsetMismatch = errors.New("tus: offset mismatch")

type transientError struct{ err error }

func (e *transientError) Error() string { return e.err.Error() }
func (e *transientError) Unwrap() error { return e.err }

func isTransient(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if tusErr, ok := errors.AsType[*TusError](err); ok {
		return tusErr.retryable()
	}
	_, transient := errors.AsType[*transientError](err)
	_, netErr := errors.AsType[net.Error](err)
	return transient || netErr
}

func isConflict(err error) bool {
	tusErr, ok := errors.AsType[*TusError](err)
	return errors.Is(err, errOffsetMismatch) || (ok && tusErr.StatusCode == http.StatusConflict)
}

func gone(status int) bool {
	return status == http.StatusNotFound || status == http.StatusGone
}

func tusErrorFrom(op string, resp *http.Response) error {
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return &TusError{Op: op, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(snippet))}
}

func offsetFrom(resp *http.Response) (int64, error) {
	raw := resp.Header.Get("Upload-Offset")
	offset, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("tus: invalid Upload-Offset %q", raw)
	}
	return offset, nil
}

func metadataValue(header, key string) string {
	for pair := range strings.SplitSeq(header, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(pair), " ")
		if k != key {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return ""
		}
		return string(decoded)
	}
	return ""
}
