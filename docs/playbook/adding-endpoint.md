# Adding a New Endpoint

This guide walks through adding a new HTTP endpoint to the service, following the hexagonal architecture patterns.

## Overview

Adding an endpoint involves these layers:

1. **Domain** - Define entity (if needed)
2. **Ports** - Define service interface (if needed)
3. **Application** - Create application service
4. **Adapters** - Create HTTP handler with DTOs
5. **Router** - Register routes
6. **Main** - Wire dependencies

---

## Step 1: Define Domain Entity (if needed)

If your endpoint introduces a new business concept, define it in the domain layer.

**File:** `internal/domain/user.go`

```go
package domain

// User represents a user in the system.
type User struct {
    ID        string
    Email     string
    Name      string
    CreatedAt time.Time
}

// Validate performs domain validation on the user.
func (u *User) Validate() error {
    if u.Email == "" {
        return NewValidationError("email is required")
    }
    return nil
}
```

**Key points:**

- Domain entities have no external dependencies
- Include domain validation methods
- Use domain error types (see `internal/domain/errors.go`)

---

## Step 2: Define Port Interface (if needed)

If your service needs to call external systems, define a port interface.

**File:** `internal/ports/services.go`

```go
package ports

import "context"

// UserClient defines the contract for user data operations.
type UserClient interface {
    GetUser(ctx context.Context, id string) (*domain.User, error)
    ListUsers(ctx context.Context) ([]*domain.User, error)
}
```

**Key points:**

- Context as first parameter
- Return domain types, never external DTOs
- Use domain error types

---

## Step 3: Create Application Service

Application services orchestrate between domain logic and infrastructure.

**File:** `internal/app/user_service.go`

```go
package app

import (
    "context"
    "log/slog"

    "github.com/jsamuelsen/go-service-template/internal/domain"
    "github.com/jsamuelsen/go-service-template/internal/ports"
)

// UserServiceConfig holds dependencies for UserService.
type UserServiceConfig struct {
    UserClient ports.UserClient
    Logger     *slog.Logger
}

// UserService handles user-related use cases.
type UserService struct {
    client ports.UserClient
    logger *slog.Logger
}

// NewUserService creates a new user service.
func NewUserService(cfg UserServiceConfig) *UserService {
    logger := cfg.Logger
    if logger == nil {
        logger = slog.Default()
    }

    return &UserService{
        client: cfg.UserClient,
        logger: logger.With(slog.String("service", "user")),
    }
}

// GetUser retrieves a user by ID.
func (s *UserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
    s.logger.DebugContext(ctx, "getting user", slog.String("user_id", id))

    user, err := s.client.GetUser(ctx, id)
    if err != nil {
        return nil, err
    }

    return user, nil
}
```

**Key points:**

- Use config struct for dependency injection
- Log at service boundaries
- Pass context through to clients

---

## Step 4: Create HTTP Handler

Handlers translate HTTP requests to service calls and format responses.

**File:** `internal/adapters/http/handlers/user.go`

```go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"

    "github.com/jsamuelsen/go-service-template/internal/adapters/http/dto"
    "github.com/jsamuelsen/go-service-template/internal/app"
    "github.com/jsamuelsen/go-service-template/internal/domain"
)

// UserHandler handles user-related HTTP endpoints.
type UserHandler struct {
    service *app.UserService
}

// NewUserHandler creates a new user handler.
func NewUserHandler(service *app.UserService) *UserHandler {
    return &UserHandler{
        service: service,
    }
}

// UserResponse is the HTTP response structure for a user.
type UserResponse struct {
    ID    string `json:"id"`
    Email string `json:"email"`
    Name  string `json:"name"`
}

// toUserResponse converts a domain User to an HTTP response.
func toUserResponse(u *domain.User) *UserResponse {
    return &UserResponse{
        ID:    u.ID,
        Email: u.Email,
        Name:  u.Name,
    }
}

// GetUser handles GET /api/v1/users/:id
func (h *UserHandler) GetUser(c *gin.Context) {
    id := c.Param("id")
    if id == "" {
        c.JSON(http.StatusBadRequest, dto.NewErrorResponse(
            dto.ErrorCodeBadRequest,
            "user ID is required",
        ).WithTraceID(dto.GetTraceID(c)))
        return
    }

    user, err := h.service.GetUser(c.Request.Context(), id)
    if err != nil {
        dto.HandleError(c, err)
        return
    }

    c.JSON(http.StatusOK, toUserResponse(user))
}

// RegisterUserRoutes registers user routes on the given router group.
func (h *UserHandler) RegisterUserRoutes(rg *gin.RouterGroup) {
    users := rg.Group("/users")
    users.GET("/:id", h.GetUser)
}
```

