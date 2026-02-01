// Package app contains application services that orchestrate use cases.
package app

import (
	"context"
	"log/slog"

	"github.com/jsamuelsen/go-service-template/internal/domain"
	"github.com/jsamuelsen/go-service-template/internal/ports"
)

// QuoteService orchestrates quote-related use cases.
// It depends on port interfaces, not concrete implementations,
// following the Dependency Inversion Principle.
type QuoteService struct {
	quoteClient ports.QuoteClient
	logger      *slog.Logger
}

// QuoteServiceConfig contains configuration for the quote service.
type QuoteServiceConfig struct {
	QuoteClient ports.QuoteClient
	Logger      *slog.Logger
}

// NewQuoteService creates a new quote service with the provided dependencies.
func NewQuoteService(cfg QuoteServiceConfig) *QuoteService {
	return &QuoteService{
		quoteClient: cfg.QuoteClient,
		logger:      cfg.Logger,
	}
}

// GetRandomQuote retrieves a random quote from the external service.
// This is a simple pass-through use case, but demonstrates the pattern.
// More complex use cases would include caching, validation, or enrichment.
func (s *QuoteService) GetRandomQuote(ctx context.Context) (*domain.Quote, error) {
	s.logger.InfoContext(ctx, "fetching random quote")

	quote, err := s.quoteClient.GetRandomQuote(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to fetch random quote",
			slog.Any("error", err),
		)
		return nil, err
	}

	s.logger.InfoContext(ctx, "fetched random quote",
		slog.String("quote_id", quote.ID),
		slog.String("author", quote.Author),
	)

	return quote, nil
}

// GetQuoteByID retrieves a specific quote by its identifier.
func (s *QuoteService) GetQuoteByID(ctx context.Context, id string) (*domain.Quote, error) {
	s.logger.InfoContext(ctx, "fetching quote by ID",
		slog.String("quote_id", id),
	)

	quote, err := s.quoteClient.GetQuoteByID(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to fetch quote",
			slog.String("quote_id", id),
			slog.Any("error", err),
		)
		return nil, err
	}

	s.logger.InfoContext(ctx, "fetched quote",
		slog.String("quote_id", quote.ID),
		slog.String("author", quote.Author),
	)

	return quote, nil
}
