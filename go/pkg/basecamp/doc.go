// Package basecamp provides a Go SDK for the Basecamp 3 API.
//
// The SDK handles authentication, HTTP caching, rate limiting, and retry logic.
// It supports both OAuth 2.0 authentication and static token authentication.
//
// # Installation
//
// To install the SDK, use go get:
//
//	go get github.com/basecamp/basecamp-sdk/go/pkg/basecamp
//
// # Authentication
//
// The SDK supports two authentication methods:
//
// Static Token Authentication (simplest):
//
//	cfg := basecamp.DefaultConfig()
//	cfg.AccountID = "12345"
//
//	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
//	client := basecamp.NewClient(cfg, token)
//
// OAuth 2.0 Authentication (for user-facing apps):
//
//	cfg := basecamp.DefaultConfig()
//	cfg.AccountID = "12345"
//
//	authMgr := basecamp.NewAuthManager(cfg, http.DefaultClient)
//	client := basecamp.NewClient(cfg, authMgr)
//
// # Configuration
//
// Configuration can be loaded from environment variables or set programmatically:
//
//	cfg := basecamp.DefaultConfig()
//	cfg.LoadConfigFromEnv()  // Loads BASECAMP_ACCOUNT_ID, BASECAMP_PROJECT_ID, etc.
//
// Environment variables:
//   - BASECAMP_ACCOUNT_ID: Your Basecamp account ID (required)
//   - BASECAMP_PROJECT_ID: Default project/bucket ID
//   - BASECAMP_TOKEN: Static API token for authentication
//   - BASECAMP_CACHE_ENABLED: Enable HTTP caching (default: true)
//
// # Services
//
// The SDK provides typed services for each Basecamp resource:
//
//   - [Client.Projects] - Project management
//   - [Client.Todos] - Todo items
//   - [Client.Todolists] - Todo lists
//   - [Client.Todosets] - Todo sets (containers for lists)
//   - [Client.Messages] - Message board posts
//   - [Client.MessageBoards] - Message boards
//   - [Client.Comments] - Comments on any recording
//   - [Client.People] - User and people management
//   - [Client.Campfires] - Chat rooms
//   - [Client.Schedules] - Calendar schedules
//   - [Client.Vaults] - Document folders
//   - [Client.Search] - Full-text search
//   - [Client.Webhooks] - Webhook management
//   - [Client.Events] - Activity events
//   - [Client.Cards] - Card table cards
//   - [Client.Attachments] - File attachments
//
// # Working with Projects
//
// List all projects:
//
//	projects, err := client.Projects().List(ctx, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, p := range projects {
//	    fmt.Println(p.Name)
//	}
//
// Create a project:
//
//	project, err := client.Projects().Create(ctx, &basecamp.CreateProjectRequest{
//	    Name:        "New Project",
//	    Description: "Project description",
//	})
//
// # Working with Todos
//
// List todos in a todolist:
//
//	todos, err := client.Todos().List(ctx, projectID, todolistID, nil)
//
// Create a todo:
//
//	todo, err := client.Todos().Create(ctx, projectID, todolistID, &basecamp.CreateTodoRequest{
//	    Content: "Ship the feature",
//	    DueOn:   "2024-12-31",
//	})
//
// Complete a todo:
//
//	err := client.Todos().Complete(ctx, projectID, todoID)
//
// # Searching
//
// Search across your Basecamp account:
//
//	results, err := client.Search().Search(ctx, "quarterly report", nil)
//	for _, r := range results {
//	    fmt.Printf("%s: %s\n", r.Type, r.Title)
//	}
//
// # Pagination
//
// The SDK handles pagination automatically via GetAll:
//
//	// GetAll fetches all pages automatically
//	results, err := client.GetAll(ctx, "/projects.json")
//
// For fine-grained control, use Get with Link headers:
//
//	resp, err := client.Get(ctx, "/projects.json")
//	// Check resp.Headers.Get("Link") for pagination
//
// # Error Handling
//
// The SDK returns typed errors that can be inspected:
//
//	resp, err := client.Get(ctx, "/projects/999.json")
//	if err != nil {
//	    var apiErr *basecamp.Error
//	    if errors.As(err, &apiErr) {
//	        switch apiErr.Code {
//	        case basecamp.CodeNotFound:
//	            // Handle 404
//	        case basecamp.CodeAuth:
//	            // Handle authentication error
//	        case basecamp.CodeRateLimit:
//	            // Handle rate limiting (auto-retried by default)
//	        }
//	    }
//	}
//
// # Automatic Features
//
// The SDK automatically handles:
//   - ETag-based HTTP caching for GET requests
//   - Exponential backoff with jitter for retryable errors
//   - Token refresh when using OAuth
//   - Rate limit handling with automatic retry
//   - Pagination via GetAll for list endpoints
//
// # Client Options
//
// Customize the client with options:
//
//	client := basecamp.NewClient(cfg, token,
//	    basecamp.WithHTTPClient(customHTTPClient),
//	    basecamp.WithUserAgent("my-app/1.0"),
//	    basecamp.WithLogger(slog.Default()),
//	    basecamp.WithCache(customCache),
//	)
//
// # Thread Safety
//
// API operations on the Client and its services are safe for concurrent use
// by multiple goroutines. However, service accessors (e.g., client.Projects())
// use lazy initialization without synchronization. For concurrent use, either
// access each service once before sharing the client across goroutines, or
// call service methods directly (e.g., client.Projects().List()) which is safe.
package basecamp
