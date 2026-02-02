// Copyright 2025 Basecamp. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The caching command demonstrates HTTP ETag-based caching in the Basecamp SDK.
// It shows how to:
//   - Enable caching via configuration
//   - Use a custom cache directory
//   - Observe cache hits and misses
//   - Clear the cache when needed
//
// ETag caching reduces API calls and improves performance for read-heavy workloads.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func main() {
	// Get credentials from environment.
	token := os.Getenv("BASECAMP_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: BASECAMP_TOKEN environment variable is required")
		os.Exit(1)
	}

	accountID := os.Getenv("BASECAMP_ACCOUNT_ID")
	if accountID == "" {
		fmt.Fprintln(os.Stderr, "Error: BASECAMP_ACCOUNT_ID environment variable is required")
		os.Exit(1)
	}

	// Use a local cache directory for this example.
	// In production, use the default cache directory or configure via environment.
	cacheDir := filepath.Join(os.TempDir(), "basecamp-cache-example")
	fmt.Printf("Cache directory: %s\n", cacheDir)
	fmt.Println()

	// Create configuration with caching enabled.
	cfg := basecamp.DefaultConfig()
	cfg.CacheEnabled = true // Enable ETag caching
	cfg.CacheDir = cacheDir // Custom cache directory

	// Create a custom cache instance.
	// This gives us direct access to cache operations for demonstration.
	cache := basecamp.NewCache(cacheDir)

	// Clear any existing cache to start fresh.
	if err := cache.Clear(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not clear cache: %v\n", err)
	}
	fmt.Println("Cache cleared for fresh demonstration.")
	fmt.Println()

	// Create the client with caching enabled.
	tokenProvider := &basecamp.StaticTokenProvider{Token: token}
	client := basecamp.NewClient(cfg, tokenProvider, basecamp.WithCache(cache))
	account := client.ForAccount(accountID)

	ctx := context.Background()

	// First request: Cache MISS
	// The SDK makes a full request to the API and caches the response.
	fmt.Println("=== First Request (Cache Miss) ===")
	start := time.Now()

	result1, err := account.Projects().List(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing projects: %v\n", err)
		os.Exit(1)
	}

	elapsed1 := time.Since(start)
	fmt.Printf("Found %d project(s)\n", len(result1.Projects))
	fmt.Printf("Request time: %v\n", elapsed1)
	fmt.Println()

	// The SDK caches responses with ETags automatically.
	// When the API returns an ETag header, the response body is stored.
	fmt.Println("Response cached. The SDK stored:")
	fmt.Println("  - Response body (JSON)")
	fmt.Println("  - ETag header value")
	fmt.Println()

	// Second request: Cache HIT (304 Not Modified)
	// The SDK sends If-None-Match with the cached ETag.
	// If the resource hasn't changed, the API returns 304 Not Modified.
	fmt.Println("=== Second Request (Cache Hit) ===")
	start = time.Now()

	result2, err := account.Projects().List(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing projects: %v\n", err)
		os.Exit(1)
	}

	elapsed2 := time.Since(start)
	fmt.Printf("Found %d project(s)\n", len(result2.Projects))
	fmt.Printf("Request time: %v\n", elapsed2)
	fmt.Println()

	// Show the cache behavior.
	fmt.Println("The SDK sent: If-None-Match: <cached-etag>")
	fmt.Println("The API returned: 304 Not Modified")
	fmt.Println("The SDK used the cached response body.")
	fmt.Println()

	// Compare request times.
	// Cache hits are faster because:
	// 1. Server returns 304 with no body (less data transfer)
	// 2. No JSON parsing of the response (body already cached)
	fmt.Println("=== Performance Comparison ===")
	fmt.Printf("First request (miss):  %v\n", elapsed1)
	fmt.Printf("Second request (hit):  %v\n", elapsed2)
	if elapsed2 < elapsed1 {
		improvement := float64(elapsed1-elapsed2) / float64(elapsed1) * 100
		fmt.Printf("Cache hit was %.1f%% faster\n", improvement)
	}
	fmt.Println()

	// Demonstrate cache invalidation.
	// In a real application, caches are invalidated when:
	// 1. You modify a resource (POST/PUT/DELETE)
	// 2. You explicitly clear the cache
	// 3. The ETag changes (server-side resource modified)
	fmt.Println("=== Cache Invalidation ===")
	fmt.Println("Clearing cache...")
	if err := cache.Clear(); err != nil {
		fmt.Fprintf(os.Stderr, "Error clearing cache: %v\n", err)
		os.Exit(1)
	}

	// Third request: Cache MISS again
	fmt.Println()
	fmt.Println("=== Third Request (After Invalidation) ===")
	start = time.Now()

	result3, err := account.Projects().List(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing projects: %v\n", err)
		os.Exit(1)
	}

	elapsed3 := time.Since(start)
	fmt.Printf("Found %d project(s)\n", len(result3.Projects))
	fmt.Printf("Request time: %v (cache miss)\n", elapsed3)
	fmt.Println()

	// Show cache directory contents.
	fmt.Println("=== Cache Storage ===")
	fmt.Printf("Cache directory: %s\n", cacheDir)
	fmt.Println()
	fmt.Println("Cache files:")
	fmt.Println("  etags.json     - Maps URLs to ETag values")
	fmt.Println("  responses/     - Cached response bodies")
	fmt.Println()

	// Best practices summary.
	fmt.Println("=== Caching Best Practices ===")
	fmt.Println()
	fmt.Println("1. Enable caching for read-heavy workloads:")
	fmt.Println("   cfg.CacheEnabled = true")
	fmt.Println()
	fmt.Println("2. Configure cache directory if needed:")
	fmt.Println("   cfg.CacheDir = \"/path/to/cache\"")
	fmt.Println("   Or set BASECAMP_CACHE_DIR environment variable")
	fmt.Println()
	fmt.Println("3. The SDK automatically handles:")
	fmt.Println("   - Sending If-None-Match headers")
	fmt.Println("   - Storing ETags and response bodies")
	fmt.Println("   - Returning cached data on 304 responses")
	fmt.Println()
	fmt.Println("4. Cache is per-token to prevent data leakage")
	fmt.Println("   between different authenticated users")
}
