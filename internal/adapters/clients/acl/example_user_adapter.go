package acl

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jsamuelsen/go-service-template/internal/adapters/clients"
	"github.com/jsamuelsen/go-service-template/internal/domain"
)

// User represents a user in the domain.
// This is a domain type - it uses domain conventions and types.
// Note: In a real application, this would likely be defined in the domain package.
type User struct {
	ID        string
	FullName  string
	Email     string
	IsActive  bool
	CreatedAt time.Time
}

// UserServiceAdapter is an ACL adapter for an external user service.
// It demonstrates the complete ACL pattern:
//   - Embeds BaseAdapter for common functionality
//   - Uses unexported DTOs for external responses
//   - Validates input before making requests
//   - Translates external responses to domain types
//   - Maps errors to domain errors
type UserServiceAdapter struct {
	BaseAdapter
}

// NewUserServiceAdapter creates a new user service adapter.
func NewUserServiceAdapter(client *clients.Client) *UserServiceAdapter {
	return &UserServiceAdapter{
		BaseAdapter: NewBaseAdapter(client, "user-service"),
	}
}

// externalUserResponse is the DTO returned by the external user service.
// This is unexported to prevent leaking into the domain.
type externalUserResponse struct {
	ID        string `json:"id"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	Status    int    `json:"status"`     // 1=active, 2=inactive, 3=suspended
	CreatedAt string `json:"created_at"` // ISO 8601 format
}

// externalUserListResponse wraps a list of users from the external service.
type externalUserListResponse struct {
	Users      []externalUserResponse `json:"users"`
	TotalCount int                    `json:"total_count"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
}

// User status codes from the external service.
const (
	externalStatusActive    = 1
	externalStatusInactive  = 2
	externalStatusSuspended = 3
)

// GetByID retrieves a user by their ID.
// Returns domain.ErrNotFound if the user does not exist.
func (a *UserServiceAdapter) GetByID(ctx context.Context, id string) (*User, error) {
	// 1. Validate input before making external call
	if err := ValidateRequired(id, "id"); err != nil {
		return nil, err
	}

	// 2. Make request via BaseAdapter (handles error mapping)
	path := fmt.Sprintf("/api/v1/users/%s", id)
	body, err := a.Get(ctx, path, "get user", id)
	if err != nil {
		return nil, err
	}

	// 3. Decode external response using domain-aware decoder
	extUser, err := DecodeResponseForService[externalUserResponse](body, a.ServiceName())
	if err != nil {
		return nil, err
	}

	// 4. Translate to domain type
	return a.translateUser(extUser)
}

// GetByEmail retrieves a user by their email address.
// Returns domain.ErrNotFound if the user does not exist.
func (a *UserServiceAdapter) GetByEmail(ctx context.Context, email string) (*User, error) {
	if err := ValidateRequired(email, "email"); err != nil {
		return nil, err
	}

	// URL-escape the email to handle special characters like @, +, %, /
	escapedEmail := url.PathEscape(email)
	path := fmt.Sprintf("/api/v1/users/by-email/%s", escapedEmail)
	body, err := a.Get(ctx, path, "get user by email", email)
	if err != nil {
		return nil, err
	}

	extUser, err := DecodeResponseForService[externalUserResponse](body, a.ServiceName())
	if err != nil {
		return nil, err
	}

	return a.translateUser(extUser)
}

// List retrieves a paginated list of users.
func (a *UserServiceAdapter) List(ctx context.Context, page, pageSize int) ([]*User, int, error) {
	// Validate pagination parameters
	if err := ValidatePositive(page, "page"); err != nil {
		return nil, 0, err
	}

	if err := ValidatePositive(pageSize, "pageSize"); err != nil {
		return nil, 0, err
	}

	path := fmt.Sprintf("/api/v1/users?page=%d&page_size=%d", page, pageSize)
	// List operations don't have a single entityID, pass empty string
	body, err := a.Get(ctx, path, "list users", "")
	if err != nil {
		return nil, 0, err
	}

	listResp, err := DecodeResponseForService[externalUserListResponse](body, a.ServiceName())
	if err != nil {
		return nil, 0, err
	}

	// Translate all users using the helper
	users, err := TranslateSlice(listResp.Users, a.translateUser)
	if err != nil {
		return nil, 0, err
	}

	return users, listResp.TotalCount, nil
}

// translateUser converts an external user DTO to a domain User.
// This is the core translation function that validates external data
// and creates domain objects.
func (a *UserServiceAdapter) translateUser(ext *externalUserResponse) (*User, error) {
	// Validate required fields from external response
	if ext.ID == "" {
		return nil, domain.NewValidationError("id", "missing from external response")
	}

	if ext.FullName == "" {
		return nil, domain.NewValidationError("full_name", "missing from external response")
	}

	// Parse timestamps with graceful degradation
	var createdAt time.Time
	if ext.CreatedAt != "" {
		parsed, err := time.Parse(time.RFC3339, ext.CreatedAt)
		if err != nil {
			// Log the error but provide a sensible default
			// In production, you might want to log this
			createdAt = time.Time{}
		} else {
			createdAt = parsed
		}
	}

	// Translate status codes to domain representation
	isActive := ext.Status == externalStatusActive

	return &User{
		ID:        ext.ID,
		FullName:  ext.FullName,
		Email:     ext.Email,
		IsActive:  isActive,
		CreatedAt: createdAt,
	}, nil
}
