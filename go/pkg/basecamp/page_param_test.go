package basecamp

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPageParamReachesWire asserts that setting Page on a list options struct
// sends ?page=N to the server, one representative operation per wrapper file
// whose endpoint honors the page query parameter server-side.
func TestPageParamReachesWire(t *testing.T) {
	cases := []struct {
		name string
		body string
		call func(ctx context.Context, ac *AccountClient) error
	}{
		{"boosts.ListRecording", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Boosts().ListRecording(ctx, 1, &BoostListOptions{Page: 3})
			return err
		}},
		{"campfires.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Campfires().List(ctx, &CampfireListOptions{Page: 3})
			return err
		}},
		{"cards.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Cards().List(ctx, 1, &CardListOptions{Page: 3})
			return err
		}},
		{"checkins.ListQuestions", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Checkins().ListQuestions(ctx, 1, &QuestionListOptions{Page: 3})
			return err
		}},
		{"client_approvals.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.ClientApprovals().List(ctx, &ClientApprovalListOptions{Page: 3})
			return err
		}},
		{"client_correspondences.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.ClientCorrespondences().List(ctx, &ClientCorrespondenceListOptions{Page: 3})
			return err
		}},
		{"client_replies.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.ClientReplies().List(ctx, 1, &ClientReplyListOptions{Page: 3})
			return err
		}},
		{"comments.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Comments().List(ctx, 1, &CommentListOptions{Page: 3})
			return err
		}},
		{"events.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Events().List(ctx, 1, &EventListOptions{Page: 3})
			return err
		}},
		{"forwards.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Forwards().List(ctx, 1, &ForwardListOptions{Page: 3})
			return err
		}},
		{"messages.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Messages().List(ctx, 1, &MessageListOptions{Page: 3})
			return err
		}},
		{"people.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.People().List(ctx, &PeopleListOptions{Page: 3})
			return err
		}},
		{"projects.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Projects().List(ctx, &ProjectListOptions{Page: 3})
			return err
		}},
		{"recordings.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Recordings().List(ctx, RecordingTypeTodo, &RecordingsListOptions{Page: 3})
			return err
		}},
		{"schedules.ListEntries", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Schedules().ListEntries(ctx, 1, &ScheduleEntryListOptions{Page: 3})
			return err
		}},
		{"search.Search", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Search().Search(ctx, "test", &SearchOptions{Page: 3})
			return err
		}},
		{"templates.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Templates().List(ctx, &TemplateListOptions{Page: 3})
			return err
		}},
		{"timeline.Progress", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Timeline().Progress(ctx, &TimelineListOptions{Page: 3})
			return err
		}},
		{"timesheet.ProjectReport", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Timesheet().ProjectReport(ctx, 1, &TimesheetReportOptions{Page: 3})
			return err
		}},
		{"todolist_groups.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.TodolistGroups().List(ctx, 1, &TodolistGroupListOptions{Page: 3})
			return err
		}},
		{"todolists.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Todolists().List(ctx, 1, &TodolistListOptions{Page: 3})
			return err
		}},
		{"todos.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Todos().List(ctx, 1, &TodoListOptions{Page: 3})
			return err
		}},
		{"vaults.List", `[]`, func(ctx context.Context, ac *AccountClient) error {
			_, err := ac.Vaults().List(ctx, 1, &VaultListOptions{Page: 3})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPage string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPage = r.URL.Query().Get("page")
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tc.body))
			}))
			defer server.Close()

			cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
			client := NewClient(cfg, &mockTokenProvider{})
			ac := client.ForAccount("12345")

			if err := tc.call(t.Context(), ac); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPage != "3" {
				t.Errorf("expected ?page=3 on the wire, got %q", gotPage)
			}
		})
	}
}

// TestPageParamRejectsOutOfRange asserts that a Page number too large for the
// int32 the generated params carry is reported as a usage error instead of
// wrapping around to a negative page on the wire.
func TestPageParamRejectsOutOfRange(t *testing.T) {
	var reached bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	ac := client.ForAccount("12345")

	// math.MaxInt32+1, built at runtime so the constant does not overflow int
	// on 32-bit platforms.
	overflowing := math.MaxInt32 + 1

	_, err := ac.Projects().List(t.Context(), &ProjectListOptions{Page: overflowing})
	if err == nil {
		t.Fatal("expected an error for an out-of-range page, got nil")
	}
	var bcErr *Error
	if !errors.As(err, &bcErr) || bcErr.Code != CodeUsage {
		t.Errorf("expected a usage error, got %T: %v", err, err)
	}
	if reached {
		t.Error("out-of-range page must not reach the wire")
	}
}
