// One file showing both auth flows. Pick one by setting env vars:
//
//   - API key:    RIXL_API_KEY=...
//   - Client JWT: RIXL_CLIENT_ID=..., RIXL_CLIENT_SECRET=..., RIXL_PROJECT_ID=..., RIXL_SUBJECT=...
//
// Copy the credentials from the RIXL dashboard.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rixlhq/rixl-go/sdk"
)

func main() {
	baseURL := envOr("RIXL_BASE_URL", "http://localhost:8081")

	client, err := buildClient(baseURL)
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

func buildClient(baseURL string) (*sdk.Client, error) {
	if key := os.Getenv("RIXL_API_KEY"); key != "" {
		log.Println("auth: API key")
		return sdk.New(key, sdk.WithBaseURL(baseURL))
	}

	clientID := os.Getenv("RIXL_CLIENT_ID")
	clientSecret := os.Getenv("RIXL_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("set RIXL_API_KEY, or RIXL_CLIENT_ID + RIXL_CLIENT_SECRET + RIXL_PROJECT_ID + RIXL_SUBJECT")
	}

	log.Println("auth: client JWT")
	token := mintToken(baseURL, map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"subject":       mustEnv("RIXL_SUBJECT"),
		"project_id":    mustEnv("RIXL_PROJECT_ID"),
	})
	return sdk.New("", sdk.WithBaseURL(baseURL), sdk.WithBearer(token))
}

func mintToken(baseURL string, body map[string]string) string {
	payload, _ := json.Marshal(body)
	resp, err := http.Post(baseURL+"/clientauth/token", "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Fatalf("mint token: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("mint token: %s: %s", resp.Status, raw)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		log.Fatalf("decode token: %v", err)
	}
	return out.AccessToken
}

func mustEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("missing %s", name)
	}
	return v
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
