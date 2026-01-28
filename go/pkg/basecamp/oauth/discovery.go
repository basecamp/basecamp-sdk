package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Discoverer fetches OAuth 2.0 server configuration from discovery endpoints.
type Discoverer struct {
	httpClient *http.Client
}

// NewDiscoverer creates a Discoverer with the given HTTP client.
// If httpClient is nil, http.DefaultClient is used.
func NewDiscoverer(httpClient *http.Client) *Discoverer {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Discoverer{httpClient: httpClient}
}

// Discover fetches OAuth configuration from the well-known discovery endpoint.
// The baseURL should be the OAuth server's base URL (e.g., "https://launchpad.37signals.com").
func (d *Discoverer) Discover(ctx context.Context, baseURL string) (*Config, error) {
	// Normalize base URL
	baseURL = strings.TrimSuffix(baseURL, "/")
	discoveryURL := baseURL + "/.well-known/oauth-authorization-server"

	req, err := http.NewRequestWithContext(ctx, "GET", discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discovery request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discovery failed with status %d: %s", resp.StatusCode, string(body))
	}

	var config Config
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("parsing discovery response: %w", err)
	}

	return &config, nil
}
