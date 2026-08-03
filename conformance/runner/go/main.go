// Package main provides a conformance test runner for the Go SDK.
//
// This runner reads JSON test definitions from conformance/tests/ and
// executes them against the SDK using a mock HTTP server.
//
// Unlike earlier iterations, this runner uses the real basecamp.Client
// (not the generated client) so that error mapping, retry, pagination,
// and HTTPS enforcement are exercised through the actual SDK code paths.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// TestCase represents a single conformance test.
type TestCase struct {
	// Mode is "mock" (default) or "live". Live tests are owned by the TS
	// runner; non-TS runners filter them out at load time so unresolved
	// fixture placeholders and unknown operations don't false-pass as
	// mock conformance.
	Mode            string                 `json:"mode"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Operation       string                 `json:"operation"`
	Method          string                 `json:"method"`
	Path            string                 `json:"path"`
	PathParams      map[string]interface{} `json:"pathParams"`
	QueryParams     map[string]interface{} `json:"queryParams"`
	RequestBody     map[string]interface{} `json:"requestBody"`
	MockResponses   []MockResponse         `json:"mockResponses"`
	Assertions      []Assertion            `json:"assertions"`
	Tags            []string               `json:"tags"`
	ConfigOverrides *ConfigOverrides       `json:"configOverrides"`
}

// ConfigOverrides allows per-test client configuration (e.g., non-localhost baseUrl).
type ConfigOverrides struct {
	BaseURL  string `json:"baseUrl"`
	MaxPages int    `json:"maxPages"`
	MaxItems int    `json:"maxItems"`
	// Page pins the list operation to a single page (SPEC §8).
	Page int `json:"page"`
}

// MockResponse defines a single mock HTTP response.
type MockResponse struct {
	Status       int               `json:"status"`
	NetworkError bool              `json:"networkError"`
	Headers      map[string]string `json:"headers"`
	Body         interface{}       `json:"body"`
	Delay        int               `json:"delay"`
}

// Assertion defines what to verify after the test.
type Assertion struct {
	Type     string      `json:"type"`
	Expected interface{} `json:"expected"`
	Min      float64     `json:"min"`
	Max      float64     `json:"max"`
	Path     string      `json:"path"`
	// Index selects which captured request to inspect for header assertions.
	// Defaults to 0 (first request); negative values index from the end.
	Index *int `json:"index,omitempty"`
}

// resolveIndex normalizes a request index against n captured requests.
// Negative indexes count from the end (-1 = last). Returns (0, false) when
// the index is out of range.
func resolveIndex(index, n int) (int, bool) {
	if n == 0 {
		return 0, false
	}
	if index < 0 {
		index += n
	}
	if index < 0 || index >= n {
		return 0, false
	}
	return index, true
}

// requestHeadersAt returns the headers captured for the given request index.
func requestHeadersAt(requestHeaders []http.Header, index int) (http.Header, bool) {
	i, ok := resolveIndex(index, len(requestHeaders))
	if !ok {
		return nil, false
	}
	return requestHeaders[i], true
}

// assertionIndex returns the Index value, defaulting to 0 if unset.
func assertionIndex(a Assertion) int {
	if a.Index == nil {
		return 0
	}
	return *a.Index
}

// TestResult captures the outcome of a test case.
type TestResult struct {
	Name    string
	Passed  bool
	Message string
}

func main() {
	// Wire-replay mode gate: when WIRE_REPLAY_DIR is set, dispatch to
	// the replay runner (replay_runner.go) and exit. The replay runner
	// consumes wire snapshots written by the canonical TS live runner;
	// see conformance/runner/typescript/live-runner.test.ts. Mock mode
	// (the rest of this function) runs only when the gate is unset.
	if dir := os.Getenv("WIRE_REPLAY_DIR"); dir != "" {
		backend := os.Getenv("BASECAMP_BACKEND")
		if backend == "" {
			fmt.Fprintln(os.Stderr, "BASECAMP_BACKEND is required when WIRE_REPLAY_DIR is set")
			os.Exit(1)
		}
		// Match the existing relative-path convention in this file: the
		// runner is invoked with cwd = conformance/runner/go.
		fixturePath := filepath.Join("..", "..", "tests", "live-my-surface.json")
		openapiPath := filepath.Join("..", "..", "..", "openapi.json")
		runner, err := NewReplayRunner(dir, backend, fixturePath, openapiPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(runner.Run())
	}

	testsDir := filepath.Join("..", "..", "tests")

	files, err := filepath.Glob(filepath.Join(testsDir, "*.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding test files: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No test files found in", testsDir)
		os.Exit(0)
	}

	var results []TestResult
	passed, failed, skipped := 0, 0, 0

	for _, file := range files {
		tests, err := loadTests(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", file, err)
			continue
		}

		fmt.Printf("\n=== %s ===\n", filepath.Base(file))

		for _, tc := range tests {
			if reason, ok := goSDKSkips[tc.Name]; ok {
				skipped++
				fmt.Printf("  SKIP: %s (%s)\n", tc.Name, reason)
				continue
			}

			result := runTest(tc)
			results = append(results, result)

			if result.Passed {
				passed++
				fmt.Printf("  PASS: %s\n", tc.Name)
			} else {
				failed++
				sanitized := strings.ReplaceAll(strings.ReplaceAll(result.Message, "\n", " "), "\r", "")
				fmt.Printf("  FAIL: %s\n        %s\n", tc.Name, sanitized)
			}
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Passed: %d, Failed: %d, Skipped: %d, Total: %d\n", passed, failed, skipped, passed+failed+skipped)

	if failed > 0 {
		os.Exit(1)
	}
}

// Tests where the Go SDK's behavior intentionally differs.
var goSDKSkips = map[string]string{
	"Mixed-case host and explicit default port stay on the mocked origin": "Go runner dials configOverrides.baseUrl directly (its httptest mock has its own origin); origin-interception normalization applies to the respx/WebMock/MSW/MockEngine runners",
	"Bracketed IPv6 loopback origin stays on the mocked origin":           "Go runner dials configOverrides.baseUrl directly (its httptest mock has its own origin); origin-interception normalization applies to the respx/WebMock/MSW/MockEngine runners",
}

func loadTests(filename string) ([]TestCase, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var tests []TestCase
	dec := json.NewDecoder(f)
	dec.UseNumber() // Preserve large integer precision in Expected values
	if err := dec.Decode(&tests); err != nil {
		return nil, err
	}

	// Live tests are TS-only — filter them out so this runner doesn't
	// attempt mock dispatch on entries with unresolved ${PROJECT_ID}
	// fixtures or operations that only the live runner knows about.
	mockTests := tests[:0]
	for _, tc := range tests {
		if tc.Mode == "" || tc.Mode == "mock" {
			mockTests = append(mockTests, tc)
		}
	}
	return mockTests, nil
}

// Default account ID for conformance tests
const testAccountID = "999"

// operationResult holds the outcome of an SDK operation call.
type operationResult struct {
	err    error
	meta   map[string]interface{} // SDK-parsed metadata (e.g., "totalCount")
	result interface{}            // Deserialized SDK response for responseBody assertions
}

func runTest(tc TestCase) TestResult {
	// Defense-in-depth backstop for the operationally-harmful mockResponses
	// shapes: neither status nor networkError set (would be served as an HTTP
	// response), or both active. The AUTHORITATIVE oneOf enforcement — including
	// rejecting {status, networkError:false} and networkError values other than
	// true — is `make conformance-fixtures-check` (check-jsonschema against
	// conformance/schema.json), which runs before the runners. This truthiness
	// check can't distinguish an absent networkError from a present false one,
	// so it deliberately covers only the harmful cases, not the full schema.
	for i, mr := range tc.MockResponses {
		hasStatus := mr.Status != 0
		if hasStatus == mr.NetworkError {
			return TestResult{
				Name:    tc.Name,
				Passed:  false,
				Message: fmt.Sprintf("mockResponses[%d] must set exactly one of status or networkError (got status=%d, networkError=%t)", i, mr.Status, mr.NetworkError),
			}
		}
	}

	// Track request count and timing with mutex protection
	var mu sync.Mutex
	var requestCount int
	var requestTimes []time.Time
	var requestPaths []string
	var requestMethods []string
	var requestBodies []map[string]interface{}
	var requestHeaders []http.Header

	// Detect if test uses Link next headers (SDK will auto-paginate)
	autoPaginates := false
	for _, mr := range tc.MockResponses {
		if link, ok := mr.Headers["Link"]; ok && strings.Contains(link, `rel="next"`) {
			autoPaginates = true
			break
		}
	}

	// Create mock server that serves responses in sequence
	responseIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the request body (decoded as JSON when possible) so
		// requestBody / requestBodyAbsent assertions can inspect it.
		var body map[string]interface{}
		if r.Body != nil {
			if raw, err := io.ReadAll(r.Body); err == nil && len(raw) > 0 {
				dec := json.NewDecoder(bytes.NewReader(raw))
				dec.UseNumber()
				_ = dec.Decode(&body)
			}
		}

		mu.Lock()
		requestCount++
		requestTimes = append(requestTimes, time.Now())
		requestPaths = append(requestPaths, r.URL.Path)
		requestMethods = append(requestMethods, r.Method)
		requestBodies = append(requestBodies, body)
		requestHeaders = append(requestHeaders, r.Header.Clone())
		idx := responseIndex
		responseIndex++
		mu.Unlock()

		if idx >= len(tc.MockResponses) {
			w.Header().Set("Content-Type", "application/json")
			if autoPaginates {
				// Beyond defined responses for paginated ops: empty 200 terminates pagination
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[]`))
			} else {
				// Non-paginated overflow: 500 so retry exhaustion surfaces the error
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "No more mock responses"}`))
			}
			return
		}

		resp := tc.MockResponses[idx]

		// Apply delay if specified
		if resp.Delay > 0 {
			time.Sleep(time.Duration(resp.Delay) * time.Millisecond)
		}

		// Genuine transport failure: hijack the connection and close it without
		// writing any response, forcing a real socket reset the SDK observes as
		// a network error. The request counter is already incremented above.
		if resp.NetworkError {
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					_ = conn.Close()
					return
				}
			}
			// Fallback: no hijack support — drop the connection via a panic the
			// httptest server converts into a reset (should not happen on the
			// default server, which supports hijacking).
			panic(http.ErrAbortHandler)
		}

		// Set Content-Type before any other headers (oapi-codegen
		// WithResponse parsing requires it for JSON body detection).
		w.Header().Set("Content-Type", "application/json")

		// Set response headers (may override Content-Type)
		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}

		w.WriteHeader(resp.Status)

		if resp.Body != nil {
			// If body is an object with a single array property (e.g.,
			// {"projects": [...]}), unwrap to just the array. The Go SDK's
			// generated client expects raw arrays for list endpoints.
			//
			// Success bodies only: an error body with one array-valued key is
			// the unwrapped field map ({"payload_url": ["is invalid"]}), and
			// unwrapping it would rewrite the fixture on the wire.
			bodyToWrite := resp.Body
			if obj, ok := bodyToWrite.(map[string]interface{}); ok && len(obj) == 1 && resp.Status < 400 {
				for _, v := range obj {
					if _, isArr := v.([]interface{}); isArr {
						bodyToWrite = v
					}
				}
			}
			bodyBytes, _ := json.Marshal(bodyToWrite)
			w.Write(bodyBytes)
		}
	}))
	defer server.Close()

	// Determine base URL: use configOverrides if present, else mock server
	baseURL := server.URL
	if tc.ConfigOverrides != nil && tc.ConfigOverrides.BaseURL != "" {
		baseURL = tc.ConfigOverrides.BaseURL
	}

	// Create SDK client using real basecamp.Client.
	// The SDK validates HTTPS at construction time for non-localhost URLs.
	// Catch panics from HTTPS enforcement to convert to *basecamp.Error.
	// Only intercept known validation panics; re-panic on unexpected ones.
	var opResult operationResult
	func() {
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprintf("%v", r)
				if strings.HasPrefix(msg, "basecamp: base URL must use HTTPS") ||
					strings.HasPrefix(msg, "basecamp: timeout must be positive") ||
					strings.HasPrefix(msg, "basecamp: max retries must be at least 1") ||
					strings.HasPrefix(msg, "basecamp: max pages must be positive") {
					opResult.err = basecamp.ErrUsage(msg)
				} else {
					panic(r)
				}
			}
		}()

		cfg := &basecamp.Config{BaseURL: baseURL}
		tp := &basecamp.StaticTokenProvider{Token: "conformance-test-token"}
		opts := []basecamp.ClientOption{
			basecamp.WithMaxRetries(3),
			basecamp.WithTimeout(10 * time.Second),
		}
		if tc.ConfigOverrides != nil && tc.ConfigOverrides.MaxPages > 0 {
			opts = append(opts, basecamp.WithMaxPages(tc.ConfigOverrides.MaxPages))
		}
		client := basecamp.NewClient(cfg, tp, opts...)
		account := client.ForAccount(testAccountID)

		opResult = executeOperation(context.Background(), account, tc)
	}()

	// Implicit method invariant: the mock server answers any verb, so a
	// wrong-verb request (e.g. a PUT regressing to POST) would consume a
	// queued response silently. When the fixture declares a method and
	// carries no explicit requestMethod assertions, the first request must
	// use the fixture method.
	hasMethodAssertion := false
	for _, a := range tc.Assertions {
		if a.Type == "requestMethod" {
			hasMethodAssertion = true
		}
	}
	if tc.Method != "" && !hasMethodAssertion && len(requestMethods) > 0 &&
		!strings.EqualFold(requestMethods[0], tc.Method) {
		return *fail(tc, fmt.Sprintf("Expected first request method %q, got %q", strings.ToUpper(tc.Method), requestMethods[0]))
	}

	// Run assertions
	for _, assertion := range tc.Assertions {
		if result := checkAssertion(tc, assertion, opResult, requestCount, requestTimes, requestPaths, requestMethods, requestBodies, requestHeaders); result != nil {
			return *result
		}
	}

	return TestResult{
		Name:    tc.Name,
		Passed:  true,
		Message: "All assertions passed",
	}
}

// executeOperation dispatches to the appropriate SDK service method.
// Returns the operation result with error and optional metadata.
func executeOperation(ctx context.Context, account *basecamp.AccountClient, tc TestCase) operationResult {
	switch tc.Operation {
	case "ListProjects":
		var opts *basecamp.ProjectListOptions
		if tc.ConfigOverrides != nil && (tc.ConfigOverrides.MaxItems > 0 || tc.ConfigOverrides.Page > 0) {
			opts = &basecamp.ProjectListOptions{
				Limit: tc.ConfigOverrides.MaxItems,
				Page:  tc.ConfigOverrides.Page,
			}
		}
		result, err := account.Projects().List(ctx, opts)
		if err != nil {
			return operationResult{err: err}
		}
		return operationResult{
			meta: map[string]interface{}{
				"totalCount": result.Meta.TotalCount,
				"truncated":  result.Meta.Truncated,
			},
		}

	case "GetProject":
		projectID := getInt64Param(tc.PathParams, "projectId")
		project, err := account.Projects().Get(ctx, projectID)
		return operationResult{err: err, result: project}

	case "CreateProject":
		name := getStringParam(tc.RequestBody, "name")
		if name == "" {
			name = "Conformance Test"
		}
		_, err := account.Projects().Create(ctx, &basecamp.CreateProjectRequest{Name: name})
		return operationResult{err: err}

	case "UpdateProject":
		projectID := getInt64Param(tc.PathParams, "projectId")
		name := getStringParam(tc.RequestBody, "name")
		if name == "" {
			name = "Conformance Test"
		}
		_, err := account.Projects().Update(ctx, projectID, &basecamp.UpdateProjectRequest{Name: name})
		return operationResult{err: err}

	case "TrashProject":
		projectID := getInt64Param(tc.PathParams, "projectId")
		err := account.Projects().Trash(ctx, projectID)
		return operationResult{err: err}

	case "ListTodos":
		todolistID := getInt64Param(tc.PathParams, "todolistId")
		var todoOpts *basecamp.TodoListOptions
		if tc.ConfigOverrides != nil && tc.ConfigOverrides.MaxItems > 0 {
			todoOpts = &basecamp.TodoListOptions{Limit: tc.ConfigOverrides.MaxItems}
		}
		result, err := account.Todos().List(ctx, todolistID, todoOpts)
		if err != nil {
			return operationResult{err: err}
		}
		return operationResult{
			meta: map[string]interface{}{
				"totalCount": result.Meta.TotalCount,
				"truncated":  result.Meta.Truncated,
			},
		}

	case "GetTodo":
		todoID := getInt64Param(tc.PathParams, "todoId")
		_, err := account.Todos().Get(ctx, todoID)
		return operationResult{err: err}

	case "CreateTodo":
		todolistID := getInt64Param(tc.PathParams, "todolistId")
		content := getStringParam(tc.RequestBody, "content")
		if content == "" {
			content = "Conformance Test"
		}
		_, err := account.Todos().Create(ctx, todolistID, &basecamp.CreateTodoRequest{Content: content})
		return operationResult{err: err}

	case "CreateTodosetTodo":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		todosetID := getInt64Param(tc.PathParams, "todosetId")
		content := getStringParam(tc.RequestBody, "content")
		if content == "" {
			content = "Conformance Test"
		}
		_, err := account.Todos().CreateInTodoset(ctx, bucketID, todosetID, &basecamp.CreateTodoRequest{Content: content})
		return operationResult{err: err}

	case "CompleteTodo":
		todoID := getInt64Param(tc.PathParams, "todoId")
		err := account.Todos().Complete(ctx, todoID)
		return operationResult{err: err}

	case "Subscribe":
		recordingID := getInt64Param(tc.PathParams, "recordingId")
		_, err := account.Subscriptions().Subscribe(ctx, recordingID)
		return operationResult{err: err}

	case "ListMyBookmarks":
		_, err := account.Bookmarks().List(ctx, 0)
		return operationResult{err: err}

	case "ListMyDrafts":
		_, err := account.Drafts().List(ctx, 0)
		return operationResult{err: err}

	case "GetMyNote":
		_, err := account.MyNotes().Get(ctx)
		return operationResult{err: err}

	case "PrioritizeAssignment":
		id := getInt64Param(tc.RequestBody, "id")
		err := account.MyAssignments().Prioritize(ctx, id)
		return operationResult{err: err}

	case "DeprioritizeAssignment":
		recordingID := getInt64Param(tc.PathParams, "recordingId")
		err := account.MyAssignments().Deprioritize(ctx, recordingID)
		return operationResult{err: err}

	case "ReorderUpNext":
		sourceID := getInt64Param(tc.RequestBody, "source_id")
		position := getInt64Param(tc.RequestBody, "position")
		err := account.MyAssignments().Reorder(ctx, sourceID, int32(position))
		return operationResult{err: err}

	case "GetCalendar":
		calendarID := getInt64Param(tc.PathParams, "calendarId")
		_, err := account.Calendars().Get(ctx, calendarID)
		return operationResult{err: err}

	case "UpdateCalendar":
		calendarID := getInt64Param(tc.PathParams, "calendarId")
		color := ""
		if cal, ok := tc.RequestBody["calendar"].(map[string]interface{}); ok {
			if c, ok := cal["color"].(string); ok {
				color = c
			}
		}
		_, err := account.Calendars().Update(ctx, calendarID, color)
		return operationResult{err: err}

	case "UpdateMyNote":
		content := ""
		if note, ok := tc.RequestBody["note"].(map[string]interface{}); ok {
			if c, ok := note["content"].(string); ok {
				content = c
			}
		}
		_, err := account.MyNotes().Update(ctx, content)
		return operationResult{err: err}

	case "GetBookmark":
		recordingID := getInt64Param(tc.PathParams, "recordingId")
		_, err := account.Bookmarks().Get(ctx, recordingID)
		return operationResult{err: err}

	case "CreateBookmark":
		recordingID := getInt64Param(tc.PathParams, "recordingId")
		_, err := account.Bookmarks().Create(ctx, recordingID)
		return operationResult{err: err}

	case "DeleteBookmark":
		recordingID := getInt64Param(tc.PathParams, "recordingId")
		err := account.Bookmarks().Delete(ctx, recordingID)
		return operationResult{err: err}

	case "ListFolders":
		_, err := account.Folders().List(ctx)
		return operationResult{err: err}

	case "GetFolder":
		folderID := getInt64Param(tc.PathParams, "folderId")
		_, err := account.Folders().Get(ctx, folderID)
		return operationResult{err: err}

	case "CreateFolder":
		req := basecamp.CreateFolderRequest{Name: getStringParam(tc.RequestBody, "name")}
		if ids, ok := tc.RequestBody["project_ids"].([]any); ok {
			req.ProjectIDs = make([]int64, 0, len(ids))
			for _, id := range ids {
				if n, ok := id.(float64); ok {
					req.ProjectIDs = append(req.ProjectIDs, int64(n))
				}
			}
		}
		_, err := account.Folders().Create(ctx, req)
		return operationResult{err: err}

	case "UpdateFolder":
		folderID := getInt64Param(tc.PathParams, "folderId")
		_, err := account.Folders().Update(ctx, folderID, getStringParam(tc.RequestBody, "name"))
		return operationResult{err: err}

	case "DeleteFolder":
		folderID := getInt64Param(tc.PathParams, "folderId")
		err := account.Folders().Delete(ctx, folderID)
		return operationResult{err: err}

	case "UpdateTodo":
		todoID := getInt64Param(tc.PathParams, "todoId")
		req := &basecamp.UpdateTodoRequest{
			Content:     getStringParam(tc.RequestBody, "content"),
			Description: getStringParam(tc.RequestBody, "description"),
			DueOn:       getStringParam(tc.RequestBody, "due_on"),
			StartsOn:    getStringParam(tc.RequestBody, "starts_on"),
			Notify:      getBoolParam(tc.RequestBody, "notify"),
		}
		if ids, ok := getInt64SliceParam(tc.RequestBody, "assignee_ids"); ok {
			req.AssigneeIDs = ids
		}
		if ids, ok := getInt64SliceParam(tc.RequestBody, "completion_subscriber_ids"); ok {
			req.CompletionSubscriberIDs = ids
		}
		_, err := account.Todos().Update(ctx, todoID, req)
		return operationResult{err: err}

	case "UpdateScheduleEntry":
		// Participants are presence-bearing: absent means "leave them alone"
		// (BC3 preserves only because the key is missing), an empty non-nil
		// slice means "remove everyone".
		entryID := getInt64Param(tc.PathParams, "entryId")
		req := &basecamp.UpdateScheduleEntryRequest{
			Summary:  getStringParam(tc.RequestBody, "summary"),
			StartsAt: getStringParam(tc.RequestBody, "starts_at"),
			EndsAt:   getStringParam(tc.RequestBody, "ends_at"),
		}
		if ids, ok := getInt64SliceParam(tc.RequestBody, "participant_ids"); ok {
			req.ParticipantIDs = ids
		}
		_, err := account.Schedules().UpdateEntry(ctx, entryID, req)
		return operationResult{err: err}

	case "UpdateCard":
		// Merge-safe composite: GET then PUT, resending the fetched due_on.
		_, err := account.Cards().Update(ctx, getInt64Param(tc.PathParams, "cardId"), cardUpdateRequest(tc.RequestBody))
		return operationResult{err: err}

	case "UpdateCardVerbatim":
		// Raw single PUT, no read-before-write.
		_, err := account.Cards().UpdateVerbatim(ctx, getInt64Param(tc.PathParams, "cardId"), cardUpdateRequest(tc.RequestBody))
		return operationResult{err: err}

	case "EditTodo":
		// Synthetic scenario key (not a wire operation): drives the SDK's
		// edit closure, assigning each fixture requestBody key onto the
		// corresponding TodoFields member (data-driven mutation).
		todoID := getInt64Param(tc.PathParams, "todoId")
		_, err := account.Todos().Edit(ctx, todoID, func(f *basecamp.TodoFields) error {
			if _, ok := tc.RequestBody["content"]; ok {
				f.Content = getStringParam(tc.RequestBody, "content")
			}
			if _, ok := tc.RequestBody["description"]; ok {
				f.Description = getStringParam(tc.RequestBody, "description")
			}
			if ids, ok := getInt64SliceParam(tc.RequestBody, "assignee_ids"); ok {
				f.AssigneeIDs = ids
			}
			if ids, ok := getInt64SliceParam(tc.RequestBody, "completion_subscriber_ids"); ok {
				f.CompletionSubscriberIDs = ids
			}
			if _, ok := tc.RequestBody["due_on"]; ok {
				f.DueOn = getStringParam(tc.RequestBody, "due_on")
			}
			if _, ok := tc.RequestBody["starts_on"]; ok {
				f.StartsOn = getStringParam(tc.RequestBody, "starts_on")
			}
			if _, ok := tc.RequestBody["notify"]; ok {
				f.Notify = getBoolParam(tc.RequestBody, "notify")
			}
			return nil
		})
		return operationResult{err: err}

	case "ReplaceTodo":
		todoID := getInt64Param(tc.PathParams, "todoId")
		req := &basecamp.ReplaceTodoRequest{
			Content:     getStringParam(tc.RequestBody, "content"),
			Description: getStringParam(tc.RequestBody, "description"),
			DueOn:       getStringParam(tc.RequestBody, "due_on"),
			StartsOn:    getStringParam(tc.RequestBody, "starts_on"),
			Notify:      getBoolParam(tc.RequestBody, "notify"),
		}
		if ids, ok := getInt64SliceParam(tc.RequestBody, "assignee_ids"); ok {
			req.AssigneeIDs = ids
		}
		if ids, ok := getInt64SliceParam(tc.RequestBody, "completion_subscriber_ids"); ok {
			req.CompletionSubscriberIDs = ids
		}
		_, err := account.Todos().Replace(ctx, todoID, req)
		return operationResult{err: err}

	case "UpdateTodolist":
		// Synthetic scenario key (not a wire operation): drives the SDK's
		// merge-safe composite, which GETs the current todolist, overlays only
		// the explicitly-set fields, and PUTs the full representation back.
		// Variant-agnostic — a todolist group decodes into the same shape, so
		// the group fixture runs through this very case with no branching.
		todolistID := getInt64Param(tc.PathParams, "id")
		req := &basecamp.UpdateTodolistRequest{
			Name:        getStringParam(tc.RequestBody, "name"),
			Description: getStringParam(tc.RequestBody, "description"),
		}
		_, err := account.Todolists().Update(ctx, todolistID, req)
		return operationResult{err: err}

	case "EditTodolist":
		// Synthetic scenario key (not a wire operation): drives the SDK's
		// edit closure, assigning each fixture requestBody key onto the
		// corresponding TodolistFields member (data-driven mutation). Absence
		// stays absence, so an untouched field keeps its fetched value.
		todolistID := getInt64Param(tc.PathParams, "id")
		_, err := account.Todolists().Edit(ctx, todolistID, func(f *basecamp.TodolistFields) error {
			if _, ok := tc.RequestBody["name"]; ok {
				f.Name = getStringParam(tc.RequestBody, "name")
			}
			if _, ok := tc.RequestBody["description"]; ok {
				f.Description = getStringParam(tc.RequestBody, "description")
			}
			return nil
		})
		return operationResult{err: err}

	case "ReplaceTodolist":
		todolistID := getInt64Param(tc.PathParams, "id")
		req := &basecamp.ReplaceTodolistRequest{
			Name:        getStringParam(tc.RequestBody, "name"),
			Description: getStringParam(tc.RequestBody, "description"),
		}
		_, err := account.Todolists().Replace(ctx, todolistID, req)
		return operationResult{err: err}

	case "UpdateDocument":
		// Synthetic scenario key (not a wire operation): drives the SDK's
		// merge-safe composite, which GETs the current document, overlays only
		// the explicitly-set fields, and PUTs the full representation back.
		documentID := getInt64Param(tc.PathParams, "documentId")
		req := &basecamp.UpdateDocumentRequest{
			Title:   getStringParam(tc.RequestBody, "title"),
			Content: getStringParam(tc.RequestBody, "content"),
		}
		_, err := account.Documents().Update(ctx, documentID, req)
		return operationResult{err: err}

	case "EditDocument":
		// Synthetic scenario key (not a wire operation): drives the SDK's
		// edit closure, assigning each fixture requestBody key onto the
		// corresponding DocumentFields member (data-driven mutation). Absence
		// stays absence, so an untouched field keeps its fetched value.
		documentID := getInt64Param(tc.PathParams, "documentId")
		_, err := account.Documents().Edit(ctx, documentID, func(f *basecamp.DocumentFields) error {
			if _, ok := tc.RequestBody["title"]; ok {
				f.Title = getStringParam(tc.RequestBody, "title")
			}
			if _, ok := tc.RequestBody["content"]; ok {
				f.Content = getStringParam(tc.RequestBody, "content")
			}
			return nil
		})
		return operationResult{err: err}

	case "ReplaceDocument":
		// Presence-bearing: only keys the fixture carries become pointers, so
		// an absent field stays absent on the wire and an explicit "" is sent.
		documentID := getInt64Param(tc.PathParams, "documentId")
		req := &basecamp.ReplaceDocumentRequest{}
		if _, ok := tc.RequestBody["title"]; ok {
			title := getStringParam(tc.RequestBody, "title")
			req.Title = &title
		}
		if _, ok := tc.RequestBody["content"]; ok {
			content := getStringParam(tc.RequestBody, "content")
			req.Content = &content
		}
		_, err := account.Documents().Replace(ctx, documentID, req)
		return operationResult{err: err}

	case "GetTimesheetEntry":
		entryID := getInt64Param(tc.PathParams, "entryId")
		_, err := account.Timesheet().Get(ctx, entryID)
		return operationResult{err: err}

	case "DestroyTimesheetEntry":
		entryID := getInt64Param(tc.PathParams, "entryId")
		err := account.Timesheet().Destroy(ctx, entryID)
		return operationResult{err: err}

	case "UpdateTimesheetEntry":
		entryID := getInt64Param(tc.PathParams, "entryId")
		req := &basecamp.UpdateTimesheetEntryRequest{}
		if date := getStringParam(tc.RequestBody, "date"); date != "" {
			req.Date = date
		}
		if hours := getStringParam(tc.RequestBody, "hours"); hours != "" {
			req.Hours = hours
		}
		if desc := getStringParam(tc.RequestBody, "description"); desc != "" {
			req.Description = desc
		}
		_, err := account.Timesheet().Update(ctx, entryID, req)
		return operationResult{err: err}

	case "GetProjectTimeline":
		projectID := getInt64Param(tc.PathParams, "projectId")
		var timelineOpts *basecamp.TimelineListOptions
		if tc.ConfigOverrides != nil && tc.ConfigOverrides.MaxItems > 0 {
			timelineOpts = &basecamp.TimelineListOptions{Limit: tc.ConfigOverrides.MaxItems}
		}
		result, err := account.Timeline().ProjectTimeline(ctx, projectID, timelineOpts)
		if err != nil {
			return operationResult{err: err}
		}
		return operationResult{
			meta: map[string]interface{}{
				"totalCount": result.Meta.TotalCount,
				"truncated":  result.Meta.Truncated,
			},
		}

	case "GetProgressReport":
		var timelineOpts *basecamp.TimelineListOptions
		if tc.ConfigOverrides != nil && tc.ConfigOverrides.MaxItems > 0 {
			timelineOpts = &basecamp.TimelineListOptions{Limit: tc.ConfigOverrides.MaxItems}
		}
		result, err := account.Timeline().Progress(ctx, timelineOpts)
		if err != nil {
			return operationResult{err: err}
		}
		return operationResult{
			meta: map[string]interface{}{
				"totalCount": result.Meta.TotalCount,
				"truncated":  result.Meta.Truncated,
			},
		}

	case "GetPersonProgress":
		personID := getInt64Param(tc.PathParams, "personId")
		var timelineOpts *basecamp.TimelineListOptions
		if tc.ConfigOverrides != nil && tc.ConfigOverrides.MaxItems > 0 {
			timelineOpts = &basecamp.TimelineListOptions{Limit: tc.ConfigOverrides.MaxItems}
		}
		result, err := account.Timeline().PersonProgress(ctx, personID, timelineOpts)
		if err != nil {
			return operationResult{err: err}
		}
		return operationResult{
			meta: map[string]interface{}{
				"totalCount": result.Meta.TotalCount,
				"truncated":  result.Meta.Truncated,
			},
		}

	case "GetProjectTimesheet":
		projectID := getInt64Param(tc.PathParams, "projectId")
		_, err := account.Timesheet().ProjectReport(ctx, projectID, nil)
		return operationResult{err: err}

	case "ListWebhooks":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		_, err := account.Webhooks().List(ctx, bucketID, nil)
		return operationResult{err: err}

	case "CreateWebhook":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		payloadURL := getStringParam(tc.RequestBody, "payload_url")
		types := getStringSliceParam(tc.RequestBody, "types")
		_, err := account.Webhooks().Create(ctx, bucketID, &basecamp.CreateWebhookRequest{
			PayloadURL: payloadURL,
			Types:      types,
		})
		return operationResult{err: err}

	case "GetTool":
		toolID := getInt64Param(tc.PathParams, "toolId")
		_, err := account.Tools().Get(ctx, toolID)
		return operationResult{err: err}

	case "CreateTool":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		toolType := getStringParam(tc.RequestBody, "tool_type")
		title := getStringParam(tc.RequestBody, "title")
		var opts *basecamp.CreateToolOptions
		if title != "" {
			opts = &basecamp.CreateToolOptions{Title: title}
		}
		_, err := account.Tools().Create(ctx, bucketID, toolType, opts)
		return operationResult{err: err}

	case "EnableTool":
		toolID := getInt64Param(tc.PathParams, "toolId")
		err := account.Tools().Enable(ctx, toolID)
		return operationResult{err: err}

	case "DownloadURL":
		// Construct an absolute URL the SDK will accept. The SDK rewrites the
		// scheme+host to the configured BaseURL, so the synthetic host here
		// is never actually hit — only tc.Path matters for mock-server routing.
		rawURL := "https://storage.3.basecamp.com" + tc.Path
		result, err := account.DownloadURL(ctx, rawURL)
		if err != nil {
			return operationResult{err: err}
		}
		defer result.Body.Close()
		if _, copyErr := io.Copy(io.Discard, result.Body); copyErr != nil {
			return operationResult{err: copyErr}
		}
		return operationResult{err: nil}

	case "UploadsDownload":
		uploadID := getInt64Param(tc.PathParams, "uploadId")
		result, err := account.Uploads().Download(ctx, uploadID)
		if err != nil {
			return operationResult{err: err}
		}
		defer result.Body.Close()
		if _, copyErr := io.Copy(io.Discard, result.Body); copyErr != nil {
			return operationResult{err: copyErr}
		}
		return operationResult{err: nil}

	case "GetEverythingMessages":
		_, err := account.Everything().Messages(ctx, 0)
		return operationResult{err: err}

	case "GetEverythingComments":
		_, err := account.Everything().Comments(ctx, 0)
		return operationResult{err: err}

	case "GetEverythingCheckins":
		_, err := account.Everything().Checkins(ctx, 0)
		return operationResult{err: err}

	case "GetEverythingForwards":
		_, err := account.Everything().Forwards(ctx, 0)
		return operationResult{err: err}

	case "GetEverythingFiles":
		_, err := account.Everything().Files(ctx, 0, nil)
		return operationResult{err: err}

	case "GetEverythingOverdueTodos":
		_, err := account.Everything().OverdueTodos(ctx, nil)
		return operationResult{err: err}

	case "GetEverythingOverdueCards":
		_, err := account.Everything().OverdueCards(ctx, nil)
		return operationResult{err: err}

	case "GetEverythingOpenTodos":
		_, err := account.Everything().OpenTodos(ctx, 0, nil)
		return operationResult{err: err}

	case "GetEverythingCompletedTodos":
		_, err := account.Everything().CompletedTodos(ctx, 0, nil)
		return operationResult{err: err}

	case "GetEverythingUnassignedTodos":
		_, err := account.Everything().UnassignedTodos(ctx, 0, nil)
		return operationResult{err: err}

	case "GetEverythingNoDueDateTodos":
		_, err := account.Everything().NoDueDateTodos(ctx, 0, nil)
		return operationResult{err: err}

	case "GetEverythingOpenCards":
		_, err := account.Everything().OpenCards(ctx, 0, nil)
		return operationResult{err: err}

	case "GetEverythingCompletedCards":
		_, err := account.Everything().CompletedCards(ctx, 0, nil)
		return operationResult{err: err}

	case "GetEverythingUnassignedCards":
		_, err := account.Everything().UnassignedCards(ctx, 0, nil)
		return operationResult{err: err}

	case "GetEverythingNoDueDateCards":
		_, err := account.Everything().NoDueDateCards(ctx, 0, nil)
		return operationResult{err: err}

	case "GetEverythingNotNowCards":
		_, err := account.Everything().NotNowCards(ctx, 0, nil)
		return operationResult{err: err}

	case "ListForwards":
		inboxID := getInt64Param(tc.PathParams, "inboxId")
		result, err := account.Forwards().List(ctx, inboxID, nil)
		if err != nil {
			return operationResult{err: err}
		}
		return operationResult{
			meta: map[string]interface{}{
				"totalCount": result.Meta.TotalCount,
				"truncated":  result.Meta.Truncated,
			},
		}

	// #588: nine flat spellings bc3 only draws bucket-scoped. Each of these
	// pins the bucketId segment on the wire — the thing that was missing when
	// they 404'd.
	case "ListChatbots":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		campfireID := getInt64Param(tc.PathParams, "campfireId")
		result, err := account.Campfires().ListChatbots(ctx, bucketID, campfireID, nil)
		if err != nil {
			return operationResult{err: err}
		}
		return operationResult{
			meta: map[string]interface{}{
				"totalCount": result.Meta.TotalCount,
				"truncated":  result.Meta.Truncated,
			},
		}

	case "GetChatbot":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		campfireID := getInt64Param(tc.PathParams, "campfireId")
		chatbotID := getInt64Param(tc.PathParams, "chatbotId")
		_, err := account.Campfires().GetChatbot(ctx, bucketID, campfireID, chatbotID)
		return operationResult{err: err}

	case "CreateChatbot":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		campfireID := getInt64Param(tc.PathParams, "campfireId")
		req := &basecamp.CreateChatbotRequest{
			ServiceName: getStringParam(tc.RequestBody, "service_name"),
			CommandURL:  getStringParam(tc.RequestBody, "command_url"),
		}
		_, err := account.Campfires().CreateChatbot(ctx, bucketID, campfireID, req)
		return operationResult{err: err}

	case "UpdateChatbot":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		campfireID := getInt64Param(tc.PathParams, "campfireId")
		chatbotID := getInt64Param(tc.PathParams, "chatbotId")
		req := &basecamp.UpdateChatbotRequest{
			ServiceName: getStringParam(tc.RequestBody, "service_name"),
			CommandURL:  getStringParam(tc.RequestBody, "command_url"),
		}
		_, err := account.Campfires().UpdateChatbot(ctx, bucketID, campfireID, chatbotID, req)
		return operationResult{err: err}

	case "DeleteChatbot":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		campfireID := getInt64Param(tc.PathParams, "campfireId")
		chatbotID := getInt64Param(tc.PathParams, "chatbotId")
		err := account.Campfires().DeleteChatbot(ctx, bucketID, campfireID, chatbotID)
		return operationResult{err: err}

	case "ListClientApprovals":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		result, err := account.ClientApprovals().List(ctx, bucketID, nil)
		if err != nil {
			return operationResult{err: err}
		}
		return operationResult{
			meta: map[string]interface{}{
				"totalCount": result.Meta.TotalCount,
				"truncated":  result.Meta.Truncated,
			},
		}

	case "ListClientCorrespondences":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		result, err := account.ClientCorrespondences().List(ctx, bucketID, nil)
		if err != nil {
			return operationResult{err: err}
		}
		return operationResult{
			meta: map[string]interface{}{
				"totalCount": result.Meta.TotalCount,
				"truncated":  result.Meta.Truncated,
			},
		}

	case "ListClientReplies":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		recordingID := getInt64Param(tc.PathParams, "recordingId")
		result, err := account.ClientReplies().List(ctx, bucketID, recordingID, nil)
		if err != nil {
			return operationResult{err: err}
		}
		return operationResult{
			meta: map[string]interface{}{
				"totalCount": result.Meta.TotalCount,
				"truncated":  result.Meta.Truncated,
			},
		}

	case "GetClientReply":
		bucketID := getInt64Param(tc.PathParams, "bucketId")
		recordingID := getInt64Param(tc.PathParams, "recordingId")
		replyID := getInt64Param(tc.PathParams, "replyId")
		_, err := account.ClientReplies().Get(ctx, bucketID, recordingID, replyID)
		return operationResult{err: err}

	case "RepositionTodolistGroup":
		groupID := getInt64Param(tc.PathParams, "groupId")
		position := getInt64Param(tc.RequestBody, "position")
		err := account.TodolistGroups().Reposition(ctx, groupID, int(position))
		return operationResult{err: err}

	default:
		return operationResult{
			err: fmt.Errorf("unknown operation: %s", tc.Operation),
		}
	}
}

// checkAssertion verifies a single assertion. Returns nil if it passes,
// or a *TestResult with the failure message.
func checkAssertion(
	tc TestCase,
	assertion Assertion,
	opResult operationResult,
	requestCount int,
	requestTimes []time.Time,
	requestPaths []string,
	requestMethods []string,
	requestBodies []map[string]interface{},
	requestHeaders []http.Header,
) *TestResult {
	sdkErr := opResult.err

	switch assertion.Type {
	case "requestCount":
		// The Go SDK auto-paginates list operations, so a fixture that counts
		// first-page requests only is inapplicable — but ONLY its count is.
		// The rest of the case still runs. See requestCountApplies (#573).
		if !requestCountApplies(tc.Tags) {
			return nil
		}
		if msg := checkRequestCount(requestCount, expectedInt(assertion.Expected)); msg != "" {
			return fail(tc, msg)
		}

	case "delayBetweenRequests":
		if msg := checkDelayGaps(requestTimes, time.Duration(assertion.Min)*time.Millisecond, assertion.Index); msg != "" {
			return fail(tc, msg)
		}

	case "noError":
		if sdkErr != nil {
			return fail(tc, fmt.Sprintf("Expected no error, got: %v", sdkErr))
		}

	// The inverse of noError, and deliberately code-agnostic. See
	// errorRaisedFailure (error_raised.go) for the contract and for why the
	// branch lives there rather than inline: no committed fixture can reach
	// its failing side, so it is unit-tested instead.
	case "errorRaised":
		if msg := errorRaisedFailure(sdkErr != nil); msg != "" {
			return fail(tc, msg)
		}

	case "errorType":
		if sdkErr == nil {
			return fail(tc, fmt.Sprintf("Expected error type %v, but got no error", assertion.Expected))
		}
		expected := expectedString(assertion.Expected)
		// Classify the error. A mapped *basecamp.Error carries a canonical
		// .Code; on the network path the generated client returns the raw
		// transport error (e.g. *url.Error) rather than a *basecamp.Error, so
		// recognize that as "network". Anything else is unknown — fail rather
		// than silently accept, so this assertion actually pins the class.
		var actualType string
		var sdkError *basecamp.Error
		if errors.As(sdkErr, &sdkError) {
			actualType = sdkError.Code
		} else if isNetworkError(sdkErr) {
			actualType = basecamp.CodeNetwork
		} else {
			return fail(tc, fmt.Sprintf("Expected error type %q, but got unrecognized error: %v", expected, sdkErr))
		}
		if actualType != expected {
			return fail(tc, fmt.Sprintf("Expected error type %q, got %q (%v)", expected, actualType, sdkErr))
		}

	case "statusCode":
		expected := expectedInt(assertion.Expected)
		if sdkErr != nil {
			var sdkError *basecamp.Error
			if errors.As(sdkErr, &sdkError) {
				if sdkError.HTTPStatus != expected {
					return fail(tc, fmt.Sprintf("Expected status code %d, got %d", expected, sdkError.HTTPStatus))
				}
			} else {
				return fail(tc, fmt.Sprintf("Expected status code %d, but error is not *basecamp.Error: %v", expected, sdkErr))
			}
		} else if expected >= 400 {
			return fail(tc, fmt.Sprintf("Expected error with status %d, but operation succeeded", expected))
		}

	case "responseStatus":
		expected := expectedInt(assertion.Expected)
		if sdkErr != nil {
			var sdkError *basecamp.Error
			if errors.As(sdkErr, &sdkError) {
				if sdkError.HTTPStatus != expected {
					return fail(tc, fmt.Sprintf("Expected response status %d, got %d", expected, sdkError.HTTPStatus))
				}
			} else {
				return fail(tc, fmt.Sprintf("Expected response status %d, but error is not *basecamp.Error: %v", expected, sdkErr))
			}
		} else if expected >= 400 {
			return fail(tc, fmt.Sprintf("Expected error with status %d, but operation succeeded", expected))
		}

	case "responseBody":
		fieldPath := assertion.Path
		if opResult.result == nil {
			return fail(tc, fmt.Sprintf("Expected responseBody.%s, but no result returned", fieldPath))
		}
		// Marshal the result to JSON, then decode with UseNumber to preserve integer precision.
		data, err := json.Marshal(opResult.result)
		if err != nil {
			return fail(tc, fmt.Sprintf("Failed to marshal result for responseBody assertion: %v", err))
		}
		var resultMap map[string]interface{}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&resultMap); err != nil {
			return fail(tc, fmt.Sprintf("Failed to unmarshal result for responseBody assertion: %v", err))
		}
		actual, ok := resultMap[fieldPath]
		if !ok {
			return fail(tc, fmt.Sprintf("Expected responseBody.%s, but field not present", fieldPath))
		}
		// Compare: both expected and actual are json.Number (preserving precision).
		if result := compareValues(tc, fmt.Sprintf("responseBody.%s", fieldPath), assertion.Expected, actual); result != nil {
			return result
		}

	case "requestPath":
		expected := expectedString(assertion.Expected)
		idx := assertionIndex(assertion)
		i, ok := resolveIndex(idx, len(requestPaths))
		if !ok {
			return fail(tc, fmt.Sprintf("Expected request path %q on request index %d, but only %d requests were recorded", expected, idx, len(requestPaths)))
		}
		if requestPaths[i] != expected {
			return fail(tc, fmt.Sprintf("Expected request path %q on request index %d, got %q", expected, idx, requestPaths[i]))
		}

	case "requestMethod":
		expected := expectedString(assertion.Expected)
		idx := assertionIndex(assertion)
		i, ok := resolveIndex(idx, len(requestMethods))
		if !ok {
			return fail(tc, fmt.Sprintf("Expected request method %q on request index %d, but only %d requests were recorded", expected, idx, len(requestMethods)))
		}
		if requestMethods[i] != expected {
			return fail(tc, fmt.Sprintf("Expected request method %q on request index %d, got %q", expected, idx, requestMethods[i]))
		}

	case "requestBody":
		fieldPath := assertion.Path
		idx := assertionIndex(assertion)
		i, ok := resolveIndex(idx, len(requestBodies))
		if !ok {
			return fail(tc, fmt.Sprintf("Expected request body field %q on request index %d, but only %d requests were recorded", fieldPath, idx, len(requestBodies)))
		}
		if requestBodies[i] == nil {
			return fail(tc, fmt.Sprintf("Expected request body field %q on request index %d, but request had no JSON body", fieldPath, idx))
		}
		actual, present := digPath(requestBodies[i], fieldPath)
		if !present {
			return fail(tc, fmt.Sprintf("Expected request body field %q on request index %d, but it was absent", fieldPath, idx))
		}
		if !jsonEqual(assertion.Expected, actual) {
			return fail(tc, fmt.Sprintf("Expected request body %s = %s on request index %d, got %s", fieldPath, jsonString(assertion.Expected), idx, jsonString(actual)))
		}

	case "requestBodyAbsent":
		fieldPath := assertion.Path
		idx := assertionIndex(assertion)
		i, ok := resolveIndex(idx, len(requestBodies))
		if !ok {
			return fail(tc, fmt.Sprintf("Expected request body field %q absent on request index %d, but only %d requests were recorded", fieldPath, idx, len(requestBodies)))
		}
		if requestBodies[i] != nil {
			if actual, present := digPath(requestBodies[i], fieldPath); present {
				return fail(tc, fmt.Sprintf("Expected request body field %q absent on request index %d, got %s", fieldPath, idx, jsonString(actual)))
			}
		}

	case "errorCode":
		expected := expectedString(assertion.Expected)
		if sdkErr == nil {
			return fail(tc, fmt.Sprintf("Expected error code %q, but got no error", expected))
		}
		var sdkError *basecamp.Error
		if !errors.As(sdkErr, &sdkError) {
			return fail(tc, fmt.Sprintf("Expected error code %q, but error is not a *basecamp.Error: %v", expected, sdkErr))
		}
		if sdkError.Code != expected {
			return fail(tc, fmt.Sprintf("Expected error code %q, got %q", expected, sdkError.Code))
		}

	case "errorMessage":
		expected := expectedString(assertion.Expected)
		if sdkErr == nil {
			return fail(tc, fmt.Sprintf("Expected error message containing %q, but got no error", expected))
		}
		if !strings.Contains(sdkErr.Error(), expected) {
			return fail(tc, fmt.Sprintf("Expected error message containing %q, got %q", expected, sdkErr.Error()))
		}

	case "errorField":
		fieldPath := assertion.Path
		if sdkErr == nil {
			return fail(tc, fmt.Sprintf("Expected error field %s, but got no error", fieldPath))
		}
		var sdkError *basecamp.Error
		if !errors.As(sdkErr, &sdkError) {
			return fail(tc, fmt.Sprintf("Expected error field %s, but error is not a *basecamp.Error: %v", fieldPath, sdkErr))
		}
		var actual interface{}
		switch fieldPath {
		case "httpStatus":
			actual = sdkError.HTTPStatus
		case "retryable":
			actual = sdkError.Retryable
		case "code":
			actual = sdkError.Code
		case "message":
			actual = sdkError.Message
		case "requestId":
			actual = sdkError.RequestID
		default:
			return fail(tc, fmt.Sprintf("Unknown error field: %s", fieldPath))
		}
		if result := compareValues(tc, fmt.Sprintf("error.%s", fieldPath), assertion.Expected, actual); result != nil {
			return result
		}

	case "headerInjected":
		headerName := assertion.Path
		expected := expectedString(assertion.Expected)
		idx := assertionIndex(assertion)
		headers, ok := requestHeadersAt(requestHeaders, idx)
		if !ok {
			return fail(tc, fmt.Sprintf("Expected header %s=%q on request index %d, but only %d requests were recorded", headerName, expected, idx, len(requestHeaders)))
		}
		actual := headers.Get(headerName)
		if actual != expected {
			return fail(tc, fmt.Sprintf("Expected header %s=%q on request index %d, got %q", headerName, expected, idx, actual))
		}

	case "headerPresent":
		headerName := assertion.Path
		idx := assertionIndex(assertion)
		headers, ok := requestHeadersAt(requestHeaders, idx)
		if !ok {
			return fail(tc, fmt.Sprintf("Expected header %s on request index %d, but only %d requests were recorded", headerName, idx, len(requestHeaders)))
		}
		if headers.Get(headerName) == "" {
			return fail(tc, fmt.Sprintf("Expected header %s on request index %d, but it was empty or missing", headerName, idx))
		}

	case "headerAbsent":
		headerName := assertion.Path
		idx := assertionIndex(assertion)
		headers, ok := requestHeadersAt(requestHeaders, idx)
		if !ok {
			return fail(tc, fmt.Sprintf("Expected header %s absent on request index %d, but only %d requests were recorded", headerName, idx, len(requestHeaders)))
		}
		// Use Values (not Get): Get returns "" for both "not present" and
		// "present with empty value"; for an absence assertion, a present-
		// but-empty header must fail.
		if values := headers.Values(headerName); len(values) > 0 {
			return fail(tc, fmt.Sprintf("Expected header %s absent on request index %d, got %q", headerName, idx, values))
		}

	case "headerValue":
		// Verify mock response config contains the expected header.
		// Note: this checks the test fixture, not SDK-observed output.
		// Use responseMeta for SDK-parsed values.
		headerName := assertion.Path
		expected := expectedString(assertion.Expected)
		if len(tc.MockResponses) == 0 {
			return fail(tc, fmt.Sprintf("Expected response header %s=%q, but no mock responses defined", headerName, expected))
		}
		actual := tc.MockResponses[0].Headers[headerName]
		if actual != expected {
			return fail(tc, fmt.Sprintf("Expected response header %s=%q, got %q", headerName, expected, actual))
		}

	case "responseMeta":
		// Verify SDK-parsed metadata (e.g., totalCount from X-Total-Count header).
		fieldPath := assertion.Path
		if opResult.meta == nil {
			return fail(tc, fmt.Sprintf("Expected response meta %s, but no metadata returned", fieldPath))
		}
		actual, ok := opResult.meta[fieldPath]
		if !ok {
			return fail(tc, fmt.Sprintf("Expected response meta %s, but field not present in metadata", fieldPath))
		}
		if result := compareValues(tc, fmt.Sprintf("meta.%s", fieldPath), assertion.Expected, actual); result != nil {
			return result
		}

	case "requestScheme":
		expected := expectedString(assertion.Expected)
		if expected == "https" && sdkErr == nil {
			return fail(tc, "Expected HTTPS enforcement error, but request succeeded over HTTP")
		}

	case "urlOrigin":
		expected := expectedString(assertion.Expected)
		if expected == "rejected" && requestCount > 1 {
			return fail(tc, fmt.Sprintf("Expected cross-origin URL rejection (1 request), but %d requests were made", requestCount))
		}

	default:
		return fail(tc, fmt.Sprintf("Unknown assertion type: %s", assertion.Type))
	}

	return nil
}

// compareValues compares an expected JSON value against an actual Go value.
// Handles json.Number (from UseNumber), float64, bool, and string.
func compareValues(tc TestCase, label string, expected, actual interface{}) *TestResult {
	switch exp := expected.(type) {
	case json.Number:
		// Compare as int64 first (preserves large integer precision), then float64.
		if expInt, err := exp.Int64(); err == nil {
			switch act := actual.(type) {
			case json.Number:
				if actInt, err := act.Int64(); err == nil {
					if actInt != expInt {
						return fail(tc, fmt.Sprintf("Expected %s = %d, got %d", label, expInt, actInt))
					}
					return nil
				}
			case int:
				if int64(act) != expInt {
					return fail(tc, fmt.Sprintf("Expected %s = %d, got %d", label, expInt, act))
				}
				return nil
			case int64:
				if act != expInt {
					return fail(tc, fmt.Sprintf("Expected %s = %d, got %d", label, expInt, act))
				}
				return nil
			}
		}
		if expFloat, err := exp.Float64(); err == nil {
			switch act := actual.(type) {
			case json.Number:
				if actFloat, err := act.Float64(); err == nil {
					if actFloat != expFloat {
						return fail(tc, fmt.Sprintf("Expected %s = %v, got %v", label, expFloat, actFloat))
					}
					return nil
				}
			case float64:
				if act != expFloat {
					return fail(tc, fmt.Sprintf("Expected %s = %v, got %v", label, expFloat, act))
				}
				return nil
			}
		}
		if fmt.Sprintf("%v", actual) != exp.String() {
			return fail(tc, fmt.Sprintf("Expected %s = %s, got %v", label, exp.String(), actual))
		}
	case float64:
		expInt := int(exp)
		switch act := actual.(type) {
		case int:
			if act != expInt {
				return fail(tc, fmt.Sprintf("Expected %s = %d, got %d", label, expInt, act))
			}
		default:
			if fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expInt) {
				return fail(tc, fmt.Sprintf("Expected %s = %v, got %v", label, expInt, actual))
			}
		}
	case bool:
		if actual != exp {
			return fail(tc, fmt.Sprintf("Expected %s = %v, got %v", label, exp, actual))
		}
	case string:
		if fmt.Sprintf("%v", actual) != exp {
			return fail(tc, fmt.Sprintf("Expected %s = %q, got %q", label, exp, actual))
		}
	}
	return nil
}

func fail(tc TestCase, msg string) *TestResult {
	return &TestResult{Name: tc.Name, Passed: false, Message: msg}
}

// expectedInt extracts an int from an expected value (json.Number or float64).
func expectedInt(v interface{}) int {
	switch n := v.(type) {
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case float64:
		return int(n)
	}
	return 0
}

// expectedString extracts a string from an expected value.
func expectedString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case json.Number:
		return s.String()
	}
	return fmt.Sprintf("%v", v)
}

// isNetworkError reports whether err is a genuine transport-level failure
// (connection reset / DNS / timeout). The generated Go client surfaces such
// failures as the raw error from http.Client.Do — a *url.Error, or something
// satisfying net.Error — rather than mapping them to a *basecamp.Error.
func isNetworkError(err error) bool {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// digPath walks a dot-notation path through nested maps, reporting presence.
func digPath(obj map[string]interface{}, path string) (interface{}, bool) {
	var current interface{} = obj
	for _, key := range strings.Split(path, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// jsonEqual compares two values by canonical JSON encoding, which handles
// arrays, objects, json.Number vs float64, and strings uniformly.
func jsonEqual(a, b interface{}) bool {
	return jsonString(a) == jsonString(b)
}

func jsonString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// getInt64Param extracts an int64 parameter from a map (JSON numbers are json.Number or float64)
func getInt64Param(params map[string]interface{}, key string) int64 {
	if val, ok := params[key]; ok {
		switch n := val.(type) {
		case json.Number:
			i, _ := n.Int64()
			return i
		case float64:
			return int64(n)
		}
	}
	return 0
}

// getStringParam extracts a string parameter from a map
func getStringParam(params map[string]interface{}, key string) string {
	if val, ok := params[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// getBoolParam extracts a bool parameter from a map.
func getBoolParam(params map[string]interface{}, key string) bool {
	if val, ok := params[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// getInt64SliceParam extracts an []int64 parameter, reporting whether the key
// was present: a present-but-empty array returns (non-nil empty slice, true)
// so explicit-empty (a clear) is distinguishable from absent (untouched).
// cardUpdateRequest builds an UpdateCardRequest from a fixture body using
// PRESENCE, not non-emptiness. UpdateCardRequest's scalars are pointers exactly
// so "explicitly set to empty" differs from "not set"; testing v != "" would
// collapse the two and let an explicit-clear fixture pass as an omission.
func cardUpdateRequest(body map[string]interface{}) *basecamp.UpdateCardRequest {
	req := &basecamp.UpdateCardRequest{}
	if v, ok := body["title"]; ok {
		s, _ := v.(string)
		req.Title = &s
	}
	if v, ok := body["content"]; ok {
		s, _ := v.(string)
		req.Content = &s
	}
	if v, ok := body["due_on"]; ok {
		s, _ := v.(string)
		req.DueOn = &s
	}
	if ids, ok := getInt64SliceParam(body, "assignee_ids"); ok {
		req.AssigneeIDs = ids
	}
	return req
}

func getInt64SliceParam(params map[string]interface{}, key string) ([]int64, bool) {
	val, ok := params[key]
	if !ok {
		return nil, false
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil, false
	}
	result := make([]int64, 0, len(arr))
	for _, item := range arr {
		switch n := item.(type) {
		case json.Number:
			i, _ := n.Int64()
			result = append(result, i)
		case float64:
			result = append(result, int64(n))
		}
	}
	return result, true
}

// getStringSliceParam extracts a []string parameter from a map (JSON arrays of strings)
func getStringSliceParam(params map[string]interface{}, key string) []string {
	if val, ok := params[key]; ok {
		if arr, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}
