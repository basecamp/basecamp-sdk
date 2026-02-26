package basecamp_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func ExampleNewClient_staticToken() {
	// Create a client with a static token (simplest authentication method)
	cfg := basecamp.DefaultConfig()

	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	// Use the client to make API calls
	_ = client
	fmt.Println("Client created with static token")
	// Output: Client created with static token
}

func ExampleNewClient_oauth() {
	// Create a client with OAuth authentication
	cfg := basecamp.DefaultConfig()

	authMgr := basecamp.NewAuthManager(cfg, http.DefaultClient)
	client := basecamp.NewClient(cfg, authMgr)

	// Use the client to make API calls
	_ = client
	fmt.Println("Client created with OAuth")
	// Output: Client created with OAuth
}

func ExampleNewClient_options() {
	// Create a client with custom options
	cfg := basecamp.DefaultConfig()

	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token,
		basecamp.WithUserAgent("my-app/1.0"),
		basecamp.WithLogger(slog.Default()),
	)

	_ = client
	fmt.Println("Client created with options")
	// Output: Client created with options
}

func ExampleDefaultConfig() {
	// Create a default configuration
	cfg := basecamp.DefaultConfig()

	// Override with environment variables
	cfg.LoadConfigFromEnv()

	// Or set values programmatically
	cfg.ProjectID = "67890"
	cfg.CacheEnabled = true

	fmt.Println("Configuration ready")
	// Output: Configuration ready
}

func ExampleProjectsService_List() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	// List all active projects
	result, err := client.ForAccount("12345").Projects().List(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range result.Projects {
		fmt.Printf("Project: %s (ID: %d)\n", p.Name, p.ID)
	}
}

func ExampleProjectsService_List_archived() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	// List archived projects
	result, err := client.ForAccount("12345").Projects().List(ctx, &basecamp.ProjectListOptions{
		Status: basecamp.ProjectStatusArchived,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range result.Projects {
		fmt.Printf("Archived: %s\n", p.Name)
	}
}

func ExampleProjectsService_Create() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	// Create a new project
	project, err := client.ForAccount("12345").Projects().Create(ctx, &basecamp.CreateProjectRequest{
		Name:        "Q1 Planning",
		Description: "Planning for the first quarter",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created project: %s (ID: %d)\n", project.Name, project.ID)
}

func ExampleTodosService_List() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	todolistID := int64(789012)

	// List all todos in a todolist
	todosResult, err := client.ForAccount("12345").Todos().List(ctx, todolistID, nil)
	if err != nil {
		log.Fatal(err)
	}

	for _, t := range todosResult.Todos {
		status := "[ ]"
		if t.Completed {
			status = "[x]"
		}
		fmt.Printf("%s %s\n", status, t.Content)
	}
}

func ExampleTodosService_Create() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	todolistID := int64(789012)

	// Create a new todo with assignees and due date
	todo, err := client.ForAccount("12345").Todos().Create(ctx, todolistID, &basecamp.CreateTodoRequest{
		Content:     "Review pull request",
		Description: "<strong>Priority:</strong> High",
		DueOn:       "2024-12-31",
		AssigneeIDs: []int64{111, 222},
		Notify:      true,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created todo: %s (ID: %d)\n", todo.Content, todo.ID)
}

func ExampleTodosService_Complete() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	todoID := int64(789012)

	// Mark a todo as complete
	err := client.ForAccount("12345").Todos().Complete(ctx, todoID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Todo completed")
}

func ExampleSearchService_Search() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	// Search across the account
	results, err := client.ForAccount("12345").Search().Search(ctx, "quarterly report", nil)
	if err != nil {
		log.Fatal(err)
	}

	for _, r := range results {
		fmt.Printf("[%s] %s\n", r.Type, r.Title)
	}
}

func ExampleSearchService_Search_sorted() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	// Search with results sorted by creation date
	results, err := client.ForAccount("12345").Search().Search(ctx, "meeting notes", &basecamp.SearchOptions{
		Sort: "created_at",
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, r := range results {
		fmt.Printf("%s: %s\n", r.CreatedAt.Format("2006-01-02"), r.Title)
	}
}

func ExampleMessagesService_Create() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	boardID := int64(789012)

	// Create a message on a message board
	message, err := client.ForAccount("12345").Messages().Create(ctx, boardID, &basecamp.CreateMessageRequest{
		Subject: "Weekly Update",
		Content: "<p>Here's what we accomplished this week...</p>",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Posted: %s\n", message.Subject)
}

func ExamplePeopleService_List() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	// List all people in the account
	peopleResult, err := client.ForAccount("12345").People().List(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range peopleResult.People {
		fmt.Printf("%s <%s>\n", p.Name, p.EmailAddress)
	}
}

func ExampleAccountClient_GetAll() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()
	account := client.ForAccount("12345")

	// GetAll automatically handles pagination for account-scoped resources
	results, err := account.GetAll(ctx, "/projects.json")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Fetched %d projects across all pages\n", len(results))
}

func Example_errorHandling() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	// Get a project that may not exist
	project, err := client.ForAccount("12345").Projects().Get(ctx, 999999999)
	if err != nil {
		if apiErr, ok := errors.AsType[*basecamp.Error](err); ok {
			switch apiErr.Code {
			case basecamp.CodeNotFound:
				fmt.Println("Project not found")
			case basecamp.CodeAuth:
				fmt.Println("Authentication required - please log in")
			case basecamp.CodeForbidden:
				fmt.Println("Access denied")
			case basecamp.CodeRateLimit:
				fmt.Println("Rate limited - try again later")
			default:
				fmt.Printf("API error: %s\n", apiErr.Message)
			}
		} else {
			fmt.Printf("Error: %v\n", err)
		}
		return
	}

	fmt.Printf("Found project: %s\n", project.Name)
}

func ExampleWebhooksService_Create() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	// Create a webhook to receive notifications
	var bucketID int64 = 67890
	webhook, err := client.ForAccount("12345").Webhooks().Create(ctx, bucketID, &basecamp.CreateWebhookRequest{
		PayloadURL: "https://example.com/webhooks/basecamp",
		Types:      []string{"Todo", "Comment"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Webhook created: %s\n", webhook.PayloadURL)
}

func ExampleCommentsService_Create() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	recordingID := int64(789012) // Can be a todo, message, etc.

	// Add a comment to any recording
	comment, err := client.ForAccount("12345").Comments().Create(ctx, recordingID, &basecamp.CreateCommentRequest{
		Content: "<p>Looks good to me!</p>",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Comment added by %s\n", comment.Creator.Name)
}

func ExampleCampfiresService_CreateLine() {
	cfg := basecamp.DefaultConfig()
	token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
	client := basecamp.NewClient(cfg, token)

	ctx := context.Background()

	campfireID := int64(789012)

	// Post a message to a campfire (chat)
	line, err := client.ForAccount("12345").Campfires().CreateLine(ctx, campfireID, "Hello team!")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Message posted: %s\n", line.Content)
}
