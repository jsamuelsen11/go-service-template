package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/domain"
	"github.com/jsamuelsen/go-service-template/internal/mocks"
)

// discardLogger returns a logger that discards all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewQuoteService_PanicsWithoutQuoteClient(t *testing.T) {
	assert.Panics(t, func() {
		NewQuoteService(QuoteServiceConfig{
			QuoteClient: nil,
			Logger:      slog.Default(),
		})
	})
}

func TestNewQuoteService_DefaultsLogger(t *testing.T) {
	mockClient := mocks.NewMockQuoteClient(t)

	svc := NewQuoteService(QuoteServiceConfig{
		QuoteClient: mockClient,
		Logger:      nil, // Should default to slog.Default()
	})

	require.NotNil(t, svc)
}

func TestNewQuoteService_Success(t *testing.T) {
	mockClient := mocks.NewMockQuoteClient(t)
	logger := discardLogger()

	svc := NewQuoteService(QuoteServiceConfig{
		QuoteClient: mockClient,
		Logger:      logger,
	})

	require.NotNil(t, svc)
}

func TestQuoteService_GetRandomQuote(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mocks.MockQuoteClient)
		expectedQuote *domain.Quote
		errCheck      func(error) bool
	}{
		{
			name: "success",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetRandomQuote(mock.Anything).
					Return(&domain.Quote{
						ID:      "q-123",
						Content: "Test quote",
						Author:  "Test Author",
						Tags:    []string{"test"},
					}, nil)
			},
			expectedQuote: &domain.Quote{
				ID:      "q-123",
				Content: "Test quote",
				Author:  "Test Author",
				Tags:    []string{"test"},
			},
			errCheck: nil,
		},
		{
			name: "client returns unavailable error",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetRandomQuote(mock.Anything).
					Return(nil, domain.NewUnavailableError("quote-service", "timeout"))
			},
			expectedQuote: nil,
			errCheck:      domain.IsUnavailable,
		},
		{
			name: "client returns generic error",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetRandomQuote(mock.Anything).
					Return(nil, errors.New("network error"))
			},
			expectedQuote: nil,
			errCheck: func(err error) bool {
				return err != nil && err.Error() == "network error"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockQuoteClient(t)
			tt.setupMock(mockClient)

			svc := NewQuoteService(QuoteServiceConfig{
				QuoteClient: mockClient,
				Logger:      discardLogger(),
			})

			quote, err := svc.GetRandomQuote(context.Background())

			if tt.errCheck != nil {
				require.Error(t, err)
				assert.True(t, tt.errCheck(err))
				assert.Nil(t, quote)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedQuote, quote)
			}
		})
	}
}

func TestQuoteService_GetQuoteByID(t *testing.T) {
	tests := []struct {
		name          string
		quoteID       string
		setupMock     func(*mocks.MockQuoteClient)
		expectedQuote *domain.Quote
		errCheck      func(error) bool
	}{
		{
			name:    "success",
			quoteID: "q-123",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetQuoteByID(mock.Anything, "q-123").
					Return(&domain.Quote{
						ID:      "q-123",
						Content: "Specific quote",
						Author:  "Author",
					}, nil)
			},
			expectedQuote: &domain.Quote{
				ID:      "q-123",
				Content: "Specific quote",
				Author:  "Author",
			},
		},
		{
			name:    "not found",
			quoteID: "nonexistent",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetQuoteByID(mock.Anything, "nonexistent").
					Return(nil, domain.NewNotFoundError("quote", "nonexistent"))
			},
			expectedQuote: nil,
			errCheck:      domain.IsNotFound,
		},
		{
			name:    "service unavailable",
			quoteID: "q-456",
			setupMock: func(m *mocks.MockQuoteClient) {
				m.EXPECT().GetQuoteByID(mock.Anything, "q-456").
					Return(nil, domain.NewUnavailableError("quote-api", "circuit open"))
			},
			errCheck: domain.IsUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.NewMockQuoteClient(t)
			tt.setupMock(mockClient)

			svc := NewQuoteService(QuoteServiceConfig{
				QuoteClient: mockClient,
				Logger:      discardLogger(),
			})

			quote, err := svc.GetQuoteByID(context.Background(), tt.quoteID)

			if tt.errCheck != nil {
				require.Error(t, err)
				assert.True(t, tt.errCheck(err))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedQuote, quote)
			}
		})
	}
}
