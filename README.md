# Rixl Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/rixlhq/rixl-go.svg)](https://pkg.go.dev/github.com/rixlhq/rixl-go/sdk)

The official Go client for the [Rixl](https://rixl.com) API.

Rixl handles the media side of your product — uploading and delivering images
and videos, organising them into feeds and posts, and reporting on how people
engage with them. It also covers the account layer around that: users and
organisations, sign-in, subscriptions and invoices. This SDK gives you all of it
from Go, with a typed client per resource and a Go struct for every request and
response.

Works with Go 1.25 and later.

## Documentation

Reference for every type and method:
**[pkg.go.dev/github.com/rixlhq/rixl-go/sdk](https://pkg.go.dev/github.com/rixlhq/rixl-go/sdk)**

## Installation

```bash
go get github.com/rixlhq/rixl-go
```

## Getting started

Here is the whole thing — create a client, list the images in a project:

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

Every method takes a `context.Context` first, gives you back a parsed struct,
and returns an ordinary Go `error` you can check.

Requests go to `https://api.rixl.com`. The SDK points there by default, so
there is nothing to configure.

## Authentication

There are two ways to identify yourself, and they answer different questions.

### API keys — your backend calling as itself

An API key represents your organisation. Use it for work your own systems do:
importing a catalogue, running a nightly report, reconciling invoices. Create
one in the [Rixl dashboard](https://rixl.com), keep it out of source control,
and read it from the environment:

```go
client, err := sdk.New(os.Getenv("RIXL_API_KEY"))
```

The key travels as the `X-API-Key` header. Anyone holding it can do anything
your organisation can, so it belongs on a server — never in a browser, a mobile
app, or anything you ship to users.

### Client credentials — acting on behalf of one of your users

If you are building on top of Rixl and your own users each need their own
slice of it, use client credentials. You exchange a client ID and secret for a
short-lived token that is scoped to a single end user, so one customer can never
read another's media.

First create the credential. This returns a secret that is shown **once**:

```go
admin, err := sdk.New(os.Getenv("RIXL_API_KEY"))
if err != nil {
    log.Fatal(err)
}

created, err := admin.Credentials.Create(ctx, sdk.CreateCredentialParams{
    Name: "Production backend",
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(created.Credential.ClientID, created.ClientSecret) // store both now
```

Then, in the service that handles your users' requests, build a client per user.
`Subject` is your own identifier for that person — whatever your database calls
them:

```go
client, err := sdk.New("", sdk.WithClientCredentials(sdk.ClientCredentials{
    ClientID:     os.Getenv("RIXL_CLIENT_ID"),
    ClientSecret: os.Getenv("RIXL_CLIENT_SECRET"),
    Subject:      user.ID,
    ProjectID:    os.Getenv("RIXL_PROJECT_ID"), // optional
    Scopes:       sdk.MediaReadScopes,
}))
```

The SDK mints a token on the first call and quietly renews it before it expires,
so there is nothing to schedule or cache yourself. Tokens last 15 minutes.

When a credential is compromised or a deployment is retired, revoke it — new
tokens stop immediately, and any already issued expire within 15 minutes:

```go
err = admin.Credentials.Revoke(ctx, credentialID, "")
```

`Scopes` states what the credential is expected to be allowed to do, and is
checked for typos when the client is built. What it can *actually* do is set by
the policies attached to it in the dashboard. The common bundles are
`sdk.MediaReadScopes` and `sdk.MediaWriteScopes`; individual scopes are exported
as `sdk.ScopeImagesRead`, `sdk.ScopeVideosWrite` and so on, with the full list in
`sdk.AllScopes`.

### Public endpoints

Some reads need no credentials at all — delivering a public image or video,
fetching a public feed, listing supported languages. Call those with an empty
key:

```go
client, _ := sdk.New("")
img, err := client.Images.GetImage(ctx, imageID)
```

## What you can do

Every resource is a field on the client. The API is organised into six areas:

**Media** — `Images`, `Videos`, `Feeds`, `AudioTracks`, `Chapters`, `Subtitles`,
`Languages`, `ImageConversion`, `VideoConversion`. Upload and deliver files,
attach audio and captions to a video, and convert media into the formats and
sizes you serve.

**Content** — `Posts`, `Feeds`, `Projects`. Group media into posts and feeds. A
project is the container everything else hangs off, which is why so many calls
take a project ID.

**Analytics** — `Dashboards`, `Events`, `PostAnalytics`, `VideoAnalytics`,
`FeedAnalytics`, `Funnels`, `Heatmaps`, `Realtime`. Track events and read back
engagement, playback, funnels and live activity.

**Billing** — `Plans`, `Subscriptions`, `Payments`, `Invoices`, `Usage`, `Sales`.
Manage subscriptions and payment methods, and read invoices and metered usage.

**Accounts** — `Users`, `Sessions`, `OneTimePasscodes`, `Passkeys`,
`SocialProviders`, `Memberships`, `AccessPolicies`, `CustomDomains`, `Email`,
`Blog`. Sign-in flows including passkeys and one-time codes, organisation
membership and roles, and transactional email.

**Platform** — `APIKeys`, `ClientCredentials`, `PlatformAuth`. Manage the
credentials above programmatically.

For the exact methods on any of these, see the
[reference](https://pkg.go.dev/github.com/rixlhq/rixl-go/sdk) or run
`go doc github.com/rixlhq/rixl-go/sdk/images`.

## Working with resources

Resources follow the same shape, so once you have used one you have used all of
them:

```go
import "github.com/rixlhq/rixl-go/sdk/images"

page, err := client.Images.ListImages(ctx, projectID, &images.ListImagesParams{})
img,  err := client.Images.GetImage(ctx, imageID)
err        = client.Images.DeleteImage(ctx, projectID, imageID)
```

Calls that send data take a generated struct:

```go
import "github.com/rixlhq/rixl-go/sdk/models"

name := "photo.jpg"
upload, err := client.Images.CreateImageUpload(ctx, projectID, models.ImagesV1CreateImageUploadRequest{
    Name: &name,
})
```

Optional fields are pointers. That is how the SDK tells "leave this alone" apart
from "set this to empty": `nil` omits the field, while a pointer to `""` sends an
empty string.

## Uploading files

Uploads happen in two steps. You ask Rixl for a URL, then send the bytes
straight to storage — they never pass through the API, so large files stay fast:

```go
upload, err := client.Images.CreateImageUpload(ctx, projectID, models.ImagesV1CreateImageUploadRequest{
    Name: &name,
})
if err != nil {
    return err
}

err = client.UploadFile(ctx, *upload.UploadURL, "photo.jpg")
```

`UploadFile` works out the content type from the file extension; pass
`sdk.WithContentType` to set it yourself. To stream from something that is not a
file, use `client.Upload` with a reader and its exact size.

There is no "finish" call to make. Storage tells Rixl when the object lands and
the image or video becomes available on its own.

## Pagination

List calls take a limit and an offset, and tell you the total:

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

## Handling errors

Anything that is not a 2xx comes back as `*ClientHttpError`, carrying the status
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

What the codes mean:

| Status | What happened | What to do |
| --- | --- | --- |
| 400 | The request was malformed or failed validation | Fix the request; retrying will not help |
| 401 | The key or token is missing, expired or invalid | Check the credential |
| 403 | The credential is valid but not allowed to do this | Check the scopes and policies on it |
| 404 | No such resource, or it belongs to another organisation | Check the ID and the project |
| 429 | You are going too fast | Back off and retry |
| 5xx | Something broke on our side | Retry with backoff |

Connection failures and timeouts surface as ordinary Go errors from `net/http`,
not as `ClientHttpError`.

## Timeouts

The SDK does not impose a timeout and does not retry — it uses the `http.Client`
you give it, so the behaviour stays yours to control. Set a deadline per call
with a context:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

Or once, for every call:

```go
client, err := sdk.New(apiKey, sdk.WithHTTPClient(&http.Client{
    Timeout: 30 * time.Second,
}))
```

`sdk.WithRequestEditor` runs on every outbound request, which is where tracing
headers or a retrying transport go:

```go
client, err := sdk.New(apiKey, sdk.WithRequestEditor(func(ctx context.Context, req *http.Request) error {
    req.Header.Set("X-Trace-ID", traceID(ctx))
    return nil
}))
```

## Examples

Runnable programs live in [examples/](./examples):

```bash
export RIXL_API_KEY=<key> RIXL_PROJECT_ID=<project>
go run ./examples/basic/images
go run ./examples/advanced/videos
```

## Versioning

This package follows [SemVer](https://semver.org/spec/v2.0.0.html). New API
resources arrive in minor releases; renamed or removed operations only in major
ones. If an upgrade breaks you unexpectedly, please open an issue — we would
rather hear about it.

## Support

Bugs and feature requests:
[github.com/rixlhq/rixl-go/issues](https://github.com/rixlhq/rixl-go/issues).
