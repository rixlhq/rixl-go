// List images in your project, optionally fetch one by ID.
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

	page, err := client.Images.GetImages(ctx, nil)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	log.Printf("listed %d images", len(page.Data))
	for _, img := range page.Data {
		if img.ID != nil {
			log.Printf("  - %s", *img.ID)
		}
	}

	id := os.Getenv("IMAGE_ID")
	if id == "" {
		return
	}
	img, err := client.Images.GetImagesImageId(ctx, id)
	if err != nil {
		log.Fatalf("get %s: %v", id, err)
	}
	log.Printf("image %s: %dx%d", *img.ID, *img.Width, *img.Height)
}
