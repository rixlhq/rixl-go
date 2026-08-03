// List posts in a feed, optionally fetch one by ID.
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
	projectID, feedID := env("RIXL_PROJECT_ID"), env("FEED_ID")
	client, ctx, cancel := newClient()
	defer cancel()

	page, err := client.Posts.ListPosts(ctx, projectID, feedID, nil)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	log.Printf("listed %d posts", len(page.Posts))
	for _, post := range page.Posts {
		if post.ID != nil {
			log.Printf("  - %s", *post.ID)
		}
	}

	id := os.Getenv("POST_ID")
	if id == "" {
		return
	}
	got, err := client.Posts.GetPost2(ctx, projectID, feedID, id)
	if err != nil {
		log.Fatalf("get %s: %v", id, err)
	}
	if got.Post != nil && got.Post.ID != nil {
		log.Printf("post %s", *got.Post.ID)
	}
}
