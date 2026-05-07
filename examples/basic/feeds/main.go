// Read a feed and print the posts attached to it.
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
	feedID := os.Getenv("RIXL_FEED_ID")
	if apiKey == "" || feedID == "" {
		log.Fatal("set RIXL_API_KEY and RIXL_FEED_ID")
	}
	baseURL := os.Getenv("RIXL_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}

	client, err := sdk.New(apiKey, sdk.WithBaseURL(baseURL))
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, err := client.Feeds.GetFeedsFeedId(ctx, feedID, nil)
	if err != nil {
		log.Fatalf("get feed %s: %v", feedID, err)
	}
	log.Printf("feed %s — %d posts", feedID, len(page.Data))
	for _, post := range page.Data {
		if post.ID != nil {
			log.Printf("  - %s", *post.ID)
		}
	}
}
