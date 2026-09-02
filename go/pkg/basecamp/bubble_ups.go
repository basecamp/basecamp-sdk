package basecamp

import (
	"context"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// BubbleUpsService handles bubbling a recording up (and back down) in the
// current user's readings — the BC5 successor to "save". Both operations are
// per-recording and answer 204 No Content.
type BubbleUpsService struct {
	client *AccountClient
}

// NewBubbleUpsService creates a new BubbleUpsService.
func NewBubbleUpsService(client *AccountClient) *BubbleUpsService {
	return &BubbleUpsService{client: client}
}

// Create bubbles up the recording for the current user, resurfacing it in the
// user's readings. at controls timing: "now" bubbles up immediately, and a
// scheduling keyword ("today", "tomorrow", "weekend", "next_week") or an ISO8601
// date schedules it to resurface later. bc3 currently requires a value — a nil
// at omits the field and errors server-side (Date.iso8601(nil)) — so pass a
// pointer to "now" for the immediate case. Idempotent: bubbling up an
// already-bubbled recording is set-membership and still succeeds (204).
func (s *BubbleUpsService) Create(ctx context.Context, recordingID int64, at *string) (err error) {
	op := OperationInfo{
		Service: "BubbleUps", Operation: "Create",
		ResourceType: "bubble_up", IsMutation: true,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.CreateBubbleUpJSONRequestBody{At: at}
	resp, err := s.client.parent.gen.CreateBubbleUpWithResponse(ctx, s.client.accountID, recordingID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Delete pops the current user's bubble-up from the recording. Idempotent:
// popping an absent bubble-up also succeeds (204 either way).
func (s *BubbleUpsService) Delete(ctx context.Context, recordingID int64) (err error) {
	op := OperationInfo{
		Service: "BubbleUps", Operation: "Delete",
		ResourceType: "bubble_up", IsMutation: true,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteBubbleUpWithResponse(ctx, s.client.accountID, recordingID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}
