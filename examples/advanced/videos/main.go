// Request a presigned video upload. The response carries separate URLs for the
// video and its poster image.
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

	name := "clip.mp4"
	upload, err := client.Videos.CreateVideoUpload(ctx, projectID, models.VideosV1CreateVideoUploadRequest{
		Name: &name,
	})
	if err != nil {
		log.Fatalf("create upload: %v", err)
	}
	if upload.VideoID == nil || upload.VideoUploadURL == nil {
		log.Fatal("upload response missing video_id or video_upload_url")
	}
	log.Printf("video %s: PUT the bytes to %s", *upload.VideoID, *upload.VideoUploadURL)
	if upload.PosterUploadURL != nil {
		log.Printf("poster: PUT to %s", *upload.PosterUploadURL)
	}
}
