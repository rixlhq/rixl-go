// List images in a project, optionally fetch one by ID.
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
	projectID := os.Getenv("RIXL_PROJECT_ID")
	if projectID == "" {
		log.Fatal("missing RIXL_PROJECT_ID")
	}

	client, err := sdk.New(apiKey)
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, err := client.Images.ListImages(ctx, projectID, nil)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	log.Printf("listed %d images", len(page.Images))
	for _, img := range page.Images {
		if img.ID != nil {
			log.Printf("  - %s", *img.ID)
		}
	}

	id := os.Getenv("IMAGE_ID")
	if id == "" {
		return
	}
	got, err := client.Images.GetImage(ctx, id)
	if err != nil {
		log.Fatalf("get %s: %v", id, err)
	}
	if got.Image != nil {
		log.Printf("image %s: %dx%d", *got.Image.ID, *got.Image.Width, *got.Image.Height)
	}
}
