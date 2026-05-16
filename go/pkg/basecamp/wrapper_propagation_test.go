package basecamp

// Tests covering field propagation through the hand-written wrapper layer
// added by the BC5 readiness wrapper sweep (PR 6). Each test populates a
// fully-fleshed generated.X fixture, runs it through xFromGenerated, and
// asserts ONLY the newly-propagated fields are non-zero on the wrapper —
// avoids duplicating the existing *FromGenerated_FullPopulated tests.
//
// Three patterns:
//
//   * `Test<Type>FromGenerated_PropagatesNewFields` — for wrappers that
//     gained struct fields. Asserts each new field is populated.
//   * `TestPerson_NewFields_NestedPropagation` — end-to-end test that the
//     standardize-nested-Person refactor (commit 1) causes commit-3's
//     Person.Tagline (and other previously-dropped Person fields) to flow
//     through every nested context.
//   * `Test<Type>_DirectDecode_PropagatesNewFields` — for wrappers that
//     decode via json.Unmarshal on raw bytes (no FromGenerated function).
//     These take a JSON fixture and assert the new fields decode.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// -----------------------------------------------------------------------------
// Commit 1 — nested Person standardization
// -----------------------------------------------------------------------------

// TestPerson_NewFields_NestedPropagation exercises the change from inline
// 6-field shallow Person copies to personFromGenerated calls inside every
// *FromGenerated function. After standardization, a fully-populated nested
// generated.Person (Bio, Tagline, Location, Title, PersonableType,
// AttachableSGID, Client, Employee, TimeZone, CanPing, CanAccessHillCharts,
// CanAccessTimesheet, CanManageProjects, CanManagePeople, CreatedAt,
// UpdatedAt, Company) must populate the matching wrapper Person fields
// regardless of the parent context.
func TestPerson_NewFields_NestedPropagation(t *testing.T) {
	fullCreator := generated.Person{
		Id:                  42,
		Name:                "Edited Person",
		EmailAddress:        "p@example.com",
		AvatarUrl:           "https://example.com/avatar.png",
		Admin:               true,
		Owner:               true,
		AttachableSgid:      "sgid://bc3/Person/42",
		PersonableType:      "User",
		Title:               "Lead",
		Bio:                 "Bio text",
		Tagline:             "Tagline text",
		Location:            "Chicago",
		Client:              false,
		Employee:            true,
		TimeZone:            "America/Chicago",
		CanPing:             true,
		CanAccessHillCharts: true,
		CanAccessTimesheet:  true,
		CanManageProjects:   true,
		CanManagePeople:     true,
		CreatedAt:           time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt:           time.Date(2024, 1, 5, 12, 0, 0, 0, time.UTC),
		Company:             generated.PersonCompany{Id: 7, Name: "Acme Inc."},
	}

	// Exercise nested Person propagation through several wrappers that
	// previously inlined a 6-field shallow copy.
	t.Run("recording.Creator", func(t *testing.T) {
		gr := generated.Recording{Id: 1, Status: "active", Creator: fullCreator}
		w := recordingFromGenerated(gr)
		assertCreatorFullyPropagated(t, w.Creator, fullCreator)
	})

	t.Run("comment.Creator", func(t *testing.T) {
		gc := generated.Comment{Id: 1, Status: "active", Creator: fullCreator}
		w := commentFromGenerated(gc)
		assertCreatorFullyPropagated(t, w.Creator, fullCreator)
	})

	t.Run("message.Creator", func(t *testing.T) {
		gm := generated.Message{Id: 1, Status: "active", Creator: fullCreator}
		w := messageFromGenerated(gm)
		assertCreatorFullyPropagated(t, w.Creator, fullCreator)
	})

	t.Run("todo.Creator", func(t *testing.T) {
		gt := generated.Todo{Id: 1, Status: "active", Creator: fullCreator}
		w := todoFromGenerated(gt)
		assertCreatorFullyPropagated(t, w.Creator, fullCreator)
	})

	t.Run("schedule_entry.Creator + Participants", func(t *testing.T) {
		ge := generated.ScheduleEntry{
			Id:           1,
			Status:       "active",
			Creator:      fullCreator,
			Participants: []generated.Person{fullCreator},
		}
		w := scheduleEntryFromGenerated(ge)
		assertCreatorFullyPropagated(t, w.Creator, fullCreator)
		if len(w.Participants) != 1 {
			t.Fatalf("expected 1 participant, got %d", len(w.Participants))
		}
		assertCreatorFullyPropagated(t, &w.Participants[0], fullCreator)
	})

	t.Run("card.Creator + Completer + Assignees", func(t *testing.T) {
		gc := generated.Card{
			Id:        1,
			Status:    "active",
			Creator:   fullCreator,
			Completer: fullCreator,
			Assignees: []generated.Person{fullCreator},
		}
		w := cardFromGenerated(gc)
		assertCreatorFullyPropagated(t, w.Creator, fullCreator)
		assertCreatorFullyPropagated(t, w.Completer, fullCreator)
		if len(w.Assignees) != 1 {
			t.Fatalf("expected 1 assignee, got %d", len(w.Assignees))
		}
		assertCreatorFullyPropagated(t, &w.Assignees[0], fullCreator)
	})
}

