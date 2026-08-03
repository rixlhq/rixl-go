// Request a presigned image upload. Uploading the bytes is a plain PUT to
// upload_url; the API completes the upload once storage confirms the object.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rixlhq/rixl-go/sdk"
	"github.com/rixlhq/rixl-go/sdk/models"
)

func env(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("missing %s", name)
	}
	return v
}

func newClient() (*sdk.Client, context.Context, context.CancelFunc) {
	client, err := sdk.New(env("RIXL_API_KEY"))
	if err != nil {
		log.Fatalf("client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	return client, ctx, cancel
}

func main() {
	projectID := env("RIXL_PROJECT_ID")
	client, ctx, cancel := newClient()
	defer cancel()

	name := "photo.jpg"
	upload, err := client.Images.CreateImageUpload(ctx, projectID, models.ImagesV1CreateImageUploadRequest{
		Name: &name,
	})
	if err != nil {
		log.Fatalf("create upload: %v", err)
	}
	if upload.ImageID == nil || upload.UploadURL == nil {
		log.Fatal("upload response missing image_id or upload_url")
	}
	log.Printf("image %s: PUT the bytes to %s", *upload.ImageID, *upload.UploadURL)
}
