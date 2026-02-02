// Package testing provides HTTP mocking utilities for SDK tests.
//
// NOTE: This package is intentionally named "testing" to match the domain
// (test utilities). Since it lives in internal/, shadowing the stdlib testing
// package is acceptable - test files import both with aliases if needed.
//
// The Registry type is the central mock server that intercepts HTTP requests
// and returns stubbed responses. It implements http.RoundTripper so it can be
// used directly as an HTTP client transport.
//
// Basic usage:
//
//	func TestMyFeature(t *testing.T) {
//	    reg := testing.NewRegistry(t)
//
//	    reg.Register(
//	        testing.MatchPath("/projects.json"),
//	        testing.RespondJSON([]Project{{ID: 1, Name: "Test"}}),
//	    )
//
//	    client := basecamp.NewClient(cfg, token,
//	        basecamp.WithTransport(reg))
//
//	    // ... use client ...
//
//	    reg.Verify(t) // Fails if stubs weren't matched
//	}
package testing

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
)

// Matcher is a function that determines if an HTTP request matches a stub.
type Matcher func(req *http.Request) bool

// Responder is a function that generates an HTTP response for a matched request.
type Responder func(req *http.Request) (*http.Response, error)

// Stub represents a registered request/response pair.
type Stub struct {
	// Stack is the call stack where this stub was registered (for debugging)
	Stack string
	// Matcher determines if a request matches this stub
	Matcher Matcher
	// Responder generates the response for matched requests
	Responder Responder
	// matched tracks whether this stub was used
	matched bool
	// exclude marks this stub as an exclusion (should fail if matched)
	exclude bool
}

// Registry is a mock HTTP server that records requests and returns stubbed responses.
// It implements http.RoundTripper for use as an HTTP client transport.
type Registry struct {
	mu sync.Mutex
	t  *testing.T

	stubs []*Stub
	// Requests records all requests that were made to the registry
	Requests []*http.Request
}

// NewRegistry creates a new Registry for testing.
func NewRegistry(t *testing.T) *Registry {
	return &Registry{t: t}
}

// Register adds a stub to the registry.
// When a request matches the matcher, the responder will be called to generate the response.
func (r *Registry) Register(m Matcher, resp Responder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stubs = append(r.stubs, &Stub{
		Stack:     string(debug.Stack()),
		Matcher:   m,
		Responder: resp,
	})
}

// Exclude registers a matcher that will fail the test if matched.
// Use this to ensure certain requests are NOT made during a test.
// Panics if the Registry was created without a *testing.T (NewRegistry(nil)).
func (r *Registry) Exclude(m Matcher) {
	if r.t == nil {
		panic("Exclude requires a non-nil *testing.T; use NewRegistry(t) instead of NewRegistry(nil)")
	}

	registrationStack := string(debug.Stack())

	r.mu.Lock()
	defer r.mu.Unlock()

	excludedStub := &Stub{
		Matcher: m,
		Responder: func(req *http.Request) (*http.Response, error) {
			callStack := string(debug.Stack())

			var errMsg strings.Builder
			errMsg.WriteString("HTTP call was made when it should have been excluded:\n")
			errMsg.WriteString(fmt.Sprintf("Request URL: %s\n", req.URL))
			errMsg.WriteString(fmt.Sprintf("Was excluded by: %s\n", registrationStack))
			errMsg.WriteString(fmt.Sprintf("Was called from: %s\n", callStack))

			r.t.Error(errMsg.String())
			r.t.FailNow()
			return nil, nil
		},
		exclude: true,
	}
	r.stubs = append(r.stubs, excludedStub)
}

// Testing is the interface for test assertion methods.
type Testing interface {
	Errorf(string, ...interface{})
	Helper()
}

// Verify checks that all registered stubs were matched.
// Call this at the end of a test to ensure all expected requests were made.
func (r *Registry) Verify(t Testing) {
	t.Helper()

	r.mu.Lock()
	defer r.mu.Unlock()

	var unmatchedStubStacks []string
	for _, s := range r.stubs {
		if !s.matched && !s.exclude {
			unmatchedStubStacks = append(unmatchedStubStacks, s.Stack)
		}
	}
	if len(unmatchedStubStacks) > 0 {
		stacks := strings.Builder{}
		for i, stack := range unmatchedStubStacks {
			stacks.WriteString(fmt.Sprintf("Stub %d:\n", i+1))
			stacks.WriteString(fmt.Sprintf("\t%s", stack))
			if stack != unmatchedStubStacks[len(unmatchedStubStacks)-1] {
				stacks.WriteString("\n")
			}
		}
		t.Errorf("%d HTTP stubs unmatched, stacks:\n%s", len(unmatchedStubStacks), stacks.String())
	}
}

// RoundTrip implements http.RoundTripper.
// It finds the first matching stub and returns its response.
func (r *Registry) RoundTrip(req *http.Request) (*http.Response, error) {
	var stub *Stub

	r.mu.Lock()
	for _, s := range r.stubs {
		if s.matched || !s.Matcher(req) {
			continue
		}
		stub = s
		break
	}

	if stub != nil {
		stub.matched = true
	}

	if stub == nil {
		r.mu.Unlock()
		return nil, fmt.Errorf("no registered HTTP stubs matched %s %s", req.Method, req.URL)
	}

	r.Requests = append(r.Requests, req)
	r.mu.Unlock()

	return stub.Responder(req)
}

// Client returns an *http.Client that uses this registry as its transport.
func (r *Registry) Client() *http.Client {
	return &http.Client{
		Transport: r,
	}
}

// Reset clears all stubs and recorded requests.
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stubs = nil
	r.Requests = nil
}

// RequestCount returns the number of requests that have been made.
func (r *Registry) RequestCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Requests)
}

// LastRequest returns the most recent request, or nil if none were made.
func (r *Registry) LastRequest() *http.Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.Requests) == 0 {
		return nil
	}
	return r.Requests[len(r.Requests)-1]
}
