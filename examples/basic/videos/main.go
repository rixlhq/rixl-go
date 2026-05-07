// List videos in your project, optionally fetch one by ID.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rixlhq/rixl-go/sdk"
)

func main() {
	apiKey := os.Getenv("RIXL_API_KEY")
	if apiKey == "" {
		log.Fatal("missing RIXL_API_KEY")
	}

	client, err := sdk.New(apiKey)
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, err := client.Videos.GetVideos(ctx, nil)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	log.Printf("listed %d videos", len(page.Data))
	for _, v := range page.Data {
		if v.ID != nil {
			log.Printf("  - %s", *v.ID)
		}
	}

	id := os.Getenv("VIDEO_ID")
	if id == "" {
		return
	}
	v, err := client.Videos.GetVideosVideoId(ctx, id)
	if err != nil {
		log.Fatalf("get %s: %v", id, err)
	}
	log.Printf("video %s", *v.ID)
}
