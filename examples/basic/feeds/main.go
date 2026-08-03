// List feeds in a project, optionally fetch one by ID.
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

	page, err := client.Feeds.ListFeeds(ctx, projectID, nil)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	log.Printf("listed %d feeds", len(page.Feeds))
	for _, feed := range page.Feeds {
		if feed.ID != nil && feed.Name != nil {
			log.Printf("  - %s (%s)", *feed.ID, *feed.Name)
		}
	}

	id := os.Getenv("FEED_ID")
	if id == "" {
		return
	}
	feed, err := client.Feeds.GetFeed(ctx, projectID, id)
	if err != nil {
		log.Fatalf("get %s: %v", id, err)
	}
	if feed.Name != nil {
		log.Printf("feed %s: %s", id, *feed.Name)
	}
}
