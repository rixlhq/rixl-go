# rixl-go

Go client for the [RIXL](https://rixl.com) API.

[![Go Reference](https://pkg.go.dev/badge/github.com/rixlhq/rixl-go.svg)](https://pkg.go.dev/github.com/rixlhq/rixl-go)

## Install

```bash
go get github.com/rixlhq/rixl-go
```

Requires Go 1.25+.

## Quick start

```go
package main

import (
    "context"
    "fmt"

    "github.com/microsoft/kiota-abstractions-go/authentication"
    kiotahttp "github.com/microsoft/kiota-http-go"
    "github.com/rixlhq/rixl-go/sdk"
)

func main() {
    auth, _ := authentication.NewApiKeyAuthenticationProvider(
        "YOUR_RIXL_API_KEY", "X-API-Key", authentication.HEADER_KEYLOCATION,
    )
    adapter, _ := kiotahttp.NewNetHttpRequestAdapter(auth)
    client := sdk.NewRixlClient(adapter)

    image, err := client.Images().ByImageId("PS5IMKoFLm").Get(context.Background(), nil)
    if err != nil {
        panic(err)
    }
    fmt.Println(*image.GetId(), *image.GetWidth(), *image.GetHeight())
}
```

Default base URL: `https://api.rixl.com`. Override with `adapter.SetBaseUrl(...)`.

## Authentication

API key:

```go
import "github.com/microsoft/kiota-abstractions-go/authentication"

auth, _ := authentication.NewApiKeyAuthenticationProvider(
    "YOUR_RIXL_API_KEY", "X-API-Key", authentication.HEADER_KEYLOCATION,
)
```

Bearer token (implement `AccessTokenProvider`):

```go
auth, _ := authentication.NewBaseBearerTokenAuthenticationProvider(tokenProvider)
```

## Feeds

```go
posts, err := client.Feeds().ByFeedId("FD4y3QB38S").Get(ctx, nil)
for _, post := range posts.GetData() {
    fmt.Println(*post.GetId())
}
```

## Images

```go
page, err := client.Images().Get(ctx, nil)
image, err := client.Images().ByImageId("PS5IMKoFLm").Get(ctx, nil)
err = client.Images().ByImageId("PS5IMKoFLm").Delete(ctx, nil)
```

Upload (init → PUT bytes → complete):

```go
import imghandler "github.com/rixlhq/rixl-go/sdk/models/internal_images_handler"

initReq := imghandler.NewUploadInitRequest()
name, format := "photo.jpg", "jpeg"
initReq.SetName(&name)
initReq.SetFormat(&format)

initRes, err := client.Images().Upload().Init().Post(ctx, initReq, nil)
// PUT bytes to *initRes.GetPresignedUrl()

completeReq := imghandler.NewCompleteRequest()
completeReq.SetImageId(initRes.GetImageId())
attached := false
completeReq.SetAttachedToVideo(&attached)
image, err := client.Images().Upload().Complete().Post(ctx, completeReq, nil)
```

## Videos

```go
videos, err := client.Videos().Get(ctx, nil)
video, err := client.Videos().ByVideoId("VI9VXQxWXQ").Get(ctx, nil)
tracks, err := client.Videos().ByVideoId("VI9VXQxWXQ").Subtitles().Get(ctx, nil)
```

Upload returns presigned URLs for both the video and a poster image:

```go
import (
    "github.com/rixlhq/rixl-go/sdk/models"
    vidupload "github.com/rixlhq/rixl-go/sdk/models/github_com_rixlhq_api_internal_videos_handler_upload"
)

initReq := models.NewVideoUploadInitRequest()
fileName, posterFormat := "clip.mp4", "jpeg"
initReq.SetFileName(&fileName)
initReq.SetImageFormat(&posterFormat)

initRes, err := client.Videos().Upload().Init().Post(ctx, initReq, nil)
// PUT to initRes.GetVideoPresignedUrl() and initRes.GetPosterPresignedUrl()

completeReq := vidupload.NewCompleteRequest()
completeReq.SetVideoId(initRes.GetVideoId())
video, err := client.Videos().Upload().Complete().Post(ctx, completeReq, nil)
```

## Pagination

List endpoints take `limit`, `offset`, `sort`, `order`:

```go
import (
    kabs "github.com/microsoft/kiota-abstractions-go"
    "github.com/rixlhq/rixl-go/sdk/images"
)

limit, offset := int32(50), int32(0)
for {
    cfg := &kabs.RequestConfiguration[images.ImagesRequestBuilderGetQueryParameters]{
        QueryParameters: &images.ImagesRequestBuilderGetQueryParameters{
            Limit: &limit, Offset: &offset,
        },
    }
    page, err := client.Images().Get(ctx, cfg)
    if err != nil {
        return err
    }
    if offset+int32(len(page.GetData())) >= *page.GetPagination().GetTotal() {
        break
    }
    offset += limit
}
```

## Errors

API errors come back as `*ErrorResponse`:

```go
import (
    "errors"
    apierr "github.com/rixlhq/rixl-go/sdk/models/github_com_rixlhq_api_internal_errors"
)

image, err := client.Images().ByImageId("PS5IMKoFLm").Get(ctx, nil)
if err != nil {
    var apiErr *apierr.ErrorResponse
    if errors.As(err, &apiErr) {
        fmt.Printf("HTTP %d: %s\n", *apiErr.GetCode(), *apiErr.GetError())
    }
    return err
}
```

## Examples

Runnable demos in [examples/](./examples):

```bash
export RIXL_API_KEY=<key>
go run ./examples/basic/images
go run ./examples/advanced/videos
```

## Issues

[github.com/rixlhq/rixl-go/issues](https://github.com/rixlhq/rixl-go/issues)
