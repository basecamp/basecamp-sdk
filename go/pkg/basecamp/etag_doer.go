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
	key, cached := d.client.conditionalGET(req)
	if key == "" {
		return d.next.Do(req)
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
		// Read no more than the cache keeps. A larger body is handed on as the
		// prefix read so far followed by the rest of the stream, uncached, so
		// it isn't buffered twice.
		body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBodyBytes+1))
		if err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
		if int64(len(body)) > MaxResponseBodyBytes {
			resp.Body = struct {
				io.Reader
				io.Closer
			}{io.MultiReader(bytes.NewReader(body), resp.Body), resp.Body}
			return resp, nil
		}
		_ = resp.Body.Close()
		_ = d.client.cache.Set(key, body, resp.Header.Get("ETag")) // Ignore cache write errors
		d.client.logger.Debug("cache stored", "etag", resp.Header.Get("ETag"))
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil

	default:
		return resp, nil
	}
}

// conditionalGET applies the ETag cache to an outgoing request. It returns the
// cache key, empty when the request isn't cached, and the body a 304 stands
// for, nil unless If-None-Match was sent.
//
//   - Only GETs, and only with a cache configured.
//   - Only requests with an Authorization header. The key is scoped to that
//     credential; an AuthStrategy that authenticates some other way (a cookie,
//     an API-key header, mTLS) gives the cache nothing to tell callers apart
//     by, and two of them sharing a cache would read each other's bodies.
//   - Not a request that already carries If-None-Match: its caller owns the 304.
//   - If-None-Match only when the entry's body is there too, read under the
//     same lock as its ETag, so a 304 always has the body its ETag named.
func (c *Client) conditionalGET(req *http.Request) (key string, cached []byte) {
	auth := req.Header.Get("Authorization")
	if c.cache == nil || req.Method != http.MethodGet || auth == "" || req.Header.Get("If-None-Match") != "" {
		return "", nil
	}

	key = c.cache.Key(req.URL.String(), "", auth) // the URL already includes the account
	if etag, body := c.cache.entry(key); etag != "" && body != nil {
		req.Header.Set("If-None-Match", etag)
		c.logger.Debug("cache conditional request", "etag", etag)
		return key, body
	}
	return key, nil
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
