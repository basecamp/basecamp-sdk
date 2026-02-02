// Package testing provides HTTP mocking utilities for SDK tests.
//
// This package is inspired by the httpmock package from gh-cli and provides
// a Registry type for stubbing HTTP requests and responses in tests.
//
// # Basic Usage
//
// Create a Registry and register request/response stubs:
//
//	func TestMyFeature(t *testing.T) {
//	    reg := testing.NewRegistry(t)
//
//	    reg.Register(
//	        testing.REST("GET", "projects.json"),
//	        testing.RespondJSON([]Project{{ID: 1, Name: "Test"}}),
//	    )
//
//	    client := basecamp.NewClient(cfg, token,
//	        basecamp.WithTransport(reg))
//
//	    // ... use client ...
//
//	    reg.Verify(t) // Fails if any stubs were not matched
//	}
//
// # Matchers
//
// Matchers determine which requests a stub should respond to:
//
//   - REST(method, path) - Match by HTTP method and path
//   - MatchPath(path) - Match by path only
//   - MatchMethod(method) - Match by HTTP method only
//   - MatchPathPattern(regex) - Match path using regular expression
//   - MatchQuery(method, path, query) - Match by method, path, and query params
//   - MatchHeader(name, value) - Match by request header
//   - MatchJSONBody(method, path, callback) - Match by JSON request body
//   - And(matchers...) - All matchers must match
//   - Or(matchers...) - Any matcher can match
//
// # Responders
//
// Responders generate HTTP responses for matched requests:
//
//   - RespondJSON(body) - Return 200 OK with JSON body
//   - StatusJSON(status, body) - Return specific status with JSON body
//   - StringResponse(body) - Return 200 OK with string body
//   - StatusStringResponse(status, body) - Return specific status with string body
//   - RespondError(status, message) - Return error response
//   - RespondNotFound() - Return 404 Not Found
//   - RespondRateLimit(retryAfter) - Return 429 Too Many Requests
//   - WithPagination(responder, nextURL) - Add Link header for pagination
//   - RESTPayload(status, body, callback) - Capture request payload
//   - Sequence(responders...) - Return different responses per call
//
// # Verification
//
// After your test completes, call Verify to ensure all registered stubs
// were matched. This helps catch cases where expected requests were never made.
//
// # Multiple Requests
//
// For endpoints called multiple times, register multiple stubs - they are
// matched and consumed in registration order:
//
//	reg.Register(testing.REST("GET", "items.json"), testing.RespondJSON(page1))
//	reg.Register(testing.REST("GET", "items.json"), testing.RespondJSON(page2))
//
// # Excluding Requests
//
// Use Exclude to fail the test if certain requests are made:
//
//	reg.Exclude(testing.MatchPathPrefix("/admin"))
package testing
