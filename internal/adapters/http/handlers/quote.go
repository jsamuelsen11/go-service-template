package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/dto"
	"github.com/jsamuelsen/go-service-template/internal/app"
	"github.com/jsamuelsen/go-service-template/internal/domain"
)

// QuoteHandler handles quote-related HTTP endpoints.
type QuoteHandler struct {
	service *app.QuoteService
}

// NewQuoteHandler creates a new quote handler.
func NewQuoteHandler(service *app.QuoteService) *QuoteHandler {
	return &QuoteHandler{
		service: service,
	}
}

// QuoteResponse is the HTTP response structure for a quote.
type QuoteResponse struct {
	ID      string   `json:"id"`
	Content string   `json:"content"`
	Author  string   `json:"author"`
	Tags    []string `json:"tags,omitempty"`
}

// toQuoteResponse converts a domain Quote to an HTTP response.
func toQuoteResponse(q *domain.Quote) *QuoteResponse {
	return &QuoteResponse{
		ID:      q.ID,
		Content: q.Content,
		Author:  q.Author,
		Tags:    q.Tags,
	}
}

// GetRandomQuote handles GET /api/v1/quotes/random
// Returns a random quote from the external quote service.
//
// @Summary Get a random quote
// @Description Fetches a random quote from the quote service
// @Tags quotes
// @Produce json
// @Success 200 {object} QuoteResponse
// @Failure 503 {object} dto.ErrorResponse
// @Router /api/v1/quotes/random [get]
func (h *QuoteHandler) GetRandomQuote(c *gin.Context) {
	quote, err := h.service.GetRandomQuote(c.Request.Context())
	if err != nil {
		dto.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, toQuoteResponse(quote))
}

// GetQuoteByID handles GET /api/v1/quotes/:id
// Returns a specific quote by its identifier.
//
// @Summary Get a quote by ID
// @Description Fetches a specific quote by its identifier
// @Tags quotes
// @Produce json
// @Param id path string true "Quote ID"
// @Success 200 {object} QuoteResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 503 {object} dto.ErrorResponse
// @Router /api/v1/quotes/{id} [get]
func (h *QuoteHandler) GetQuoteByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, dto.NewErrorResponse(
			dto.ErrorCodeBadRequest,
			"quote ID is required",
		).WithTraceID(dto.GetTraceID(c)))
		return
	}

	quote, err := h.service.GetQuoteByID(c.Request.Context(), id)
	if err != nil {
		dto.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, toQuoteResponse(quote))
}

// RegisterQuoteRoutes registers quote routes on the given router group.
func (h *QuoteHandler) RegisterQuoteRoutes(rg *gin.RouterGroup) {
	quotes := rg.Group("/quotes")
	quotes.GET("/random", h.GetRandomQuote)
	quotes.GET("/:id", h.GetQuoteByID)
}
