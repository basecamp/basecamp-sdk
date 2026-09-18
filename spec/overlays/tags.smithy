$version: "2"
namespace basecamp

use smithy.api#tags

// Projects
apply ListProjects @tags(["Projects"])
apply GetProject @tags(["Projects"])
apply CreateProject @tags(["Projects"])
apply UpdateProject @tags(["Projects"])
apply TrashProject @tags(["Projects"])
apply ArchiveProject @tags(["Projects"])
apply UnarchiveProject @tags(["Projects"])
apply ListRecentProjects @tags(["Projects"])
apply RecordProjectVisit @tags(["Projects"])

// Todos (includes Todolists, TodolistGroups, Todosets)
apply ListTodos @tags(["Todos"])
apply GetTodo @tags(["Todos"])
apply CreateTodo @tags(["Todos"])
apply CreateTodosetTodo @tags(["Todos"])
apply ReplaceTodo @tags(["Todos"])
apply CompleteTodo @tags(["Todos"])
apply UncompleteTodo @tags(["Todos"])
apply RepositionTodo @tags(["Todos"])
apply GetTodoset @tags(["Todos"])
apply GetHillChart @tags(["Todos"])
apply UpdateHillChartSettings @tags(["Todos"])
apply ListTodolists @tags(["Todos"])
apply GetTodolistOrGroup @tags(["Todos"])
apply CreateTodolist @tags(["Todos"])
apply UpdateTodolistOrGroup @tags(["Todos"])
apply RepositionTodolist @tags(["Todos"])
apply ListTodolistGroups @tags(["Todos"])
apply CreateTodolistGroup @tags(["Todos"])
apply RepositionTodolistGroup @tags(["Todos"])

// Messages (includes Comments, MessageBoards, MessageTypes)
apply ListComments @tags(["Messages"])
apply GetComment @tags(["Messages"])
apply CreateComment @tags(["Messages"])
apply UpdateComment @tags(["Messages"])
apply ListMessages @tags(["Messages"])
apply GetMessage @tags(["Messages"])
apply CreateMessage @tags(["Messages"])
apply UpdateMessage @tags(["Messages"])
apply PinMessage @tags(["Messages"])
apply UnpinMessage @tags(["Messages"])
apply GetMessageBoard @tags(["Messages"])
apply ListMessageTypes @tags(["Messages"])
apply GetMessageType @tags(["Messages"])
apply CreateMessageType @tags(["Messages"])
apply UpdateMessageType @tags(["Messages"])
apply DeleteMessageType @tags(["Messages"])

// Files (Vaults, Documents, Uploads, Cloud files, Google documents, Attachments)
apply ListVaults @tags(["Files"])
apply GetVault @tags(["Files"])
apply CreateVault @tags(["Files"])
apply UpdateVault @tags(["Files"])
apply ListDocuments @tags(["Files"])
apply GetDocument @tags(["Files"])
apply CreateDocument @tags(["Files"])
apply ReplaceDocument @tags(["Files"])
apply ListUploads @tags(["Files"])
apply GetUpload @tags(["Files"])
apply CreateUpload @tags(["Files"])
apply UpdateUpload @tags(["Files"])
apply ListUploadVersions @tags(["Files"])
apply CreateUploadVersion @tags(["Files"])
apply GetCloudFile @tags(["Files"])
apply CreateCloudFile @tags(["Files"])
apply UpdateCloudFile @tags(["Files"])
apply GetGoogleDocument @tags(["Files"])
apply CreateGoogleDocument @tags(["Files"])
apply UpdateGoogleDocument @tags(["Files"])
apply CreateAttachment @tags(["Files"])

