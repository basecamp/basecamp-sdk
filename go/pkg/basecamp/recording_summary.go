package basecamp

// RecordingSummary is a compact projection of one recording, resolved from
// the pointer an account event feed row or a webhook carries — bucket id,
// recording id, and the event type or recording type — through the typed read
// that type names. It exists for consumers that must decide something about a
// recording without paying for its full payload: an agent connector's
// admission step, an MCP tool answering "what is this?".
//
// The SDK has no untyped recording read (BC3 has no such route), so the type
// is the routing key: comment.created reads a comment, card.created reads a
// card, and so on — one typed read per type. Chat lines are the exception,
// because their read needs the Campfire id and the pointer does not carry it;
// Summarize discovers the Campfire first (see resolveChatLine).
//
// This is hand-written composition over the Go service wrappers, which are
// themselves the generated operations (AGENTS.md; SPEC.md Appendix F). It
// makes no wire request of its own, and it mints no operation identity: hooks
// see the constituent reads under their own names (SPEC.md §18 rule 3).

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RecordingRef points at one recording the way an event feed row does.
type RecordingRef struct {
	// BucketID is the project the recording lives in. Required: it scopes the
	// Campfire discovery for chat lines, and the read is checked against it so a
	// pointer from one project can never resolve to a recording in another.
	BucketID int64

	// RecordingID is the recording's id.
	RecordingID int64

	// EventType is the account event feed type that named the recording —
	// "comment.created", "card.assignment_changed", "chat.line.created". The
	// segment before the action names the recording type. Used when
	// RecordingType is empty.
	EventType string

	// RecordingType is the recording's own type as BC3 spells it — "Comment",
	// "Kanban::Card", "Chat::Lines::Text". When set it takes precedence over
	// EventType, being the more exact of the two.
	RecordingType string
}

// RecordingSummary is the projection Summarize returns. Fields a type does
// not have are zero: a Comment has no Assignees, a Vault no Content.
type RecordingSummary struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	// Type is the recording type as BC3 spells it ("Comment", "Kanban::Card").
	Type   string `json:"type"`
	Title  string `json:"title"`
	AppURL string `json:"app_url"`
	// Parent is the recording this one hangs off — the commented recording for
	// a comment, the Campfire for a chat line, the column for a card.
	Parent  *Parent `json:"parent,omitempty"`
	Bucket  *Bucket `json:"bucket,omitempty"`
	Creator *Person `json:"creator,omitempty"`
	// Assignees is set for the assignable types (to-dos, cards, card steps).
	Assignees []Person `json:"assignees,omitempty"`
	// MentionedPersonIDs are the people Content mentions, per MentionedPersonIDs.
	// Never nil, so a JSON consumer reads [] rather than a missing key.
	MentionedPersonIDs []int64 `json:"mentioned_person_ids"`
	// Content is the recording's rich text, in full: the comment body, the
	// message body, a to-do's description, a card's content, the chat line.
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
	// CampfireID is the Campfire a chat line was found under — the reply
	// destination for a chat trigger. Zero for every other type.
	CampfireID int64 `json:"campfire_id,omitempty"`
}

// Sentinel errors Summarize returns, each also carried by a typed error that
// names the pointer (RecordingRoutingError, UnresolvedRecordingError,
// BucketMismatchError). Match with errors.Is.
var (
	// ErrNoRecordingType is returned for an event type that names no recording
	// type — boost.created, whose recording is the boost's target and whose
	// type the feed row does not carry. A consumer resolves those from its own
	// record of what it posted, not through Summarize.
	ErrNoRecordingType = errors.New("event type names no recording type")

	// ErrUnknownRecordingType is returned when neither EventType nor
	// RecordingType names a type in the routing table.
	ErrUnknownRecordingType = errors.New("no typed read for recording type")

	// ErrRecordingUnresolved is returned when a chat line was found under none
	// of the Campfires the caller can currently see in its bucket. It is
	// distinct from a failed read (any non-404 answer is returned as itself)
	// and from discovery that could not finish (ErrCampfireDiscoveryIncomplete):
	// every candidate answered 404. It is not distinct from lost visibility —
	// BC3 answers 404 for a Campfire the caller may not see, too — so a
	// consumer marks the record blocked and retries on its own schedule; see
	// UnresolvedRecordingError.StaleCampfireIDs.
	ErrRecordingUnresolved = errors.New("chat line found under no visible campfire")

	// ErrBucketMismatch is returned when the recording the read returned lives
	// in a different bucket from the one the pointer named.
	ErrBucketMismatch = errors.New("recording is not in the requested bucket")
)

