package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/dto"
	"github.com/jsamuelsen/go-service-template/internal/app"
	"github.com/jsamuelsen/go-service-template/internal/domain"
	"github.com/jsamuelsen/go-service-template/internal/mocks"
)

// setupQuoteHandler creates a QuoteHandler with a mock client for testing.
func setupQuoteHandler(t *testing.T, setupMock func(*mocks.MockQuoteClient)) *QuoteHandler {
	t.Helper()
	mockClient := mocks.NewMockQuoteClient(t)
	if setupMock != nil {
		setupMock(mockClient)
	}

	service := app.NewQuoteService(app.QuoteServiceConfig{
		QuoteClient: mockClient,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	return NewQuoteHandler(service)
}

func TestNewQuoteHandler(t *testing.T) {
	mockClient := mocks.NewMockQuoteClient(t)
	service := app.NewQuoteService(app.QuoteServiceConfig{
		QuoteClient: mockClient,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	handler := NewQuoteHandler(service)

	require.NotNil(t, handler)
}

func TestToQuoteResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    *domain.Quote
		expected *QuoteResponse
	}{
		{
			name: "full quote",
			input: &domain.Quote{
				ID:      "q-123",
				Content: "Test content",
				Author:  "Test Author",
				Tags:    []string{"tag1", "tag2"},
			},
			expected: &QuoteResponse{
				ID:      "q-123",
				Content: "Test content",
				Author:  "Test Author",
				Tags:    []string{"tag1", "tag2"},
			},
		},
		{
			name: "quote without tags",
			input: &domain.Quote{
				ID:      "q-456",
				Content: "Another content",
				Author:  "Another Author",
				Tags:    nil,
			},
			expected: &QuoteResponse{
				ID:      "q-456",
				Content: "Another content",
				Author:  "Another Author",
				Tags:    nil,
			},
		},
		{
			name: "quote with empty tags",
			input: &domain.Quote{
				ID:      "q-789",
				Content: "Content",
				Author:  "Author",
				Tags:    []string{},
			},
			expected: &QuoteResponse{
				ID:      "q-789",
				Content: "Content",
				Author:  "Author",
				Tags:    []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toQuoteResponse(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQuoteHandler_GetRandomQuote(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*mocks.MockQuoteClient)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "success",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetRandomQuote(mock.Anything).Return(&domain.Quote{
					ID:      "q-random",
					Content: "Random quote content",
					Author:  "Random Author",
					Tags:    []string{"inspiration"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				var resp QuoteResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, "q-random", resp.ID)
				assert.Equal(t, "Random quote content", resp.Content)
				assert.Equal(t, "Random Author", resp.Author)
			},
		},
		{
			name: "service unavailable",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetRandomQuote(mock.Anything).
					Return(nil, domain.NewUnavailableError("quote-api", "timeout"))
			},
			expectedStatus: http.StatusServiceUnavailable,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				var resp dto.ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, dto.ErrorCodeUnavailable, resp.Error.Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupQuoteHandler(t, tt.setupMock)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/quotes/random", nil)

			handler.GetRandomQuote(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestQuoteHandler_GetQuoteByID(t *testing.T) {
	tests := []struct {
		name           string
		quoteID        string
		setupMock      func(*mocks.MockQuoteClient)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:    "success",
			quoteID: "q-123",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetQuoteByID(mock.Anything, "q-123").Return(&domain.Quote{
					ID:      "q-123",
					Content: "Specific quote",
					Author:  "Specific Author",
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				var resp QuoteResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, "q-123", resp.ID)
			},
		},
		{
			name:    "empty ID returns bad request",
			quoteID: "",
			setupMock: func(m *mocks.MockQuoteClient) {
				// No mock call expected - validation happens first
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				var resp dto.ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, dto.ErrorCodeBadRequest, resp.Error.Code)
				assert.Contains(t, resp.Error.Message, "quote ID is required")
			},
		},
		{
			name:    "not found",
			quoteID: "nonexistent",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetQuoteByID(mock.Anything, "nonexistent").
					Return(nil, domain.NewNotFoundError("quote", "nonexistent"))
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				var resp dto.ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, dto.ErrorCodeNotFound, resp.Error.Code)
			},
		},
		{
			name:    "service unavailable",
			quoteID: "q-456",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetQuoteByID(mock.Anything, "q-456").
					Return(nil, domain.NewUnavailableError("quote-api", "circuit open"))
			},
			expectedStatus: http.StatusServiceUnavailable,
			checkResponse:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupQuoteHandler(t, tt.setupMock)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+tt.quoteID, nil)
			c.Params = gin.Params{{Key: "id", Value: tt.quoteID}}

			handler.GetQuoteByID(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestQuoteHandler_RegisterQuoteRoutes(t *testing.T) {
	mockClient := mocks.NewMockQuoteClient(t)
	mockClient.EXPECT().GetRandomQuote(mock.Anything).Return(&domain.Quote{
		ID: "test", Content: "test", Author: "test",
	}, nil).Maybe()
	mockClient.EXPECT().GetQuoteByID(mock.Anything, mock.Anything).Return(&domain.Quote{
		ID: "test", Content: "test", Author: "test",
	}, nil).Maybe()

	service := app.NewQuoteService(app.QuoteServiceConfig{
		QuoteClient: mockClient,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	handler := NewQuoteHandler(service)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterQuoteRoutes(api)

	routes := router.Routes()

	expectedRoutes := []string{
		"GET /api/v1/quotes/random",
		"GET /api/v1/quotes/:id",
	}

	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Method+" "+r.Path] = true
	}

	for _, expected := range expectedRoutes {
		assert.True(t, routeMap[expected], "missing route: %s", expected)
	}
}
