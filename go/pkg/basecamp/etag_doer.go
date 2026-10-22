package basecamp

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// etagDoer makes the generated client's GETs conditional on the client's ETag
// cache, as singleRequest does for the raw Get/GetAll path and for the pages
// followPagination fetches.
//
// Every typed operation sends its first request through the generated client,
// so before this a list call revalidated pages 2 and on but always fetched page
// 1 in full, and a single-resource Get never revalidated at all. The server can
// only answer an unchanged page cheaply when the client asks.
//
// A 304 becomes the cached 200, so generated response parsing and the service
// layer see the same thing they would have without the cache. Its headers are
// the 304's own: bc3 sets Link and X-Total-Count on every response to a paged
// collection, 304s included, and the raw path already reads them from the 304.
type etagDoer struct {
	client *Client
	next   generated.HttpRequestDoer
}

func (d *etagDoer) Do(req *http.Request) (*http.Response, error) {
	cache := d.client.cache
	// A caller that set its own validator owns the 304.
	if cache == nil || req.Method != http.MethodGet || req.Header.Get("If-None-Match") != "" {
		return d.next.Do(req)
	}

	// The key matches singleRequest's: the full URL and the credential the auth
	// editor applied, so each credential gets its own namespace.
	key := cache.Key(req.URL.String(), "", req.Header.Get("Authorization"))

	// Only ask for a 304 that can be answered: an ETag whose body is gone would
	// turn an unchanged resource into an error.
	var cached []byte
	if etag := cache.GetETag(key); etag != "" {
		if cached = cache.GetBody(key); cached != nil {
			req.Header.Set("If-None-Match", etag)
			d.client.logger.Debug("cache conditional request", "etag", etag)
		}
	}

	resp, err := d.next.Do(req)
	if err != nil {
		return resp, err
	}

	switch {
	case resp.StatusCode == http.StatusNotModified && cached != nil:
		d.client.logger.Debug("cache hit", "status", http.StatusNotModified)
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, MaxErrorBodyBytes))
		_ = resp.Body.Close()
		return notModifiedAsCached(resp, cached), nil

	case resp.StatusCode == http.StatusOK && resp.Header.Get("ETag") != "" && isJSONResponse(resp):
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(body)) <= MaxResponseBodyBytes {
			_ = cache.Set(key, body, resp.Header.Get("ETag")) // Ignore cache write errors
			d.client.logger.Debug("cache stored", "etag", resp.Header.Get("ETag"))
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil

	default:
		return resp, nil
	}
}

// notModifiedAsCached turns a 304 into the 200 it stands for. Generated parsing
// decodes a body only when the Content-Type says JSON, and a 304 carries none.
func notModifiedAsCached(resp *http.Response, body []byte) *http.Response {
	out := *resp
	out.StatusCode = http.StatusOK
	out.Status = "200 OK"
	out.Header = resp.Header.Clone()
	out.Header.Set("Content-Type", "application/json; charset=utf-8")
	out.Header.Set("Content-Length", strconv.Itoa(len(body)))
	out.ContentLength = int64(len(body))
	out.Body = io.NopCloser(bytes.NewReader(body))
	return &out
}

func isJSONResponse(resp *http.Response) bool {
	return strings.Contains(resp.Header.Get("Content-Type"), "json")
}
