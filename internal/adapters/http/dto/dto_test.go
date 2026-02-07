package dto

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/domain"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestNewErrorResponse tests creating a basic error response.
func TestNewErrorResponse(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		want    *ErrorResponse
	}{
		{
			name:    "basic error response",
			code:    ErrorCodeNotFound,
			message: "resource not found",
			want: &ErrorResponse{
				Error: ErrorDetail{
					Code:    ErrorCodeNotFound,
					Message: "resource not found",
				},
			},
		},
		{
			name:    "validation error response",
			code:    ErrorCodeValidation,
			message: "invalid input",
			want: &ErrorResponse{
				Error: ErrorDetail{
					Code:    ErrorCodeValidation,
					Message: "invalid input",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewErrorResponse(tt.code, tt.message)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNewErrorResponseWithDetails tests creating an error response with details.
func TestNewErrorResponseWithDetails(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		details map[string]string
		want    *ErrorResponse
	}{
		{
			name:    "error with details",
			code:    ErrorCodeValidation,
			message: "validation failed",
			details: map[string]string{
				"email": "must be a valid email",
				"name":  "this field is required",
			},
			want: &ErrorResponse{
				Error: ErrorDetail{
					Code:    ErrorCodeValidation,
					Message: "validation failed",
					Details: map[string]string{
						"email": "must be a valid email",
						"name":  "this field is required",
					},
				},
			},
		},
		{
			name:    "error with empty details",
			code:    ErrorCodeBadRequest,
			message: "bad request",
			details: map[string]string{},
			want: &ErrorResponse{
				Error: ErrorDetail{
					Code:    ErrorCodeBadRequest,
					Message: "bad request",
					Details: map[string]string{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewErrorResponseWithDetails(tt.code, tt.message, tt.details)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestWithTraceID tests adding trace ID to error response.
func TestWithTraceID(t *testing.T) {
	tests := []struct {
		name     string
		response *ErrorResponse
		traceID  string
		want     string
	}{
		{
			name:     "add trace ID",
			response: NewErrorResponse(ErrorCodeInternal, "internal error"),
			traceID:  "trace-123",
			want:     "trace-123",
		},
		{
			name:     "empty trace ID",
			response: NewErrorResponse(ErrorCodeNotFound, "not found"),
			traceID:  "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.WithTraceID(tt.traceID)
			assert.Equal(t, tt.want, got.TraceID)
			assert.Same(t, tt.response, got) // Should return same instance
		})
	}
}

// TestHTTPStatusFromCode tests mapping error codes to HTTP status codes.
func TestHTTPStatusFromCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want int
	}{
		{
			name: "not found",
			code: ErrorCodeNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "conflict",
			code: ErrorCodeConflict,
			want: http.StatusConflict,
		},
		{
			name: "validation error",
			code: ErrorCodeValidation,
			want: http.StatusBadRequest,
		},
		{
			name: "bad request",
			code: ErrorCodeBadRequest,
			want: http.StatusBadRequest,
		},
		{
			name: "forbidden",
			code: ErrorCodeForbidden,
			want: http.StatusForbidden,
		},
		{
			name: "unauthorized",
			code: ErrorCodeUnauthorized,
			want: http.StatusUnauthorized,
		},
		{
			name: "unavailable",
			code: ErrorCodeUnavailable,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "timeout",
			code: ErrorCodeTimeout,
			want: http.StatusGatewayTimeout,
		},
		{
			name: "internal error",
			code: ErrorCodeInternal,
			want: http.StatusInternalServerError,
		},
		{
			name: "unknown code defaults to internal error",
			code: "UNKNOWN_CODE",
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HTTPStatusFromCode(tt.code)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestGetTraceID tests extracting trace ID from gin context.
func TestGetTraceID(t *testing.T) {
	tests := []struct {
		name         string
		setupContext func(*gin.Context)
		want         string
	}{
		{
			name: "trace ID in context",
			setupContext: func(c *gin.Context) {
				c.Set("trace_id", "context-trace-123")
			},
			want: "context-trace-123",
		},
		{
			name: "trace ID in header",
			setupContext: func(c *gin.Context) {
				c.Request.Header.Set("X-Request-ID", "header-trace-456")
			},
			want: "header-trace-456",
		},
		{
			name: "trace ID in context takes precedence",
			setupContext: func(c *gin.Context) {
				c.Set("trace_id", "context-trace-123")
				c.Request.Header.Set("X-Request-ID", "header-trace-456")
			},
			want: "context-trace-123",
		},
		{
			name: "no trace ID",
			setupContext: func(c *gin.Context) {
				// No trace ID set
			},
			want: "",
		},
		{
			name: "trace ID in context but wrong type",
			setupContext: func(c *gin.Context) {
				c.Set("trace_id", 12345) // Not a string
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

			tt.setupContext(c)

			got := GetTraceID(c)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestHandleError tests error handling middleware.
func TestHandleError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		traceID        string
		wantStatus     int
		wantCode       string
		wantMessageKey string
	}{
		{
			name:           "not found error",
			err:            domain.NewNotFoundError("user", "123"),
			traceID:        "trace-123",
			wantStatus:     http.StatusNotFound,
			wantCode:       ErrorCodeNotFound,
			wantMessageKey: "user",
		},
		{
			name:           "conflict error",
			err:            domain.NewConflictError("email", "already exists"),
			traceID:        "trace-456",
			wantStatus:     http.StatusConflict,
			wantCode:       ErrorCodeConflict,
			wantMessageKey: "email",
		},
		{
			name:           "validation error",
			err:            domain.NewValidationError("text", "must not be empty"),
			traceID:        "trace-789",
			wantStatus:     http.StatusBadRequest,
			wantCode:       ErrorCodeValidation,
			wantMessageKey: "text",
		},
		{
			name:           "forbidden error",
			err:            domain.NewForbiddenError("delete", "insufficient permissions"),
			traceID:        "trace-abc",
			wantStatus:     http.StatusForbidden,
			wantCode:       ErrorCodeForbidden,
			wantMessageKey: "delete",
		},
		{
			name:           "unavailable error",
			err:            domain.NewUnavailableError("database", "connection failed"),
			traceID:        "trace-def",
			wantStatus:     http.StatusServiceUnavailable,
			wantCode:       ErrorCodeUnavailable,
			wantMessageKey: "temporarily unavailable",
		},
		{
			name:           "internal error",
			err:            errors.New("unexpected error"),
			traceID:        "trace-ghi",
			wantStatus:     http.StatusInternalServerError,
			wantCode:       ErrorCodeInternal,
			wantMessageKey: "internal error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Set("trace_id", tt.traceID)

			HandleError(c, tt.err)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.wantCode, response.Error.Code)
			assert.Contains(t, response.Error.Message, tt.wantMessageKey)
			assert.Equal(t, tt.traceID, response.TraceID)
		})
	}
}

// TestGetLimit tests pagination limit calculation.
func TestGetLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{
			name:  "zero returns default",
			limit: 0,
			want:  DefaultLimit,
		},
		{
			name:  "negative returns default",
			limit: -1,
			want:  DefaultLimit,
		},
		{
			name:  "valid limit",
			limit: 50,
			want:  50,
		},
		{
			name:  "over max returns max",
			limit: 150,
			want:  MaxLimit,
		},
		{
			name:  "max limit",
			limit: MaxLimit,
			want:  MaxLimit,
		},
		{
			name:  "one",
			limit: 1,
			want:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PaginationRequest{Limit: tt.limit}
			got := p.GetLimit()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPaginationRequestDecodeCursor tests cursor decoding from pagination request.
func TestPaginationRequestDecodeCursor(t *testing.T) {
	validCursor := NewCursor("created_at", "2024-01-01", "123")
	validEncoded := EncodeCursor(validCursor)

	tests := []struct {
		name       string
		cursor     string
		wantCursor *CursorData
		wantErr    error
	}{
		{
			name:       "empty cursor returns ErrNoCursor",
			cursor:     "",
			wantCursor: nil,
			wantErr:    ErrNoCursor,
		},
		{
			name:       "valid cursor",
			cursor:     validEncoded,
			wantCursor: validCursor,
			wantErr:    nil,
		},
		{
			name:       "invalid cursor",
			cursor:     "invalid-base64!",
			wantCursor: nil,
			wantErr:    ErrInvalidCursor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PaginationRequest{Cursor: tt.cursor}
			got, err := p.DecodeCursor()

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantCursor, got)
			}
		})
	}
}

// TestNewPaginatedResponse tests creating paginated responses.
func TestNewPaginatedResponse(t *testing.T) {
	type item struct {
		ID   string
		Name string
	}

	cursorBuilder := func(i item) *CursorData {
		return NewCursor("name", i.Name, i.ID)
	}

	tests := []struct {
		name          string
		items         []item
		limit         int
		cursorBuilder func(item) *CursorData
		wantItemCount int
		wantHasMore   bool
		wantCursor    bool
	}{
		{
			name: "items less than limit",
			items: []item{
				{ID: "1", Name: "Alice"},
				{ID: "2", Name: "Bob"},
			},
			limit:         3,
			cursorBuilder: cursorBuilder,
			wantItemCount: 2,
			wantHasMore:   false,
			wantCursor:    false,
		},
		{
			name: "items equal to limit",
			items: []item{
				{ID: "1", Name: "Alice"},
				{ID: "2", Name: "Bob"},
				{ID: "3", Name: "Charlie"},
			},
			limit:         3,
			cursorBuilder: cursorBuilder,
			wantItemCount: 3,
			wantHasMore:   false,
			wantCursor:    false,
		},
		{
			name: "items more than limit",
			items: []item{
				{ID: "1", Name: "Alice"},
				{ID: "2", Name: "Bob"},
				{ID: "3", Name: "Charlie"},
				{ID: "4", Name: "David"},
			},
			limit:         3,
			cursorBuilder: cursorBuilder,
			wantItemCount: 3,
			wantHasMore:   true,
			wantCursor:    true,
		},
		{
			name:          "empty items",
			items:         []item{},
			limit:         3,
			cursorBuilder: cursorBuilder,
			wantItemCount: 0,
			wantHasMore:   false,
			wantCursor:    false,
		},
		{
			name: "nil cursor builder",
			items: []item{
				{ID: "1", Name: "Alice"},
				{ID: "2", Name: "Bob"},
				{ID: "3", Name: "Charlie"},
				{ID: "4", Name: "David"},
			},
			limit:         3,
			cursorBuilder: nil,
			wantItemCount: 3,
			wantHasMore:   true,
			wantCursor:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPaginatedResponse(tt.items, tt.limit, tt.cursorBuilder)

			assert.Len(t, got.Items, tt.wantItemCount)
			assert.Equal(t, tt.wantHasMore, got.HasMore)

			if tt.wantCursor {
				assert.NotEmpty(t, got.NextCursor)
			} else {
				assert.Empty(t, got.NextCursor)
			}
		})
	}
}

// TestEncodeCursor tests cursor encoding.
func TestEncodeCursor(t *testing.T) {
	tests := []struct {
		name string
		data *CursorData
		want string
	}{
		{
			name: "nil cursor returns empty string",
			data: nil,
			want: "",
		},
		{
			name: "valid cursor",
			data: &CursorData{
				Field: "created_at",
				Value: "2024-01-01",
				ID:    "123",
			},
			want: func() string {
				jsonBytes, _ := json.Marshal(&CursorData{
					Field: "created_at",
					Value: "2024-01-01",
					ID:    "123",
				})
				return base64.URLEncoding.EncodeToString(jsonBytes)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeCursor(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDecodeCursor tests cursor decoding.
func TestDecodeCursor(t *testing.T) {
	validCursor := &CursorData{
		Field: "created_at",
		Value: "2024-01-01",
		ID:    "123",
	}
	validEncoded := EncodeCursor(validCursor)

	tests := []struct {
		name    string
		encoded string
		want    *CursorData
		wantErr error
	}{
		{
			name:    "empty string returns ErrNoCursor",
			encoded: "",
			want:    nil,
			wantErr: ErrNoCursor,
		},
		{
			name:    "valid cursor",
			encoded: validEncoded,
			want:    validCursor,
			wantErr: nil,
		},
		{
			name:    "invalid base64",
			encoded: "invalid-base64!",
			want:    nil,
			wantErr: ErrInvalidCursor,
		},
		{
			name:    "valid base64 but invalid JSON",
			encoded: base64.URLEncoding.EncodeToString([]byte("not json")),
			want:    nil,
			wantErr: ErrInvalidCursor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeCursor(tt.encoded)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestNewCursor tests cursor creation.
func TestNewCursor(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
		id    string
		want  *CursorData
	}{
		{
			name:  "create cursor",
			field: "created_at",
			value: "2024-01-01",
			id:    "123",
			want: &CursorData{
				Field: "created_at",
				Value: "2024-01-01",
				ID:    "123",
			},
		},
		{
			name:  "empty values",
			field: "",
			value: "",
			id:    "",
			want: &CursorData{
				Field: "",
				Value: "",
				ID:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCursor(tt.field, tt.value, tt.id)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEmptyPaginatedResponse tests empty response creation.
func TestEmptyPaginatedResponse(t *testing.T) {
	type item struct {
		ID string
	}

	got := EmptyPaginatedResponse[item]()

	assert.NotNil(t, got)
	assert.Empty(t, got.Items)
	assert.False(t, got.HasMore)
	assert.Empty(t, got.NextCursor)
}

// TestValidator tests validator singleton.
func TestValidator(t *testing.T) {
	v1 := Validator()
	v2 := Validator()

	assert.NotNil(t, v1)
	assert.Same(t, v1, v2) // Should be same instance (singleton)
}

// TestValidate tests struct validation.
func TestValidate(t *testing.T) {
	type testStruct struct {
		Name  string `validate:"required"`
		Email string `validate:"email"`
		Age   int    `validate:"gte=0,lte=120"`
	}

	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{
			name: "valid struct",
			input: &testStruct{
				Name:  "John",
				Email: "john@example.com",
				Age:   30,
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			input: &testStruct{
				Name:  "",
				Email: "john@example.com",
				Age:   30,
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			input: &testStruct{
				Name:  "John",
				Email: "not-an-email",
				Age:   30,
			},
			wantErr: true,
		},
		{
			name: "age out of range",
			input: &testStruct{
				Name:  "John",
				Email: "john@example.com",
				Age:   150,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrValidation)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestBindAndValidate tests JSON binding and validation.
func TestBindAndValidate(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"email"`
	}

	tests := []struct {
		name        string
		body        string
		contentType string
		wantErr     bool
		errType     error
	}{
		{
			name:        "valid JSON",
			body:        `{"name":"John","email":"john@example.com"}`,
			contentType: "application/json",
			wantErr:     false,
		},
		{
			name:        "invalid JSON",
			body:        `{invalid}`,
			contentType: "application/json",
			wantErr:     true,
			errType:     ErrBinding,
		},
		{
			name:        "validation fails",
			body:        `{"name":"","email":"john@example.com"}`,
			contentType: "application/json",
			wantErr:     true,
			errType:     ErrValidation,
		},
		{
			name:        "invalid email",
			body:        `{"name":"John","email":"not-an-email"}`,
			contentType: "application/json",
			wantErr:     true,
			errType:     ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", tt.contentType)

			var input testStruct
			err := BindAndValidate(c, &input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, "John", input.Name)
				assert.Equal(t, "john@example.com", input.Email)
			}
		})
	}
}

// TestBindQueryAndValidate tests query binding and validation.
func TestBindQueryAndValidate(t *testing.T) {
	type queryStruct struct {
		Limit  int    `form:"limit" validate:"omitempty,gte=1,lte=100"`
		Cursor string `form:"cursor"`
	}

	tests := []struct {
		name    string
		query   string
		wantErr bool
		errType error
	}{
		{
			name:    "valid query",
			query:   "?limit=10&cursor=abc",
			wantErr: false,
		},
		{
			name:    "empty query",
			query:   "",
			wantErr: false,
		},
		{
			name:    "limit out of range",
			query:   "?limit=150",
			wantErr: true,
			errType: ErrValidation,
		},
		{
			name:    "negative limit",
			query:   "?limit=-1",
			wantErr: true,
			errType: ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/path"+tt.query, nil)

			var input queryStruct
			err := BindQueryAndValidate(c, &input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidationErrors tests extracting field errors.
func TestValidationErrors(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"email"`
		Age   int    `json:"age" validate:"gte=0,lte=120"`
	}

	tests := []struct {
		name      string
		input     *testStruct
		wantCount int
		wantKeys  []string
	}{
		{
			name: "multiple validation errors",
			input: &testStruct{
				Name:  "",
				Email: "not-an-email",
				Age:   150,
			},
			wantCount: 3,
			wantKeys:  []string{"name", "email", "age"},
		},
		{
			name: "single validation error",
			input: &testStruct{
				Name:  "",
				Email: "john@example.com",
				Age:   30,
			},
			wantCount: 1,
			wantKeys:  []string{"name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)
			require.Error(t, err)

			got := ValidationErrors(err)
			assert.Len(t, got, tt.wantCount)

			for _, key := range tt.wantKeys {
				assert.Contains(t, got, key)
				assert.NotEmpty(t, got[key])
			}
		})
	}

	t.Run("non-validation error returns empty map", func(t *testing.T) {
		err := errors.New("some error")
		got := ValidationErrors(err)
		assert.Empty(t, got)
	})
}

// TestIsValidationError tests validation error detection.
func TestIsValidationError(t *testing.T) {
	type testStruct struct {
		Name string `validate:"required"`
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "validation error",
			err: func() error {
				input := &testStruct{Name: ""}
				return Validate(input)
			}(),
			want: true,
		},
		{
			name: "non-validation error",
			err:  errors.New("some error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidationError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestValidationMessage tests validation message generation.
func TestValidationMessage(t *testing.T) {
	type testStruct struct {
		Name     string `validate:"required"`
		Email    string `validate:"email"`
		UUID     string `validate:"uuid"`
		Count    int    `validate:"min=1,max=10"`
		Role     string `validate:"oneof=admin user"`
		Text     string `validate:"min=5,max=100"`
		Age      int    `validate:"gte=0,lte=120"`
		Score    int    `validate:"gt=0,lt=100"`
		URL      string `validate:"url"`
		Username string `validate:"notempty"`
	}

	// Create a struct that will fail all validations
	input := &testStruct{
		Name:     "",
		Email:    "not-an-email",
		UUID:     "not-a-uuid",
		Count:    20,
		Role:     "invalid",
		Text:     "abc",
		Age:      150,
		Score:    150,
		URL:      "not-a-url",
		Username: "  ",
	}

	err := Validator().Struct(input)
	require.Error(t, err)

	var validationErrs validator.ValidationErrors
	require.ErrorAs(t, err, &validationErrs)

	// Map expected messages for each field
	expectedMessages := map[string]string{
		"name":     "this field is required",
		"email":    "must be a valid email address",
		"uuid":     "must be a valid UUID",
		"count":    "must be at most 10",
		"role":     "must be one of: admin user",
		"text":     "must be at least 5 characters",
		"age":      "must be less than or equal to 120",
		"score":    "must be less than 100",
		"url":      "must be a valid URL",
		"username": "must not be empty",
	}

	for _, fe := range validationErrs {
		fieldName := fe.Field()
		message := validationMessage(fe)

		expectedMsg, ok := expectedMessages[fieldName]
		if ok {
			assert.Equal(t, expectedMsg, message, "field: %s", fieldName)
		}
	}
}

// TestMinMaxMessage tests min/max message generation.
func TestMinMaxMessage(t *testing.T) {
	tests := []struct {
		name  string
		tag   string
		param string
		kind  reflect.Kind
		want  string
	}{
		{
			name:  "min for string",
			tag:   "min",
			param: "5",
			kind:  reflect.String,
			want:  "must be at least 5 characters",
		},
		{
			name:  "max for string",
			tag:   "max",
			param: "100",
			kind:  reflect.String,
			want:  "must be at most 100 characters",
		},
		{
			name:  "min for int",
			tag:   "min",
			param: "1",
			kind:  reflect.Int,
			want:  "must be at least 1",
		},
		{
			name:  "max for int",
			tag:   "max",
			param: "10",
			kind:  reflect.Int,
			want:  "must be at most 10",
		},
		{
			name:  "min for float",
			tag:   "min",
			param: "0.5",
			kind:  reflect.Float64,
			want:  "must be at least 0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minMaxMessage(tt.tag, tt.param, tt.kind)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestValidateUUID tests UUID validation.
func TestValidateUUID(t *testing.T) {
	type testStruct struct {
		ID string `validate:"uuid"`
	}

	tests := []struct {
		name    string
		uuid    string
		wantErr bool
	}{
		{
			name:    "valid UUID",
			uuid:    "123e4567-e89b-12d3-a456-426614174000",
			wantErr: false,
		},
		{
			name:    "invalid UUID",
			uuid:    "not-a-uuid",
			wantErr: true,
		},
		{
			name:    "empty UUID is valid",
			uuid:    "",
			wantErr: false,
		},
		{
			name:    "UUID without hyphens is valid",
			uuid:    "123e4567e89b12d3a456426614174000",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &testStruct{ID: tt.uuid}
			err := Validator().Struct(input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateNotEmpty tests not empty validation.
func TestValidateNotEmpty(t *testing.T) {
	type testStruct struct {
		Name string `validate:"notempty"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "non-empty string",
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "empty string",
			value:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			value:   "   ",
			wantErr: true,
		},
		{
			name:    "tabs and spaces",
			value:   "\t  \n",
			wantErr: true,
		},
		{
			name:    "string with spaces but also content",
			value:   "  hello  ",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &testStruct{Name: tt.value}
			err := Validator().Struct(input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// validatableTestStruct is a test struct that implements Validatable.
type validatableTestStruct struct {
	Name string `validate:"required"`
}

func (v *validatableTestStruct) Validate() error {
	if v.Name == "forbidden" {
		return errors.New("name cannot be forbidden")
	}
	return nil
}

// TestValidateAll tests combined struct and custom validation.
func TestValidateAll(t *testing.T) {
	// Verify it implements Validatable
	var _ Validatable = (*validatableTestStruct)(nil)

	tests := []struct {
		name    string
		input   *validatableTestStruct
		wantErr bool
		errType error
	}{
		{
			name: "valid input",
			input: &validatableTestStruct{
				Name: "valid",
			},
			wantErr: false,
		},
		{
			name: "struct validation fails",
			input: &validatableTestStruct{
				Name: "",
			},
			wantErr: true,
			errType: ErrValidation,
		},
		{
			name: "custom validation fails",
			input: &validatableTestStruct{
				Name: "forbidden",
			},
			wantErr: true,
			errType: ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAll(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}

	// Test with non-validatable struct
	t.Run("non-validatable struct", func(t *testing.T) {
		type simpleStruct struct {
			Name string `validate:"required"`
		}

		input := &simpleStruct{Name: "test"}
		err := ValidateAll(input)
		assert.NoError(t, err)
	})
}

// validatableImplStruct is another test struct that implements Validatable.
type validatableImplStruct struct {
	Value string `validate:"required"`
}

func (v *validatableImplStruct) Validate() error {
	if v.Value == "error" {
		return errors.New("custom validation failed")
	}
	return nil
}

// TestValidatableInterface tests the Validatable interface implementation.
func TestValidatableInterface(t *testing.T) {
	var _ Validatable = (*validatableImplStruct)(nil)

	tests := []struct {
		name    string
		input   *validatableImplStruct
		wantErr bool
	}{
		{
			name:    "valid",
			input:   &validatableImplStruct{Value: "valid"},
			wantErr: false,
		},
		{
			name:    "custom validation error",
			input:   &validatableImplStruct{Value: "error"},
			wantErr: true,
		},
		{
			name:    "struct validation error",
			input:   &validatableImplStruct{Value: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAll(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidationMessageUnknownTag tests fallback message for unknown tags.
func TestValidationMessageUnknownTag(t *testing.T) {
	// Create a mock field error for an unknown tag
	type testStruct struct {
		Field string `validate:"customtag"`
	}

	// Register a custom validator that will fail
	v := Validator()
	_ = v.RegisterValidation("customtag", func(fl validator.FieldLevel) bool {
		return false
	})

	input := &testStruct{Field: "value"}
	err := v.Struct(input)
	require.Error(t, err)

	var validationErrs validator.ValidationErrors
	require.ErrorAs(t, err, &validationErrs)

	for _, fe := range validationErrs {
		msg := validationMessage(fe)
		assert.Equal(t, "failed validation: customtag", msg)
	}
}
