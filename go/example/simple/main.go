// Copyright 2025 Basecamp. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The simple command demonstrates basic usage of the Basecamp SDK.
// It authenticates using a static token from an environment variable
// and lists all projects in the specified account.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func main() {
	// Get authentication token from environment.
	// You can obtain a token by creating an OAuth application at
	// https://launchpad.37signals.com/integrations
	token := os.Getenv("BASECAMP_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: BASECAMP_TOKEN environment variable is required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "To get a token:")
		fmt.Fprintln(os.Stderr, "  1. Go to https://launchpad.37signals.com/integrations")
		fmt.Fprintln(os.Stderr, "  2. Create a new integration")
		fmt.Fprintln(os.Stderr, "  3. Complete the OAuth flow to get an access token")
		os.Exit(1)
	}

	// Get the account ID from environment.
	// This is the numeric ID in your Basecamp URL, e.g., https://3.basecamp.com/12345/
	accountID := os.Getenv("BASECAMP_ACCOUNT_ID")
	if accountID == "" {
		fmt.Fprintln(os.Stderr, "Error: BASECAMP_ACCOUNT_ID environment variable is required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Find your account ID in your Basecamp URL:")
		fmt.Fprintln(os.Stderr, "  https://3.basecamp.com/12345/ -> Account ID is 12345")
		os.Exit(1)
	}

	// Create a configuration with default settings.
	// The SDK uses https://3.basecampapi.com as the default base URL.
	cfg := basecamp.DefaultConfig()

	// Create a static token provider.
	// This is the simplest way to authenticate when you already have a token.
	// For interactive applications, use AuthManager with OAuth flow instead.
	tokenProvider := &basecamp.StaticTokenProvider{Token: token}

	// Create the SDK client.
	// The client handles HTTP transport, retries, rate limiting, and caching.
	client := basecamp.NewClient(cfg, tokenProvider)

	// Bind the client to a specific Basecamp account.
	// All API operations require an account context.
	account := client.ForAccount(accountID)

	// List all active projects in the account.
	ctx := context.Background()
	result, err := account.Projects().List(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing projects: %v\n", err)
		os.Exit(1)
	}

	// Display the results.
	fmt.Printf("Found %d project(s):\n\n", len(result.Projects))

	for i, project := range result.Projects {
		fmt.Printf("%d. %s\n", i+1, project.Name)
		if project.Description != "" {
			fmt.Printf("   Description: %s\n", project.Description)
		}
		fmt.Printf("   Status: %s\n", project.Status)
		fmt.Printf("   URL: %s\n", project.AppURL)
		fmt.Println()
	}

	// If there are no projects, suggest creating one.
	if len(result.Projects) == 0 {
		fmt.Println("No projects found. Create one in Basecamp to see it here!")
	}
}