// Schedule (Schedules, Timesheets)
apply GetSchedule @tags(["Schedule"])
apply UpdateScheduleSettings @tags(["Schedule"])
apply ListScheduleEntries @tags(["Schedule"])
apply GetScheduleEntry @tags(["Schedule"])
apply GetScheduleEntryOccurrence @tags(["Schedule"])
apply CreateScheduleEntry @tags(["Schedule"])
apply ReplaceScheduleEntry @tags(["Schedule"])
apply GetTimesheetReport @tags(["Schedule"])
apply GetProjectTimesheet @tags(["Schedule"])
apply GetRecordingTimesheet @tags(["Schedule"])
apply GetTimesheetEntry @tags(["Schedule"])
apply CreateTimesheetEntry @tags(["Schedule"])
apply UpdateTimesheetEntry @tags(["Schedule"])
apply DestroyTimesheetEntry @tags(["Schedule"])

// Campfire (Campfires, Chatbots)
apply ListCampfires @tags(["Campfire"])
apply GetCampfire @tags(["Campfire"])
apply ListCampfireLines @tags(["Campfire"])
apply GetCampfireLine @tags(["Campfire"])
apply CreateCampfireLine @tags(["Campfire"])
apply UpdateCampfireLine @tags(["Campfire"])
apply DeleteCampfireLine @tags(["Campfire"])
apply ListCampfireUploads @tags(["Campfire"])
apply CreateCampfireUpload @tags(["Campfire"])
apply ListChatbots @tags(["Campfire"])
apply GetChatbot @tags(["Campfire"])
apply CreateChatbot @tags(["Campfire"])
apply UpdateChatbot @tags(["Campfire"])
apply DeleteChatbot @tags(["Campfire"])

// Forwards (Email forwarding)
apply GetInbox @tags(["Forwards"])
apply ListForwards @tags(["Forwards"])
apply GetForward @tags(["Forwards"])
apply ListForwardReplies @tags(["Forwards"])
apply GetForwardReply @tags(["Forwards"])

// Card Tables (Cards, Columns, Steps)
apply GetCardTable @tags(["Card Tables"])
apply ListCards @tags(["Card Tables"])
apply GetCard @tags(["Card Tables"])
apply CreateCard @tags(["Card Tables"])
apply UpdateCard @tags(["Card Tables"])
apply MoveCard @tags(["Card Tables"])
apply GetCardColumn @tags(["Card Tables"])
apply CreateCardColumn @tags(["Card Tables"])
apply UpdateCardColumn @tags(["Card Tables"])
apply MoveCardColumn @tags(["Card Tables"])
apply SetCardColumnColor @tags(["Card Tables"])
apply EnableCardColumnOnHold @tags(["Card Tables"])
apply DisableCardColumnOnHold @tags(["Card Tables"])
apply SubscribeToCardColumn @tags(["Card Tables"])
apply UnsubscribeFromCardColumn @tags(["Card Tables"])
apply GetCardStep @tags(["Card Tables"])
apply CreateCardStep @tags(["Card Tables"])
apply UpdateCardStep @tags(["Card Tables"])
apply SetCardStepCompletion @tags(["Card Tables"])
apply RepositionCardStep @tags(["Card Tables"])
apply CreateWormhole @tags(["Card Tables"])
apply UpdateWormhole @tags(["Card Tables"])
apply DeleteWormhole @tags(["Card Tables"])

// People (People, Subscriptions)
apply ListPeople @tags(["People"])
apply GetPerson @tags(["People"])
apply GetMyProfile @tags(["People"])
apply ListProjectPeople @tags(["People"])
apply ListPingablePeople @tags(["People"])
apply ListAssignablePeople @tags(["People"])
apply UpdateProjectAccess @tags(["People"])
apply UpdateProjectClientAccess @tags(["People"])
apply EnableProjectClients @tags(["People"])
apply DisableProjectClients @tags(["People"])
apply GetSubscription @tags(["People"])
apply Subscribe @tags(["People"])
apply Unsubscribe @tags(["People"])
apply UpdateSubscription @tags(["People"])

// ClientFeatures
apply ListClientApprovals @tags(["ClientFeatures"])
apply GetClientApproval @tags(["ClientFeatures"])
apply ListClientCorrespondences @tags(["ClientFeatures"])
apply GetClientCorrespondence @tags(["ClientFeatures"])
apply ListClientReplies @tags(["ClientFeatures"])
apply GetClientReply @tags(["ClientFeatures"])
apply SetClientVisibility @tags(["ClientFeatures"])