// RecordingRoutingError reports a RecordingRef that Summarize cannot route. It
// wraps ErrNoRecordingType or ErrUnknownRecordingType.
type RecordingRoutingError struct {
	Ref RecordingRef
	Err error
}

func (e *RecordingRoutingError) Error() string {
	key := e.Ref.RecordingType
	if key == "" {
		key = e.Ref.EventType
	}
	return fmt.Sprintf("%v: %q", e.Err, key)
}

// Unwrap exposes the sentinel for errors.Is.
func (e *RecordingRoutingError) Unwrap() error { return e.Err }

// UnresolvedRecordingError reports a chat line found under no visible
// Campfire. CampfireIDs are the candidates tried, in order; empty when the
// bucket has no visible Campfire at all.
type UnresolvedRecordingError struct {
	BucketID    int64
	RecordingID int64
	CampfireIDs []int64
	// Refreshed reports whether the cached discovery sources were re-read
	// before concluding. False when every source had been read within the
	// last campfireIndexMinRefresh, so a Campfire created in that window was
	// not seen: the conclusion stands on data up to that old, and a retry after
	// the floor sees the current sources.
	Refreshed bool
	// StaleCampfireIDs are candidates from the cache that the refreshed
	// sources no longer list — Campfires the caller could see when the cache
	// filled and cannot now. Set only when Refreshed.
	StaleCampfireIDs []int64
}

func (e *UnresolvedRecordingError) Error() string {
	return fmt.Sprintf("%v: line %d in bucket %d (tried %d campfires)", ErrRecordingUnresolved, e.RecordingID, e.BucketID, len(e.CampfireIDs))
}

// Unwrap exposes ErrRecordingUnresolved for errors.Is.
func (e *UnresolvedRecordingError) Unwrap() error { return ErrRecordingUnresolved }

// BucketMismatchError reports a read whose bucket disagrees with the pointer.
type BucketMismatchError struct {
	Ref      RecordingRef
	BucketID int64 // the bucket the read returned
}

func (e *BucketMismatchError) Error() string {
	return fmt.Sprintf("%v: recording %d is in bucket %d, not %d", ErrBucketMismatch, e.Ref.RecordingID, e.BucketID, e.Ref.BucketID)
}

// Unwrap exposes ErrBucketMismatch for errors.Is.
func (e *BucketMismatchError) Unwrap() error { return ErrBucketMismatch }

// summaryKind is the routing key: which typed read serves a RecordingRef.
type summaryKind int

const (
	kindUnknown summaryKind = iota
	kindComment
	kindMessage
	kindTodo
	kindCard
	kindChatLine
	kindDocument
	kindUpload
	kindScheduleEntry
	kindQuestion
	kindQuestionAnswer
	kindTodolist
	kindVault
	kindForward
	kindClientApproval
	kindClientCorrespondence
	kindGoogleDocument
	kindCloudFile
	kindCardStep
)

// eventSubjects maps the subject of an account event feed type — everything
// before its final "." — to a read. This is the feed's catalog
// (bc3 Event::EventType) minus boost, which names no recording type and is
// refused explicitly below rather than left to fall through as unknown.
var eventSubjects = map[string]summaryKind{
	"comment":   kindComment,
	"message":   kindMessage,
	"todo":      kindTodo,
	"card":      kindCard,
	"chat.line": kindChatLine,
}

// recordingTypes maps BC3's recording type strings to a read. Chat lines
// are matched by prefix (Chat::Lines::Text, ::RichText, ::Code, ::Upload,
// ::Integration all read through the same route); everything else exactly.
//
// Absent on purpose, because their reads need a parent id the pointer does
// not carry: Client::Reply (bucket + correspondence + reply), Forward::Reply
// (forward + reply), Question::Answer's sibling Questionnaire, and Kanban
// columns and tables, which are not recordings a feed row points at.
var recordingTypes = map[string]summaryKind{
	"Comment":                kindComment,
	"Message":                kindMessage,
	"Todo":                   kindTodo,
	"Kanban::Card":           kindCard,
	"Document":               kindDocument,
	"Upload":                 kindUpload,
	"Schedule::Entry":        kindScheduleEntry,
	"Question":               kindQuestion,
	"Question::Answer":       kindQuestionAnswer,
	"Todolist":               kindTodolist,
	"Vault":                  kindVault,
	"Inbox::Forward":         kindForward,
	"Client::Approval":       kindClientApproval,
	"Client::Correspondence": kindClientCorrespondence,
	"GoogleDocument":         kindGoogleDocument,
	"CloudFile":              kindCloudFile,
	"Kanban::Step":           kindCardStep,
}

const chatLineTypePrefix = "Chat::Lines::"

