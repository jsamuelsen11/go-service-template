// Package acl implements the Anti-Corruption Layer pattern for external services.
// ACL adapters translate between external API models and domain models,
// protecting the domain from external system changes.
package acl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/jsamuelsen/go-service-template/internal/adapters/clients"
	"github.com/jsamuelsen/go-service-template/internal/domain"
)

// QuoteClientConfig contains configuration for the quote client.
type QuoteClientConfig struct {
	// Client is the HTTP client to use for requests.
	// The client's BaseURL should be set to the quote API endpoint.
	Client *clients.Client

	// Logger is the structured logger.
	Logger *slog.Logger
}

// QuoteClient implements ports.QuoteClient using the quotable.io API.
// It demonstrates the ACL pattern by translating external API responses
// to domain types.
type QuoteClient struct {
	client *clients.Client
	logger *slog.Logger
}

// NewQuoteClient creates a new quote client adapter.
func NewQuoteClient(cfg QuoteClientConfig) *QuoteClient {
	return &QuoteClient{
		client: cfg.Client,
		logger: cfg.Logger,
	}
}

// quotableResponse is the external DTO from the quotable.io API.
// This is an internal type - never exposed outside the ACL.
type quotableResponse struct {
	ID      string   `json:"_id"`
	Content string   `json:"content"`
	Author  string   `json:"author"`
	Tags    []string `json:"tags"`
}

// GetRandomQuote fetches a random quote from the external API.
// Implements ports.QuoteClient.
func (c *QuoteClient) GetRandomQuote(ctx context.Context) (*domain.Quote, error) {
	c.logger.DebugContext(ctx, "fetching random quote")

	resp, err := c.client.Get(ctx, "/random")
	if err != nil {
		return nil, domain.NewUnavailableError("quote-service", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	return c.parseQuoteResponse(resp.Body)
}

// GetQuoteByID fetches a specific quote by its identifier.
// Implements ports.QuoteClient.
func (c *QuoteClient) GetQuoteByID(ctx context.Context, id string) (*domain.Quote, error) {
	c.logger.DebugContext(ctx, "fetching quote by ID", slog.String("quote_id", id))

	resp, err := c.client.Get(ctx, "/quotes/"+id)
	if err != nil {
		return nil, domain.NewUnavailableError("quote-service", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.NewNotFoundError("quote", id)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	return c.parseQuoteResponse(resp.Body)
}

// parseQuoteResponse reads and translates the external DTO to a domain Quote.
// This is the core ACL translation function.
func (c *QuoteClient) parseQuoteResponse(body io.Reader) (*domain.Quote, error) {
	var external quotableResponse

	err := json.NewDecoder(body).Decode(&external)
	if err != nil {
		return nil, fmt.Errorf("decoding quote response: %w", err)
	}

	// Translate external DTO to domain entity
	return c.translateToDomain(&external), nil
}

// translateToDomain converts the external API response to a domain Quote.
// This isolates the domain from external API changes.
func (c *QuoteClient) translateToDomain(ext *quotableResponse) *domain.Quote {
	return &domain.Quote{
		ID:      ext.ID,
		Content: ext.Content,
		Author:  ext.Author,
		Tags:    ext.Tags,
	}
}

// handleErrorResponse converts HTTP error responses to domain errors.
func (c *QuoteClient) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	c.logger.Warn("quote API error",
		slog.Int("status_code", resp.StatusCode),
		slog.String("body", string(body)),
	)

	switch resp.StatusCode {
	case http.StatusNotFound:
		return domain.ErrNotFound
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return domain.NewUnavailableError("quote-service", fmt.Sprintf("HTTP %d", resp.StatusCode))
	default:
		return domain.NewUnavailableError("quote-service", fmt.Sprintf("unexpected HTTP %d", resp.StatusCode))
	}
}

// Name returns the health check name for this client.
// Implements ports.HealthChecker.
func (c *QuoteClient) Name() string {
	return "quote-service"
}

// Check performs a health check by calling the API's health endpoint.
// Implements ports.HealthChecker.
func (c *QuoteClient) Check(ctx context.Context) error {
	// Use a simple endpoint to verify connectivity
	resp, err := c.client.Get(ctx, "/random")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("quote API returned status %d", resp.StatusCode)
	}

	return nil
}
