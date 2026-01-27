// Package main provides a conformance test runner for the Go SDK.
//
// This runner reads JSON test definitions from conformance/tests/ and
// executes them against the SDK using a mock HTTP server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// TestCase represents a single conformance test.
type TestCase struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Operation     string                 `json:"operation"`
	Method        string                 `json:"method"`
	Path          string                 `json:"path"`
	PathParams    map[string]interface{} `json:"pathParams"`
	QueryParams   map[string]interface{} `json:"queryParams"`
	RequestBody   map[string]interface{} `json:"requestBody"`
	MockResponses []MockResponse         `json:"mockResponses"`
	Assertions    []Assertion            `json:"assertions"`
	Tags          []string               `json:"tags"`
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
	passed, failed := 0, 0

	for _, file := range files {
		tests, err := loadTests(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", file, err)
			continue
		}

		fmt.Printf("\n=== %s ===\n", filepath.Base(file))

		for _, tc := range tests {
			result := runTest(tc)
			results = append(results, result)

			if result.Passed {
				passed++
				fmt.Printf("  PASS: %s\n", tc.Name)
			} else {
				failed++
				fmt.Printf("  FAIL: %s\n        %s\n", tc.Name, result.Message)
			}
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Passed: %d, Failed: %d, Total: %d\n", passed, failed, passed+failed)

	if failed > 0 {
		os.Exit(1)
	}
}

func loadTests(filename string) ([]TestCase, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var tests []TestCase
	if err := json.Unmarshal(data, &tests); err != nil {
		return nil, err
	}

	return tests, nil
}

func runTest(tc TestCase) TestResult {
	// Track request count and timing
	var requestCount int32
	var requestTimes []time.Time

	// Create mock server that serves responses in sequence
	responseIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		requestTimes = append(requestTimes, time.Now())

		if responseIndex >= len(tc.MockResponses) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "No more mock responses"}`))
			return
		}

		resp := tc.MockResponses[responseIndex]
		responseIndex++

		// Apply delay if specified
		if resp.Delay > 0 {
			time.Sleep(time.Duration(resp.Delay) * time.Millisecond)
		}

		// Set headers
		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}

		// Set status code
		w.WriteHeader(resp.Status)

		// Write body
		if resp.Body != nil {
			bodyBytes, _ := json.Marshal(resp.Body)
			w.Write(bodyBytes)
		}
	}))
	defer server.Close()

	// Create SDK client pointing to mock server
	client, err := generated.NewClient(server.URL)
	if err != nil {
		return TestResult{
			Name:    tc.Name,
			Passed:  false,
			Message: fmt.Sprintf("Failed to create SDK client: %v", err),
		}
	}

	// Execute the appropriate SDK method based on the test operation
	ctx := context.Background()
	var sdkErr error
	var sdkResp *http.Response

	switch tc.Operation {
	case "ListProjects":
		sdkResp, sdkErr = client.ListProjects(ctx, nil)

	case "GetProject":
		projectId := getFloatParam(tc.PathParams, "projectId")
		sdkResp, sdkErr = client.GetProject(ctx, float32(projectId))

	case "UpdateProject":
		projectId := getFloatParam(tc.PathParams, "projectId")
		body := generated.UpdateProjectJSONRequestBody{
			Name: getStringParam(tc.RequestBody, "name"),
		}
		sdkResp, sdkErr = client.UpdateProject(ctx, float32(projectId), body)

	case "CreateProject":
		body := generated.CreateProjectJSONRequestBody{
			Name: getStringParam(tc.RequestBody, "name"),
		}
		sdkResp, sdkErr = client.CreateProject(ctx, body)

	case "TrashProject":
		projectId := getFloatParam(tc.PathParams, "projectId")
		sdkResp, sdkErr = client.TrashProject(ctx, float32(projectId))

	case "CreateTodo":
		projectId := getFloatParam(tc.PathParams, "projectId")
		todolistId := getFloatParam(tc.PathParams, "todolistId")
		body := generated.CreateTodoJSONRequestBody{
			Content: getStringParam(tc.RequestBody, "content"),
		}
		if dueOn, ok := tc.RequestBody["due_on"].(string); ok {
			body.DueOn = &dueOn
		}
		sdkResp, sdkErr = client.CreateTodo(ctx, float32(projectId), float32(todolistId), body)

	case "ListTodos":
		projectId := getFloatParam(tc.PathParams, "projectId")
		todolistId := getFloatParam(tc.PathParams, "todolistId")
		sdkResp, sdkErr = client.ListTodos(ctx, float32(projectId), float32(todolistId), nil)

	default:
		return TestResult{
			Name:    tc.Name,
			Passed:  false,
			Message: fmt.Sprintf("Unknown operation: %s", tc.Operation),
		}
	}

	// Run assertions
	for _, assertion := range tc.Assertions {
		switch assertion.Type {
		case "requestCount":
			expected := int32(assertion.Expected.(float64))
			actual := atomic.LoadInt32(&requestCount)
			if actual != expected {
				return TestResult{
					Name:    tc.Name,
					Passed:  false,
					Message: fmt.Sprintf("Expected %d requests, got %d", expected, actual),
				}
			}

		case "delayBetweenRequests":
			if len(requestTimes) >= 2 {
				delay := requestTimes[1].Sub(requestTimes[0])
				minDelay := time.Duration(assertion.Min) * time.Millisecond
				if delay < minDelay {
					return TestResult{
						Name:    tc.Name,
						Passed:  false,
						Message: fmt.Sprintf("Expected delay >= %v, got %v", minDelay, delay),
					}
				}
			}

		case "noError":
			if sdkErr != nil {
				return TestResult{
					Name:    tc.Name,
					Passed:  false,
					Message: fmt.Sprintf("Expected no error, got: %v", sdkErr),
				}
			}

		case "errorType":
			if sdkErr == nil {
				return TestResult{
					Name:    tc.Name,
					Passed:  false,
					Message: fmt.Sprintf("Expected error type %v, but got no error", assertion.Expected),
				}
			}
			// For now, just verify an error occurred - detailed error type checking can be enhanced

		case "statusCode":
			expected := int(assertion.Expected.(float64))
			if sdkResp == nil {
				return TestResult{
					Name:    tc.Name,
					Passed:  false,
					Message: fmt.Sprintf("Expected status code %d, but got no response", expected),
				}
			}
			if sdkResp.StatusCode != expected {
				return TestResult{
					Name:    tc.Name,
					Passed:  false,
					Message: fmt.Sprintf("Expected status code %d, got %d", expected, sdkResp.StatusCode),
				}
			}
		}
	}

	return TestResult{
		Name:    tc.Name,
		Passed:  true,
		Message: "All assertions passed",
	}
}

// getFloatParam extracts a float64 parameter from a map
func getFloatParam(params map[string]interface{}, key string) float64 {
	if val, ok := params[key]; ok {
		if f, ok := val.(float64); ok {
			return f
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
