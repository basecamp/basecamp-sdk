package basecamp

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// FilingContainer is the kind of container Move and Copy file a recording
// into. Every kind but FilingIntoTodolist is a tool on a project's dock, and
// its value is the tool's dock name (DockItem.Name): the destination project's
// container is its enabled dock item of that name. A to-do list is not on the
// dock; list the destination project's to-do lists under its todoset.
type FilingContainer string

// The containers recordings are filed into.
const (
	FilingIntoCardTable     FilingContainer = "kanban_board"
	FilingIntoMessageBoard  FilingContainer = "message_board"
	FilingIntoVault         FilingContainer = "vault"
	FilingIntoSchedule      FilingContainer = "schedule"
	FilingIntoQuestionnaire FilingContainer = "questionnaire"
	FilingIntoInbox         FilingContainer = "inbox"
	FilingIntoTodoset       FilingContainer = "todoset"
	FilingIntoTodolist      FilingContainer = "todolist"
)

// Folders reports whether a destination of this kind can take ParentID, a
// folder inside it to put the recording in.
func (c FilingContainer) Folders() bool { return c == FilingIntoVault }

// filingContainers holds the recordings that move and copy on their own, by
// recordable type. A Vault here is a folder, and a Todolist a to-do list in a
// to-do set; FilingContainerFor tells them from a docked Docs & Files tool and
// a to-do list group by their parent.
var filingContainers = map[string]FilingContainer{
	"Kanban::Card":    FilingIntoCardTable,
	"Kanban::Column":  FilingIntoCardTable,
	"Message":         FilingIntoMessageBoard,
	"Document":        FilingIntoVault,
	"Upload":          FilingIntoVault,
	"CloudFile":       FilingIntoVault,
	"GoogleDocument":  FilingIntoVault,
	"Vault":           FilingIntoVault,
	"Schedule::Entry": FilingIntoSchedule,
	"Question":        FilingIntoQuestionnaire,
	"Inbox::Forward":  FilingIntoInbox,
	"Todolist":        FilingIntoTodoset,
	"Todo":            FilingIntoTodolist,
}

// FilingContainerFor returns the kind of container Move and Copy file a
// recording into, from its recordable type and its parent's type as the
// recording's own read reports them (Parent.Type, "" when it has no parent).
// It is the recording's own kind of container: a message files into a message
// board, a card or a column you added into a card table, a to-do or a to-do
// list group into a to-do list. A card lands in the card table's Triage, or
// in Done if it is completed. A schedule entry's schedule may also be a
// calendar's, which is not on any project's dock.
//
// It reports false for everything else: a recording that moves and copies
// only with its parent, such as a comment or the columns every card table has,
// and a dock tool, such as a docked Vault, which has no container and is
// filed by DestinationProjectID.
func FilingContainerFor(recordingType, parentType string) (FilingContainer, bool) {
	switch {
	case recordingType == "Todolist" && parentType == "Todolist":
		return FilingIntoTodolist, true
	case recordingType == "Vault" && parentType != "Vault":
		return "", false
	}
	container, ok := filingContainers[recordingType]
	return container, ok
}

// FilingWaitOptions bounds how long MoveAndWait and CopyAndWait wait for
// Basecamp to finish a filing.
type FilingWaitOptions struct {
	// Timeout ends the wait this long after the filing starts. Zero waits a
	// minute. The context's deadline ends it sooner.
	Timeout time.Duration
}

const (
	defaultFilingWait = time.Minute
	firstFilingPoll   = 250 * time.Millisecond
	maxFilingPoll     = 2 * time.Second
)

// FilingRefusedError is a move or copy Basecamp refused to start, or the SDK
// refused before sending it: nothing was filed, so trying again is safe.
type FilingRefusedError struct {
	Err error
}

func (e *FilingRefusedError) Error() string { return "filing refused: " + e.Err.Error() }
func (e *FilingRefusedError) Unwrap() error { return e.Err }

// FilingMayHaveStartedError is a request to start a move or copy that failed
// without Basecamp refusing it, such as a 5xx or a dropped connection.
// Basecamp may have started it, and each copy request makes another copy, so
// check for the result before trying again.
//
// It does not unwrap to Err, so that a retryable error inside it cannot read
// as permission to send the request again.
type FilingMayHaveStartedError struct {
	Err error
}

