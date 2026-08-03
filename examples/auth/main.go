// One file showing both auth flows. Pick one by setting env vars:
//
//   - API key:     RIXL_API_KEY=...
//   - Client auth: RIXL_CLIENT_ID=..., RIXL_CLIENT_SECRET=..., RIXL_SUBJECT=...
//     (RIXL_PROJECT_ID optional)
//
// Copy the credentials from the RIXL dashboard.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rixlhq/rixl-go/sdk"
)

func main() {
	client, err := buildClient()
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, err := client.Images.GetImages(ctx, nil)
	if err != nil {
		log.Fatalf("verify: %v", err)
	}
	log.Printf("auth ok — listed %d images", len(page.Data))
}

func buildClient() (*sdk.Client, error) {
	if key := os.Getenv("RIXL_API_KEY"); key != "" {
		log.Println("auth: API key")
		return sdk.New(key)
	}

	clientID := os.Getenv("RIXL_CLIENT_ID")
	clientSecret := os.Getenv("RIXL_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("set RIXL_API_KEY, or RIXL_CLIENT_ID + RIXL_CLIENT_SECRET + RIXL_SUBJECT")
	}

	log.Println("auth: client credentials")
	return sdk.New("", sdk.WithClientCredentials(sdk.ClientCredentials{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Subject:      mustEnv("RIXL_SUBJECT"),
		ProjectID:    os.Getenv("RIXL_PROJECT_ID"),
		Scopes:       sdk.MediaReadScopes,
	}))
}

func mustEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("missing %s", name)
	}
	return v
}
