package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
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
	CreatedAt    string              `json:"created_at,omitempty"`
	UpdatedAt    string              `json:"updated_at,omitempty"`
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
