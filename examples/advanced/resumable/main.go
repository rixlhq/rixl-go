// Upload a large video over tus. The upload service creates the video record
// itself, so there is no CreateVideoUpload call. Run it again after an
// interruption and it continues from the bytes the server already holds.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/rixlhq/rixl-go/sdk"
)

func env(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("missing %s", name)
	}
	return v
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: resumable <video file>")
	}

	client, err := sdk.New(env("RIXL_API_KEY"))
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	upload, err := client.UploadFileResumable(ctx, sdk.ResumableUploadParams{
		ProjectID: env("RIXL_PROJECT_ID"),
		Kind:      sdk.UploadKindVideo,
	}, os.Args[1],
		sdk.WithProgress(func(uploaded, total int64) {
			log.Printf("%d / %d bytes", uploaded, total)
		}),
	)
	if err != nil {
		log.Fatalf("upload: %v", err)
	}
	log.Printf("video %s uploaded", upload.FileID)
}