// Webhooks
//
// A per-project subscription: register a callback URL, say which event types it
// wants, read back what is registered. The one surface here that automation is
// actually built on, which is why it is named for the thing rather than for the
// category the thing belongs to.
apply ListWebhooks @tags(["Webhooks"])
apply GetWebhook @tags(["Webhooks"])
apply CreateWebhook @tags(["Webhooks"])
apply UpdateWebhook @tags(["Webhooks"])
apply DeleteWebhook @tags(["Webhooks"])

// Search
//
// Account-wide query over recordings, plus the metadata describing what can be
// filtered on. A read surface, and the only one of the six that touches every
// other domain rather than owning a resource of its own.
apply Search @tags(["Search"])
apply GetSearchMetadata @tags(["Search"])

// Templates (project blueprints and the account template library)
//
// One domain, not two. A template is a stored project shape; a project
// construction is the asynchronous job that builds a project from one. The
// template library is the same act from a different source — the account's
// shared catalogue instead of a template you own — and a library copy is its
// construction. Splitting the library out would put the two halves of "make a
// new project from something stored" under different tools.
apply ListTemplates @tags(["Templates"])
apply GetTemplate @tags(["Templates"])
apply CreateTemplate @tags(["Templates"])
apply UpdateTemplate @tags(["Templates"])
apply DeleteTemplate @tags(["Templates"])
apply CreateProjectFromTemplate @tags(["Templates"])
apply GetProjectConstruction @tags(["Templates"])
apply GetTemplateLibrary @tags(["Templates"])
apply CreateTemplateLibraryCopy @tags(["Templates"])
apply GetTemplateLibraryCopy @tags(["Templates"])

// Dock (a project's tool strip)
//
// Which tools a project has, what they are called, what order they sit in, and
// whether they are turned on. Named for the dock rather than for the Tools
// service these route to: a consumer that builds one domain tool per tag would
// otherwise get a tool called "tools", and the dock is what the operations are
// about — the strip, not the message board or the to-do set behind each entry.
//
// EnableTool, DisableTool and RepositionTool sit under /recordings/{toolId}/,
// and the path label is toolId, not recordingId. #929 left them out of the
// Recordings retag for that reason; they are dock operations that share a
// prefix.
apply GetTool @tags(["Dock"])
apply CreateTool @tags(["Dock"])
apply UpdateTool @tags(["Dock"])
apply DeleteTool @tags(["Dock"])
apply EnableTool @tags(["Dock"])
apply DisableTool @tags(["Dock"])
apply RepositionTool @tags(["Dock"])

// Lineup (account-wide schedule markers)
//
// ListLineupMarkers is the one operation here whose SERVICE is not Lineup. It
// has always generated into AutomationService — it was the only operation the
// Automation tag reached without a SERVICE_SPLITS entry, so it fell through to
// the tag-derived service while its three siblings were split out. Retagging it
// Lineup without saying so would move a public method between services, so each
// tag-keyed generator now carries 'Lineup' => { 'Automation' => [...] } and the
// Rust table an explicit ListLineupMarkers = "Automation" row. The tag is gone;
// the service it named survives, holding exactly this one method, as it did
// before. Merging it into LineupService is a breaking change and a separate
// decision.
apply ListLineupMarkers @tags(["Lineup"])
apply CreateLineupMarker @tags(["Lineup"])
apply UpdateLineupMarker @tags(["Lineup"])
apply DeleteLineupMarker @tags(["Lineup"])

// Boosts
apply ListRecordingBoosts @tags(["Boosts"])
apply ListEventBoosts @tags(["Boosts"])
apply GetBoost @tags(["Boosts"])
apply CreateRecordingBoost @tags(["Boosts"])
apply CreateEventBoost @tags(["Boosts"])
apply DeleteBoost @tags(["Boosts"])

