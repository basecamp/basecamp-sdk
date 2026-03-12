package basecamp

import (
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

func TestZeroValueOptionalQueryParams_Omitted(t *testing.T) {
	t.Run("search: string zero values omitted", func(t *testing.T) {
		req, err := generated.NewSearchRequest("https://3.basecampapi.com", "12345", &generated.SearchParams{
			Q: "omacon",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		q := req.URL.Query()

		if got := q.Get("q"); got != "omacon" {
			t.Errorf("expected q=omacon, got %q", got)
		}
		if q.Has("sort") {
			t.Errorf("expected sort to be absent, got %q", q.Get("sort"))
		}
	})

	t.Run("todos: string zero value omitted, bool false still serialized", func(t *testing.T) {
		req, err := generated.NewListTodosRequest("https://3.basecampapi.com", "12345", 999, &generated.ListTodosParams{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		q := req.URL.Query()

		if q.Has("status") {
			t.Errorf("expected status to be absent, got %q", q.Get("status"))
		}
		// Bool params serialize unconditionally — false is a meaningful value
		// (e.g., completed=false means "show incomplete todos")
		if got := q.Get("completed"); got != "false" {
			t.Errorf("expected completed=false, got %q", got)
		}
	})

	t.Run("messages: optional sort/direction omitted when zero", func(t *testing.T) {
		req, err := generated.NewListMessagesRequest("https://3.basecampapi.com", "12345", 999, &generated.ListMessagesParams{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		q := req.URL.Query()

		if q.Has("sort") {
			t.Errorf("expected sort to be absent, got %q", q.Get("sort"))
		}
		if q.Has("direction") {
			t.Errorf("expected direction to be absent, got %q", q.Get("direction"))
		}
	})

	t.Run("recordings: required param present, optional params omitted", func(t *testing.T) {
		req, err := generated.NewListRecordingsRequest("https://3.basecampapi.com", "12345", &generated.ListRecordingsParams{
			Type: "Todo",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		q := req.URL.Query()

		if got := q.Get("type"); got != "Todo" {
			t.Errorf("expected type=Todo, got %q", got)
		}
		if q.Has("bucket") {
			t.Errorf("expected bucket to be absent, got %q", q.Get("bucket"))
		}
		if q.Has("status") {
			t.Errorf("expected status to be absent, got %q", q.Get("status"))
		}
		if q.Has("sort") {
			t.Errorf("expected sort to be absent, got %q", q.Get("sort"))
		}
		if q.Has("direction") {
			t.Errorf("expected direction to be absent, got %q", q.Get("direction"))
		}
	})
}

func TestNonZeroOptionalQueryParams_Included(t *testing.T) {
	t.Run("search: all params present", func(t *testing.T) {
		req, err := generated.NewSearchRequest("https://3.basecampapi.com", "12345", &generated.SearchParams{
			Q:    "omacon",
			Sort: "created_at",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		q := req.URL.Query()

		if got := q.Get("q"); got != "omacon" {
			t.Errorf("expected q=omacon, got %q", got)
		}
		if got := q.Get("sort"); got != "created_at" {
			t.Errorf("expected sort=created_at, got %q", got)
		}
	})

	t.Run("todos: bool true included", func(t *testing.T) {
		req, err := generated.NewListTodosRequest("https://3.basecampapi.com", "12345", 999, &generated.ListTodosParams{
			Status:    "active",
			Completed: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		q := req.URL.Query()

		if got := q.Get("status"); got != "active" {
			t.Errorf("expected status=active, got %q", got)
		}
		if got := q.Get("completed"); got != "true" {
			t.Errorf("expected completed=true, got %q", got)
		}
	})

	t.Run("messages: sort and direction included when set", func(t *testing.T) {
		req, err := generated.NewListMessagesRequest("https://3.basecampapi.com", "12345", 999, &generated.ListMessagesParams{
			Sort:      "created_at",
			Direction: "desc",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		q := req.URL.Query()

		if got := q.Get("sort"); got != "created_at" {
			t.Errorf("expected sort=created_at, got %q", got)
		}
		if got := q.Get("direction"); got != "desc" {
			t.Errorf("expected direction=desc, got %q", got)
		}
	})

	t.Run("recordings: optional strings included when set", func(t *testing.T) {
		req, err := generated.NewListRecordingsRequest("https://3.basecampapi.com", "12345", &generated.ListRecordingsParams{
			Type:      "Todo",
			Status:    "archived",
			Sort:      "updated_at",
			Direction: "desc",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		q := req.URL.Query()

		if got := q.Get("type"); got != "Todo" {
			t.Errorf("expected type=Todo, got %q", got)
		}
		if got := q.Get("status"); got != "archived" {
			t.Errorf("expected status=archived, got %q", got)
		}
		if got := q.Get("sort"); got != "updated_at" {
			t.Errorf("expected sort=updated_at, got %q", got)
		}
		if got := q.Get("direction"); got != "desc" {
			t.Errorf("expected direction=desc, got %q", got)
		}
	})
}