// chatLineIsRichText reports whether a chat line subtype carries rich text —
// the two that declare rich_text_attribute :content in BC3, and so the only
// two whose content can hold a mention. A Text line's content is HTML-escaped
// on the way out (content_helper.rb, format_chat_line_with), a Code line's is
// served verbatim — a snippet that happens to contain a bc-attachment tag —
// and an Upload line has no content.
func chatLineIsRichText(lineType string) bool {
	switch lineType {
	case "Chat::Lines::RichText", "Chat::Lines::Integration":
		return true
	}
	return false
}

// routeRecording picks the read for a ref. RecordingType wins when set.
func routeRecording(ref RecordingRef) (summaryKind, error) {
	if t := strings.TrimSpace(ref.RecordingType); t != "" {
		if strings.HasPrefix(t, chatLineTypePrefix) {
			return kindChatLine, nil
		}
		if k, ok := recordingTypes[t]; ok {
			return k, nil
		}
		return kindUnknown, &RecordingRoutingError{Ref: ref, Err: ErrUnknownRecordingType}
	}
	et := strings.TrimSpace(ref.EventType)
	if et == "" {
		return kindUnknown, &RecordingRoutingError{Ref: ref, Err: ErrUnknownRecordingType}
	}
	// A feed type is "<subject>.<action>"; the subject names the recording
	// type. A string with no action is not a feed type and is not routed.
	i := strings.LastIndex(et, ".")
	if i <= 0 || i == len(et)-1 {
		return kindUnknown, &RecordingRoutingError{Ref: ref, Err: ErrUnknownRecordingType}
	}
	subject := et[:i]
	if subject == "boost" {
		return kindUnknown, &RecordingRoutingError{Ref: ref, Err: ErrNoRecordingType}
	}
	if k, ok := eventSubjects[subject]; ok {
		return k, nil
	}
	return kindUnknown, &RecordingRoutingError{Ref: ref, Err: ErrUnknownRecordingType}
}

// Summarize resolves a recording pointer into a RecordingSummary through the
// typed read its type names. See RecordingRef for the routing inputs and the
// package comment above for the design.
//
// Errors: a routing failure (errors.Is ErrNoRecordingType or
// ErrUnknownRecordingType) before any request; the read's own *Error
// otherwise — a 404 is CodeNotFound, as from the typed read itself; for chat
// lines, ErrRecordingUnresolved when every visible Campfire answered 404, which
// is distinct from a read that failed (any non-404 from a candidate is returned
// as that error, and the loop stops there) and from
// ErrCampfireDiscoveryIncomplete (candidates were left unsearched);
// ErrBucketMismatch when the read returned a recording from another bucket.
func (s *RecordingsService) Summarize(ctx context.Context, ref RecordingRef) (*RecordingSummary, error) {
	if ref.BucketID <= 0 || ref.RecordingID <= 0 {
		return nil, ErrUsage("bucket id and recording id are required")
	}
	kind, err := routeRecording(ref)
	if err != nil {
		return nil, err
	}
	summary, err := s.readSummary(ctx, ref, kind)
	if err != nil {
		return nil, err
	}
	if summary.Bucket != nil && summary.Bucket.ID != 0 && summary.Bucket.ID != ref.BucketID {
		return nil, &BucketMismatchError{Ref: ref, BucketID: summary.Bucket.ID}
	}
	if summary.MentionedPersonIDs == nil {
		summary.MentionedPersonIDs = []int64{}
	}
	return summary, nil
}

