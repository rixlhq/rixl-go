// Upload an image end-to-end:
//
//   1. Init     — tell the API you want to upload; it returns a presigned PUT URL.
//   2. PUT      — push the bytes straight to storage (the API never sees them).
//   3. Complete — tell the API the upload landed so it can finalize the record.
package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rixlhq/rixl-go/sdk"
	"github.com/rixlhq/rixl-go/sdk/models"
)

const sampleImageURL = "https://picsum.photos/seed/rixl/800/600.jpg"

func main() {
	apiKey := os.Getenv("RIXL_API_KEY")
	if apiKey == "" {
		log.Fatal("missing RIXL_API_KEY")
	}
	baseURL := os.Getenv("RIXL_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}

	client, err := sdk.New(apiKey, sdk.WithBaseURL(baseURL))
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	body, err := download(ctx, sampleImageURL)
	if err != nil {
		log.Fatalf("download: %v", err)
	}
	log.Printf("downloaded %d bytes", len(body))

	name, format := "sample.jpg", "jpeg"
	init, err := client.Images.PostImagesUploadInit(ctx, models.InternalImagesHandlerUploadInitRequest{
		Name:   &name,
		Format: &format,
	})
	if err != nil {
		log.Fatalf("init: %v", err)
	}
	log.Printf("init: image_id=%s", *init.ImageID)

	if err := putBytes(ctx, *init.PresignedURL, body, "image/jpeg"); err != nil {
		log.Fatalf("PUT: %v", err)
	}

	notAttached := false
	img, err := client.Images.PostImagesUploadComplete(ctx, models.InternalImagesHandlerCompleteRequest{
		ImageID:         init.ImageID,
		AttachedToVideo: &notAttached,
	})
	if err != nil {
		log.Fatalf("complete: %v", err)
	}
	log.Printf("complete: id=%s %dx%d", *img.ID, *img.Width, *img.Height)
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func putBytes(ctx context.Context, url string, body []byte, contentType string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return &httpError{Status: resp.Status, Body: string(raw)}
	}
	return nil
}

type httpError struct{ Status, Body string }

func (e *httpError) Error() string { return e.Status + ": " + e.Body }
