package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"

	"mockify/backend/config"
)

// CustomClaims defines the JWT payload used by the application.
type CustomClaims struct {
	UserID string `json:"uid"`
	Role   string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// AuthMiddleware returns an Echo middleware that validates JWT tokens.
// It expects the token to be provided in the `Authorization` header using the
// `Bearer <token>` scheme. On successful validation the middleware injects the
// user ID and role into the request context under the keys "userID" and "userRole".
func AuthMiddleware(cfg *config.Config) echo.MiddlewareFunc {
	secret := []byte(cfg.JWTSecret)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Extract token from Authorization header.
			authHeader := c.Request().Header.Get(echo.HeaderAuthorization)
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing Authorization header")
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid Authorization header format")
			}
			tokenString := parts[1]

			// Parse and validate the token.
			token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
				// Ensure the signing method is HMAC (HS256/HS384/HS512).
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return secret, nil
			})
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("invalid token: %v", err))
			}

			claims, ok := token.Claims.(*CustomClaims)
			if !ok || !token.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token claims")
			}

			// Token is valid – inject user information into the context.
			c.Set("userID", claims.UserID)
			c.Set("userRole", claims.Role)

			return next(c)
		}
	}
}

// GetUserID extracts the authenticated user ID from the Echo context.
// Returns the user ID and a boolean indicating whether it was present.
func GetUserID(c echo.Context) (string, bool) {
	if uid, ok := c.Get("userID").(string); ok {
		return uid, true
	}
	return "", false
}

// GetUserRole extracts the authenticated user role from the Echo context.
// Returns the role and a boolean indicating whether it was present.
func GetUserRole(c echo.Context) (string, bool) {
	if role, ok := c.Get("userRole").(string); ok {
		return role, true
	}
	return "", false
}