// readSummary performs the one typed read a kind names and projects it.
func (s *RecordingsService) readSummary(ctx context.Context, ref RecordingRef, kind summaryKind) (*RecordingSummary, error) {
	ac := s.client
	id := ref.RecordingID
	switch kind {
	case kindComment:
		c, err := ac.Comments().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(c.ID, c.Status, c.Type, c.Title, c.AppURL, c.Parent, c.Bucket, c.Creator, nil, c.Content, c.UpdatedAt), nil
	case kindMessage:
		m, err := ac.Messages().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(m.ID, m.Status, m.Type, firstNonEmpty(m.Title, m.Subject), m.AppURL, m.Parent, m.Bucket, m.Creator, nil, m.Content, m.UpdatedAt), nil
	case kindTodo:
		// A to-do's content is its plain title; the rich text — where mentions
		// live — is the description.
		t, err := ac.Todos().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(t.ID, t.Status, t.Type, firstNonEmpty(t.Title, t.Content), t.AppURL, t.Parent, t.Bucket, t.Creator, t.Assignees, t.Description, t.UpdatedAt), nil
	case kindCard:
		c, err := ac.Cards().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(c.ID, c.Status, c.Type, c.Title, c.AppURL, c.Parent, c.Bucket, c.Creator, c.Assignees, firstNonEmpty(c.Content, c.Description), c.UpdatedAt), nil
	case kindChatLine:
		line, campfireID, err := s.resolveChatLine(ctx, ref.BucketID, id)
		if err != nil {
			return nil, err
		}
		sum := summarize(line.ID, line.Status, line.Type, line.Title, line.AppURL, line.Parent, line.Bucket, line.Creator, nil, line.Content, line.UpdatedAt)
		if !chatLineIsRichText(line.Type) {
			// A plain-text or code line's content is text BC3 never read as
			// markup, so a literal "<bc-attachment>" in it mentions nobody.
			sum.MentionedPersonIDs = nil
		}
		sum.CampfireID = campfireID
		return sum, nil
	case kindDocument:
		d, err := ac.Documents().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(d.ID, d.Status, d.Type, d.Title, d.AppURL, d.Parent, d.Bucket, d.Creator, nil, d.Content, d.UpdatedAt), nil
	case kindUpload:
		u, err := ac.Uploads().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(u.ID, u.Status, u.Type, firstNonEmpty(u.Title, u.Filename), u.AppURL, u.Parent, u.Bucket, u.Creator, nil, u.Description, u.UpdatedAt), nil
	case kindScheduleEntry:
		e, err := ac.Schedules().GetEntry(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(e.ID, e.Status, e.Type, firstNonEmpty(e.Title, e.Summary), e.AppURL, e.Parent, e.Bucket, e.Creator, nil, e.Description, e.UpdatedAt), nil
	case kindQuestion:
		q, err := ac.Checkins().GetQuestion(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(q.ID, q.Status, q.Type, q.Title, q.AppURL, q.Parent, q.Bucket, q.Creator, nil, "", q.UpdatedAt), nil
	case kindQuestionAnswer:
		a, err := ac.Checkins().GetAnswer(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(a.ID, a.Status, a.Type, a.Title, a.AppURL, a.Parent, a.Bucket, a.Creator, nil, a.Content, a.UpdatedAt), nil
	case kindTodolist:
		l, err := ac.Todolists().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(l.ID, l.Status, l.Type, firstNonEmpty(l.Title, l.Name), l.AppURL, l.Parent, l.Bucket, l.Creator, nil, l.Description, l.UpdatedAt), nil
	case kindVault:
		v, err := ac.Vaults().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(v.ID, v.Status, v.Type, v.Title, v.AppURL, v.Parent, v.Bucket, v.Creator, nil, "", v.UpdatedAt), nil
	case kindForward:
		f, err := ac.Forwards().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(f.ID, f.Status, f.Type, firstNonEmpty(f.Title, f.Subject), f.AppURL, f.Parent, f.Bucket, f.Creator, nil, f.Content, f.UpdatedAt), nil
	case kindClientApproval:
		a, err := ac.ClientApprovals().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(a.ID, a.Status, a.Type, firstNonEmpty(a.Title, a.Subject), a.AppURL, a.Parent, a.Bucket, a.Creator, nil, a.Content, a.UpdatedAt), nil
	case kindClientCorrespondence:
		c, err := ac.ClientCorrespondences().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(c.ID, c.Status, c.Type, firstNonEmpty(c.Title, c.Subject), c.AppURL, c.Parent, c.Bucket, c.Creator, nil, c.Content, c.UpdatedAt), nil
	case kindGoogleDocument:
		g, err := ac.GoogleDocuments().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(g.ID, g.Status, g.Type, g.Title, g.AppURL, g.Parent, g.Bucket, g.Creator, nil, g.Description, g.UpdatedAt), nil
	case kindCloudFile:
		f, err := ac.CloudFiles().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(f.ID, f.Status, f.Type, f.Title, f.AppURL, f.Parent, f.Bucket, f.Creator, nil, f.Description, f.UpdatedAt), nil
	case kindCardStep:
		st, err := ac.CardSteps().Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return summarize(st.ID, st.Status, st.Type, st.Title, st.AppURL, st.Parent, st.Bucket, st.Creator, st.Assignees, "", st.UpdatedAt), nil
	case kindUnknown:
		return nil, &RecordingRoutingError{Ref: ref, Err: ErrUnknownRecordingType}
	}
	return nil, &RecordingRoutingError{Ref: ref, Err: ErrUnknownRecordingType}
}

func summarize(id int64, status, typ, title, appURL string, parent *Parent, bucket *Bucket, creator *Person, assignees []Person, content string, updatedAt time.Time) *RecordingSummary {
	return &RecordingSummary{
		ID:                 id,
		Status:             status,
		Type:               typ,
		Title:              title,
		AppURL:             appURL,
		Parent:             parent,
		Bucket:             bucket,
		Creator:            creator,
		Assignees:          assignees,
		MentionedPersonIDs: MentionedPersonIDs(content),
		Content:            content,
		UpdatedAt:          updatedAt,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Campfire discovery for chat lines.
//
// A chat.line.created row carries the line's id and bucket, not its Campfire,
// and the line read is /chats/{campfireId}/lines/{lineId}. Candidates come
// from two sources, tried in order and each cached ten minutes:
//
//  1. The bucket's project dock, whose "chat" tool is the project's Campfire:
//     one project read per bucket, and the answer for every line posted in a
//     project. Cached per bucket.
//  2. The account-wide Campfire listing (BC3 has no per-bucket one), filtered
//     to the bucket, for buckets that are not projects or whose dock did not
//     hold the line. Cached per account, so a burst of lines costs one listing.
//
// The loop tries the line under each candidate until one answers, within one
// total budget of MaxCampfireCandidates per call.
//
// Two failure shapes are kept apart on purpose. A candidate that answers
// anything but 404 — 401, 403, 5xx, a network error, a cancelled context —
// stops the loop and is returned as that error: the read failed, and trying
// the next Campfire would only hide it. A 404 means "not here", so the loop
// moves on. Only when every candidate said "not here" is the line unresolved
// (ErrRecordingUnresolved) — and before concluding that, the cached sources
// are refreshed (subject to a floor, below) so a Campfire created after the
// cache filled is tried too. Discovery that could not be completed — a
// listing cut off at its cap, a bucket with more candidates than the budget
// — is ErrCampfireDiscoveryIncomplete, never "unresolved": nothing unsearched
// is ever reported absent.
//
// What HTTP cannot tell apart: BC3 answers 404 both for a line that is not in
// a Campfire and for a Campfire the caller may no longer see. "Unresolved"
// therefore means "under no Campfire the caller can currently see", and the
// error reports the cached candidates that the refreshed sources no longer
// list (StaleCampfireIDs) so a consumer can see when visibility, not
// existence, is what changed.

const (
	// CampfireIndexTTL is how long a cached discovery source — a bucket's
	// project dock, the account's Campfire listing — is reused before it is
	// read again.
	CampfireIndexTTL = 10 * time.Minute

	// campfireIndexMinRefresh bounds the refresh-on-miss: a line found under
	// no candidate re-reads the cached sources, but not more often than this
	// per source, so a run of unresolvable lines cannot turn into a listing
	// per line. UnresolvedRecordingError.Refreshed says whether the floor
	// applied.
	campfireIndexMinRefresh = 30 * time.Second

	// MaxCampfireCandidates bounds how many Campfires one Summarize call
	// tries, across both sources and the refresh. A project has one Campfire
	// and a handful of pings; a bucket past this bound is not a shape BC3
	// produces, and the call reports ErrCampfireDiscoveryIncomplete rather
	// than calling the rest absent.
	MaxCampfireCandidates = 50

	// MaxCampfireListing caps the account-wide Campfire listing the fallback
	// source reads. A listing that overflows it is not cached and the call
	// reports ErrCampfireDiscoveryIncomplete: the dock covers every project,
	// so the listing only ever serves the leftover, and an account with more
	// Campfires than this should not pay a full walk per ten minutes for it.
	MaxCampfireListing = 1000
)

// ErrCampfireDiscoveryIncomplete is returned when a chat line's discovery
// could not be carried to a conclusion — the Campfire listing overflowed
// MaxCampfireListing, or a bucket has more visible Campfires than
// MaxCampfireCandidates. Distinct from ErrRecordingUnresolved: candidates
// were left unsearched, so nothing can be reported absent.
var ErrCampfireDiscoveryIncomplete = errors.New("campfire discovery incomplete")

// CampfireDiscoveryIncompleteError carries the pointer and why discovery
// stopped short.
type CampfireDiscoveryIncompleteError struct {
	BucketID    int64
	RecordingID int64
	Reason      string
}

func (e *CampfireDiscoveryIncompleteError) Error() string {
	return fmt.Sprintf("%v: line %d in bucket %d: %s", ErrCampfireDiscoveryIncomplete, e.RecordingID, e.BucketID, e.Reason)
}

// Unwrap exposes ErrCampfireDiscoveryIncomplete for errors.Is.
func (e *CampfireDiscoveryIncompleteError) Unwrap() error { return ErrCampfireDiscoveryIncomplete }

// ttlCache is a per-key cache with single-flight loading: concurrent callers
// for one key wait on the one load in progress rather than loading again, a
// failed load leaves the previous value in place, and a refresh is honoured
// only once the value is older than a floor.
type ttlCache[K comparable, V any] struct {
	mu       sync.Mutex
	now      func() time.Time
	ttl      time.Duration
	floor    time.Duration
	entries  map[K]*ttlEntry[V]
	inflight map[K]*ttlLoad
	// onWait, when set, runs just before a caller waits on another caller's
	// load. A test seam: it lets a test know the waiting path was reached
	// rather than guess at it with a sleep.
	onWait func()
}

type ttlEntry[V any] struct {
	value   V
	fetched time.Time
}

type ttlLoad struct {
	done    chan struct{}
	err     error
	fetched time.Time // set at publication, for the loader's own hit
	// callerDone records whether the loading caller's own context was done
	// when the load ended. It is the only reliable sign that the failure was
	// that caller's cancellation or deadline: an http.Client.Timeout also
	// satisfies errors.Is(err, context.DeadlineExceeded) with the context
	// still live, and that failure is the load's own, to be shared.
	callerDone bool
}

func newTTLCache[K comparable, V any](now func() time.Time, ttl, floor time.Duration) *ttlCache[K, V] {
	return &ttlCache[K, V]{now: now, ttl: ttl, floor: floor, entries: map[K]*ttlEntry[V]{}, inflight: map[K]*ttlLoad{}}
}

// ttlHit is what a cache read hands back: the value, when it was fetched,
// and whether it predated the call (as opposed to being loaded during it, by
// this caller or by one it waited on). The fetch time is what lets a caller
// tell a snapshot it already consulted from a newer one, whoever loaded it.
type ttlHit[V any] struct {
	value   V
	fetched time.Time
	cached  bool
}

// get returns the value for key, loading it when absent or older than the
// TTL — or, with refresh set, older than the floor.
func (c *ttlCache[K, V]) get(ctx context.Context, key K, refresh bool, load func(context.Context) (V, error)) (ttlHit[V], error) {
	for {
		if err := ctx.Err(); err != nil {
			return ttlHit[V]{}, err
		}
		c.mu.Lock()
		if entry := c.entries[key]; entry != nil {
			age := c.now().Sub(entry.fetched)
			if age < c.ttl && (!refresh || age < c.floor) {
				c.mu.Unlock()
				return ttlHit[V]{value: entry.value, fetched: entry.fetched, cached: true}, nil
			}
		}
		if pending := c.inflight[key]; pending != nil {
			c.mu.Unlock()
			if c.onWait != nil {
				c.onWait()
			}
			select {
			case <-pending.done:
			case <-ctx.Done():
				return ttlHit[V]{}, ctx.Err()
			}
			if pending.err != nil {
				// The load ran under the loading caller's context. If that
				// context was done when the load ended, the failure is that
				// caller's, not this one's: a waiter whose own context is
				// live goes round again and loads for itself (the key is
				// free, so it becomes the loader and any other waiters queue
				// behind it — one load, not a stampede). Any other error —
				// a transport timeout included — is the load's own and is
				// shared, so N waiters never re-run one failed load N times.
				if pending.callerDone && ctx.Err() == nil {
					continue
				}
				return ttlHit[V]{}, pending.err
			}
			// The load this call waited on is this call's load: hand its
			// value over as fresh, not as something that predated the call.
			c.mu.Lock()
			entry := c.entries[key]
			c.mu.Unlock()
			if entry != nil {
				return ttlHit[V]{value: entry.value, fetched: entry.fetched}, nil
			}
			continue
		}
		pending := &ttlLoad{done: make(chan struct{})}
		c.inflight[key] = pending
		c.mu.Unlock()

		loaded, loadErr := c.load(ctx, key, pending, load)
		if loadErr != nil {
			return ttlHit[V]{}, loadErr
		}
		return ttlHit[V]{value: loaded, fetched: pending.fetched}, nil
	}
}

// peek returns the cached value for key when one is within the TTL, without
// loading. It lets a caller consult what a source already holds before
// deciding whether to pay for a fetch of it.
func (c *ttlCache[K, V]) peek(key K) (ttlHit[V], bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[key]
	if entry == nil || c.now().Sub(entry.fetched) >= c.ttl {
		return ttlHit[V]{}, false
	}
	return ttlHit[V]{value: entry.value, fetched: entry.fetched, cached: true}, true
}

// load runs one loader and publishes its outcome. Publication is deferred so
// a loader that panics — a hook, a token provider — still releases the key
// and wakes its waiters (with an error) before the panic continues; without
// that the key would stay in flight forever and every later caller would
// wait on it.
func (c *ttlCache[K, V]) load(ctx context.Context, key K, pending *ttlLoad, loader func(context.Context) (V, error)) (loaded V, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("basecamp: cache loader panicked: %v", r)
			c.publish(key, pending, loaded, err, ctx.Err() != nil)
			panic(r)
		}
		c.publish(key, pending, loaded, err, ctx.Err() != nil)
	}()
	return loader(ctx)
}

func (c *ttlCache[K, V]) publish(key K, pending *ttlLoad, loaded V, err error, callerDone bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.inflight, key)
	pending.err = err
	pending.callerDone = err != nil && callerDone
	if err == nil {
		pending.fetched = c.now()
		c.entries[key] = &ttlEntry[V]{value: loaded, fetched: pending.fetched}
	}
	close(pending.done)
}

// campfireIndex holds the two discovery sources. It lives on Client (shared
// by every AccountClient the Client hands out); a Client is bound to one
// credential, so entries are never shared across authorization contexts, and
// every key carries the account id.
type campfireIndex struct {
	docks    *ttlCache[campfireBucketKey, []int64]
	listings *ttlCache[string, map[int64][]int64]
}

type campfireBucketKey struct {
	accountID string
	bucketID  int64
}

func newCampfireIndex() *campfireIndex {
	return newCampfireIndexAt(time.Now)
}

func newCampfireIndexAt(now func() time.Time) *campfireIndex {
	return &campfireIndex{
		docks:    newTTLCache[campfireBucketKey, []int64](now, CampfireIndexTTL, campfireIndexMinRefresh),
		listings: newTTLCache[string, map[int64][]int64](now, CampfireIndexTTL, campfireIndexMinRefresh),
	}
}

// sourceRead is one consultation of a discovery source: the candidate ids
// it holds for the bucket, when that snapshot was fetched, and whether it
// predated the call.
type sourceRead struct {
	ids     []int64
	fetched time.Time
	cached  bool
}

// dockCampfires returns the Campfire ids a bucket's project dock names. A
// bucket that is not a project (a 404 on the project read) has none; any
// other failure of the read is returned.
func (ix *campfireIndex) dockCampfires(ctx context.Context, ac *AccountClient, bucketID int64, refresh bool) (sourceRead, error) {
	key := campfireBucketKey{accountID: ac.accountID, bucketID: bucketID}
	hit, err := ix.docks.get(ctx, key, refresh, func(ctx context.Context) ([]int64, error) {
		project, err := ac.Projects().Get(ctx, bucketID)
		if err != nil {
			if apiErr, ok := errors.AsType[*Error](err); ok && apiErr.Code == CodeNotFound {
				return nil, nil
			}
			return nil, err
		}
		var ids []int64
		for _, item := range project.Dock {
			if item.Name == "chat" && item.ID != 0 {
				ids = append(ids, item.ID)
			}
		}
		return ids, nil
	})
	if err != nil {
		return sourceRead{}, err
	}
	return sourceRead{ids: hit.value, fetched: hit.fetched, cached: hit.cached}, nil
}

// cachedListedCampfires returns the Campfire ids the cached account-wide
// listing shows in a bucket, without fetching: false when the listing is not
// cached or has expired.
func (ix *campfireIndex) cachedListedCampfires(accountID string, bucketID int64) (sourceRead, bool) {
	hit, ok := ix.listings.peek(accountID)
	if !ok {
		return sourceRead{}, false
	}
	return sourceRead{ids: append([]int64(nil), hit.value[bucketID]...), fetched: hit.fetched, cached: true}, true
}

// listedCampfires returns the Campfire ids the account-wide listing shows in
// a bucket. A listing that overflows MaxCampfireListing is not cached and is
// reported as ErrCampfireDiscoveryIncomplete by the caller.
func (ix *campfireIndex) listedCampfires(ctx context.Context, ac *AccountClient, bucketID int64, refresh bool) (sourceRead, error) {
	hit, err := ix.listings.get(ctx, ac.accountID, refresh, func(ctx context.Context) (map[int64][]int64, error) {
		list, err := ac.Campfires().List(ctx, &CampfireListOptions{Limit: MaxCampfireListing})
		if err != nil {
			return nil, err
		}
		if list.Meta.Truncated {
			return nil, errCampfireListingOverflow
		}
		byBucket := map[int64][]int64{}
		for _, c := range list.Campfires {
			if c.Bucket == nil || c.Bucket.ID == 0 {
				continue
			}
			byBucket[c.Bucket.ID] = append(byBucket[c.Bucket.ID], c.ID)
		}
		return byBucket, nil
	})
	if err != nil {
		return sourceRead{}, err
	}
	return sourceRead{ids: append([]int64(nil), hit.value[bucketID]...), fetched: hit.fetched, cached: hit.cached}, nil
}

// errCampfireListingOverflow is the load error for a listing past its cap;
// resolveChatLine turns it into the typed CampfireDiscoveryIncompleteError.
var errCampfireListingOverflow = errors.New("campfire listing exceeds MaxCampfireListing")

// chatLineSearch is one Summarize call's discovery state.
type chatLineSearch struct {
	svc      *RecordingsService
	bucketID int64
	lineID   int64
	tried    []int64 // candidates that answered 404, in order
	budget   int     // candidates still allowed
	skipped  bool    // a candidate was left untried for want of budget
}

// try reads the line under each candidate not yet tried. It returns the line
// and its Campfire on a hit; on a miss it returns nil with no error and
// records the candidates in tried. Any answer but 404 is returned as is.
func (s *chatLineSearch) try(ctx context.Context, candidates []int64) (*CampfireLine, int64, error) {
	for _, campfireID := range candidates {
		if containsID(s.tried, campfireID) {
			continue
		}
		if s.budget <= 0 {
			s.skipped = true
			return nil, 0, nil
		}
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		s.budget--
		line, err := s.svc.client.Campfires().GetLine(ctx, campfireID, s.lineID)
		if err == nil {
			return line, campfireID, nil
		}
		if apiErr, ok := errors.AsType[*Error](err); ok && apiErr.Code == CodeNotFound {
			s.tried = append(s.tried, campfireID)
			continue
		}
		return nil, 0, err
	}
	return nil, 0, nil
}

// resolveChatLine finds the Campfire a line lives in and reads it. See the
// discovery comment above for the contract.
func (s *RecordingsService) resolveChatLine(ctx context.Context, bucketID, lineID int64) (*CampfireLine, int64, error) {
	ac := s.client
	index := ac.parent.campfires()
	search := &chatLineSearch{svc: s, bucketID: bucketID, lineID: lineID, budget: MaxCampfireCandidates}
	incomplete := func(reason string) error {
		return &CampfireDiscoveryIncompleteError{BucketID: bucketID, RecordingID: lineID, Reason: reason}
	}

	// Pass 1: what the sources already hold — the dock (read if it must be),
	// then the listing only if it is cached. A listing fetch is the expensive,
	// slow request, and it is not made until the dock — including its refresh
	// — has had its say, so a listing that is down, over its cap, or stalled
	// on the context's deadline never stands between a project's line and
	// the one project read that finds it.
	dock, err := index.dockCampfires(ctx, ac, bucketID, false)
	if err != nil {
		return nil, 0, err
	}
	if line, id, err := search.try(ctx, dock.ids); err != nil || line != nil {
		return line, id, err
	}
	listed, listCached := index.cachedListedCampfires(ac.accountID, bucketID)
	if listCached {
		if line, id, err := search.try(ctx, listed.ids); err != nil || line != nil {
			return line, id, err
		}
	}

	// Pass 2: re-read the dock if it was served from cache (the floor may
	// decline), then fetch or refresh the listing. Whatever comes back is the
	// current snapshot of that source, whoever loaded it — another caller may
	// have populated or refreshed it in the meantime — so it always replaces
	// the pass-1 one; "refreshed" is whether a source the conclusion had
	// consulted is now newer than when it was consulted.
	refreshed := false
	if dock.cached {
		again, err := index.dockCampfires(ctx, ac, bucketID, true)
		if err != nil {
			return nil, 0, err
		}
		if again.fetched.After(dock.fetched) || !again.cached {
			refreshed = true
		}
		dock = again
		if line, id, err := search.try(ctx, dock.ids); err != nil || line != nil {
			return line, id, err
		}
	}
	again, err := index.listedCampfires(ctx, ac, bucketID, listCached)
	if err != nil {
		if errors.Is(err, errCampfireListingOverflow) {
			return nil, 0, incomplete(err.Error())
		}
		return nil, 0, err
	}
	if listCached && (again.fetched.After(listed.fetched) || !again.cached) {
		refreshed = true
	}
	listed = again
	if line, id, err := search.try(ctx, listed.ids); err != nil || line != nil {
		return line, id, err
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if search.skipped {
		return nil, 0, incomplete(fmt.Sprintf("more than %d visible campfires in the bucket", MaxCampfireCandidates))
	}
	unresolved := &UnresolvedRecordingError{BucketID: bucketID, RecordingID: lineID, CampfireIDs: search.tried, Refreshed: refreshed}
	if refreshed {
		for _, id := range search.tried {
			if !containsID(dock.ids, id) && !containsID(listed.ids, id) {
				unresolved.StaleCampfireIDs = append(unresolved.StaleCampfireIDs, id)
			}
		}
	}
	return nil, 0, unresolved
}

func containsID(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
