// Package basecamp provides a Go SDK for the Basecamp API.
//
// The SDK handles authentication, HTTP caching, rate limiting, and retry logic.
// It supports both OAuth 2.0 authentication and static token authentication.
//
// Basic usage with a static token:
//
//	cfg := basecamp.DefaultConfig()
//	cfg.AccountID = "12345"
//
//	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
//	client := basecamp.NewClient(cfg, token)
//
//	resp, err := client.Get(ctx, "/projects.json")
//
// Usage with OAuth authentication:
//
//	cfg := basecamp.DefaultConfig()
//	cfg.AccountID = "12345"
//
//	authMgr := basecamp.NewAuthManager(cfg, http.DefaultClient)
//	client := basecamp.NewClient(cfg, authMgr)
//
//	resp, err := client.Get(ctx, "/projects.json")
//
// The SDK automatically handles:
//   - ETag-based HTTP caching for GET requests
//   - Exponential backoff with jitter for retryable errors
//   - Token refresh when using OAuth
//   - Pagination for list endpoints via GetAll
package basecamp
