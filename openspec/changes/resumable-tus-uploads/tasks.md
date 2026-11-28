## 1. Client
- [x] 1.1 Add tus create, `HEAD` and chunked `PATCH` in `sdk/tus.go`
- [x] 1.2 Retry, backoff, resync and cancellation handling
- [x] 1.3 `ResumeStore`, `TerminateUpload`, options and `WithUploadEndpoint`

## 2. Docs and examples
- [x] 2.1 README section and `examples/advanced/resumable`

## 3. Verification
- [x] 3.1 Table-driven tests against an `httptest` tus fake with fault injection
- [x] 3.2 `go build ./... && go vet ./... && go test -race ./...` and `golangci-lint run ./...`
