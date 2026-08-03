package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOperationProjectResourceScope pins the {ProjectID, ResourceID} split that
// OnOperationStart observes for every operation whose path is project- or
// bucket-scoped. It drives real service calls through a capturing hook so a
// forgotten ProjectID assignment (or a stale ResourceID) is caught mechanically
// — `make check` alone cannot guarantee this. The server returns a trivial body
// because the assertions only inspect the operation metadata captured before the
// request is issued, not the decoded response.
func TestOperationProjectResourceScope(t *testing.T) {
	const (
		accountID   = "5245563"
		bucketID    = int64(2085958499)
		columnID    = int64(1101)
		cardTableID = int64(2202)
		wormholeID  = int64(3303)
		projectID   = int64(4404)
		typeID      = int64(5505)
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	tests := []struct {
		name           string
		invoke         func(ctx context.Context, ac *AccountClient)
		wantProjectID  int64
		wantResourceID int64
	}{
		// Group 1 — project scope (bucket) plus a deeper resource id that is kept.
		{"CardColumns.SetColor", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.CardColumns().SetColor(ctx, bucketID, columnID, "white")
		}, bucketID, columnID},
		{"CardColumns.EnableOnHold", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.CardColumns().EnableOnHold(ctx, bucketID, columnID)
		}, bucketID, columnID},
		{"CardColumns.DisableOnHold", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.CardColumns().DisableOnHold(ctx, bucketID, columnID)
		}, bucketID, columnID},
		{"Wormholes.Create", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Wormholes().Create(ctx, bucketID, cardTableID, 9001)
		}, bucketID, cardTableID},
		{"Wormholes.Update", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Wormholes().Update(ctx, bucketID, wormholeID, 9001)
		}, bucketID, wormholeID},
		{"Wormholes.Delete", func(ctx context.Context, ac *AccountClient) {
			_ = ac.Wormholes().Delete(ctx, bucketID, wormholeID)
		}, bucketID, wormholeID},
		{"MessageTypes.Get", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.MessageTypes().Get(ctx, bucketID, typeID)
		}, bucketID, typeID},
		{"MessageTypes.Update", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.MessageTypes().Update(ctx, bucketID, typeID, &UpdateMessageTypeRequest{})
		}, bucketID, typeID},
		{"MessageTypes.Delete", func(ctx context.Context, ac *AccountClient) {
			_ = ac.MessageTypes().Delete(ctx, bucketID, typeID)
		}, bucketID, typeID},

		// Group 2 — project scope only; no deeper resource id.
		{"Gauges.ListNeedles", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Gauges().ListNeedles(ctx, projectID, nil)
		}, projectID, 0},
		{"Gauges.CreateNeedle", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Gauges().CreateNeedle(ctx, projectID, &CreateGaugeNeedleRequest{})
		}, projectID, 0},
		{"Gauges.Toggle", func(ctx context.Context, ac *AccountClient) {
			_ = ac.Gauges().Toggle(ctx, projectID, true)
		}, projectID, 0},
		{"People.ListProjectPeople", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.People().ListProjectPeople(ctx, projectID, nil)
		}, projectID, 0},
		{"People.UpdateProjectAccess", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.People().UpdateProjectAccess(ctx, projectID, &UpdateProjectAccessRequest{})
		}, projectID, 0},
		{"Timesheet.ProjectReport", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Timesheet().ProjectReport(ctx, projectID, nil)
		}, projectID, 0},
		{"Timeline.ProjectTimeline", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Timeline().ProjectTimeline(ctx, projectID, nil)
		}, projectID, 0},
		{"Tools.Create", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Tools().Create(ctx, bucketID, "Message::Board", nil)
		}, bucketID, 0},
		{"Webhooks.List", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Webhooks().List(ctx, bucketID, &WebhookListOptions{Page: 1})
		}, bucketID, 0},
		{"Webhooks.Create", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Webhooks().Create(ctx, bucketID, &CreateWebhookRequest{})
		}, bucketID, 0},
		{"MessageTypes.List", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.MessageTypes().List(ctx, bucketID, &MessageTypeListOptions{Page: 1})
		}, bucketID, 0},
		{"MessageTypes.Create", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.MessageTypes().Create(ctx, bucketID, &CreateMessageTypeRequest{Name: "Announcement", Icon: "📣"})
		}, bucketID, 0},
		{"Projects.Get", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Projects().Get(ctx, projectID)
		}, projectID, 0},
		{"Projects.Update", func(ctx context.Context, ac *AccountClient) {
			_, _ = ac.Projects().Update(ctx, projectID, &UpdateProjectRequest{})
		}, projectID, 0},
		{"Projects.Trash", func(ctx context.Context, ac *AccountClient) {
			_ = ac.Projects().Trash(ctx, projectID)
		}, projectID, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.BaseURL = server.URL

			var capturedOp OperationInfo
			var captured bool
			hooks := &testHooks{
				onOperationStart: func(ctx context.Context, op OperationInfo) context.Context {
					capturedOp = op
					captured = true
					return ctx
				},
			}
			client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}, WithHooks(hooks))

			tt.invoke(context.Background(), client.ForAccount(accountID))

			if !captured {
				t.Fatalf("%s: OnOperationStart never fired", tt.name)
			}
			if capturedOp.ProjectID != tt.wantProjectID {
				t.Errorf("%s: ProjectID = %d, want %d", tt.name, capturedOp.ProjectID, tt.wantProjectID)
			}
			if capturedOp.ResourceID != tt.wantResourceID {
				t.Errorf("%s: ResourceID = %d, want %d", tt.name, capturedOp.ResourceID, tt.wantResourceID)
			}
		})
	}
}
