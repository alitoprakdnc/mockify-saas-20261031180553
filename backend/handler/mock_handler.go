package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

// ContractService abstracts the persistence layer for OpenAPI contracts.
// Implementations should retrieve the raw OpenAPI document (JSON or YAML) for a given contract ID.
type ContractService interface {
	// GetContractSpec returns the OpenAPI specification associated with the contract ID.
	// The spec must be a valid JSON or YAML document.
	GetContractSpec(ctx context.Context, contractID string) (string, error)
}

// MockHandler provides HTTP handlers that generate mock responses from stored OpenAPI contracts.
type MockHandler struct {
	service ContractService
	logger  *zap.Logger
}

// NewMockHandler creates a new MockHandler with the provided dependencies.
func NewMockHandler(svc ContractService, logger *zap.Logger) *MockHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MockHandler{
		service: svc,
		logger:  logger,
	}
}

// ServeMock is an Echo handler that resolves the incoming request against the stored
// OpenAPI contract identified by the `contractID` path parameter and returns a mock
// response based on the operation's defined examples or schemas.
//
// Route pattern (registered elsewhere):
//   /mock/:contractID/*
// Example: GET /mock/1234/users/42
func (h *MockHandler) ServeMock(c echo.Context) error {
	ctx := c.Request().Context()
	contractID := c.Param("contractID")
	if contractID == "" {
		h.logger.Warn("missing contractID in request")
		return echo.NewHTTPError(http.StatusBadRequest, "contractID is required")
	}

	// Load the OpenAPI spec from the persistence layer.
	specStr, err := h.service.GetContractSpec(ctx, contractID)
	if err != nil {
		h.logger.Error("failed to retrieve contract spec", zap.Error(err), zap.String("contractID", contractID))
		return echo.NewHTTPError(http.StatusNotFound, "contract not found")
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromData([]byte(specStr))
	if err != nil {
		h.logger.Error("invalid OpenAPI spec", zap.Error(err), zap.String("contractID", contractID))
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid OpenAPI specification")
	}
	if err = doc.Validate(ctx); err != nil {
		h.logger.Error("OpenAPI spec validation failed", zap.Error(err), zap.String("contractID", contractID))
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid OpenAPI specification")
	}

	// Use kin-openapi's router to match the incoming request to an operation.
	router := openapi3filter.NewRouter().WithSwagger(doc)
	route, pathParams, err := router.FindRoute(c.Request().Method, c.Request().URL.Path)
	if err != nil {
		h.logger.Info("no matching operation for request", zap.Error(err), zap.String("method", c.Request().Method), zap.String("path", c.Request().URL.Path))
		return echo.NewHTTPError(http.StatusNotFound, "no matching operation in contract")
	}
	operation := route.Operation

	// Resolve the response to use. Prefer a 200 response, otherwise pick the first defined.
	var resp *openapi3.Response
	var status int
	if operation.Responses != nil {
		if rRef, ok := operation.Responses.Get("200"); ok && rRef != nil && rRef.Value != nil {
			resp = rRef.Value
			status = 200
		} else {
			// Fallback: iterate over the map and pick the first entry.
			for code, rRef := range operation.Responses {
				if rRef == nil || rRef.Value == nil {
					continue
				}
				resp = rRef.Value
				if code == "default" {
					status = 200
				} else {
					if i, convErr := strconv.Atoi(code); convErr == nil {
						status = i
					} else {
						status = 200
					}
				}
				break
			}
		}
	}
	if resp == nil {
		h.logger.Warn("operation has no response definitions", zap.String("operationID", operation.OperationID))
		return echo.NewHTTPError(http.StatusNotImplemented, "no response defined for operation")
	}

	// Build the mock payload.
	var payload []byte
	content := resp.Content
	if content != nil {
		// Prefer application/json; extend as needed for other media types.
		if media, ok := content["application/json"]; ok && media != nil {
			// 1. Explicit example.
			if media.Example != nil {
				payload, _ = json.Marshal(media.Example)
			} else if len(media.Examples) > 0 {
				// 2. First named example.
				for _, exRef := range media.Examples {
					if exRef != nil && exRef.Value != nil && exRef.Value.Value != nil {
						payload, _ = json.Marshal(exRef.Value.Value)
						break
					}
				}
			} else if media.Schema != nil {
				// 3. Generate a minimal JSON object based on the schema.
				// For simplicity we return an empty object; a full generator could be added later.
				payload = []byte("{}")
			}
		}
	}
	// If we couldn't determine a payload, default to empty JSON.
	if payload == nil {
		payload = []byte("{}")
	}

	// Set any defined response headers (using static examples if present).
	for name, hdrRef := range resp.Headers {
		if hdrRef != nil && hdrRef.Value != nil && hdrRef.Value.Schema != nil {
			// Simple placeholder; real implementation could generate realistic values.
			c.Response().Header().Set(name, "example")
		}
	}

	// Log the mock generation for observability.
	h.logger.Info("served mock response",
		zap.String("contractID", contractID),
		zap.String("operationID", operation.OperationID),
		zap.Int("status", status),
		zap.Any("pathParams", pathParams),
	)

	// Return the mock response.
	return c.Blob(status, echo.MIMEApplicationJSON, payload)
}

// RegisterRoutes attaches the mock handler routes to the provided Echo router.
// This function is called from the central router setup.
func (h *MockHandler) RegisterRoutes(e *echo.Echo) {
	// The wildcard `*` captures the remainder of the path after the contract ID.
	// Example: GET /mock/abc123/users/42
	e.Any("/mock/:contractID/*", h.ServeMock)
}