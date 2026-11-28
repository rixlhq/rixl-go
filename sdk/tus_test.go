package sdk

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type tusFault func(w http.ResponseWriter, r *http.Request, f *fakeTus, u *fakeTusUpload) bool

type fakeTusUpload struct {
	length int64
	data   []byte
	meta   map[string]string
}

type fakeTus struct {
	t       *testing.T
	mu      sync.Mutex
	uploads map[string]*fakeTusUpload
	source  []byte
	faults  []tusFault
	nextID  int
	posts   int
	patches int
	heads   int
	deletes int
}

func newFakeTus(t *testing.T, source []byte, faults ...tusFault) *fakeTus {
	t.Helper()
	return &fakeTus{t: t, uploads: map[string]*fakeTusUpload{}, source: source, faults: faults}
}

func (f *fakeTus) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Tus-Resumable") != tusVersion {
		http.Error(w, "missing Tus-Resumable", http.StatusPreconditionFailed)
		return
	}
	if r.Header.Get("X-Api-Key") != "test-key" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	id := strings.TrimPrefix(r.URL.Path, "/files/")
	if r.Method == http.MethodPost && id == "" {
		f.create(w, r)
		return
	}
	u, ok := f.uploads[id]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodHead:
		f.heads++
		w.Header().Set("Upload-Offset", strconv.Itoa(len(u.data)))
		w.Header().Set("Upload-Length", strconv.FormatInt(u.length, 10))
		w.Header().Set("Upload-Metadata", "fileId "+base64.StdEncoding.EncodeToString([]byte(u.meta["fileId"])))
		w.WriteHeader(http.StatusOK)
	case http.MethodPatch:
		f.patches++
		if len(f.faults) > 0 {
			fault := f.faults[0]
			f.faults = f.faults[1:]
			if fault != nil && fault(w, r, f, u) {
				return
			}
		}
		f.patch(w, r, u)
	case http.MethodDelete:
		f.deletes++
		delete(f.uploads, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (f *fakeTus) create(w http.ResponseWriter, r *http.Request) {
	f.posts++
	length, err := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
	if err != nil {
		http.Error(w, "bad Upload-Length", http.StatusBadRequest)
		return
	}
	meta := map[string]string{}
	for pair := range strings.SplitSeq(r.Header.Get("Upload-Metadata"), ",") {
		k, v, _ := strings.Cut(pair, " ")
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			http.Error(w, "bad Upload-Metadata", http.StatusBadRequest)
			return
		}
		meta[k] = string(decoded)
	}
	if meta["projectId"] == "" || (meta["kind"] != "image" && meta["kind"] != "video") {
		http.Error(w, "projectId and kind are required", http.StatusBadRequest)
		return
	}

	f.nextID++
	meta["fileId"] = fmt.Sprintf("file-%d", f.nextID)
	id := fmt.Sprintf("%ss/%s/%s+mp", meta["kind"], meta["projectId"], meta["fileId"])
	f.uploads[id] = &fakeTusUpload{length: length, meta: meta}
	w.Header().Set("Location", "/files/"+id)
	w.WriteHeader(http.StatusCreated)
}

func (f *fakeTus) patch(w http.ResponseWriter, r *http.Request, u *fakeTusUpload) {
	if r.Header.Get("Content-Type") != tusOffsetType {
		http.Error(w, "bad Content-Type", http.StatusUnsupportedMediaType)
		return
	}
	offset, err := strconv.Atoi(r.Header.Get("Upload-Offset"))
	if err != nil || offset != len(u.data) {
		http.Error(w, "offset mismatch", http.StatusConflict)
		return
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	if int64(len(u.data)+len(b)) > u.length {
		http.Error(w, "exceeds length", http.StatusRequestEntityTooLarge)
		return
	}
	u.data = append(u.data, b...)
	w.Header().Set("Upload-Offset", strconv.Itoa(len(u.data)))
	w.WriteHeader(http.StatusNoContent)
}

func (f *fakeTus) only() fakeTusUpload {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.uploads) != 1 {
		f.t.Fatalf("uploads = %d, want 1", len(f.uploads))
	}
	for _, u := range f.uploads {
		return fakeTusUpload{length: u.length, data: bytes.Clone(u.data), meta: u.meta}
	}
	return fakeTusUpload{}
}

type tusCounts struct{ posts, patches, heads, deletes, uploads int }

func (f *fakeTus) counts() tusCounts {
	f.mu.Lock()
	defer f.mu.Unlock()
	return tusCounts{posts: f.posts, patches: f.patches, heads: f.heads, deletes: f.deletes, uploads: len(f.uploads)}
}

func dropAfter(n int) tusFault {
	return func(w http.ResponseWriter, r *http.Request, _ *fakeTus, u *fakeTusUpload) bool {
		buf := make([]byte, n)
		read, _ := io.ReadFull(r.Body, buf)
		u.data = append(u.data, buf[:read]...)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
		return true
	}
}

func respond(status int) tusFault {
	return func(w http.ResponseWriter, _ *http.Request, _ *fakeTus, _ *fakeTusUpload) bool {
		http.Error(w, http.StatusText(status), status)
		return true
	}
}

