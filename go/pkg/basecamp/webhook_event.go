package basecamp

// WebhookEvent is the payload delivered by Basecamp webhooks.
type WebhookEvent struct {
	ID        int64                 `json:"id"`
	Kind      string                `json:"kind"`
	Details   any                   `json:"details"`
	CreatedAt string                `json:"created_at"`
	Recording WebhookEventRecording `json:"recording"`
	Creator   WebhookEventPerson    `json:"creator"`
	Copy      *WebhookCopy          `json:"copy,omitempty"`
}

// WebhookEventRecording is the recording included in webhook event payloads.
// It has additional fields beyond what the API returns for recordings.
type WebhookEventRecording struct {
	ID               int64               `json:"id"`
	Status           string              `json:"status"`
	VisibleToClients bool                `json:"visible_to_clients"`
	CreatedAt        string              `json:"created_at"`
	UpdatedAt        string              `json:"updated_at"`
	Title            string              `json:"title"`
	InheritsStatus   bool                `json:"inherits_status"`
	Type             string              `json:"type"`
	URL              string              `json:"url"`
	AppURL           string              `json:"app_url"`
	BookmarkURL      string              `json:"bookmark_url"`
	Content          string              `json:"content"`
	CommentsCount    int                 `json:"comments_count"`
	CommentsURL      string              `json:"comments_url"`
	SubscriptionURL  string              `json:"subscription_url"`
	Parent           *WebhookEventParent `json:"parent,omitempty"`
	Bucket           *WebhookEventBucket `json:"bucket,omitempty"`
	Creator          *WebhookEventPerson `json:"creator,omitempty"`
}

// WebhookEventPerson is a person in webhook event payloads.
type WebhookEventPerson struct {
	ID                  int64                `json:"id"`
	AttachableSGID      string               `json:"attachable_sgid"`
	Name                string               `json:"name"`
	EmailAddress        string               `json:"email_address"`
	PersonableType      string               `json:"personable_type"`
	Title               string               `json:"title"`
	Bio                 *string              `json:"bio"`
	Location            *string              `json:"location"`
	CreatedAt           string               `json:"created_at"`
	UpdatedAt           string               `json:"updated_at"`
	Admin               bool                 `json:"admin"`
	Owner               bool                 `json:"owner"`
	Client              bool                 `json:"client"`
	Employee            bool                 `json:"employee"`
	TimeZone            string               `json:"time_zone"`
	AvatarURL           string               `json:"avatar_url"`
	Company             *WebhookEventCompany `json:"company,omitempty"`
	CanManageProjects   bool                 `json:"can_manage_projects"`
	CanManagePeople     bool                 `json:"can_manage_people"`
	CanPing             bool                 `json:"can_ping"`
	CanAccessTimesheet  bool                 `json:"can_access_timesheet"`
	CanAccessHillCharts bool                 `json:"can_access_hill_charts"`
}

// WebhookEventCompany is a company embedded in a webhook event person.
type WebhookEventCompany struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// WebhookEventParent is the parent recording in webhook event payloads.
type WebhookEventParent struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	URL    string `json:"url"`
	AppURL string `json:"app_url"`
}

// WebhookEventBucket is the bucket (project) in webhook event payloads.
type WebhookEventBucket struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// WebhookCopy contains copy/move reference data.
type WebhookCopy struct {
	ID     int64             `json:"id"`
	URL    string            `json:"url"`
	AppURL string            `json:"app_url"`
	Bucket WebhookCopyBucket `json:"bucket"`
}

// WebhookCopyBucket is the bucket reference in a webhook copy.
type WebhookCopyBucket struct {
	ID int64 `json:"id"`
}

// Known webhook recording types (convenience constants, not exhaustive).
const (
	WebhookTypeCheckinReply   = "Checkin::Reply"
	WebhookTypeCloudFile      = "CloudFile"
	WebhookTypeComment        = "Comment"
	WebhookTypeDocument       = "Document"
	WebhookTypeForwardReply   = "Forward::Reply"
	WebhookTypeGoogleDocument = "GoogleDocument"
	WebhookTypeInboxForward   = "Inbox::Forward"
	WebhookTypeMessage        = "Message"
	WebhookTypeQuestion       = "Question"
	WebhookTypeQuestionAnswer = "Question::Answer"
	WebhookTypeScheduleEntry  = "Schedule::Entry"
	WebhookTypeTodo           = "Todo"
	WebhookTypeTodolist       = "Todolist"
	WebhookTypeTodolistGroup  = "Todolist::Group"
	WebhookTypeUpload         = "Upload"
	WebhookTypeVault          = "Vault"
)

// ParseEventKind splits a webhook event kind string into its type and action components.
// For example, "todo_created" returns ("todo", "created").
// For compound types like "question_answer_created", it returns ("question_answer", "created").
func ParseEventKind(kind string) (recordingType, action string) {
	// The action is always the last underscore-separated segment.
	for i := len(kind) - 1; i >= 0; i-- {
		if kind[i] == '_' {
			return kind[:i], kind[i+1:]
		}
	}
	return kind, ""
}