func assertCreatorFullyPropagated(t *testing.T, p *Person, gp generated.Person) {
	t.Helper()
	if p == nil {
		t.Fatal("expected wrapper Person to be non-nil")
	}
	if p.ID != int64(gp.Id) {
		t.Errorf("ID: got %d, want %d", p.ID, int64(gp.Id))
	}
	if p.Name != gp.Name {
		t.Errorf("Name: got %q, want %q", p.Name, gp.Name)
	}
	if p.EmailAddress != gp.EmailAddress {
		t.Errorf("EmailAddress: got %q, want %q", p.EmailAddress, gp.EmailAddress)
	}
	if p.AvatarURL != gp.AvatarUrl {
		t.Errorf("AvatarURL: got %q, want %q", p.AvatarURL, gp.AvatarUrl)
	}
	if p.Admin != gp.Admin {
		t.Errorf("Admin: got %v, want %v", p.Admin, gp.Admin)
	}
	if p.Owner != gp.Owner {
		t.Errorf("Owner: got %v, want %v", p.Owner, gp.Owner)
	}
	// Fields that the old shallow copy silently dropped.
	if p.AttachableSGID != gp.AttachableSgid {
		t.Errorf("AttachableSGID: got %q, want %q", p.AttachableSGID, gp.AttachableSgid)
	}
	if p.PersonableType != gp.PersonableType {
		t.Errorf("PersonableType: got %q, want %q", p.PersonableType, gp.PersonableType)
	}
	if p.Title != gp.Title {
		t.Errorf("Title: got %q, want %q", p.Title, gp.Title)
	}
	if p.Bio != gp.Bio {
		t.Errorf("Bio: got %q, want %q", p.Bio, gp.Bio)
	}
	if p.Tagline != gp.Tagline {
		t.Errorf("Tagline (BC5): got %q, want %q", p.Tagline, gp.Tagline)
	}
	if p.Location != gp.Location {
		t.Errorf("Location: got %q, want %q", p.Location, gp.Location)
	}
	if p.Client != gp.Client {
		t.Errorf("Client: got %v, want %v", p.Client, gp.Client)
	}
	if p.Employee != gp.Employee {
		t.Errorf("Employee: got %v, want %v", p.Employee, gp.Employee)
	}
	if p.TimeZone != gp.TimeZone {
		t.Errorf("TimeZone: got %q, want %q", p.TimeZone, gp.TimeZone)
	}
	if p.CanPing != gp.CanPing {
		t.Errorf("CanPing: got %v, want %v", p.CanPing, gp.CanPing)
	}
	if p.CanAccessHillCharts != gp.CanAccessHillCharts {
		t.Errorf("CanAccessHillCharts: got %v, want %v", p.CanAccessHillCharts, gp.CanAccessHillCharts)
	}
	if p.CanAccessTimesheet != gp.CanAccessTimesheet {
		t.Errorf("CanAccessTimesheet: got %v, want %v", p.CanAccessTimesheet, gp.CanAccessTimesheet)
	}
	if p.CanManageProjects != gp.CanManageProjects {
		t.Errorf("CanManageProjects: got %v, want %v", p.CanManageProjects, gp.CanManageProjects)
	}
	if p.CanManagePeople != gp.CanManagePeople {
		t.Errorf("CanManagePeople: got %v, want %v", p.CanManagePeople, gp.CanManagePeople)
	}
	if p.Company == nil {
		t.Error("Company: expected non-nil")
	} else {
		if p.Company.ID != gp.Company.Id {
			t.Errorf("Company.ID: got %d, want %d", p.Company.ID, gp.Company.Id)
		}
		if p.Company.Name != gp.Company.Name {
			t.Errorf("Company.Name: got %q, want %q", p.Company.Name, gp.Company.Name)
		}
	}
	if p.CreatedAt == "" {
		t.Error("CreatedAt: expected non-empty string from non-zero time")
	}
	if p.UpdatedAt == "" {
		t.Error("UpdatedAt: expected non-empty string from non-zero time")
	}
}

