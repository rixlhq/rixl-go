# Contributing

## Regenerating the SDK

```bash
./gen.sh
```

`gen.sh` reads the published OpenAPI spec and generates one package per API
resource, plus the shared `models` and `runtime` packages and the client facade.
Point it at a local spec with `SPEC=../openapi/openapi.yaml ./gen.sh`.

```
sdk/
├── models/           every request and response schema
├── runtime/          shared codegen helpers
├── <resource>/       Client + SimpleClient, one package per API resource
├── resources.gen.go  the generated Client fields
├── sdk.go            hand-written: New and the client options
├── clientauth.go     hand-written: client credentials and token refresh
├── credentials.go    hand-written: credential lifecycle
├── scopes.go         hand-written: scope constants
└── upload.go         hand-written: presigned uploads
```

Everything under `sdk/` except the hand-written files is regenerated from
scratch on each run, so a resource removed upstream disappears here too.
Authentication and uploads cannot be expressed in OpenAPI and are maintained by
hand.

`internal/gen` prepares the spec before codegen runs. oapi-codegen uses
`operationId` verbatim for request body type names, and the spec ships
fully-qualified protobuf ids such as
`clientauth.v1.ClientCredentialService.MintClientToken` — the dots produce Go
that does not compile, so the ids are shortened to their trailing segment first.
Remove that step once the spec emits short operationIds.

It also writes `aliases.gen.go` files: enum types used by parameters are
declared in `sdk/models` but referenced unqualified from the resource packages,
so each one gets a local alias.

## Before pushing

`mise install` sets up the toolchain and installs the git hooks, which run
`gofmt`, `golangci-lint`, `go vet` and the tests.
