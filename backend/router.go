package router

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"mockify/backend/config"
	"mockify/backend/handler"
	authmw "mockify/backend/middleware"
)

// NewRouter creates and configures an Echo router instance.
// It registers all public and protected routes, applies global middleware
// (logging, recovery, CORS, request IDs) and returns the ready‑to‑use *echo.Echo.
func NewRouter(cfg *config.Config) *echo.Echo {
	// Create Echo instance with sane defaults.
	e := echo.New()

	// -------------------------------------------------------------------------
	// Global middleware
	// -------------------------------------------------------------------------
	// Recover from panics and return a 500.
	e.Use(middleware.Recover())

	// Structured request logging.
	e.Use(middleware.Logger())

	// Add a request ID header for tracing.
	e.Use(middleware.RequestID())

	// Basic CORS configuration – allow any origin for the SaaS UI.
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
			http.MethodHead,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
		},
	}))

	// -------------------------------------------------------------------------
	// Public routes (no authentication required)
	// -------------------------------------------------------------------------
	// Simple health‑check endpoint used by k8s probes and CI.
	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// -------------------------------------------------------------------------
	// Protected API routes – JWT authentication required
	// -------------------------------------------------------------------------
	api := e.Group("/api/v1")
	// Apply JWT middleware that validates the token and injects user context.
	api.Use(authmw.JWTMiddleware(cfg.JWTSecret))

	// -------------------------------------------------------------------------
	// Contract management endpoints
	// -------------------------------------------------------------------------
	contractHandler := handler.NewContractHandler(cfg)

	api.GET("/contracts", contractHandler.ListContracts)          // List all contracts (paginated)
	api.POST("/contracts", contractHandler.CreateContract)      // Create a new contract
	api.GET("/contracts/:id", contractHandler.GetContract)      // Retrieve a single contract
	api.PUT("/contracts/:id", contractHandler.UpdateContract)   // Full update
	api.PATCH("/contracts/:id", contractHandler.PatchContract) // Partial update (optional)
	api.DELETE("/contracts/:id", contractHandler.DeleteContract) // Remove a contract

	// -------------------------------------------------------------------------
	// Mock server endpoints
	// -------------------------------------------------------------------------
	// The mock handler serves any HTTP method and any sub‑path under /mock/:contractID.
	// This mirrors the behaviour of a real mock server where the contract ID determines
	// which OpenAPI spec to use for request matching.
	mockHandler := handler.NewMockHandler(cfg)

	// Using Echo's wildcard to capture the full request path after the contract ID.
	api.Any("/mock/:contractID/*", mockHandler.ServeMock)

	// Optionally expose a root mock endpoint without a trailing slash.
	api.Any("/mock/:contractID", mockHandler.ServeMock)

	return e
}