package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
)

// StringResponse returns a responder that returns a 200 response with the given string body.
func StringResponse(body string) Responder {
	return func(req *http.Request) (*http.Response, error) {
		return httpResponse(http.StatusOK, req, bytes.NewBufferString(body)), nil
	}
}

// StatusStringResponse returns a responder with the given status code and string body.
func StatusStringResponse(status int, body string) Responder {
	return func(req *http.Request) (*http.Response, error) {
		return httpResponse(status, req, bytes.NewBufferString(body)), nil
	}
}

// BinaryResponse returns a responder that returns a 200 response with the given binary body.
func BinaryResponse(body []byte) Responder {
	return func(req *http.Request) (*http.Response, error) {
		return httpResponse(http.StatusOK, req, bytes.NewBuffer(body)), nil
	}
}

// RespondJSON returns a responder that returns a 200 response with a JSON body.
// The body is marshaled to JSON automatically.
func RespondJSON(body interface{}) Responder {
	return func(req *http.Request) (*http.Response, error) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON response: %w", err)
		}
		header := http.Header{
			"Content-Type": []string{"application/json"},
		}
		return httpResponseWithHeader(http.StatusOK, req, bytes.NewBuffer(b), header), nil
	}
}

// StatusJSON returns a responder with the given status code and JSON body.
func StatusJSON(status int, body interface{}) Responder {
	return func(req *http.Request) (*http.Response, error) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON response: %w", err)
		}
		header := http.Header{
			"Content-Type": []string{"application/json"},
		}
		return httpResponseWithHeader(status, req, bytes.NewBuffer(b), header), nil
	}
}

// RespondError returns a responder that returns an API error response.
// The error is formatted as {"error": "message"}.
func RespondError(status int, message string) Responder {
	return StatusJSON(status, map[string]string{"error": message})
}

// RespondNotFound returns a 404 Not Found response.
func RespondNotFound() Responder {
	return RespondError(http.StatusNotFound, "Not found")
}

// RespondUnauthorized returns a 401 Unauthorized response.
func RespondUnauthorized() Responder {
	return RespondError(http.StatusUnauthorized, "Unauthorized")
}

// RespondForbidden returns a 403 Forbidden response.
func RespondForbidden() Responder {
	return RespondError(http.StatusForbidden, "Forbidden")
}

// RespondRateLimit returns a 429 Too Many Requests response with a Retry-After header.
func RespondRateLimit(retryAfter int) Responder {
	return func(req *http.Request) (*http.Response, error) {
		b, _ := json.Marshal(map[string]string{"error": "Rate limit exceeded"})
		header := http.Header{
			"Content-Type": []string{"application/json"},
			"Retry-After":  []string{fmt.Sprintf("%d", retryAfter)},
		}
		return httpResponseWithHeader(http.StatusTooManyRequests, req, bytes.NewBuffer(b), header), nil
	}
}

// RespondServerError returns a 500 Internal Server Error response.
func RespondServerError() Responder {
	return RespondError(http.StatusInternalServerError, "Internal server error")
}

// FileResponse returns a responder that reads the response body from a file.
func FileResponse(filename string) Responder {
	return func(req *http.Request) (*http.Response, error) {
		f, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open fixture file %s: %w", filename, err)
		}
		return httpResponse(http.StatusOK, req, f), nil
	}
}

// WithHeader decorates a responder to add a response header.
func WithHeader(responder Responder, header, value string) Responder {
	return func(req *http.Request) (*http.Response, error) {
		resp, err := responder(req)
		if err != nil {
			return nil, err
		}
		if resp.Header == nil {
			resp.Header = make(http.Header)
		}
		resp.Header.Set(header, value)
		return resp, nil
	}
}

// WithPagination decorates a responder to add a Link header for pagination.
func WithPagination(responder Responder, nextURL string) Responder {
	if nextURL == "" {
		return responder
	}
	return WithHeader(responder, "Link", fmt.Sprintf(`<%s>; rel="next"`, nextURL))
}

// RESTPayload returns a responder that decodes the request JSON body
// and passes it to a callback before returning the response.
// This is useful for verifying request payloads in tests.
func RESTPayload(responseStatus int, responseBody string, cb func(payload map[string]interface{})) Responder {
	return func(req *http.Request) (*http.Response, error) {
		bodyData := make(map[string]interface{})
		if err := decodeJSONBody(req, &bodyData); err != nil {
			return nil, fmt.Errorf("failed to decode request body: %w", err)
		}
		cb(bodyData)

		header := http.Header{
			"Content-Type": []string{"application/json"},
		}
		return httpResponseWithHeader(responseStatus, req, bytes.NewBufferString(responseBody), header), nil
	}
}

// Sequence returns a responder that returns different responses on each call.
// After all responses are exhausted, it returns the last response repeatedly.
// This responder is safe for concurrent use.
func Sequence(responders ...Responder) Responder {
	if len(responders) == 0 {
		panic("Sequence requires at least one responder")
	}
	var callCount int64
	return func(req *http.Request) (*http.Response, error) {
		idx := int(atomic.AddInt64(&callCount, 1)) - 1
		if idx >= len(responders) {
			idx = len(responders) - 1
		}
		return responders[idx](req)
	}
}

// httpResponse creates an HTTP response with the given status, request, and body.
func httpResponse(status int, req *http.Request, body io.Reader) *http.Response {
	return httpResponseWithHeader(status, req, body, http.Header{})
}

// httpResponseWithHeader creates an HTTP response with the given status, request, body, and headers.
func httpResponseWithHeader(status int, req *http.Request, body io.Reader, header http.Header) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Request:    req,
		Body:       io.NopCloser(body),
		Header:     header,
	}
}