// -----------------------------------------------------------------------------
// Commit 3 — BC5 forward-compat fields (Todo, Todoset)
// -----------------------------------------------------------------------------

func TestTodoFromGenerated_PropagatesNewFields(t *testing.T) {
	gt := generated.Todo{
		Id:               42,
		Status:           "active",
		Title:            "T",
		VisibleToClients: true,
		BoostsCount:      3,
		BoostsUrl:        "https://example.com/boosts",
		CommentsCount:    5,
		CommentsUrl:      "https://example.com/comments",
		CompletionUrl:    "https://example.com/completion",
		SubscriptionUrl:  "https://example.com/subscription",
		CompletionSubscribers: []generated.Person{
			{Id: 7, Name: "Subscriber"},
		},
		Steps: []generated.CardStep{
			{Id: 99, Title: "Step 1", Status: "active"},
			{Id: 100, Title: "Step 2", Status: "active"},
		},
	}
	w := todoFromGenerated(gt)

	if !w.VisibleToClients {
		t.Error("VisibleToClients: expected true")
	}
	if w.BoostsCount != 3 {
		t.Errorf("BoostsCount: got %d, want 3", w.BoostsCount)
	}
	if w.BoostsURL != "https://example.com/boosts" {
		t.Errorf("BoostsURL: got %q", w.BoostsURL)
	}
	if w.CommentsCount != 5 {
		t.Errorf("CommentsCount: got %d, want 5", w.CommentsCount)
	}
	if w.CommentsURL != "https://example.com/comments" {
		t.Errorf("CommentsURL: got %q", w.CommentsURL)
	}
	if w.CompletionURL != "https://example.com/completion" {
		t.Errorf("CompletionURL: got %q", w.CompletionURL)
	}
	if w.SubscriptionURL != "https://example.com/subscription" {
		t.Errorf("SubscriptionURL: got %q", w.SubscriptionURL)
	}
	if len(w.CompletionSubscribers) != 1 || w.CompletionSubscribers[0].Name != "Subscriber" {
		t.Errorf("CompletionSubscribers: got %+v", w.CompletionSubscribers)
	}
	if len(w.Steps) != 2 {
		t.Fatalf("Steps (BC5): got %d, want 2", len(w.Steps))
	}
	if w.Steps[0].Title != "Step 1" || w.Steps[1].Title != "Step 2" {
		t.Errorf("Steps (BC5) titles: got %q, %q", w.Steps[0].Title, w.Steps[1].Title)
	}
}

func TestTodosetFromGenerated_PropagatesNewFields(t *testing.T) {
	gts := generated.Todoset{
		Id:                       42,
		Status:                   "active",
		Title:                    "T",
		TodosCount:               17,
		CompletedLooseTodosCount: 3,
		TodosUrl:                 "https://example.com/todos",
		AppTodosUrl:              "https://example.com/app/todos",
	}
	w := todosetFromGenerated(gts)
	if w.TodosCount != 17 {
		t.Errorf("TodosCount: got %d, want 17", w.TodosCount)
	}
	if w.CompletedLooseTodosCount != 3 {
		t.Errorf("CompletedLooseTodosCount: got %d, want 3", w.CompletedLooseTodosCount)
	}
	if w.TodosURL != "https://example.com/todos" {
		t.Errorf("TodosURL: got %q", w.TodosURL)
	}
	if w.AppTodosURL != "https://example.com/app/todos" {
		t.Errorf("AppTodosURL: got %q", w.AppTodosURL)
	}
}

// -----------------------------------------------------------------------------
// Commit 2 — cross-cutting Recording-shaped fields
// -----------------------------------------------------------------------------

