package dto

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

// DefaultLimit is the default number of items per page.
const DefaultLimit = 20

// MaxLimit is the maximum allowed items per page.
const MaxLimit = 100

// Cursor errors.
var (
	// ErrInvalidCursor is returned when cursor decoding fails.
	ErrInvalidCursor = errors.New("invalid cursor")

	// ErrNoCursor indicates no cursor was provided (first page request).
	// This is not an error condition but signals the start of pagination.
	ErrNoCursor = errors.New("no cursor provided")
)

// PaginationRequest represents pagination parameters from the request.
type PaginationRequest struct {
	// Cursor is an opaque string from a previous response's NextCursor.
	Cursor string `form:"cursor"`

	// Limit is the maximum number of items to return (1-100, default 20).
	Limit int `form:"limit" validate:"omitempty,gte=1,lte=100"`
}

// GetLimit returns the limit with defaults applied.
func (p *PaginationRequest) GetLimit() int {
	if p.Limit <= 0 {
		return DefaultLimit
	}

	if p.Limit > MaxLimit {
		return MaxLimit
	}

	return p.Limit
}

// DecodeCursor decodes the cursor string into CursorData.
// Returns ErrNoCursor if cursor is empty (first page request).
func (p *PaginationRequest) DecodeCursor() (*CursorData, error) {
	if p.Cursor == "" {
		return nil, ErrNoCursor
	}

	return DecodeCursor(p.Cursor)
}

// PaginatedResponse is a generic paginated response structure.
type PaginatedResponse[T any] struct {
	// Items is the array of items for this page.
	Items []T `json:"items"`

	// NextCursor is the cursor to use for the next page.
	// Empty if there are no more items.
	NextCursor string `json:"nextCursor,omitempty"`

	// HasMore indicates whether there are more items after this page.
	HasMore bool `json:"hasMore"`
}

// NewPaginatedResponse creates a new paginated response.
// Pass limit+1 items to detect if there are more pages, then trim to limit.
func NewPaginatedResponse[T any](items []T, limit int, cursorBuilder func(T) *CursorData) *PaginatedResponse[T] {
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	var nextCursor string

	if hasMore && len(items) > 0 && cursorBuilder != nil {
		lastItem := items[len(items)-1]
		cursor := cursorBuilder(lastItem)
		nextCursor = EncodeCursor(cursor)
	}

	return &PaginatedResponse[T]{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}

// CursorData contains the data encoded in a pagination cursor.
// This structure allows stable, consistent pagination across sorted results.
type CursorData struct {
	// Field is the name of the sort field (e.g., "created_at", "name").
	Field string `json:"f"`

	// Value is the value of the sort field for cursor position.
	Value string `json:"v"`

	// ID is the unique identifier for tie-breaking when sort values are equal.
	ID string `json:"id"`
}

// EncodeCursor encodes cursor data to a base64 string.
func EncodeCursor(data *CursorData) string {
	if data == nil {
		return ""
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return ""
	}

	return base64.URLEncoding.EncodeToString(jsonBytes)
}

// DecodeCursor decodes a base64 cursor string to cursor data.
// Returns ErrNoCursor if the encoded string is empty.
func DecodeCursor(encoded string) (*CursorData, error) {
	if encoded == "" {
		return nil, ErrNoCursor
	}

	jsonBytes, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidCursor
	}

	var data CursorData

	err = json.Unmarshal(jsonBytes, &data)
	if err != nil {
		return nil, ErrInvalidCursor
	}

	return &data, nil
}

// NewCursor creates a new cursor from field, value, and ID.
func NewCursor(field, value, id string) *CursorData {
	return &CursorData{
		Field: field,
		Value: value,
		ID:    id,
	}
}

// EmptyPaginatedResponse returns an empty paginated response.
func EmptyPaginatedResponse[T any]() *PaginatedResponse[T] {
	return &PaginatedResponse[T]{
		Items:   []T{},
		HasMore: false,
	}
}