**Key points:**

- Handler struct holds service dependency
- Response DTOs are separate from domain entities
- Use `dto.HandleError()` for automatic error mapping
- Include `WithTraceID()` for error responses
- Route registration method follows naming convention

---

## Step 5: Add Handler to RouterConfig

Add the handler field to the router configuration.

**File:** `internal/adapters/http/router.go`

```go
// RouterConfig contains configuration for setting up the router.
type RouterConfig struct {
    // ... existing fields ...

    // UserHandler handles user endpoints (optional).
    UserHandler *handlers.UserHandler

    // ... existing fields ...
}
```

---

## Step 6: Register Routes

Register the handler's routes in `setupAPIRoutes`.

**File:** `internal/adapters/http/router.go`

```go
// setupAPIRoutes registers business API routes.
func setupAPIRoutes(rg *gin.RouterGroup, cfg RouterConfig) {
    // ... existing routes ...

    // Register user routes
    if cfg.UserHandler != nil {
        cfg.UserHandler.RegisterUserRoutes(rg)
    }
}
```

---

## Step 7: Wire in main.go

Connect all the pieces in the main function.

**File:** `cmd/service/main.go`

```go
func run() error {
    // ... existing initialization ...

    // Create user service
    userService := app.NewUserService(app.UserServiceConfig{
        UserClient: userClient, // Inject the client
        Logger:     logger,
    })

    // Create handlers
    userHandler := handlers.NewUserHandler(userService)

    // Setup router with user handler
    routerCfg := http.RouterConfig{
        // ... existing config ...
        UserHandler:  userHandler,
    }
    http.SetupRouter(server.Engine(), routerCfg)

    // ... rest of startup ...
}
```

---

## Adding Protected Routes

To require authentication on routes:

```go
func setupAPIRoutes(rg *gin.RouterGroup, cfg RouterConfig) {
    // Public routes (no auth)
    if cfg.UserHandler != nil {
        rg.GET("/users/:id/profile", cfg.UserHandler.GetPublicProfile)
    }

    // Protected routes (require auth)
    protected := rg.Group("")
    protected.Use(middleware.RequireAuth(cfg.AuthConfig))

    if cfg.UserHandler != nil {
        protected.GET("/users/me", cfg.UserHandler.GetCurrentUser)
        protected.PUT("/users/me", cfg.UserHandler.UpdateCurrentUser)
    }

    // Admin routes (require admin role)
    admin := rg.Group("/admin")
    admin.Use(middleware.RequireAuth(cfg.AuthConfig))
    admin.Use(middleware.RequireRole(cfg.AuthConfig, "admin"))

    if cfg.UserHandler != nil {
        admin.GET("/users", cfg.UserHandler.ListAllUsers)
    }
}
```

---

## Checklist

- [ ] Domain entity defined (if new business concept)
- [ ] Port interface defined (if calling external systems)
- [ ] Application service created with proper dependencies
- [ ] HTTP handler with response DTOs
- [ ] Routes registered in router.go
- [ ] Handler wired in main.go
- [ ] Unit tests for handler
- [ ] Integration tests for endpoint

---

## Related Documentation

- [Architecture](../ARCHITECTURE.md) - Layer descriptions and patterns
- [Adding a Downstream Client](./adding-downstream-client.md) - If your endpoint needs an external client
- [Writing Unit Tests](./writing-unit-tests.md) - Testing handlers