func (e *FilingMayHaveStartedError) Error() string {
	return "filing may have started: " + e.Err.Error()
}

// FilingFailedError is a move or copy that Basecamp started and then reported
// failed.
type FilingFailedError struct {
	Filing *Filing
}

func (e *FilingFailedError) Error() string {
	return fmt.Sprintf("filing %d failed", e.Filing.ID)
}

// FilingUnsettledError is a move or copy that Basecamp started and had not
// finished when the wait ended. Filing is its last known state; poll GetFiling
// with its ID rather than starting it again. Cause is what ended the wait: the
// context's error, context.DeadlineExceeded for the wait's own Timeout, or a
// GetFiling that failed.
//
// It does not unwrap to Cause, so that a failed GetFiling inside it cannot read
// as a refused start.
type FilingUnsettledError struct {
	Filing *Filing
	Cause  error
}

func (e *FilingUnsettledError) Error() string {
	return fmt.Sprintf("filing %d is still %s: %v", e.Filing.ID, e.Filing.Status, e.Cause)
}

// MoveAndWait starts a move as Move does, once, and waits for Basecamp to
// finish it, polling GetFiling with backoff. It returns the completed filing,
// whose DestinationRecording is the moved recording.
//
// When it does not complete, the error says how far it got:
// *FilingRefusedError, *FilingMayHaveStartedError, *FilingFailedError or
// *FilingUnsettledError.
func (s *RecordingsService) MoveAndWait(ctx context.Context, recordingID int64, req *MoveRecordingRequest, opts *FilingWaitOptions) (*Filing, error) {
	filing, err := s.Move(ctx, recordingID, req)
	if err != nil {
		return nil, filingStartError(err)
	}
	return s.waitForFiling(ctx, filing, opts)
}

// CopyAndWait starts a copy as Copy does, once, and waits for Basecamp to
// finish it, polling GetFiling with backoff. It returns the completed filing,
// whose DestinationRecording is the copy.
//
// When it does not complete, the error says how far it got:
// *FilingRefusedError, *FilingMayHaveStartedError, *FilingFailedError or
// *FilingUnsettledError. Only after a *FilingRefusedError is copying again
// safe; anything else may already have made a copy.
func (s *RecordingsService) CopyAndWait(ctx context.Context, recordingID int64, req *CopyRecordingRequest, opts *FilingWaitOptions) (*Filing, error) {
	filing, err := s.Copy(ctx, recordingID, req)
	if err != nil {
		return nil, filingStartError(err)
	}
	return s.waitForFiling(ctx, filing, opts)
}

// filingStartError sorts a failed start into refused and may have started.
// Only a 4xx answer, or a refusal before the request was sent, shows nothing
// was filed: a 5xx or a dropped connection can follow a filing Basecamp began.
func filingStartError(err error) error {
	if apiErr, ok := errors.AsType[*Error](err); ok {
		if (apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500) || apiErr.Code == CodeUsage || apiErr.Code == CodeRateLimit {
			return &FilingRefusedError{Err: err}
		}
	}
	if errors.Is(err, ErrCircuitOpen) || errors.Is(err, ErrBulkheadFull) || errors.Is(err, ErrRateLimited) {
		return &FilingRefusedError{Err: err}
	}
	return &FilingMayHaveStartedError{Err: err}
}

func (s *RecordingsService) waitForFiling(ctx context.Context, filing *Filing, opts *FilingWaitOptions) (*Filing, error) {
	timeout := defaultFilingWait
	if opts != nil && opts.Timeout > 0 {
		timeout = opts.Timeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for delay := firstFilingPoll; ; delay = min(2*delay, maxFilingPoll) {
		switch filing.Status {
		case FilingCompleted:
			if filing.DestinationRecording != nil {
				return filing, nil
			}
		case FilingFailed:
			return nil, &FilingFailedError{Filing: filing}
		case FilingPending, FilingProcessing:
		}

		timer := time.NewTimer(delay)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			return nil, &FilingUnsettledError{Filing: filing, Cause: waitCtx.Err()}
		case <-timer.C:
		}

		latest, err := s.GetFiling(waitCtx, filing.ID)
		if err != nil {
			if waitCtx.Err() != nil {
				err = waitCtx.Err()
			}
			return nil, &FilingUnsettledError{Filing: filing, Cause: err}
		}
		filing = latest
	}
}