func TestRecordingFromGenerated_PropagatesNewFields(t *testing.T) {
	gr := generated.Recording{
		Id:              42,
		Status:          "active",
		Title:           "R",
		Content:         "the content",
		CommentsCount:   9,
		CommentsUrl:     "https://example.com/comments",
		SubscriptionUrl: "https://example.com/sub",
	}
	w := recordingFromGenerated(gr)
	if w.Content != "the content" {
		t.Errorf("Content: got %q", w.Content)
	}
	if w.CommentsCount != 9 {
		t.Errorf("CommentsCount: got %d", w.CommentsCount)
	}
	if w.CommentsURL != "https://example.com/comments" {
		t.Errorf("CommentsURL: got %q", w.CommentsURL)
	}
	if w.SubscriptionURL != "https://example.com/sub" {
		t.Errorf("SubscriptionURL: got %q", w.SubscriptionURL)
	}
}

func TestCommentFromGenerated_PropagatesNewFields(t *testing.T) {
	gc := generated.Comment{
		Id:               42,
		Status:           "active",
		Title:            "C",
		VisibleToClients: true,
		InheritsStatus:   true,
		BookmarkUrl:      "https://example.com/bm",
		BoostsCount:      3,
		BoostsUrl:        "https://example.com/boosts",
	}
	w := commentFromGenerated(gc)
	if !w.VisibleToClients {
		t.Error("VisibleToClients")
	}
	if w.Title != "C" {
		t.Errorf("Title: got %q", w.Title)
	}
	if !w.InheritsStatus {
		t.Error("InheritsStatus")
	}
	if w.BookmarkURL != "https://example.com/bm" {
		t.Errorf("BookmarkURL: got %q", w.BookmarkURL)
	}
	if w.BoostsCount != 3 {
		t.Errorf("BoostsCount: got %d", w.BoostsCount)
	}
	if w.BoostsURL != "https://example.com/boosts" {
		t.Errorf("BoostsURL: got %q", w.BoostsURL)
	}
}

func TestMessageFromGenerated_PropagatesNewFields(t *testing.T) {
	gm := generated.Message{
		Id:               42,
		Status:           "active",
		Title:            "M",
		VisibleToClients: true,
		InheritsStatus:   true,
		BookmarkUrl:      "https://example.com/bm",
		BoostsUrl:        "https://example.com/boosts",
		CommentsCount:    7,
		CommentsUrl:      "https://example.com/comments",
		SubscriptionUrl:  "https://example.com/sub",
	}
	w := messageFromGenerated(gm)
	if !w.VisibleToClients || w.Title != "M" || !w.InheritsStatus {
		t.Errorf("base recording-shaped fields not propagated: %+v", w)
	}
	if w.BookmarkURL == "" || w.BoostsURL == "" || w.CommentsURL == "" || w.SubscriptionURL == "" {
		t.Errorf("URL fields not propagated: %+v", w)
	}
	if w.CommentsCount != 7 {
		t.Errorf("CommentsCount: got %d", w.CommentsCount)
	}
}

func TestMessageBoardFromGenerated_PropagatesNewFields(t *testing.T) {
	gb := generated.MessageBoard{
		Id:               42,
		Status:           "active",
		Title:            "MB",
		VisibleToClients: true,
		InheritsStatus:   true,
		BookmarkUrl:      "https://example.com/bm",
		AppMessagesUrl:   "https://example.com/app/messages",
		Position:         3,
	}
	w := messageBoardFromGenerated(gb)
	if !w.VisibleToClients || !w.InheritsStatus {
		t.Errorf("flags: %+v", w)
	}
	if w.BookmarkURL != "https://example.com/bm" {
		t.Errorf("BookmarkURL: got %q", w.BookmarkURL)
	}
	if w.AppMessagesURL != "https://example.com/app/messages" {
		t.Errorf("AppMessagesURL: got %q", w.AppMessagesURL)
	}
	if w.Position != 3 {
		t.Errorf("Position: got %d", w.Position)
	}
}

func TestForwardFromGenerated_PropagatesNewFields(t *testing.T) {
	gf := generated.Forward{
		Id:               42,
		Status:           "active",
		Title:            "F",
		Subject:          "subj",
		VisibleToClients: true,
		InheritsStatus:   true,
		BookmarkUrl:      "https://example.com/bm",
		SubscriptionUrl:  "https://example.com/sub",
		RepliesCount:     5,
		RepliesUrl:       "https://example.com/replies",
	}
	w := forwardFromGenerated(gf)
	if !w.VisibleToClients || !w.InheritsStatus || w.Title != "F" {
		t.Errorf("recording-shaped flags: %+v", w)
	}
	if w.BookmarkURL == "" || w.SubscriptionURL == "" {
		t.Errorf("URLs not propagated: %+v", w)
	}
	if w.RepliesCount != 5 || w.RepliesURL == "" {
		t.Errorf("Replies: count=%d url=%q", w.RepliesCount, w.RepliesURL)
	}
}

