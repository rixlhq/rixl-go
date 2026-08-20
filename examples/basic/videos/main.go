// List videos in a project, optionally fetch one by ID.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rixlhq/rixl-go/sdk"
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

	page, err := client.Videos.ListVideos(ctx, projectID, nil)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	log.Printf("listed %d videos", len(page.Videos))
	for _, video := range page.Videos {
		if video.ID != nil {
			log.Printf("  - %s", *video.ID)
		}
	}

	id := os.Getenv("VIDEO_ID")
	if id == "" {
		return
	}
	got, err := client.Videos.GetVideo(ctx, id)
	if err != nil {
		log.Fatalf("get %s: %v", id, err)
	}
	if got.Video != nil && got.Video.ID != nil {
		log.Printf("video %s", *got.Video.ID)
	}
}
