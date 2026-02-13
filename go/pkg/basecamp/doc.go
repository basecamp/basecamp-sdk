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
//	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
//	client := basecamp.NewClient(cfg, token)
//
//	// Create an account-scoped client for API operations
//	account := client.ForAccount("12345")
//
// OAuth 2.0 Authentication (for user-facing apps):
//
//	cfg := basecamp.DefaultConfig()
//	authMgr := basecamp.NewAuthManager(cfg, http.DefaultClient)
//	client := basecamp.NewClient(cfg, authMgr)
//
//	// Discover available accounts
//	info, _ := client.Authorization().GetInfo(ctx, nil)
//	account := client.ForAccount(fmt.Sprint(info.Accounts[0].ID))
//
// # Configuration
//
// Configuration can be loaded from environment variables or set programmatically:
//
//	cfg := basecamp.DefaultConfig()
//	cfg.LoadConfigFromEnv()  // Loads BASECAMP_PROJECT_ID, etc.
//
// Environment variables:
//   - BASECAMP_PROJECT_ID: Default project/bucket ID
//   - BASECAMP_TOKEN: Static API token for authentication
//   - BASECAMP_CACHE_ENABLED: Enable HTTP caching (default: true)
//
// # Services
//
// The SDK provides typed services for each Basecamp resource:
//
//   - [AccountClient.Projects] - Project management
//   - [AccountClient.Todos] - Todo items
//   - [AccountClient.Todolists] - Todo lists
//   - [AccountClient.Todosets] - Todo sets (containers for lists)
//   - [AccountClient.Messages] - Message board posts
//   - [AccountClient.MessageBoards] - Message boards
//   - [AccountClient.Comments] - Comments on any recording
//   - [AccountClient.People] - User and people management
//   - [AccountClient.Campfires] - Chat rooms
//   - [AccountClient.Schedules] - Calendar schedules
//   - [AccountClient.Vaults] - Document folders
//   - [AccountClient.Search] - Full-text search
//   - [AccountClient.Webhooks] - Webhook management
//   - [AccountClient.Events] - Activity events
//   - [AccountClient.Cards] - Card table cards
//   - [AccountClient.Attachments] - File attachments
//   - [Client.Authorization] - Account-agnostic authorization info
//
// # Working with Projects
//
// List all projects:
//
//	account := client.ForAccount("12345")
//	projects, err := account.Projects().List(ctx, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, p := range projects {
//	    fmt.Println(p.Name)
//	}
//
// Create a project:
//
//	project, err := account.Projects().Create(ctx, &basecamp.CreateProjectRequest{
//	    Name:        "New Project",
//	    Description: "Project description",
//	})
//
// # Working with Todos
//
// List todos in a todolist:
//
//	todos, err := account.Todos().List(ctx, projectID, todolistID, nil)
//
// Create a todo:
//
//	todo, err := account.Todos().Create(ctx, projectID, todolistID, &basecamp.CreateTodoRequest{
//	    Content: "Ship the feature",
//	    DueOn:   "2024-12-31",
//	})
//
// Complete a todo:
//
//	err := account.Todos().Complete(ctx, projectID, todoID)
//
// # Searching
//
// Search across your Basecamp account:
//
//	results, err := account.Search().Search(ctx, "quarterly report", nil)
//	for _, r := range results {
//	    fmt.Printf("%s: %s\n", r.Type, r.Title)
//	}
//
// # Pagination
//
// The SDK handles pagination automatically via GetAll:
//
//	// GetAll fetches all pages automatically
//	account := client.ForAccount("12345")
//	results, err := account.GetAll(ctx, "/projects.json")
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
//	    if apiErr, ok := errors.AsType[*basecamp.Error](err); ok {
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
// The Client is safe for concurrent use. The ForAccount method may be called
// concurrently from multiple goroutines to create AccountClient instances.
//
// Each AccountClient is also safe for concurrent use. Service accessors
// (e.g., account.Projects()) use mutex-protected lazy initialization.
//
// Example of concurrent multi-account usage:
//
//	acme := client.ForAccount("12345")
//	initech := client.ForAccount("67890")
//
//	go func() { acme.Todos().List(ctx, projectID, todolistID, nil) }()
//	go func() { initech.Projects().List(ctx, nil) }()
package basecamp