func TestForwardReplyFromGenerated_PropagatesNewFields(t *testing.T) {
	gr := generated.ForwardReply{
		Id:               42,
		Status:           "active",
		Title:            "FR",
		Content:          "x",
		VisibleToClients: true,
		InheritsStatus:   true,
		BookmarkUrl:      "https://example.com/bm",
		BoostsCount:      3,
		BoostsUrl:        "https://example.com/boosts",
	}
	w := forwardReplyFromGenerated(gr)
	if !w.VisibleToClients || !w.InheritsStatus || w.Title != "FR" {
		t.Errorf("flags: %+v", w)
	}
	if w.BookmarkURL == "" || w.BoostsURL == "" {
		t.Errorf("URLs not propagated: %+v", w)
	}
	if w.BoostsCount != 3 {
		t.Errorf("BoostsCount: %d", w.BoostsCount)
	}
}

func TestCampfireLineFromGenerated_PropagatesNewFields(t *testing.T) {
	gl := generated.CampfireLine{
		Id:          42,
		Status:      "active",
		Title:       "L",
		BookmarkUrl: "https://example.com/bm",
		BoostsCount: 3,
		BoostsUrl:   "https://example.com/boosts",
	}
	w := campfireLineFromGenerated(gl)
	if w.BookmarkURL == "" || w.BoostsURL == "" {
		t.Errorf("URLs: %+v", w)
	}
	if w.BoostsCount != 3 {
		t.Errorf("BoostsCount: %d", w.BoostsCount)
	}
}

func TestScheduleEntryFromGenerated_PropagatesNewFields(t *testing.T) {
	ge := generated.ScheduleEntry{
		Id:          42,
		Status:      "active",
		Summary:     "s",
		BoostsCount: 3,
		BoostsUrl:   "https://example.com/boosts",
	}
	w := scheduleEntryFromGenerated(ge)
	if w.BoostsCount != 3 || w.BoostsURL == "" {
		t.Errorf("Boosts: %+v", w)
	}
}

func TestQuestionAnswerFromGenerated_PropagatesNewFields(t *testing.T) {
	ga := generated.QuestionAnswer{
		Id:          42,
		Status:      "active",
		Title:       "A",
		Content:     "x",
		BoostsCount: 3,
		BoostsUrl:   "https://example.com/boosts",
	}
	w := questionAnswerFromGenerated(ga)
	if w.BoostsCount != 3 || w.BoostsURL == "" {
		t.Errorf("Boosts: %+v", w)
	}
}

func TestTodolistFromGenerated_PropagatesNewFields(t *testing.T) {
	gtl := generated.Todolist{
		Id:          42,
		Status:      "active",
		Title:       "TL",
		BoostsCount: 3,
		BoostsUrl:   "https://example.com/boosts",
	}
	w := todolistFromGenerated(gtl)
	if w.BoostsCount != 3 || w.BoostsURL == "" {
		t.Errorf("Boosts: %+v", w)
	}
}

func TestTimesheetEntryFromGenerated_PropagatesNewFields(t *testing.T) {
	ge := generated.TimesheetEntry{
		Id:               42,
		Date:             "2024-01-15",
		Hours:            "1.5",
		Status:           "active",
		Title:            "TE",
		Type:             "Timesheets::Entry",
		Url:              "https://example.com/u",
		AppUrl:           "https://example.com/au",
		BookmarkUrl:      "https://example.com/bm",
		VisibleToClients: true,
		InheritsStatus:   true,
	}
	w := timesheetEntryFromGenerated(ge)
	if w.Status != "active" || w.Title != "TE" || w.Type == "" || w.URL == "" || w.AppURL == "" || w.BookmarkURL == "" {
		t.Errorf("Recording-shape fields not propagated: %+v", w)
	}
	if !w.VisibleToClients || !w.InheritsStatus {
		t.Errorf("Flags not propagated: %+v", w)
	}
}

