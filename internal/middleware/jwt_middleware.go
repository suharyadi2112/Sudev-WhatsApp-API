// internal/middleware/jwt_middleware.go
package middleware

import (
	"net/http"
	"strings"

	"gowa-yourself/internal/service"

	"github.com/labstack/echo/v4"
)

// JWTAuthMiddleware validates JWT and extracts user claims to context
func JWTAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"message": "Unauthorized",
					"error": map[string]string{
						"code": "UNAUTHORIZED",
					},
				})
			}

			// Check Bearer prefix
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"message": "Invalid authorization header format",
					"error": map[string]string{
						"code": "INVALID_AUTH_HEADER",
					},
				})
			}

			tokenString := parts[1]

			// Validate token and extract claims
			claims, err := service.ValidateAccessToken(tokenString)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"message": "Invalid or expired token",
					"error": map[string]string{
						"code": "INVALID_TOKEN",
					},
				})
			}

			// Set claims to context
			c.Set("user_claims", claims)
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("role", claims.Role)

			return next(c)
		}
	}
}
