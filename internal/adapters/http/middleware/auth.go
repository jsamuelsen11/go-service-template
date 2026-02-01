package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/dto"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
)

const (
	// ContextKeyClaims is the gin context key for storing extracted claims.
	ContextKeyClaims = "claims"

	// Default header names if not configured.
	defaultSubjectHeader = "X-User-ID"
	defaultRolesHeader   = "X-User-Roles"
	defaultScopesHeader  = "X-User-Scopes"
)

// Claims represents user claims extracted from gateway headers.
// The gateway (e.g., API Gateway, Envoy) validates the JWT and passes
// claims via headers to downstream services.
type Claims struct {
	// Subject is the user ID (sub claim).
	Subject string

	// Roles is the list of roles assigned to the user.
	Roles []string

	// Scopes is the list of OAuth2 scopes granted.
	Scopes []string

	// Permissions is the list of fine-grained permissions.
	Permissions []string
}

// HasRole checks if the user has the specified role.
func (c *Claims) HasRole(role string) bool {
	return slices.Contains(c.Roles, role)
}

// HasAnyRole checks if the user has any of the specified roles.
func (c *Claims) HasAnyRole(roles ...string) bool {
	return slices.ContainsFunc(roles, c.HasRole)
}

// HasScope checks if the user has the specified scope.
func (c *Claims) HasScope(scope string) bool {
	return slices.Contains(c.Scopes, scope)
}

// HasAllScopes checks if the user has ALL specified scopes.
func (c *Claims) HasAllScopes(scopes ...string) bool {
	for _, scope := range scopes {
		if !c.HasScope(scope) {
			return false
		}
	}

	return true
}

// HasAnyScope checks if the user has any of the specified scopes.
func (c *Claims) HasAnyScope(scopes ...string) bool {
	return slices.ContainsFunc(scopes, c.HasScope)
}

// HasPermission checks if the user has the specified permission.
func (c *Claims) HasPermission(perm string) bool {
	return slices.Contains(c.Permissions, perm)
}

// ExtractClaims extracts user claims from request headers.
// Header names are configurable via AuthConfig.
func ExtractClaims(c *gin.Context, cfg *config.AuthConfig) *Claims {
	subjectHeader := defaultSubjectHeader
	rolesHeader := defaultRolesHeader
	scopesHeader := defaultScopesHeader

	if cfg != nil {
		if cfg.SubjectHeader != "" {
			subjectHeader = cfg.SubjectHeader
		}

		if cfg.RolesHeader != "" {
			rolesHeader = cfg.RolesHeader
		}

		if cfg.ScopesHeader != "" {
			scopesHeader = cfg.ScopesHeader
		}
	}

	claims := &Claims{
		Subject: c.GetHeader(subjectHeader),
	}

	// Parse roles (comma-separated)
	if rolesStr := c.GetHeader(rolesHeader); rolesStr != "" {
		claims.Roles = parseCommaSeparated(rolesStr)
	}

	// Parse scopes (space-separated per OAuth2 spec)
	if scopesStr := c.GetHeader(scopesHeader); scopesStr != "" {
		claims.Scopes = parseSpaceSeparated(scopesStr)
	}

	return claims
}

// GetClaims retrieves claims from the gin context.
// Returns nil if claims are not present.
func GetClaims(c *gin.Context) *Claims {
	if claims, exists := c.Get(ContextKeyClaims); exists {
		if cl, ok := claims.(*Claims); ok {
			return cl
		}
	}

	return nil
}

// RequireAuth returns middleware that requires authentication.
// It extracts claims and verifies a subject is present.
func RequireAuth(cfg *config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := ExtractClaims(c, cfg)

		if claims.Subject == "" {
			abortWithForbidden(c, "authentication required")
			return
		}

		// Store claims in context
		c.Set(ContextKeyClaims, claims)
		c.Next()
	}
}

// RequireRole returns middleware that requires a specific role.
func RequireRole(cfg *config.AuthConfig, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := getOrExtractClaims(c, cfg)

		if !claims.HasRole(role) {
			abortWithForbidden(c, "insufficient permissions: role "+role+" required")
			return
		}

		c.Next()
	}
}

// RequireAnyRole returns middleware that requires at least one of the roles.
func RequireAnyRole(cfg *config.AuthConfig, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := getOrExtractClaims(c, cfg)

		if !claims.HasAnyRole(roles...) {
			abortWithForbidden(c, "insufficient permissions: one of roles ["+strings.Join(roles, ", ")+"] required")
			return
		}

		c.Next()
	}
}

// RequireScopes returns middleware that requires ALL specified scopes.
func RequireScopes(cfg *config.AuthConfig, scopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := getOrExtractClaims(c, cfg)

		if !claims.HasAllScopes(scopes...) {
			abortWithForbidden(c, "insufficient permissions: scopes ["+strings.Join(scopes, ", ")+"] required")
			return
		}

		c.Next()
	}
}

// RequireAnyScope returns middleware that requires at least one of the scopes.
func RequireAnyScope(cfg *config.AuthConfig, scopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := getOrExtractClaims(c, cfg)

		if !claims.HasAnyScope(scopes...) {
			abortWithForbidden(c, "insufficient permissions: one of scopes ["+strings.Join(scopes, ", ")+"] required")
			return
		}

		c.Next()
	}
}

// RequirePermission returns middleware that requires a specific permission.
func RequirePermission(cfg *config.AuthConfig, perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := getOrExtractClaims(c, cfg)

		if !claims.HasPermission(perm) {
			abortWithForbidden(c, "insufficient permissions: permission "+perm+" required")
			return
		}

		c.Next()
	}
}

// RequireAny returns middleware that passes if ANY of the provided check functions pass.
// This enables OR logic for authorization rules.
//
// Example:
//
//	router.GET("/resource", RequireAny(cfg,
//	    func(c *Claims) bool { return c.HasRole("admin") },
//	    func(c *Claims) bool { return c.HasScope("resource:read") },
//	))
func RequireAny(cfg *config.AuthConfig, checks ...func(*Claims) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := getOrExtractClaims(c, cfg)

		for _, check := range checks {
			if check(claims) {
				c.Next()
				return
			}
		}

		// None of the checks passed
		abortWithForbidden(c, "insufficient permissions")
	}
}

// getOrExtractClaims gets claims from context or extracts them.
func getOrExtractClaims(c *gin.Context, cfg *config.AuthConfig) *Claims {
	if claims := GetClaims(c); claims != nil {
		return claims
	}

	claims := ExtractClaims(c, cfg)
	c.Set(ContextKeyClaims, claims)

	return claims
}

// abortWithForbidden aborts with a 403 Forbidden response.
func abortWithForbidden(c *gin.Context, message string) {
	errResp := dto.NewErrorResponse(dto.ErrorCodeForbidden, message)

	// Add trace ID if available
	if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
		errResp.TraceID = span.SpanContext().TraceID().String()
	}

	c.AbortWithStatusJSON(http.StatusForbidden, errResp)
}

// parseCommaSeparated splits a comma-separated string into trimmed values.
func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")

	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// parseSpaceSeparated splits a space-separated string (OAuth2 scope format).
func parseSpaceSeparated(s string) []string {
	parts := strings.Fields(s)

	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}

	return result
}
