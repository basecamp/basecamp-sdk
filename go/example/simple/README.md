# Simple Example

This example demonstrates basic usage of the Basecamp SDK with a static access token.

## What it demonstrates

- Configuring the SDK with default settings
- Using a static token for authentication
- Binding a client to a specific Basecamp account
- Listing projects in an account
- Basic error handling

## Prerequisites

1. A Basecamp account
2. An OAuth access token (see below)
3. Your Basecamp account ID

## Getting your credentials

### Account ID

Your account ID is the number in your Basecamp URL:
```
https://3.basecamp.com/12345/projects
                      ^^^^^
                      This is your account ID
```

### Access Token

To get an access token:

1. Go to [Basecamp Integrations](https://launchpad.37signals.com/integrations)
2. Click "Register another application"
3. Fill in the application details:
   - Name: Your app name
   - Redirect URI: `urn:ietf:wg:oauth:2.0:oob` (for CLI apps)
4. Complete the OAuth authorization flow
5. Exchange the authorization code for an access token

For production applications, see the [oauth example](../oauth/) for implementing the full OAuth flow.

## Running the example

Set the required environment variables:

```bash
export BASECAMP_TOKEN="your-access-token"
export BASECAMP_ACCOUNT_ID="12345"
```

Then run the example:

```bash
go run main.go
```

## Expected output

```
Found 3 project(s):

1. Marketing Campaign
   Description: Current campaign materials
   Status: active
   URL: https://3.basecamp.com/12345/projects/67890

2. Product Launch
   Status: active
   URL: https://3.basecamp.com/12345/projects/67891

3. Internal Docs
   Description: Company documentation
   Status: active
   URL: https://3.basecamp.com/12345/projects/67892
```

## Code walkthrough

### Configuration

```go
cfg := basecamp.DefaultConfig()
```

Creates a configuration with sensible defaults:
- Base URL: `https://3.basecampapi.com`
- Caching: disabled by default
- Standard timeouts and retry policies

### Authentication

```go
tokenProvider := &basecamp.StaticTokenProvider{Token: token}
```

The `StaticTokenProvider` is the simplest authentication method. It's ideal when:
- You already have a valid access token
- You're building a script or CLI tool
- Token refresh is handled externally

For long-running applications that need automatic token refresh, use `AuthManager` instead.

### Client creation

```go
client := basecamp.NewClient(cfg, tokenProvider)
```

The client handles:
- HTTP transport with connection pooling
- Automatic retries with exponential backoff
- Rate limit handling (429 responses)
- Request/response logging (when configured)

### Account binding

```go
account := client.ForAccount(accountID)
```

Basecamp's API requires an account ID in every request path. The `ForAccount` method creates an `AccountClient` that automatically includes the account ID.

### Making API calls

```go
result, err := account.Projects().List(ctx, nil)
```

Services are accessed via methods on `AccountClient`. Each service provides typed methods for CRUD operations.
