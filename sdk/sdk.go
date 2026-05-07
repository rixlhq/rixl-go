// Package sdk is the entry point for the RIXL Go client.
//
//	client, err := sdk.New(apiKey)
//	page, err := client.Images.GetImages(ctx, nil)
package sdk

import (
	"context"
	"net/http"

	"github.com/rixlhq/rixl-go/sdk/feeds"
	"github.com/rixlhq/rixl-go/sdk/images"
	"github.com/rixlhq/rixl-go/sdk/videos"
)

const baseURL = "https://api.rixl.com"

type Client struct {
	Feeds  *feeds.SimpleClient
	Images *images.SimpleClient
	Videos *videos.SimpleClient
}

func New(apiKey string, opts ...Option) (*Client, error) {
	var cfg config
	if apiKey != "" {
		cfg.editors = append(cfg.editors, headerEditor("X-API-Key", apiKey))
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	feedsCli, err := feeds.NewSimpleClient(baseURL, feedsOpts(cfg)...)
	if err != nil {
		return nil, err
	}
	imagesCli, err := images.NewSimpleClient(baseURL, imagesOpts(cfg)...)
	if err != nil {
		return nil, err
	}
	videosCli, err := videos.NewSimpleClient(baseURL, videosOpts(cfg)...)
	if err != nil {
		return nil, err
	}

	return &Client{Feeds: feedsCli, Images: imagesCli, Videos: videosCli}, nil
}

type Option func(*config)

// WithBearer replaces the API key passed to New with a bearer token.
func WithBearer(token string) Option {
	return func(c *config) {
		c.editors = []editorFn{headerEditor("Authorization", "Bearer "+token)}
	}
}

func WithHTTPClient(h *http.Client) Option {
	return func(c *config) { c.httpClient = h }
}

func WithRequestEditor(fn func(ctx context.Context, req *http.Request) error) Option {
	return func(c *config) { c.editors = append(c.editors, fn) }
}

type editorFn = func(ctx context.Context, req *http.Request) error

type config struct {
	httpClient *http.Client
	editors    []editorFn
}

func headerEditor(name, value string) editorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set(name, value)
		return nil
	}
}

func feedsOpts(cfg config) []feeds.ClientOption {
	out := make([]feeds.ClientOption, 0, len(cfg.editors)+1)
	if cfg.httpClient != nil {
		out = append(out, feeds.WithHTTPClient(cfg.httpClient))
	}
	for _, e := range cfg.editors {
		out = append(out, feeds.WithRequestEditorFn(e))
	}
	return out
}

func imagesOpts(cfg config) []images.ClientOption {
	out := make([]images.ClientOption, 0, len(cfg.editors)+1)
	if cfg.httpClient != nil {
		out = append(out, images.WithHTTPClient(cfg.httpClient))
	}
	for _, e := range cfg.editors {
		out = append(out, images.WithRequestEditorFn(e))
	}
	return out
}

func videosOpts(cfg config) []videos.ClientOption {
	out := make([]videos.ClientOption, 0, len(cfg.editors)+1)
	if cfg.httpClient != nil {
		out = append(out, videos.WithHTTPClient(cfg.httpClient))
	}
	for _, e := range cfg.editors {
		out = append(out, videos.WithRequestEditorFn(e))
	}
	return out
}