func TestInboxFromGenerated_PropagatesNewFields(t *testing.T) {
	gi := generated.Inbox{
		Id:               42,
		Status:           "active",
		Title:            "IB",
		Position:         5,
		VisibleToClients: true,
		InheritsStatus:   true,
		BookmarkUrl:      "https://example.com/bm",
		ForwardsCount:    12,
		ForwardsUrl:      "https://example.com/forwards",
	}
	w := inboxFromGenerated(gi)
	if !w.VisibleToClients || !w.InheritsStatus {
		t.Errorf("flags: %+v", w)
	}
	if w.Position != 5 || w.BookmarkURL == "" || w.ForwardsCount != 12 || w.ForwardsURL == "" {
		t.Errorf("new fields: %+v", w)
	}
}

// -----------------------------------------------------------------------------
// Commit 4 — one-off structural fields
// -----------------------------------------------------------------------------

func TestCampfireFromGenerated_PropagatesNewFields(t *testing.T) {
	gc := generated.Campfire{
		Id:              42,
		Status:          "active",
		Title:           "C",
		Topic:           "fellowship of the campfire",
		Position:        3,
		BookmarkUrl:     "https://example.com/bm",
		SubscriptionUrl: "https://example.com/sub",
	}
	w := campfireFromGenerated(gc)
	if w.Topic != "fellowship of the campfire" {
		t.Errorf("Topic: got %q", w.Topic)
	}
	if w.Position != 3 {
		t.Errorf("Position: got %d", w.Position)
	}
	if w.BookmarkURL == "" || w.SubscriptionURL == "" {
		t.Errorf("URLs: %+v", w)
	}
}

func TestTemplateFromGenerated_PropagatesNewFields(t *testing.T) {
	gt := generated.Template{
		Id:     42,
		Name:   "T",
		Url:    "https://example.com/u",
		AppUrl: "https://example.com/au",
		Dock: []generated.DockItem{
			{Id: 1, Title: "Chat", Name: "campfire", Enabled: true, Url: "u1", AppUrl: "au1", Position: 0},
			{Id: 2, Title: "Docs", Name: "vault", Enabled: false, Url: "u2", AppUrl: "au2", Position: 1},
		},
	}
	w := templateFromGenerated(gt)
	if w.URL != "https://example.com/u" || w.AppURL != "https://example.com/au" {
		t.Errorf("URLs: %+v", w)
	}
	if len(w.Dock) != 2 {
		t.Fatalf("Dock len: got %d", len(w.Dock))
	}
	if w.Dock[0].Title != "Chat" || w.Dock[0].Name != "campfire" || !w.Dock[0].Enabled {
		t.Errorf("Dock[0]: %+v", w.Dock[0])
	}
	if w.Dock[1].Title != "Docs" || w.Dock[1].Name != "vault" || w.Dock[1].Enabled {
		t.Errorf("Dock[1]: %+v", w.Dock[1])
	}
}

// -----------------------------------------------------------------------------
// Direct-decode wrappers — JSON fixture tests
// -----------------------------------------------------------------------------

func TestNotification_DirectDecode_PropagatesNewFields(t *testing.T) {
	// BC5 forward-compat fields BubbleUpURL + BubbleUpAt plus the
	// one-off Creator + Participants + PreviewableAttachments fields
	// (added in commit 4).
	raw := []byte(`{
		"id": 1,
		"title": "Bubble",
		"created_at": "2024-01-01T00:00:00Z",
		"updated_at": "2024-01-02T00:00:00Z",
		"bubble_up_url": "https://example.com/bubble",
		"bubble_up_at": "2024-02-01T08:00:00Z",
		"creator": {
			"id": 7,
			"name": "Author",
			"personable_type": "User"
		},
		"participants": [
			{"id": 8, "name": "P1", "personable_type": "User"},
			{"id": 9, "name": "P2", "personable_type": "User"}
		],
		"previewable_attachments": [
			{"id": 100, "url": "https://example.com/u", "app_url": "https://example.com/au", "content_type": "image/png", "filename": "img.png", "filesize": 1234, "width": 100, "height": 200}
		]
	}`)

	var n Notification
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if n.BubbleUpURL != "https://example.com/bubble" {
		t.Errorf("BubbleUpURL: got %q", n.BubbleUpURL)
	}
	if n.BubbleUpAt.IsZero() {
		t.Error("BubbleUpAt: expected non-zero")
	}
	if n.Creator == nil || n.Creator.Name != "Author" {
		t.Errorf("Creator: %+v", n.Creator)
	}
	if len(n.Participants) != 2 {
		t.Errorf("Participants: got %d", len(n.Participants))
	}
	if len(n.PreviewableAttachments) != 1 {
		t.Fatalf("PreviewableAttachments: got %d", len(n.PreviewableAttachments))
	}
	pa := n.PreviewableAttachments[0]
	if pa.ID == nil || *pa.ID != 100 || pa.URL == "" || pa.ContentType == "" {
		t.Errorf("PreviewableAttachment[0]: %+v", pa)
	}
}

