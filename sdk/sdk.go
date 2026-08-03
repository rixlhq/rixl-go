// Package sdk is the entry point for the RIXL Go client.
//
//	client, err := sdk.New(apiKey)
//	page, err := client.Images.ListImages(ctx, projectID, nil)
package sdk

import (
	"context"
	"net/http"
)

const baseURL = "https://api.rixl.com"

// Client exposes one typed client per API resource. The resource fields are
// generated from the OpenAPI spec — see sdk/resources.gen.go.
type Client struct {
	*resources

	Credentials *CredentialsClient

	httpClient *http.Client
}

func New(apiKey string, opts ...Option) (*Client, error) {
	var cfg config
	if apiKey != "" {
		cfg.editors = append(cfg.editors, headerEditor("X-API-Key", apiKey))
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := resolveCredentials(&cfg); err != nil {
		return nil, err
	}

	res, err := newResources(cfg)
	if err != nil {
		return nil, err
	}

	httpClient := cfg.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		resources:   res,
		Credentials: &CredentialsClient{baseURL: baseURL, httpClient: httpClient, editors: cfg.editors},
		httpClient:  httpClient,
	}, nil
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
	creds      *ClientCredentials
}

func headerEditor(name, value string) editorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set(name, value)
		return nil
	}
}
