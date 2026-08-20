# Rixl Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/rixlhq/rixl-go.svg)](https://pkg.go.dev/github.com/rixlhq/rixl-go)

The Rixl Go SDK gives you access to the [Rixl](https://rixl.com) REST API from
any Go 1.25+ application. It covers media (images, videos, feeds, posts),
projects, analytics, billing and account management, with a typed client per
resource and Go structs for every request and response.

## Documentation

Full reference for every type and method: **[pkg.go.dev/github.com/rixlhq/rixl-go](https://pkg.go.dev/github.com/rixlhq/rixl-go/sdk)**.

## Installation

```bash
go get github.com/rixlhq/rixl-go
```

## Getting started

Listing the images in a project is three lines of setup and one call:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/rixlhq/rixl-go/sdk"
)

func main() {
    client, err := sdk.New(os.Getenv("RIXL_API_KEY"))
    if err != nil {
        log.Fatal(err)
    }

    page, err := client.Images.ListImages(context.Background(), os.Getenv("RIXL_PROJECT_ID"), nil)
    if err != nil {
        log.Fatal(err)
    }

    for _, img := range page.Images {
        fmt.Println(*img.ID)
    }
}
```

Every method takes a `context.Context` first, returns a parsed struct, and
returns an ordinary Go `error` you can check.

## Authentication

You need an API key. Create one in the [Rixl dashboard](https://rixl.com) and
keep it out of source control — read it from the environment, as above.

```go
client, err := sdk.New(apiKey)
```

The key is sent as the `X-API-Key` header on every request.

If you already hold a bearer token, pass an empty key and supply the token
instead:

```go
client, err := sdk.New("", sdk.WithBearer(token))
```

## Usage

Each API resource is a field on the client:

```go
client.Images       // media/v1 images
client.Videos       // media/v1 videos
client.Feeds        // media/v1 feeds
client.Posts        // posts/v1
client.Projects     // project/v1
client.Memberships  // organisation members
```

There are 38 in total, one per resource in the API. Run
`go doc github.com/rixlhq/rixl-go/sdk Client` to see them all, or browse the
[reference](https://pkg.go.dev/github.com/rixlhq/rixl-go/sdk).

They all follow the same shape — list, get, create, delete:

```go
import "github.com/rixlhq/rixl-go/sdk/images"

page, err := client.Images.ListImages(ctx, projectID, &images.ListImagesParams{})
img,  err := client.Images.GetImage(ctx, imageID)
err        = client.Images.DeleteImage(ctx, projectID, imageID)
```

Methods that send a body take a generated model:

```go
import "github.com/rixlhq/rixl-go/sdk/models"

name := "photo.jpg"
upload, err := client.Images.CreateImageUpload(ctx, projectID, models.ImagesV1CreateImageUploadRequest{
    Name: &name,
})
// upload.UploadURL is a presigned URL — PUT the file bytes to it.
```

Optional fields are pointers, which is how the SDK tells "not set" apart from
the zero value. `nil` for a `*string` means the field is omitted; a pointer to
`""` sends an empty string.

## Pagination

List methods take `limit` and `offset`, and the response tells you the total:

```go
limit, offset := int32(50), int32(0)
for {
    page, err := client.Images.ListImages(ctx, projectID, &images.ListImagesParams{
        PaginationLimit:  &limit,
        PaginationOffset: &offset,
    })
    if err != nil {
        return err
    }
    for _, img := range page.Images {
        fmt.Println(*img.ID)
    }

    offset += limit
    if page.Total == nil || int64(offset) >= *page.Total {
        break
    }
}
```

There is no auto-paginating iterator yet — you advance `offset` yourself.

## Handling errors

Any non-2xx response comes back as `*ClientHttpError[E]`, carrying the status
code and the raw response body. Each resource package defines its own, so
type-assert against the package whose method you called:

```go
import (
    "errors"

    "github.com/rixlhq/rixl-go/sdk/images"
)

img, err := client.Images.GetImage(ctx, imageID)
if err != nil {
    var apiErr *images.ClientHttpError[struct{}]
    if errors.As(err, &apiErr) {
        fmt.Printf("rixl returned %d: %s\n", apiErr.StatusCode, apiErr.RawBody)
    }
    return err
}
```

What the status codes mean:

| Status | Meaning |
| --- | --- |
| 400 | The request was malformed or failed validation |
| 401 | The API key or token is missing, expired or invalid |
| 403 | The credential is valid but lacks permission for this resource |
| 404 | No such resource, or it belongs to another organisation |
| 429 | Rate limited — slow down and retry |
| 5xx | Something failed on our side; safe to retry |

Network failures surface as ordinary Go errors from `net/http`, not as
`ClientHttpError`.

## Timeouts and retries

The SDK does not retry, and it has no timeout of its own — it uses the
`http.Client` you give it, or `http.DefaultClient`. Set a deadline with a
context, per request:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

Or configure the underlying client once:

```go
client, err := sdk.New(apiKey, sdk.WithHTTPClient(&http.Client{
    Timeout: 30 * time.Second,
}))
```

`sdk.WithRequestEditor` lets you mutate every outbound request, which is where
tracing headers or a custom retrying transport go:

```go
client, err := sdk.New(apiKey, sdk.WithRequestEditor(func(ctx context.Context, req *http.Request) error {
    req.Header.Set("X-Trace-ID", traceID(ctx))
    return nil
}))
```

## Examples

Runnable programs in [examples/](./examples):

```bash
export RIXL_API_KEY=<key> RIXL_PROJECT_ID=<project>
go run ./examples/basic/images
go run ./examples/advanced/videos
```

## Requirements

Go 1.25.0 or higher.

## Versioning

This package follows [SemVer](https://semver.org/spec/v2.0.0.html). The SDK is
generated from the Rixl OpenAPI specification, so new API resources arrive as
minor releases and renamed or removed operations as major ones. If an upgrade
breaks you unexpectedly, please open an issue.

## Regenerating the SDK

```bash
./gen.sh
```

`gen.sh` reads the published OpenAPI spec and generates one package per API
resource, plus the shared `models` and `runtime` packages and the client
facade. Point it at a local spec with `SPEC=../openapi/openapi.yaml ./gen.sh`.

```
sdk/
├── models/           every request and response schema
├── runtime/          shared codegen helpers
├── <resource>/       Client + SimpleClient, one package per API resource
├── resources.gen.go  the generated Client fields
├── sdk.go            hand-written: New and the client options
└── upload.go         hand-written: presigned uploads
```

Everything under `sdk/` except the hand-written files is regenerated from
scratch on each run, so a resource removed upstream disappears here too.

`internal/gen` prepares the spec before codegen. It exists because
oapi-codegen uses `operationId` verbatim for request body type names while the
spec ships fully-qualified protobuf ids such as
`clientauth.v1.ClientCredentialService.MintClientToken` — the dots produce Go
that does not compile. It shortens them to the trailing segment. Delete that
step once the spec emits short operationIds.

## Issues

Bugs and feature requests:
[github.com/rixlhq/rixl-go/issues](https://github.com/rixlhq/rixl-go/issues).
