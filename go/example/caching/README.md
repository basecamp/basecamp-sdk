# Caching Example

This example demonstrates ETag-based HTTP caching in the Basecamp SDK.

## What it demonstrates

- Enabling caching via configuration
- How cache hits and misses work
- Performance benefits of caching
- Cache invalidation
- Cache storage structure

## How ETag caching works

ETags (Entity Tags) are HTTP headers that identify a specific version of a resource:

1. **First request**: The API returns a response with an `ETag` header
2. **SDK caches**: The response body and ETag are stored locally
3. **Subsequent requests**: The SDK sends `If-None-Match: <etag>`
4. **If unchanged**: API returns `304 Not Modified` (no body)
5. **SDK returns**: The cached response body

This reduces bandwidth and server load while ensuring data freshness.

## Prerequisites

1. A Basecamp account
2. An access token (see [simple example](../simple/))
3. Your Basecamp account ID

## Running the example

```bash
export BASECAMP_TOKEN="your-access-token"
export BASECAMP_ACCOUNT_ID="12345"
go run main.go
```

## Expected output

```
Cache directory: /tmp/basecamp-cache-example
Cache cleared for fresh demonstration.

=== First Request (Cache Miss) ===
Found 3 project(s)
Request time: 245ms

Response cached. The SDK stored:
  - Response body (JSON)
  - ETag header value

=== Second Request (Cache Hit) ===
Found 3 project(s)
Request time: 89ms

The SDK sent: If-None-Match: <cached-etag>
The API returned: 304 Not Modified
The SDK used the cached response body.

=== Performance Comparison ===
First request (miss):  245ms
Second request (hit):  89ms
Cache hit was 63.7% faster
```

## Configuration options

### Enable caching

```go
cfg := basecamp.DefaultConfig()
cfg.CacheEnabled = true
```

Or via environment variable:

```bash
export BASECAMP_CACHE_ENABLED=true
```

### Custom cache directory

```go
cfg.CacheDir = "/path/to/cache"
```

Or via environment variable:

```bash
export BASECAMP_CACHE_DIR=/path/to/cache
```

### Custom cache instance

For more control, create and pass your own cache:

```go
cache := basecamp.NewCache("/path/to/cache")
client := basecamp.NewClient(cfg, tokenProvider, basecamp.WithCache(cache))

// Later, clear the cache
cache.Clear()

// Or invalidate a specific key
cache.Invalidate(cacheKey)
```

## Cache storage structure

```
~/.cache/basecamp/
├── etags.json           # URL -> ETag mapping
└── responses/
    ├── abc123.body      # Cached response (hashed key)
    └── def456.body
```

The cache key is a hash of:
- Request URL
- Account ID
- Token hash (first 16 chars of SHA-256)

This ensures different users don't share cached data.

## When caching helps

Caching is most beneficial for:

- **Read-heavy workloads**: Repeatedly fetching the same data
- **Polling patterns**: Checking for changes periodically
- **List operations**: Projects, todos, messages that don't change often
- **Dashboards**: Displaying data that updates infrequently

## When to disable caching

Consider disabling caching for:

- **Real-time data**: When you need immediate consistency
- **Write-heavy workloads**: Data changes frequently
- **One-time operations**: Data won't be read again

## Cache invalidation

The cache is automatically invalidated when:

1. **ETag changes**: The server returns a new ETag
2. **Manual clear**: You call `cache.Clear()`
3. **Mutations**: POST/PUT/DELETE operations don't use cache

For explicit invalidation:

```go
// Clear everything
cache.Clear()

// Invalidate specific resource (if you know the key)
cache.Invalidate(key)
```

## Security considerations

1. **Token isolation**: Cache keys include a token hash, preventing data leakage between users
2. **Local storage**: Cache is stored locally, not shared across machines
3. **File permissions**: Cache files are created with 0600 permissions
4. **No sensitive data**: Only cacheable GET responses are stored
