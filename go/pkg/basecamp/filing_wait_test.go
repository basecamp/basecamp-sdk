package basecamp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// filingServer answers the start of a filing with a pending filing 4, then
// answers each GetFiling with the next of statuses, repeating the last.
type filingServer struct {
	mu       sync.Mutex
	starts   int
	polls    int
	statuses []string
}

func (f *filingServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost:
			f.starts++
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":4,"status":"pending","type":"Copy","source_recording_id":456,"url":"https://3.basecampapi.com/99999/filings/4.json"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/99999/filings/4":
			status := f.statuses[min(f.polls, len(f.statuses)-1)]
			f.polls++
			destination := ""
			if status == "completed" {
				destination = `,"destination_recording":{"id":1069480270,"type":"Message","url":"https://3.basecampapi.com/99999/buckets/2/messages/1069480270.json","app_url":"https://3.basecamp.com/99999/buckets/2/messages/1069480270"}`
			}
			fmt.Fprintf(w, `{"id":4,"status":%q,"type":"Copy","source_recording_id":456,"url":"https://3.basecampapi.com/99999/filings/4.json"%s}`, status, destination)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}
}

func TestRecordingsServiceMoveAndWaitCompletes(t *testing.T) {
	t.Parallel()
	server := &filingServer{statuses: []string{"pending", "processing", "completed"}}
	svc := testRecordingsServiceClient(t, server.handler(t))

	filing, err := svc.MoveAndWait(context.Background(), 456, &MoveRecordingRequest{FilingDestinationID: 789}, nil)
	if err != nil {
		t.Fatalf("MoveAndWait failed: %v", err)
	}
	if filing.Status != FilingCompleted || filing.DestinationRecording == nil || filing.DestinationRecording.ID != 1069480270 {
		t.Errorf("filing = %+v, want completed with destination 1069480270", filing)
	}
	if server.starts != 1 || server.polls != 3 {
		t.Errorf("starts = %d, polls = %d; want 1 start and 3 polls", server.starts, server.polls)
	}
}

func TestRecordingsServiceCopyAndWaitFailed(t *testing.T) {
	t.Parallel()
	server := &filingServer{statuses: []string{"processing", "failed"}}
	svc := testRecordingsServiceClient(t, server.handler(t))

	filing, err := svc.CopyAndWait(context.Background(), 456, &CopyRecordingRequest{FilingDestinationID: 789}, nil)
	failed, ok := errors.AsType[*FilingFailedError](err)
	if !ok || failed.Filing.ID != 4 || failed.Filing.Status != FilingFailed {
		t.Fatalf("expected a failed filing 4, got %v", err)
	}
	if filing != nil {
		t.Errorf("filing = %+v, want nil", filing)
	}
	if server.starts != 1 {
		t.Errorf("starts = %d, want 1", server.starts)
	}
}

func TestRecordingsServiceCopyAndWaitEndsWhileProcessing(t *testing.T) {
	t.Parallel()
	for name, wait := range map[string]func() (context.Context, *FilingWaitOptions, context.CancelFunc){
		"timeout option": func() (context.Context, *FilingWaitOptions, context.CancelFunc) {
			return context.Background(), &FilingWaitOptions{Timeout: 600 * time.Millisecond}, func() {}
		},
		"context deadline": func() (context.Context, *FilingWaitOptions, context.CancelFunc) {
			ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
			return ctx, nil, cancel
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := &filingServer{statuses: []string{"processing"}}
			svc := testRecordingsServiceClient(t, server.handler(t))
			ctx, opts, cancel := wait()
			defer cancel()

			_, err := svc.CopyAndWait(ctx, 456, &CopyRecordingRequest{FilingDestinationID: 789}, opts)
			unsettled, ok := errors.AsType[*FilingUnsettledError](err)
			if !ok || unsettled.Filing.ID != 4 || unsettled.Filing.Status != FilingProcessing {
				t.Fatalf("expected filing 4 still processing, got %v", err)
			}
			if !errors.Is(unsettled.Cause, context.DeadlineExceeded) {
				t.Errorf("cause = %v, want the deadline", unsettled.Cause)
			}
			if server.starts != 1 || server.polls == 0 {
				t.Errorf("starts = %d, polls = %d; want 1 start and some polls", server.starts, server.polls)
			}
		})
	}
}

func TestRecordingsServiceCopyAndWaitPollFailureIsUnsettled(t *testing.T) {
	t.Parallel()
	svc := testRecordingsServiceClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":4,"status":"pending","type":"Copy","source_recording_id":456,"url":"https://3.basecampapi.com/99999/filings/4.json"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status":404,"error":"Not Found"}`))
	})

	_, err := svc.CopyAndWait(context.Background(), 456, &CopyRecordingRequest{FilingDestinationID: 789}, nil)
	unsettled, ok := errors.AsType[*FilingUnsettledError](err)
	if !ok || unsettled.Filing.ID != 4 {
		t.Fatalf("expected filing 4 unsettled, got %v", err)
	}
	if cause, ok := errors.AsType[*Error](unsettled.Cause); !ok || cause.HTTPStatus != http.StatusNotFound {
		t.Errorf("cause = %v, want the poll's 404", unsettled.Cause)
	}
	if _, ok := errors.AsType[*Error](err); ok {
		t.Error("an accepted filing's poll error must not read as the start's *Error")
	}
}

