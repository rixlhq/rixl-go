// Upload a video end-to-end. Same shape as the image flow, but Init returns
// two presigned URLs (one for the video, one for the poster thumbnail) and
// we PUT to both.
package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rixlhq/rixl-go/sdk"
	"github.com/rixlhq/rixl-go/sdk/models"
)

const (
	sampleVideoURL  = "https://download.samplelib.com/mp4/sample-5s.mp4"
	samplePosterURL = "https://picsum.photos/seed/rixl/800/600.jpg"
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	video, err := download(ctx, sampleVideoURL)
	if err != nil {
		log.Fatalf("download video: %v", err)
	}
	poster, err := download(ctx, samplePosterURL)
	if err != nil {
		log.Fatalf("download poster: %v", err)
	}
	log.Printf("downloaded video=%d poster=%d", len(video), len(poster))

	posterFormat := "jpeg"
	init, err := client.Videos.PostVideosUploadInit(ctx, models.VideoUploadInitRequest{
		FileName:    "sample.mp4",
		ImageFormat: &posterFormat,
	})
	if err != nil {
		log.Fatalf("init: %v", err)
	}
	log.Printf("init: video_id=%s poster_id=%s", *init.VideoID, *init.PosterID)

	if err := putBytes(ctx, *init.VideoPresignedURL, video, "video/mp4"); err != nil {
		log.Fatalf("PUT video: %v", err)
	}
	if err := putBytes(ctx, *init.PosterPresignedURL, poster, "image/jpeg"); err != nil {
		log.Fatalf("PUT poster: %v", err)
	}

	v, err := client.Videos.PostVideosUploadComplete(ctx, models.GithubComRixlhqAPIInternalVideosHandlerUploadCompleteRequest{
		VideoID: init.VideoID,
	})
	if err != nil {
		log.Fatalf("complete: %v", err)
	}
	log.Printf("complete: id=%s", *v.ID)
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func putBytes(ctx context.Context, url string, body []byte, contentType string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return &httpError{Status: resp.Status, Body: string(raw)}
	}
	return nil
}

type httpError struct{ Status, Body string }

func (e *httpError) Error() string { return e.Status + ": " + e.Body }