// Checkins (questionnaires, questions, answers)
//
// #922 created this tag for the six operations that had none and noted that
// consolidating the rest of the check-in family under it was a follow-up
// editorial call. This is that follow-up. The questionnaire/question/answer
// CRUD below already generated into the Checkins SERVICE alongside these six;
// only the tag disagreed, so the two halves of one domain were being offered
// to tag-keyed consumers as two.
apply GetQuestionnaire @tags(["Checkins"])
apply ListQuestions @tags(["Checkins"])
apply GetQuestion @tags(["Checkins"])
apply CreateQuestion @tags(["Checkins"])
apply UpdateQuestion @tags(["Checkins"])
apply ListAnswers @tags(["Checkins"])
apply GetAnswer @tags(["Checkins"])
apply CreateAnswer @tags(["Checkins"])
apply UpdateAnswer @tags(["Checkins"])
apply GetQuestionReminders @tags(["Checkins"])
apply ListQuestionAnswerers @tags(["Checkins"])
apply GetAnswersByPerson @tags(["Checkins"])
apply UpdateQuestionNotificationSettings @tags(["Checkins"])
apply PauseQuestion @tags(["Checkins"])
apply ResumeQuestion @tags(["Checkins"])

// Account
apply GetAccount @tags(["Account"])
apply UpdateAccountName @tags(["Account"])
apply UpdateAccountLogo @tags(["Account"])
apply RemoveAccountLogo @tags(["Account"])

// Gauges
apply ListGauges @tags(["Gauges"])
apply ListGaugeNeedles @tags(["Gauges"])
apply GetGaugeNeedle @tags(["Gauges"])
apply CreateGaugeNeedle @tags(["Gauges"])
apply UpdateGaugeNeedle @tags(["Gauges"])
apply DestroyGaugeNeedle @tags(["Gauges"])
apply ToggleGauge @tags(["Gauges"])

// My Assignments
apply GetMyAssignments @tags(["MyAssignments"])
apply PrioritizeAssignment @tags(["MyAssignments"])
apply DeprioritizeAssignment @tags(["MyAssignments"])
apply ReorderUpNext @tags(["MyAssignments"])
apply GetMyCompletedAssignments @tags(["MyAssignments"])
apply GetMyDueAssignments @tags(["MyAssignments"])

// Everything Aggregates (flat family)
apply GetEverythingMessages @tags(["Everything"])
apply GetEverythingComments @tags(["Everything"])
apply GetEverythingCheckins @tags(["Everything"])
apply GetEverythingForwards @tags(["Everything"])
apply GetEverythingFiles @tags(["Everything"])
apply GetEverythingOverdueTodos @tags(["Everything"])
apply GetEverythingOverdueCards @tags(["Everything"])
apply GetEverythingOpenTodos @tags(["Everything"])
apply GetEverythingCompletedTodos @tags(["Everything"])
apply GetEverythingUnassignedTodos @tags(["Everything"])
apply GetEverythingNoDueDateTodos @tags(["Everything"])
apply GetEverythingOpenCards @tags(["Everything"])
apply GetEverythingCompletedCards @tags(["Everything"])
apply GetEverythingUnassignedCards @tags(["Everything"])
apply GetEverythingNoDueDateCards @tags(["Everything"])
apply GetEverythingNotNowCards @tags(["Everything"])

// Reports (account-wide report reads). New domain tag mirroring the Reports
// service the SDK generators already emit for this family (each generator's
// SERVICE_SPLITS routes these here while they are untagged); tagging them keeps
// the generated grouping byte-identical and gives catalog.Load one tag per op.
apply GetProgressReport @tags(["Reports"])
apply GetUpcomingSchedule @tags(["Reports"])
apply GetAssignedTodos @tags(["Reports"])
apply GetOverdueTodos @tags(["Reports"])
apply GetPersonProgress @tags(["Reports"])

// Timeline (project/person activity feed). New domain tag mirroring the
// existing Timeline service.
apply GetProjectTimeline @tags(["Timeline"])