func TestRecordingsServiceFilingAndWaitStartRefused(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			requests := 0
			svc := testRecordingsServiceClient(t, func(w http.ResponseWriter, _ *http.Request) {
				requests++
				w.WriteHeader(status)
			})

			for name, start := range map[string]func() (*Filing, error){
				"move": func() (*Filing, error) {
					return svc.MoveAndWait(context.Background(), 456, &MoveRecordingRequest{FilingDestinationID: 789}, nil)
				},
				"copy": func() (*Filing, error) {
					return svc.CopyAndWait(context.Background(), 456, &CopyRecordingRequest{FilingDestinationID: 789}, nil)
				},
			} {
				requests = 0
				_, err := start()
				if _, ok := errors.AsType[*FilingRefusedError](err); !ok {
					t.Fatalf("%s: expected a refused filing, got %v", name, err)
				}
				if apiErr, ok := errors.AsType[*Error](err); !ok || apiErr.HTTPStatus != status {
					t.Errorf("%s: expected the %d answer inside, got %v", name, status, err)
				}
				if requests != 1 {
					t.Errorf("%s: %d requests, want only the start", name, requests)
				}
			}
		})
	}
}

func TestRecordingsServiceFilingAndWaitRefusesWithoutADestination(t *testing.T) {
	svc := testRecordingsServiceClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("a filing without a destination must not reach the wire")
	})

	_, err := svc.CopyAndWait(context.Background(), 456, &CopyRecordingRequest{}, nil)
	if _, ok := errors.AsType[*FilingRefusedError](err); !ok {
		t.Errorf("expected a refused filing, got %v", err)
	}
}

func TestRecordingsServiceFilingAndWaitNeverRetriesTheStart(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			requests := 0
			svc := testRecordingsServiceClient(t, func(w http.ResponseWriter, _ *http.Request) {
				requests++
				w.WriteHeader(status)
			})

			for name, start := range map[string]func() (*Filing, error){
				"move": func() (*Filing, error) {
					return svc.MoveAndWait(context.Background(), 456, &MoveRecordingRequest{FilingDestinationID: 789}, nil)
				},
				"copy": func() (*Filing, error) {
					return svc.CopyAndWait(context.Background(), 456, &CopyRecordingRequest{FilingDestinationID: 789}, nil)
				},
			} {
				requests = 0
				_, err := start()
				started, ok := errors.AsType[*FilingMayHaveStartedError](err)
				if !ok {
					t.Fatalf("%s: expected a filing that may have started, got %v", name, err)
				}
				if apiErr, ok := errors.AsType[*Error](started.Err); !ok || apiErr.HTTPStatus != status {
					t.Errorf("%s: expected the %d answer, got %v", name, status, started.Err)
				}
				if _, ok := errors.AsType[*Error](err); ok {
					t.Errorf("%s: a start that may have filed must not unwrap to a retryable *Error", name)
				}
				if requests != 1 {
					t.Errorf("%s: the start was sent %d times, want once", name, requests)
				}
			}
		})
	}
}

func TestFilingContainerFor(t *testing.T) {
	tests := []struct {
		recording, parent string
		want              FilingContainer
		ok                bool
	}{
		{"Kanban::Card", "Kanban::Triage", FilingIntoCardTable, true},
		{"Kanban::Column", "Kanban::Board", FilingIntoCardTable, true},
		{"Message", "Message::Board", FilingIntoMessageBoard, true},
		{"Document", "Vault", FilingIntoVault, true},
		{"Upload", "Vault", FilingIntoVault, true},
		{"CloudFile", "Vault", FilingIntoVault, true},
		{"GoogleDocument", "Vault", FilingIntoVault, true},
		{"Vault", "Vault", FilingIntoVault, true},
		{"Schedule::Entry", "Schedule", FilingIntoSchedule, true},
		{"Question", "Questionnaire", FilingIntoQuestionnaire, true},
		{"Inbox::Forward", "Inbox", FilingIntoInbox, true},
		{"Todolist", "Todoset", FilingIntoTodoset, true},
		{"Todolist", "Todolist", FilingIntoTodolist, true},
		{"Todo", "Todolist", FilingIntoTodolist, true},
		{"Vault", "", "", false},
		{"Message::Board", "", "", false},
		{"Kanban::Triage", "Kanban::Board", "", false},
		{"Comment", "Message", "", false},
		{"Question::Answer", "Question", "", false},
	}
	for _, tt := range tests {
		got, ok := FilingContainerFor(tt.recording, tt.parent)
		if got != tt.want || ok != tt.ok {
			t.Errorf("FilingContainerFor(%q, %q) = %q, %v; want %q, %v", tt.recording, tt.parent, got, ok, tt.want, tt.ok)
		}
		if got.Folders() != (tt.want == FilingIntoVault) {
			t.Errorf("FilingContainerFor(%q, %q).Folders() = %v", tt.recording, tt.parent, got.Folders())
		}
	}
}