func advanceBehindClient(n int) tusFault {
	return func(w http.ResponseWriter, _ *http.Request, f *fakeTus, u *fakeTusUpload) bool {
		u.data = append(u.data, f.source[len(u.data):len(u.data)+n]...)
		http.Error(w, "offset mismatch", http.StatusConflict)
		return true
	}
}

func newTusClient(t *testing.T, f *fakeTus) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	c, err := New("test-key", WithHTTPClient(srv.Client()), WithUploadEndpoint(srv.URL+"/files/"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, srv
}

func testPayload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

var testParams = ResumableUploadParams{ProjectID: "proj", Kind: UploadKindVideo, FileName: "clip.mp4"}

func TestUploadResumable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		size        int
		faults      []tusFault
		retries     int
		wantErr     string
		wantPatches int
		wantHeads   int
	}{
		{name: "clean upload in chunks", size: 25, retries: 3, wantPatches: 3, wantHeads: 1},
		{name: "empty body", size: 0, retries: 3, wantPatches: 0, wantHeads: 1},
		{name: "resumes after an interrupted patch", size: 25, faults: []tusFault{nil, dropAfter(4)}, retries: 3, wantPatches: 4, wantHeads: 2},
		{name: "resyncs on offset mismatch", size: 25, faults: []tusFault{advanceBehindClient(3)}, retries: 3, wantPatches: 4, wantHeads: 2},
		{name: "retries server errors", size: 25, faults: []tusFault{respond(http.StatusServiceUnavailable), respond(http.StatusLocked)}, retries: 3, wantPatches: 5, wantHeads: 3},
		{
			name:    "gives up after max retries",
			size:    25,
			faults:  []tusFault{respond(http.StatusBadGateway), respond(http.StatusBadGateway), respond(http.StatusBadGateway)},
			retries: 2,
			wantErr: "giving up after 2 retries",
		},
		{name: "does not retry client errors", size: 25, faults: []tusFault{respond(http.StatusForbidden)}, retries: 3, wantErr: "status 403"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := testPayload(tt.size)
			f := newFakeTus(t, payload, tt.faults...)
			c, _ := newTusClient(t, f)

			var progress []int64
			res, err := c.UploadResumable(context.Background(), testParams, bytes.NewReader(payload), int64(len(payload)),
				WithChunkSize(10),
				WithRetries(tt.retries, time.Millisecond),
				WithProgress(func(uploaded, total int64) {
					if total != int64(tt.size) {
						t.Errorf("progress total = %d, want %d", total, tt.size)
					}
					progress = append(progress, uploaded)
				}),
			)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("UploadResumable: %v", err)
			}

			assertFinished(t, f, c, res, payload)
			if n := f.counts(); n.posts != 1 || n.patches != tt.wantPatches || n.heads != tt.wantHeads {
				t.Errorf("posts/patches/heads = %d/%d/%d, want 1/%d/%d", n.posts, n.patches, n.heads, tt.wantPatches, tt.wantHeads)
			}
			if len(progress) == 0 || progress[len(progress)-1] != int64(tt.size) {
				t.Errorf("progress = %v, want to end at %d", progress, tt.size)
			}
		})
	}
}

func assertFinished(t *testing.T, f *fakeTus, c *Client, res *ResumableUpload, payload []byte) {
	t.Helper()
	u := f.only()
	if !bytes.Equal(u.data, payload) {
		t.Errorf("server data = %v, want %v", u.data, payload)
	}
	if res.FileID != "file-1" || res.Size != int64(len(payload)) || !strings.Contains(res.URL, "/files/videos/proj/file-1+mp") {
		t.Errorf("result = %+v", res)
	}
	if u.meta["filename"] != "clip.mp4" || u.meta["kind"] != "video" || u.meta["projectId"] != "proj" {
		t.Errorf("metadata = %v", u.meta)
	}
	if _, ok, _ := c.resumeStore.Get(context.Background(), testParams.fingerprint(int64(len(payload)))); ok {
		t.Error("finished upload left in the resume store")
	}
}

func TestUploadResumableCancelThenResume(t *testing.T) {
	t.Parallel()

	payload := testPayload(25)
	f := newFakeTus(t, payload)
	c, _ := newTusClient(t, f)

	ctx, cancel := context.WithCancel(context.Background())
	_, err := c.UploadResumable(ctx, testParams, bytes.NewReader(payload), int64(len(payload)),
		WithChunkSize(10),
		WithProgress(func(uploaded, _ int64) {
			if uploaded >= 10 {
				cancel()
			}
		}),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if got := len(f.only().data); got != 10 {
		t.Fatalf("server offset after cancel = %d, want 10", got)
	}

	if _, err := c.UploadResumable(context.Background(), testParams, bytes.NewReader(payload), int64(len(payload)), WithChunkSize(10)); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if n := f.counts(); n.posts != 1 {
		t.Errorf("posts = %d, want 1: resume should reuse the upload", n.posts)
	}
	if !bytes.Equal(f.only().data, payload) {
		t.Error("server data does not match payload after resume")
	}
}

func TestUploadResumableCancelDuringBackoff(t *testing.T) {
	t.Parallel()

	payload := testPayload(5)
	f := newFakeTus(t, payload, respond(http.StatusServiceUnavailable))
	c, _ := newTusClient(t, f)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.UploadResumable(ctx, testParams, bytes.NewReader(payload), int64(len(payload)), WithRetries(3, time.Minute))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("backoff ignored context cancellation")
	}
}