// My Notifications
apply GetMyNotifications @tags(["MyNotifications"])
apply GetBubbleUps @tags(["MyNotifications"])
apply GetCalendar @tags(["Calendars"])
apply UpdateCalendar @tags(["Calendars"])
apply GetMyNote @tags(["MyNotes"])
apply UpdateMyNote @tags(["MyNotes"])
apply ListMyDrafts @tags(["Drafts"])
apply ListMyBookmarks @tags(["Bookmarks"])
apply GetBookmark @tags(["Bookmarks"])
apply CreateBookmark @tags(["Bookmarks"])
apply DeleteBookmark @tags(["Bookmarks"])
apply CreateBubbleUp @tags(["BubbleUps"])
apply DeleteBubbleUp @tags(["BubbleUps"])
apply MarkAsRead @tags(["MyNotifications"])

// Out of Office
apply GetOutOfOffice @tags(["People"])
apply EnableOutOfOffice @tags(["People"])
apply DisableOutOfOffice @tags(["People"])

// People (Profile & Preferences)
apply UpdateMyProfile @tags(["People"])
apply GetMyPreferences @tags(["People"])
apply UpdateMyPreferences @tags(["People"])

// Folders (wire type "Stack")
apply ListFolders @tags(["Folders"])
apply GetFolder @tags(["Folders"])
apply CreateFolder @tags(["Folders"])
apply UpdateFolder @tags(["Folders"])
apply DeleteFolder @tags(["Folders"])

// Event Feed (account-wide feed, agent inbox, stream tickets; SPEC.md §23)
apply PollEvents @tags(["EventFeed"])
apply PollInbox @tags(["EventFeed"])
apply CreateStreamTicket @tags(["EventFeed"])

// Recordings (recording lifecycle: list, spotlight, trash, archive; and the
// recording's own event history).
//
// The six lifecycle operations came first (#922), and this paragraph is about
// them alone: their new domain tag mirrors the Recordings service every SDK
// generator already emits for exactly those six (each generator's
// SERVICE_SPLITS routed them under Automation -> Recordings while they were
// tagged Automation). They previously folded into the Automation domain, which
// left MCP catalog generation with no dedicated recordings tool. Recording
// boosts stay under Boosts and the recording timesheet stays under Schedule ->
// Timesheets, matching the SDK service groupings. Tagging those six Recordings
// keeps the generated grouping byte-identical and gives catalog.Load one tag
// per op.
//
// ListEvents is the same domain: GET /{accountId}/recordings/{recordingId}/
// events.json is a recording's own change history — the timeline of the very
// lifecycle transitions above — so it belongs with them rather than in the
// Automation catch-all. Unlike the six above it keeps a service of its own,
// Events, and the two kinds of generator reach that service differently. The
// five tag-keyed ones (ruby, python, typescript, swift, kotlin) look an
// operation up in SERVICE_SPLITS under its tag, so their 'Events' entry moved
// from the Automation key to a Recordings one; leaving it under Automation
// would make it unreachable and fold ListEvents into RecordingsService.
// Rust resolves names.toml's [operation_services] by operationId BEFORE
// consulting the tag, so its ListEvents = "Events" row is unchanged and stays
// load-bearing: the service it names differs from the tag both before and
// after this retag, which is the opposite of the six rows #928 removed for
// merely restating theirs.
//
// The URL prefix is not the argument and must not be read as one: sub-resources
// under /recordings/ are tagged by what they are, so the same prefix carries
// Bookmarks, Boosts, BubbleUps, ClientFeatures, Messages, People and Schedule
// operations, and ListEventBoosts (.../events/{eventId}/boosts.json) is Boosts
// even though it hangs off this very sub-resource. EnableTool, DisableTool and
// RepositionTool are not Recordings: their /{accountId}/recordings/{toolId}/
// position.json path takes a toolId, not a recordingId — they are dock-tool
// operations that happen to share the prefix, and they are tagged Dock.
apply ListRecordings @tags(["Recordings"])
apply SpotlightRecording @tags(["Recordings"])
apply UnspotlightRecording @tags(["Recordings"])
apply TrashRecording @tags(["Recordings"])
apply ArchiveRecording @tags(["Recordings"])
apply UnarchiveRecording @tags(["Recordings"])
apply ListEvents @tags(["Recordings"])
