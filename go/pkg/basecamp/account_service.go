package basecamp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Account represents Basecamp account information.
type Account struct {
	ID           int64               `json:"id"`
	Name         string              `json:"name"`
	Active       bool                `json:"active,omitempty"`
	Frozen       bool                `json:"frozen,omitempty"`
	Paused       bool                `json:"paused,omitempty"`
	Trial        bool                `json:"trial,omitempty"`
	TrialEndsOn  string              `json:"trial_ends_on,omitempty"`
	Logo         string              `json:"logo,omitempty"`
	OwnerName    string              `json:"owner_name,omitempty"`
	Limits       AccountLimits       `json:"limits,omitempty"`
	Settings     AccountSettings     `json:"settings,omitempty"`
	Subscription AccountSubscription `json:"subscription,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

// AccountLimits represents account limits.
type AccountLimits struct {
	CanCreateProjects bool `json:"can_create_projects,omitempty"`
	CanCreateUsers    bool `json:"can_create_users,omitempty"`
	CanPinProjects    bool `json:"can_pin_projects,omitempty"`
	CanUploadFiles    bool `json:"can_upload_files,omitempty"`
}

// AccountSettings represents account settings.
type AccountSettings struct {
	CompanyHqEnabled bool `json:"company_hq_enabled,omitempty"`
	ProjectsEnabled  bool `json:"projects_enabled,omitempty"`
	TeamsEnabled     bool `json:"teams_enabled,omitempty"`
}

// AccountSubscription represents account subscription info.
type AccountSubscription struct {
	Clients      bool   `json:"clients,omitempty"`
	Logo         bool   `json:"logo,omitempty"`
	ProjectLimit int32  `json:"project_limit,omitempty"`
	ProperName   string `json:"proper_name,omitempty"`
	ShortName    string `json:"short_name,omitempty"`
	Teams        bool   `json:"teams,omitempty"`
	Templates    bool   `json:"templates,omitempty"`
	Timesheet    bool   `json:"timesheet,omitempty"`
}

// AccountService handles account operations.
type AccountService struct {
	client *AccountClient
}

// NewAccountService creates a new AccountService.
func NewAccountService(client *AccountClient) *AccountService {
	return &AccountService{client: client}
}

// GetAccount returns the account information.
func (s *AccountService) GetAccount(ctx context.Context) (result *Account, err error) {
	op := OperationInfo{
		Service: "Account", Operation: "GetAccount",
		ResourceType: "account", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetAccountWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var acct Account
	if err = json.Unmarshal(resp.Body, &acct); err != nil {
		return nil, fmt.Errorf("failed to parse account: %w", err)
	}

	return &acct, nil
}

// UpdateName renames the account.
func (s *AccountService) UpdateName(ctx context.Context, name string) (result *Account, err error) {
	op := OperationInfo{
		Service: "Account", Operation: "UpdateName",
		ResourceType: "account", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if name == "" {
		err = ErrUsage("account name is required")
		return nil, err
	}

	body := generated.UpdateAccountNameJSONRequestBody{
		Name: name,
	}

	resp, err := s.client.parent.gen.UpdateAccountNameWithResponse(ctx, s.client.accountID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var acct Account
	if err = json.Unmarshal(resp.Body, &acct); err != nil {
		return nil, fmt.Errorf("failed to parse account: %w", err)
	}

	return &acct, nil
}

// RemoveLogo removes the account logo.
func (s *AccountService) RemoveLogo(ctx context.Context) (err error) {
	op := OperationInfo{
		Service: "Account", Operation: "RemoveLogo",
		ResourceType: "account", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.RemoveAccountLogoWithResponse(ctx, s.client.accountID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// UpdateLogo uploads or replaces the account logo.
// logo is the image data, filename is the file name (e.g., "logo.png"),
// and contentType is the MIME type (e.g., "image/png").
// Accepted formats: PNG, JPEG, GIF, WebP, AVIF, HEIC. Maximum 5 MB.
func (s *AccountService) UpdateLogo(ctx context.Context, logo io.Reader, filename, contentType string) (err error) {
	op := OperationInfo{
		Service: "Account", Operation: "UpdateLogo",
		ResourceType: "account", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if logo == nil {
		err = ErrUsage("logo reader is required")
		return err
	}
	if filename == "" {
		err = ErrUsage("filename is required")
		return err
	}
	if contentType == "" {
		err = ErrUsage("content type is required")
		return err
	}

	// Build multipart/form-data body
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	partHeader := make(textproto.MIMEHeader)
	safeFilename := strings.NewReplacer("\r", "", "\n", "", `\`, `\\`, `"`, `\"`).Replace(filename)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="logo"; filename="%s"`, safeFilename))
	safeContentType := strings.NewReplacer("\r", "", "\n", "").Replace(contentType)
	partHeader.Set("Content-Type", safeContentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err = io.Copy(part, logo); err != nil {
		return fmt.Errorf("failed to write logo data: %w", err)
	}
	if err = writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Delegate to the generated client — retry and auth are handled by the transport
	resp, err := s.client.parent.gen.UpdateAccountLogoWithBodyWithResponse(
		ctx,
		s.client.accountID,
		writer.FormDataContentType(),
		&buf,
	)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}
