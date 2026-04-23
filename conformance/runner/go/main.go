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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// TestCase represents a single conformance test.
type TestCase struct {
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
}

// MockResponse defines a single mock HTTP response.
type MockResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
	Delay   int               `json:"delay"`
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

// requestHeadersAt returns the headers captured for the given request index.
// Negative indexes count from the end (-1 = last). Returns (nil, false) when
// the index is out of range.
func requestHeadersAt(requestHeaders []http.Header, index int) (http.Header, bool) {
	n := len(requestHeaders)
	if n == 0 {
		return nil, false
	}
	if index < 0 {
		index += n
	}
	if index < 0 || index >= n {
		return nil, false
	}
	return requestHeaders[index], true
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
var goSDKSkips = map[string]string{}

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

	return tests, nil
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
	// Track request count and timing with mutex protection
	var mu sync.Mutex
	var requestCount int
	var requestTimes []time.Time
	var requestPaths []string
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
		mu.Lock()
		requestCount++
		requestTimes = append(requestTimes, time.Now())
		requestPaths = append(requestPaths, r.URL.Path)
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
			bodyToWrite := resp.Body
			if obj, ok := bodyToWrite.(map[string]interface{}); ok && len(obj) == 1 {
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

	// Run assertions
	for _, assertion := range tc.Assertions {
		if result := checkAssertion(tc, assertion, opResult, requestCount, requestTimes, requestPaths, requestHeaders); result != nil {
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
		if tc.ConfigOverrides != nil && tc.ConfigOverrides.MaxItems > 0 {
			opts = &basecamp.ProjectListOptions{Limit: tc.ConfigOverrides.MaxItems}
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

	case "GetTimesheetEntry":
		entryID := getInt64Param(tc.PathParams, "entryId")
		_, err := account.Timesheet().Get(ctx, entryID)
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

	case "CloneTool":
		sourceRecordingID := getInt64Param(tc.RequestBody, "source_recording_id")
		title := getStringParam(tc.RequestBody, "title")
		var opts *basecamp.CloneToolOptions
		if title != "" {
			opts = &basecamp.CloneToolOptions{Title: title}
		}
		_, err := account.Tools().Create(ctx, sourceRecordingID, opts)
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
	requestHeaders []http.Header,
) *TestResult {
	sdkErr := opResult.err

	// Detect if any mock response includes a Link header with rel="next".
	// The real SDK auto-paginates, so actual requestCount will be >= expected.
	hasLinkNextHeader := false
	for _, mr := range tc.MockResponses {
		if link, ok := mr.Headers["Link"]; ok && strings.Contains(link, `rel="next"`) {
			hasLinkNextHeader = true
			break
		}
	}

	switch assertion.Type {
	case "requestCount":
		expected := expectedInt(assertion.Expected)
		if hasLinkNextHeader {
			if requestCount < expected {
				return fail(tc, fmt.Sprintf("Expected >= %d requests (SDK auto-paginates), got %d", expected, requestCount))
			}
		} else if requestCount != expected {
			return fail(tc, fmt.Sprintf("Expected %d requests, got %d", expected, requestCount))
		}

	case "delayBetweenRequests":
		if len(requestTimes) >= 2 {
			delay := requestTimes[1].Sub(requestTimes[0])
			minDelay := time.Duration(assertion.Min) * time.Millisecond
			if delay < minDelay {
				return fail(tc, fmt.Sprintf("Expected delay >= %v, got %v", minDelay, delay))
			}
		}

	case "noError":
		if sdkErr != nil {
			return fail(tc, fmt.Sprintf("Expected no error, got: %v", sdkErr))
		}

	case "errorType":
		if sdkErr == nil {
			return fail(tc, fmt.Sprintf("Expected error type %v, but got no error", assertion.Expected))
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
		if len(requestPaths) == 0 {
			return fail(tc, "Expected a request to be made, but no requests were recorded")
		}
		if requestPaths[0] != expected {
			return fail(tc, fmt.Sprintf("Expected request path %q, got %q", expected, requestPaths[0]))
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
