# rixl-go

Go client for the [RIXL](https://rixl.com) API.

[![Go Reference](https://pkg.go.dev/badge/github.com/rixlhq/rixl-go.svg)](https://pkg.go.dev/github.com/rixlhq/rixl-go)

## Install

```bash
go get github.com/rixlhq/rixl-go
```

Requires Go 1.25.0+.

## Quick start

```go
package main

import (
    "context"
    "fmt"

    "github.com/rixlhq/rixl-go/sdk"
)

func main() {
    client, err := sdk.New("YOUR_RIXL_API_KEY")
    if err != nil {
        panic(err)
    }

    img, err := client.Images.GetImagesImageId(context.Background(), "PS5IMKoFLm")
    if err != nil {
        panic(err)
    }
    fmt.Println(*img.ID, *img.Width, *img.Height)
}
```

`sdk.New(apiKey, opts...)` returns a Client with four resource fields — `client.Feeds`, `client.Images`, `client.Videos` and `client.Credentials` — each a typed client whose methods return parsed models and a Go error.

## Authentication

Two credential types are supported.

**API key** — one long-lived secret for your whole backend, sent as `X-API-Key`:

```go
client, err := sdk.New("YOUR_RIXL_API_KEY")
```

**Client auth** — a client ID/secret pair that mints a short-lived token scoped
to one of your end users. Use this when requests are made on behalf of a
specific user rather than your backend as a whole:

```go
client, err := sdk.New("", sdk.WithClientCredentials(sdk.ClientCredentials{
    ClientID:     os.Getenv("RIXL_CLIENT_ID"),
    ClientSecret: os.Getenv("RIXL_CLIENT_SECRET"),
    Subject:      userID,             // your identifier for the end user
    ProjectID:    os.Getenv("RIXL_PROJECT_ID"), // optional
    Scopes:       sdk.MediaReadScopes,
}))
```

The SDK mints a token on the first API call and re-mints it automatically about
a minute before it expires — tokens live 15 minutes by default (`TTL`, max 15).
Share one token across clients with `sdk.NewTokenSource` + `sdk.WithTokenSource`.

### Managing credentials

`client.Credentials` covers the credential lifecycle. Reads need
`credentials:clientauth:read`, writes need `credentials:clientauth:write`.

```go
created, err := client.Credentials.Create(ctx, sdk.CreateCredentialParams{
    Name: "Server credential",
})
// created.ClientSecret is returned once and never again — store it now.

creds, total, err := client.Credentials.List(ctx, sdk.ListCredentialsParams{Limit: 25})
err = client.Credentials.Revoke(ctx, creds[0].ID, "")
```

Revoking blocks new mints; tokens already issued stay valid until they expire
(15 minutes at most).

### Scopes

Scopes are granted to a credential by the policies attached to it in the RIXL
dashboard; the mint endpoint does not narrow them per token. Listing them in
`Scopes` documents what the credential needs and catches typos at construction
time. The recognised scopes are exported as constants:

| Area | Scopes |
| --- | --- |
| Media | `media:images:read` / `:write`, `media:videos:read` / `:write`, `media:files:read` / `:write`, `media:feeds:read` / `:write`, `media:posts:read` / `:write` |
| Projects | `project:projects:read` / `:write` |
| Analytics | `analytics:events:read` |
| Billing | `billing:subscription:read` / `:write` |
| Credentials | `credentials:apikeys:read` / `:write`, `credentials:clientauth:read` / `:write` |
| Organisation | `org:members:read` / `:write`, `org:domains:read` / `:write`, `org:policies:read` / `:write` |

In Go: `sdk.ScopeImagesRead`, `sdk.ScopeVideosWrite`, … plus `sdk.AllScopes`,
`sdk.MediaReadScopes` and `sdk.MediaWriteScopes` for the common bundles.
`client.Images`, `client.Videos` and `client.Feeds` need the matching
`media:*:read` scopes; uploads additionally need `media:files:write`.

## Uploading files

Uploads are two steps: ask the API for a presigned upload URL, then send the
bytes straight to storage with `client.Upload` / `client.UploadFile`.

```go
// 1. Obtain a presigned upload_url from the relevant API call, then:
err := client.UploadFile(ctx, uploadURL, "avatar.png")
// or stream from any reader with a known size:
err = client.Upload(ctx, uploadURL, reader, size, sdk.WithContentType("image/png"))
```

`UploadFile` derives the `Content-Type` from the file extension; pass
`sdk.WithContentType` to override it. The API completes the upload asynchronously
once storage confirms the object, so no explicit "complete" call is required.

## Configuration

```go
client, err := sdk.New(apiKey,
    sdk.WithHTTPClient(myHTTPClient),  // custom timeouts, transport, etc.
    sdk.WithRequestEditor(addTracing), // mutate every outbound request
)
```

For bearer-token auth, pass an empty key and use `WithBearer`:

```go
client, err := sdk.New("", sdk.WithBearer(token))
```

## Feeds

```go
import "github.com/rixlhq/rixl-go/sdk/feeds"

page, err := client.Feeds.GetFeedsFeedId(ctx, "FD4y3QB38S", &feeds.GetFeedsFeedIdParams{})
for _, post := range page.Data {
    fmt.Println(*post.ID)
}

post, err := client.Feeds.GetFeedsFeedIdPostId(ctx, "FD4y3QB38S", "PO9XQxWXQ")
```

## Images

```go
import "github.com/rixlhq/rixl-go/sdk/images"

list, err := client.Images.GetImages(ctx, nil)
img,  err := client.Images.GetImagesImageId(ctx, "PS5IMKoFLm")

// Delete returns the raw *http.Response (no JSON body to parse).
resp, err := client.Images.DeleteImagesImageId(ctx, "PS5IMKoFLm")
```

Upload (init → PUT bytes → complete):

```go
import "github.com/rixlhq/rixl-go/sdk/models"

name, format := "photo.jpg", "jpeg"
init, err := client.Images.PostImagesUploadInit(ctx, models.InternalImagesHandlerUploadInitRequest{
    Name:   &name,
    Format: &format,
})
// PUT bytes to *init.PresignedURL

attached := false
done, err := client.Images.PostImagesUploadComplete(ctx, models.InternalImagesHandlerCompleteRequest{
    ImageID:         init.ImageID,
    AttachedToVideo: &attached,
})
fmt.Println(*done.ID)
```

## Videos

```go
import "github.com/rixlhq/rixl-go/sdk/models"

list,  err := client.Videos.GetVideos(ctx, nil)
video, err := client.Videos.GetVideosVideoId(ctx, "VI9VXQxWXQ")
```

Upload returns presigned URLs for both the video and a poster image:

```go
posterFormat := "jpeg"
init, err := client.Videos.PostVideosUploadInit(ctx, models.VideoUploadInitRequest{
    FileName:    "clip.mp4",
    ImageFormat: &posterFormat,
})
// PUT to init.VideoPresignedURL and init.PosterPresignedURL

done, err := client.Videos.PostVideosUploadComplete(ctx, models.GithubComRixlhqAPIInternalVideosHandlerUploadCompleteRequest{
    VideoID: init.VideoID,
})
fmt.Println(*done.ID)
```

## Pagination

List endpoints accept `limit`, `offset`, `sort`, `order`:

```go
import "github.com/rixlhq/rixl-go/sdk/images"

limit, offset := 50, 0
for {
    page, err := client.Images.GetImages(ctx, &images.GetImagesParams{
        Limit: &limit, Offset: &offset,
    })
    if err != nil {
        return err
    }
    for _, img := range page.Data {
        fmt.Println(*img.ID)
    }
    if offset+len(page.Data) >= *page.Pagination.Total {
        break
    }
    offset += limit
}
```

## Errors

API errors come back as a typed `*ClientHttpError[E]` carrying the HTTP status, raw body, and the parsed error response.

```go
import (
    "errors"

    "github.com/rixlhq/rixl-go/sdk/images"
    "github.com/rixlhq/rixl-go/sdk/models"
)

img, err := client.Images.GetImagesImageId(ctx, "PS5IMKoFLm")
if err != nil {
    var apiErr *images.ClientHttpError[models.GithubComRixlhqAPIInternalErrorsErrorResponse]
    if errors.As(err, &apiErr) {
        fmt.Printf("HTTP %d: %s\n", apiErr.StatusCode, *apiErr.Body.Error)
    }
    return err
}
```

Each resource package (`feeds`, `images`, `videos`) defines its own `ClientHttpError[E]` — type-assert against the package whose method you called.

## Examples

Runnable demos in [examples/](./examples):

```bash
export RIXL_API_KEY=<key>
go run ./examples/basic/images
go run ./examples/advanced/videos
```

## Regenerating the SDK

The SDK is generated by [oapi-codegen-exp](https://github.com/oapi-codegen/oapi-codegen-exp) — the experimental fork that supports OpenAPI 3.1. Layout:

```
sdk/
├── models/          generated: every component schema
├── runtime/         generated: shared codegen helpers
├── feeds/           generated: Client + SimpleClient for tag=Feeds
├── images/          generated: same for Images
├── videos/          generated: same for Videos
├── sdk.go           hand-written: the `sdk.New(...)` facade and options
├── clientauth.go    hand-written: client credentials, token minting and refresh
├── credentials.go   hand-written: credential lifecycle
├── scopes.go        hand-written: scope constants
└── upload.go        hand-written: presigned uploads
```

The generator only produces API wrappers. Everything the API cannot describe in
OpenAPI — authentication, token refresh and binary uploads — is hand-written in
`sdk/*.go` and is not touched by `./gen.sh`, which writes only into the
subdirectories above. Adding a resource means regenerating; changing auth or
upload behaviour means editing the hand-written files.

`oapi-codegen-exp` is pinned in `go.mod` as a project tool — `gen.sh` invokes it via `go tool oapi-codegen`, no separate install needed.

```bash
./gen.sh
```

Per-package codegen configs live in `cfg/`.

## Issues

[github.com/rixlhq/rixl-go/issues](https://github.com/rixlhq/rixl-go/issues)
