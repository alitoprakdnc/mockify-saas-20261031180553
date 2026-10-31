package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/yourorg/mockify/backend/prisma"
)

// ContractHandler provides CRUD operations for OpenAPI contracts.
type ContractHandler struct {
	db *prisma.PrismaClient
}

// NewContractHandler creates a new ContractHandler.
func NewContractHandler(db *prisma.PrismaClient) *ContractHandler {
	return &ContractHandler{db: db}
}

// ---------------------------------------------------------------------
// Request / Response payloads
// ---------------------------------------------------------------------

type CreateContractRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description,omitempty"`
	OpenAPI     string `json:"openapi" validate:"required"` // raw OpenAPI spec (JSON/YAML)
}

type UpdateContractRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	OpenAPI     *string `json:"openapi,omitempty"`
}

type ContractResponse struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	OpenAPI     string    `json:"openapi"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// toResponse converts a Prisma Contract model to the API response struct.
func toResponse(c *prisma.ContractModel) ContractResponse {
	return ContractResponse{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		OpenAPI:     c.Openapi,
		Version:     c.Version,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// ---------------------------------------------------------------------
// Helper utilities
// ---------------------------------------------------------------------

// getUserID extracts the authenticated user ID from the Echo context.
// The auth middleware must store the user ID under the key "userID".
func getUserID(c echo.Context) (int, error) {
	val := c.Get("userID")
	if val == nil {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "missing authentication")
	}
	uid, ok := val.(int)
	if !ok {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "invalid user identifier")
	}
	return uid, nil
}

// parseIDParam reads an integer ID from the URL parameter.
func parseIDParam(c echo.Context, name string) (int, error) {
	param := c.Param(name)
	if param == "" {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "missing "+name+" parameter")
	}
	id, err := strconv.Atoi(param)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, name+" must be an integer")
	}
	return id, nil
}

// ---------------------------------------------------------------------
// Handler implementations
// ---------------------------------------------------------------------

// CreateContract handles POST /contracts
func (h *ContractHandler) CreateContract(c echo.Context) error {
	var req CreateContractRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request payload")
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	created, err := h.db.Contract.CreateOne(
		prisma.ContractCreateInput{
			Name:        req.Name,
			Description: req.Description,
			Openapi:     req.OpenAPI,
			Version:     1,
			Owner: prisma.UserWhereUniqueInput{
				ID: &userID,
			},
		},
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to create contract")
		return echo.NewHTTPError(http.StatusInternalServerError, "could not create contract")
	}

	return c.JSON(http.StatusCreated, toResponse(created))
}

// GetContract handles GET /contracts/:id
func (h *ContractHandler) GetContract(c echo.Context) error {
	contractID, err := parseIDParam(c, "id")
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	contract, err := h.db.Contract.FindUnique(
		prisma.ContractWhereUniqueInput{
			ID: &contractID,
		},
	).Exec(ctx)
	if err != nil {
		if prisma.IsNotFound(err) {
			return echo.NewHTTPError(http.StatusNotFound, "contract not found")
		}
		log.Error().Err(err).Msg("failed to fetch contract")
		return echo.NewHTTPError(http.StatusInternalServerError, "could not retrieve contract")
	}

	// Authorization: only the owner can view the contract
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if contract.OwnerID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	return c.JSON(http.StatusOK, toResponse(contract))
}

// ListContracts handles GET /contracts
// Supports pagination via ?page=1&limit=20
func (h *ContractHandler) ListContracts(c echo.Context) error {
	// Default pagination
	page := 1
	limit := 20

	if p := c.QueryParam("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	skip := (page - 1) * limit

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	contracts, err := h.db.Contract.FindMany(
		prisma.ContractWhereInput{
			OwnerID: &userID,
		},
	).Skip(skip).Take(limit).OrderBy(
		prisma.ContractOrderByCreatedAtDesc,
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list contracts")
		return echo.NewHTTPError(http.StatusInternalServerError, "could not list contracts")
	}

	resp := make([]ContractResponse, len(contracts))
	for i, ct := range contracts {
		resp[i] = toResponse(ct)
	}
	return c.JSON(http.StatusOK, resp)
}

// UpdateContract handles PATCH /contracts/:id
func (h *ContractHandler) UpdateContract(c echo.Context) error {
	contractID, err := parseIDParam(c, "id")
	if err != nil {
		return err
	}

	var req UpdateContractRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request payload")
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	// Verify ownership first
	existing, err := h.db.Contract.FindUnique(
		prisma.ContractWhereUniqueInput{
			ID: &contractID,
		},
	).Exec(ctx)
	if err != nil {
		if prisma.IsNotFound(err) {
			return echo.NewHTTPError(http.StatusNotFound, "contract not found")
		}
		log.Error().Err(err).Msg("failed to fetch contract for update")
		return echo.NewHTTPError(http.StatusInternalServerError, "could not update contract")
	}
	if existing.OwnerID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	// Build update input
	update := prisma.ContractUpdateInput{}
	if req.Name != nil {
		update.Name = *req.Name
	}
	if req.Description != nil {
		update.Description = *req.Description
	}
	if req.OpenAPI != nil {
		update.Openapi = *req.OpenAPI
	}
	// Increment version on any change
	if req.Name != nil || req.Description != nil || req.OpenAPI != nil {
		update.Version = existing.Version + 1
	}

	updated, err := h.db.Contract.UpdateOne(
		prisma.ContractWhereUniqueInput{
			ID: &contractID,
		},
		update,
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to update contract")
		return echo.NewHTTPError(http.StatusInternalServerError, "could not update contract")
	}

	return c.JSON(http.StatusOK, toResponse(updated))
}

// DeleteContract handles DELETE /contracts/:id
func (h *ContractHandler) DeleteContract(c echo.Context) error {
	contractID, err := parseIDParam(c, "id")
	if err != nil {
		return err
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	// Verify ownership
	existing, err := h.db.Contract.FindUnique(
		prisma.ContractWhereUniqueInput{
			ID: &contractID,
		},
	).Exec(ctx)
	if err != nil {
		if prisma.IsNotFound(err) {
			return echo.NewHTTPError(http.StatusNotFound, "contract not found")
		}
		log.Error().Err(err).Msg("failed to fetch contract for deletion")
		return echo.NewHTTPError(http.StatusInternalServerError, "could not delete contract")
	}
	if existing.OwnerID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	if err := h.db.Contract.DeleteOne(
		prisma.ContractWhereUniqueInput{
			ID: &contractID,
		},
	).Exec(ctx); err != nil {
		log.Error().Err(err).Msg("failed to delete contract")
		return echo.NewHTTPError(http.StatusInternalServerError, "could not delete contract")
	}

	return c.NoContent(http.StatusNoContent)
}

// ---------------------------------------------------------------------
// Route registration (called from router.go)
// ---------------------------------------------------------------------

// RegisterRoutes attaches contract CRUD endpoints to the Echo router.
func (h *ContractHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/contracts")
	g.Use(authMiddleware) // assumes auth middleware is exported from middleware package

	g.POST("", h.CreateContract)
	g.GET("/:id", h.GetContract)
	g.GET("", h.ListContracts)
	g.PATCH("/:id", h.UpdateContract)
	g.DELETE("/:id", h.DeleteContract)
}

// authMiddleware placeholder – the real implementation lives in
// backend/middleware/auth_middleware.go. Importing it here avoids a
// circular dependency while still allowing the router to apply it.
var authMiddleware echo.MiddlewareFunc = func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// The actual middleware will replace this stub at runtime.
		return next(c)
	}
}