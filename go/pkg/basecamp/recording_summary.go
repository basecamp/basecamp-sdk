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
	// of the Campfires visible in its bucket. It is distinct from a failed read:
	// every candidate answered 404, the candidate list was refreshed, and the
	// line is still not there. A consumer marks the record blocked and retries
	// on its own schedule; the line may be in a Campfire the caller cannot see.
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
// as that error, and the loop stops there); ErrBucketMismatch when the read
// returned a recording from another bucket.
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
// and the line read is /chats/{campfireId}/lines/{lineId}. The candidates are
// the Campfires the caller can see in that bucket, read off the account-wide
// Campfire listing (there is no per-bucket one) and cached — per account, ten
// minutes — so a burst of chat lines costs one listing, not one per line. The
// loop tries the line under each candidate until one answers.
//
// Two failure shapes are kept apart on purpose. A candidate that answers
// anything but 404 — 401, 403, 5xx, a network error, a cancelled context —
// stops the loop and is returned as that error: the read failed, and trying
// the next Campfire would only hide it. A 404 means "not here", so the loop
// moves on. Only when every candidate said "not here" is the line unresolved
// (ErrRecordingUnresolved) — and before concluding that, the listing is
// refreshed once, so a Campfire created after the cache filled is tried too.

const (
	// CampfireIndexTTL is how long the per-account Campfire listing is reused
	// for chat line discovery before it is listed again.
	CampfireIndexTTL = 10 * time.Minute

	// campfireIndexMinRefresh bounds the refresh-on-miss: a line found under
	// no candidate re-lists the account's Campfires, but not more often than
	// this, so a run of unresolvable lines cannot turn into a listing per line.
	campfireIndexMinRefresh = 30 * time.Second

	// MaxCampfireCandidates bounds how many Campfires in one bucket the
	// discovery loop tries. A project has one Campfire and a handful of pings;
	// a bucket past this bound is not a shape BC3 produces, and an unbounded
	// loop over a hostile listing is the failure the bound prevents.
	MaxCampfireCandidates = 50
)

// campfireIndex is the cached account-wide Campfire listing, keyed by bucket.
// It lives on Client (shared by every AccountClient the Client hands out) and
// is keyed by account id; a Client is bound to one credential, so entries are
// never shared across authorization contexts.
type campfireIndex struct {
	mu       sync.Mutex
	now      func() time.Time
	entries  map[string]*campfireIndexEntry
	inflight map[string]*campfireListing
}

type campfireIndexEntry struct {
	byBucket map[int64][]int64 // campfire ids in listing order
	fetched  time.Time
}

// campfireListing is one in-progress listing; concurrent callers for the same
// account wait on done and read err rather than listing again themselves.
type campfireListing struct {
	done chan struct{}
	err  error
}

func newCampfireIndex() *campfireIndex {
	return &campfireIndex{
		now:      time.Now,
		entries:  map[string]*campfireIndexEntry{},
		inflight: map[string]*campfireListing{},
	}
}

// candidates returns the Campfire ids visible in bucketID, and whether they
// came from the cache. With refresh set, a listing younger than
// campfireIndexMinRefresh is still reused; anything older is re-listed.
func (ix *campfireIndex) candidates(ctx context.Context, ac *AccountClient, bucketID int64, refresh bool) (ids []int64, cached bool, err error) {
	for {
		ix.mu.Lock()
		if entry := ix.entries[ac.accountID]; entry != nil {
			age := ix.now().Sub(entry.fetched)
			if age < CampfireIndexTTL && (!refresh || age < campfireIndexMinRefresh) {
				ids = append([]int64(nil), entry.byBucket[bucketID]...)
				ix.mu.Unlock()
				return ids, true, nil
			}
		}
		if pending := ix.inflight[ac.accountID]; pending != nil {
			ix.mu.Unlock()
			select {
			case <-pending.done:
			case <-ctx.Done():
				return nil, false, ctx.Err()
			}
			if pending.err != nil {
				return nil, false, pending.err
			}
			continue // the entry is now fresh; read it under the lock
		}
		listing := &campfireListing{done: make(chan struct{})}
		ix.inflight[ac.accountID] = listing
		ix.mu.Unlock()

		byBucket, listErr := listCampfiresByBucket(ctx, ac)

		ix.mu.Lock()
		delete(ix.inflight, ac.accountID)
		listing.err = listErr
		if listErr == nil {
			// A failed listing leaves the previous entry in place: a transient
			// failure must not evict a usable index.
			ix.entries[ac.accountID] = &campfireIndexEntry{byBucket: byBucket, fetched: ix.now()}
			ids = append([]int64(nil), byBucket[bucketID]...)
		}
		close(listing.done)
		ix.mu.Unlock()
		if listErr != nil {
			return nil, false, listErr
		}
		return ids, false, nil
	}
}

func listCampfiresByBucket(ctx context.Context, ac *AccountClient) (map[int64][]int64, error) {
	list, err := ac.Campfires().List(ctx, &CampfireListOptions{})
	if err != nil {
		return nil, err
	}
	byBucket := map[int64][]int64{}
	for _, c := range list.Campfires {
		if c.Bucket == nil || c.Bucket.ID == 0 {
			continue
		}
		byBucket[c.Bucket.ID] = append(byBucket[c.Bucket.ID], c.ID)
	}
	return byBucket, nil
}

// resolveChatLine finds the Campfire a line lives in and reads it. See the
// discovery comment above for the loop's contract.
func (s *RecordingsService) resolveChatLine(ctx context.Context, bucketID, lineID int64) (*CampfireLine, int64, error) {
	ac := s.client
	index := ac.parent.campfires()
	candidates, cached, err := index.candidates(ctx, ac, bucketID, false)
	if err != nil {
		return nil, 0, err
	}
	tried := make([]int64, 0, len(candidates))
	line, campfireID, err := s.tryCampfires(ctx, candidates, lineID, &tried)
	if err != nil || line != nil {
		return line, campfireID, err
	}
	if cached {
		// Every cached candidate said "not here". The listing may predate the
		// line's Campfire; refresh once and try only what is new.
		fresh, _, err := index.candidates(ctx, ac, bucketID, true)
		if err != nil {
			return nil, 0, err
		}
		var untried []int64
		for _, id := range fresh {
			if !containsID(tried, id) {
				untried = append(untried, id)
			}
		}
		line, campfireID, err = s.tryCampfires(ctx, untried, lineID, &tried)
		if err != nil || line != nil {
			return line, campfireID, err
		}
	}
	return nil, 0, &UnresolvedRecordingError{BucketID: bucketID, RecordingID: lineID, CampfireIDs: tried}
}

// tryCampfires reads lineID under each candidate in order. A 404 appends the
// candidate to tried and moves on; any other error returns at once.
func (s *RecordingsService) tryCampfires(ctx context.Context, candidates []int64, lineID int64, tried *[]int64) (*CampfireLine, int64, error) {
	if len(candidates) > MaxCampfireCandidates {
		candidates = candidates[:MaxCampfireCandidates]
	}
	for _, campfireID := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		line, err := s.client.Campfires().GetLine(ctx, campfireID, lineID)
		if err == nil {
			return line, campfireID, nil
		}
		if apiErr, ok := errors.AsType[*Error](err); ok && apiErr.Code == CodeNotFound {
			*tried = append(*tried, campfireID)
			continue
		}
		return nil, 0, err
	}
	return nil, 0, nil
}

func containsID(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