func TestNotificationsResult_DirectDecode_PropagatesNewFields(t *testing.T) {
	// BC5 forward-compat fields BubbleUps + ScheduledBubbleUps on the
	// envelope.
	raw := []byte(`{
		"unreads": [],
		"reads": [],
		"memories": [],
		"bubble_ups": [
			{"id": 1, "title": "Today", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-02T00:00:00Z"}
		],
		"scheduled_bubble_ups": [
			{"id": 2, "title": "Future", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-02T00:00:00Z", "bubble_up_at": "2024-02-01T08:00:00Z"}
		]
	}`)

	var r NotificationsResult
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(r.BubbleUps) != 1 || r.BubbleUps[0].Title != "Today" {
		t.Errorf("BubbleUps: %+v", r.BubbleUps)
	}
	if len(r.ScheduledBubbleUps) != 1 || r.ScheduledBubbleUps[0].Title != "Future" {
		t.Errorf("ScheduledBubbleUps: %+v", r.ScheduledBubbleUps)
	}
	if r.ScheduledBubbleUps[0].BubbleUpAt.IsZero() {
		t.Error("ScheduledBubbleUps[0].BubbleUpAt: expected non-zero")
	}
}

func TestMyAssignment_DirectDecode_PropagatesNewFields(t *testing.T) {
	// MyAssignment is direct-decode. Its fields predate this PR but the
	// drift check exercises the contract; this test pins the JSON shape.
	raw := []byte(`{
		"id": 1,
		"type": "Todo",
		"content": "Do thing",
		"completed": false,
		"due_on": "2024-02-15",
		"starts_on": "2024-02-01",
		"comments_count": 3,
		"app_url": "https://example.com/app/1",
		"assignees": [{"id": 7, "name": "A", "avatar_url": "u"}],
		"bucket": {"id": 9, "name": "Project", "app_url": "au"},
		"parent": {"id": 11, "title": "List", "app_url": "au"}
	}`)
	var a MyAssignment
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if a.ID != 1 || a.Type != "Todo" || a.Content != "Do thing" || a.CommentsCount != 3 || a.AppURL == "" {
		t.Errorf("base fields: %+v", a)
	}
	if a.DueOn != "2024-02-15" || a.StartsOn != "2024-02-01" {
		t.Errorf("dates: %+v", a)
	}
	if len(a.Assignees) != 1 || a.Assignees[0].Name != "A" {
		t.Errorf("Assignees: %+v", a.Assignees)
	}
}

func TestGauge_DirectDecode_PropagatesNewFields(t *testing.T) {
	raw := []byte(`{
		"id": 1,
		"title": "G",
		"creator": {"id": 7, "name": "Author"},
		"bucket": {"id": 9, "name": "Project", "type": "Project"}
	}`)
	var g Gauge
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if g.Creator == nil || g.Creator.Name != "Author" {
		t.Errorf("Creator: %+v", g.Creator)
	}
	if g.Bucket == nil || g.Bucket.Name != "Project" {
		t.Errorf("Bucket: %+v", g.Bucket)
	}
}

func TestGaugeNeedle_DirectDecode_PropagatesNewFields(t *testing.T) {
	raw := []byte(`{
		"id": 1,
		"title": "GN",
		"creator": {"id": 7, "name": "Author"},
		"bucket": {"id": 9, "name": "Project", "type": "Project"},
		"parent": {"id": 11, "title": "G", "type": "Gauge", "url": "u", "app_url": "au"}
	}`)
	var n GaugeNeedle
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if n.Creator == nil || n.Creator.Name != "Author" {
		t.Errorf("Creator: %+v", n.Creator)
	}
	if n.Bucket == nil || n.Bucket.Name != "Project" {
		t.Errorf("Bucket: %+v", n.Bucket)
	}
	if n.Parent == nil || n.Parent.Title != "G" {
		t.Errorf("Parent: %+v", n.Parent)
	}
}

// -----------------------------------------------------------------------------
// types helper (silences unused-import lint when go test rebuilds)
// -----------------------------------------------------------------------------

var _ = types.Date{}
