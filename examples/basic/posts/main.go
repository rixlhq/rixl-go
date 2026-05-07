// Fetch a single post inside a feed. Posts always live under a feed —
// there's no top-level posts collection.
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
	postID := os.Getenv("RIXL_POST_ID")
	if apiKey == "" || feedID == "" || postID == "" {
		log.Fatal("set RIXL_API_KEY, RIXL_FEED_ID, and RIXL_POST_ID")
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

	post, err := client.Feeds.GetFeedsFeedIdPostId(ctx, feedID, postID)
	if err != nil {
		log.Fatalf("get post: %v", err)
	}
	log.Printf("post %s", *post.ID)
}
