// internal/middleware/role_middleware.go
package middleware

import (
	"net/http"

	"gowa-yourself/internal/service"

	"github.com/labstack/echo/v4"
)

// RequireAdmin ensures the user has an admin role
func RequireAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get claims from context (set by JWTMiddleware)
		userClaims, ok := c.Get("user_claims").(*service.Claims)
		if !ok || userClaims.Role != "admin" {
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"success": false,
				"message": "Access denied. Admin role required.",
				"error": map[string]string{
					"code": "FORBIDDEN",
				},
			})
		}

		return next(c)
	}
}

// RequireRole ensures the user has at least one of the required roles
func RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userClaims, ok := c.Get("user_claims").(*service.Claims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"message": "Unauthorized",
				})
			}

			roleAllowed := false
			for _, role := range roles {
				if userClaims.Role == role {
					roleAllowed = true
					break
				}
			}

			if !roleAllowed {
				return c.JSON(http.StatusForbidden, map[string]interface{}{
					"success": false,
					"message": "Access denied. Insufficient permissions.",
				})
			}

			return next(c)
		}
	}
}