func TestUploadResumableAcrossClientsWithStore(t *testing.T) {
	t.Parallel()

	payload := testPayload(25)
	f := newFakeTus(t, payload, nil, dropAfter(0))
	store := NewMemoryResumeStore()
	first, _ := newTusClient(t, f)

	_, err := first.UploadResumable(context.Background(), testParams, bytes.NewReader(payload), int64(len(payload)),
		WithChunkSize(10), WithRetries(0, 0), WithResumeStore(store), WithFingerprint("clip"))
	if err == nil {
		t.Fatal("expected the interrupted upload to fail without retries")
	}

	second, _ := newTusClient(t, f)
	var resumedAt int64 = -1
	res, err := second.UploadResumable(context.Background(), testParams, bytes.NewReader(payload), int64(len(payload)),
		WithChunkSize(10), WithResumeStore(store), WithFingerprint("clip"),
		WithProgress(func(uploaded, _ int64) {
			if resumedAt < 0 {
				resumedAt = uploaded
			}
		}))
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resumedAt != 10 {
		t.Errorf("resumed at %d, want 10", resumedAt)
	}
	if n := f.counts(); n.posts != 1 || res.FileID != "file-1" {
		t.Errorf("posts = %d, file id = %q; want one upload reused", n.posts, res.FileID)
	}
	if !bytes.Equal(f.only().data, payload) {
		t.Error("server data does not match payload after resume")
	}
}

func TestUploadResumableReplacesStaleStoreEntry(t *testing.T) {
	t.Parallel()

	payload := testPayload(5)
	f := newFakeTus(t, payload)
	c, srv := newTusClient(t, f)
	store := NewMemoryResumeStore()
	_ = store.Set(context.Background(), "clip", srv.URL+"/files/expired")

	res, err := c.UploadResumable(context.Background(), testParams, bytes.NewReader(payload), int64(len(payload)),
		WithResumeStore(store), WithFingerprint("clip"))
	if err != nil {
		t.Fatalf("UploadResumable: %v", err)
	}
	if n := f.counts(); n.posts != 1 || res.FileID != "file-1" {
		t.Errorf("posts = %d, file id = %q", n.posts, res.FileID)
	}
}

func TestUploadFileResumable(t *testing.T) {
	t.Parallel()

	payload := testPayload(25)
	f := newFakeTus(t, payload)
	c, _ := newTusClient(t, f)

	path := filepath.Join(t.TempDir(), "poster.jpg")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := c.UploadFileResumable(context.Background(), ResumableUploadParams{ProjectID: "proj", Kind: UploadKindImage}, path, WithChunkSize(10)); err != nil {
		t.Fatalf("UploadFileResumable: %v", err)
	}
	u := f.only()
	if u.meta["filename"] != "poster.jpg" || u.meta["kind"] != "image" {
		t.Errorf("metadata = %v", u.meta)
	}
	if !bytes.Equal(u.data, payload) {
		t.Error("server data does not match file")
	}
}

func TestUploadResumableValidates(t *testing.T) {
	t.Parallel()

	c, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		name   string
		params ResumableUploadParams
		size   int64
		opts   []ResumableOption
	}{
		{name: "missing project", params: ResumableUploadParams{Kind: UploadKindImage}},
		{name: "unknown kind", params: ResumableUploadParams{ProjectID: "p", Kind: "audio"}},
		{name: "negative size", params: testParams, size: -1},
		{name: "zero chunk size", params: testParams, opts: []ResumableOption{WithChunkSize(0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := c.UploadResumable(context.Background(), tt.params, strings.NewReader(""), tt.size, tt.opts...); err == nil {
				t.Error("expected a validation error")
			}
		})
	}
}

func TestTerminateUpload(t *testing.T) {
	t.Parallel()

	f := newFakeTus(t, nil)
	c, srv := newTusClient(t, f)
	f.uploads["images/proj/x+mp"] = &fakeTusUpload{length: 10, meta: map[string]string{}}

	if err := c.TerminateUpload(context.Background(), srv.URL+"/files/images/proj/x+mp"); err != nil {
		t.Fatalf("TerminateUpload: %v", err)
	}
	if n := f.counts(); n.deletes != 1 || n.uploads != 0 {
		t.Errorf("deletes = %d, uploads = %d", n.deletes, n.uploads)
	}

	var tusErr *TusError
	if err := c.TerminateUpload(context.Background(), srv.URL+"/files/missing"); !errors.As(err, &tusErr) || tusErr.StatusCode != http.StatusNotFound {
		t.Errorf("err = %v, want TusError 404", err)
	}
}
