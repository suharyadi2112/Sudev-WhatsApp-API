// internal/handler/user_auth.go
package handler

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"gowa-yourself/internal/helper"
	"gowa-yourself/internal/model"
	"gowa-yourself/internal/service"

	"github.com/labstack/echo/v4"
)

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name,omitempty"`
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RefreshTokenRequest represents the refresh token request payload
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// AuthResponse represents the authentication response with tokens
type AuthResponse struct {
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	User         model.UserResponse `json:"user"`
}

// Register handles user registration
// POST /register
func Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	// Validate required fields
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return ErrorResponse(c, http.StatusBadRequest, "Username, email, and password are required", "MISSING_FIELDS", "")
	}

	// Create user
	createReq := model.CreateUserRequest{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
		Role:     "user", // Default role for registration
	}

	user, err := service.RegisterUser(createReq)
	if err != nil {
		return ErrorResponse(c, http.StatusBadRequest, err.Error(), "REGISTRATION_FAILED", "")
	}

	// Generate tokens
	accessToken, err := service.GenerateAccessToken(user)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to generate access token", "TOKEN_GENERATION_FAILED", err.Error())
	}

	ipAddress := c.RealIP()
	userAgent := c.Request().UserAgent()
	refreshToken, err := service.GenerateRefreshTokenForUser(user, ipAddress, userAgent)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to generate refresh token", "TOKEN_GENERATION_FAILED", err.Error())
	}

	// Log registration
	log.Printf("🔍 DEBUG: Starting audit log for user registration")
	log.Printf("🔍 DEBUG: User ID: %d, Username: %s", user.ID, user.Username)
	log.Printf("🔍 DEBUG: IP: %s, UserAgent: %s", ipAddress, userAgent)

	auditLog := &model.AuditLog{
		UserID:       sql.NullInt64{Int64: user.ID, Valid: true},
		Action:       "user.register",
		ResourceType: sql.NullString{String: "user", Valid: true},
		ResourceID:   sql.NullString{String: user.Username, Valid: true},
		IPAddress:    sql.NullString{String: ipAddress, Valid: true},
		UserAgent:    sql.NullString{String: userAgent, Valid: true},
	}

	log.Printf("🔍 DEBUG: Calling model.LogAction...")
	err = model.LogAction(auditLog)
	if err != nil {
		// Log error tapi jangan fail request
		log.Printf("❌ ERROR: Failed to log audit: %v", err)
	} else {
		log.Printf("✅ SUCCESS: Audit log saved successfully")
	}

	return SuccessResponse(c, http.StatusCreated, "User registered successfully", AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user.ToResponse(),
	})
}

// LoginUser handles user login with username/password
// POST /login
func LoginUser(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	// Validate required fields
	if req.Username == "" || req.Password == "" {
		return ErrorResponse(c, http.StatusBadRequest, "Username and password are required", "MISSING_FIELDS", "")
	}

	// Authenticate user
	user, err := service.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		if err == model.ErrInvalidCredentials {
			return ErrorResponse(c, http.StatusUnauthorized, "Invalid username or password", "INVALID_CREDENTIALS", "")
		}
		return ErrorResponse(c, http.StatusBadRequest, err.Error(), "AUTHENTICATION_FAILED", "")
	}

	// Generate tokens
	accessToken, err := service.GenerateAccessToken(user)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to generate access token", "TOKEN_GENERATION_FAILED", err.Error())
	}

	ipAddress := c.RealIP()
	userAgent := c.Request().UserAgent()
	refreshToken, err := service.GenerateRefreshTokenForUser(user, ipAddress, userAgent)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to generate refresh token", "TOKEN_GENERATION_FAILED", err.Error())
	}

	// Log login
	err = model.LogAction(&model.AuditLog{
		UserID:       sql.NullInt64{Int64: user.ID, Valid: true},
		Action:       "user.login",
		ResourceType: sql.NullString{String: "user", Valid: true},
		ResourceID:   sql.NullString{String: user.Username, Valid: true},
		IPAddress:    sql.NullString{String: ipAddress, Valid: true},
		UserAgent:    sql.NullString{String: userAgent, Valid: true},
	})
	if err != nil {
		log.Printf("⚠️ Failed to log audit: %v", err)
	}

	return SuccessResponse(c, http.StatusOK, "Login successful", AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user.ToResponse(),
	})
}

// RefreshToken handles refresh token to get new access token
// POST /refresh
func RefreshToken(c echo.Context) error {
	var req RefreshTokenRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	if req.RefreshToken == "" {
		return ErrorResponse(c, http.StatusBadRequest, "Refresh token is required", "MISSING_TOKEN", "")
	}

	// Validate refresh token and generate new access token
	accessToken, user, err := service.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		if err == model.ErrTokenNotFound || err == model.ErrTokenExpired || err == model.ErrTokenRevoked {
			return ErrorResponse(c, http.StatusUnauthorized, "Invalid or expired refresh token", "INVALID_REFRESH_TOKEN", err.Error())
		}
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to refresh token", "REFRESH_FAILED", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Token refreshed successfully", map[string]interface{}{
		"access_token": accessToken,
		"user":         user.ToResponse(),
	})
}

// LogoutUser handles user logout by revoking refresh token
// POST /api/logout
func LogoutUser(c echo.Context) error {
	var req RefreshTokenRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	if req.RefreshToken == "" {
		return ErrorResponse(c, http.StatusBadRequest, "Refresh token is required", "MISSING_TOKEN", "")
	}

	// Get user from context
	userClaims, _ := c.Get("user_claims").(*service.Claims)

	// Blacklist current access token (immediate logout)
	if userClaims != nil {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 {
				accessToken := parts[1]
				// Blacklist with expiry = token's original expiry
				expiresAt := time.Unix(userClaims.ExpiresAt.Unix(), 0)
				err := model.BlacklistToken(accessToken, userClaims.UserID, "logout", expiresAt)
				if err != nil {
					log.Printf("⚠️ Failed to blacklist token: %v", err)
				} else {
					log.Printf("✅ Access token blacklisted for user ID: %d", userClaims.UserID)
				}
			}
		}
	}

	// Revoke refresh token
	err := service.RevokeUserSession(req.RefreshToken)
	if err != nil {
		// Don't fail if token not found (already logged out)
		if err != model.ErrTokenNotFound {
			return ErrorResponse(c, http.StatusInternalServerError, "Failed to logout", "LOGOUT_FAILED", err.Error())
		}
	}

	// Log logout
	if userClaims != nil {
		_ = model.LogAction(&model.AuditLog{
			UserID:       sql.NullInt64{Int64: userClaims.UserID, Valid: true},
			Action:       "user.logout",
			ResourceType: sql.NullString{String: "user", Valid: true},
			ResourceID:   sql.NullString{String: userClaims.Username, Valid: true},
			IPAddress:    sql.NullString{String: c.RealIP(), Valid: true},
			UserAgent:    sql.NullString{String: c.Request().UserAgent(), Valid: true},
		})
	}

	return SuccessResponse(c, http.StatusOK, "Logged out successfully", nil)
}

// GetCurrentUser returns the current authenticated user's profile
// GET /api/me
func GetCurrentUser(c echo.Context) error {
	// Get user from context (set by JWT middleware)
	userClaims, ok := c.Get("user_claims").(*service.Claims)
	if !ok {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	// Get full user details
	user, err := model.GetUserByID(userClaims.UserID)
	if err != nil {
		return ErrorResponse(c, http.StatusNotFound, "User not found", "USER_NOT_FOUND", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "User profile retrieved", user.ToResponse())
}

// UpdateCurrentUser updates the current user's profile
// PUT /api/me
func UpdateCurrentUser(c echo.Context) error {
	userClaims, ok := c.Get("user_claims").(*service.Claims)
	if !ok {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	var req model.UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	// Get current user
	user, err := model.GetUserByID(userClaims.UserID)
	if err != nil {
		return ErrorResponse(c, http.StatusNotFound, "User not found", "USER_NOT_FOUND", err.Error())
	}

	// Update fields if provided
	if req.FullName != nil {
		user.FullName = sql.NullString{String: *req.FullName, Valid: true}
	}
	if req.AvatarURL != nil {
		user.AvatarURL = sql.NullString{String: *req.AvatarURL, Valid: true}
	}

	err = model.UpdateUser(user)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to update user", "UPDATE_FAILED", err.Error())
	}

	// Log update
	_ = model.LogAction(&model.AuditLog{
		UserID:       sql.NullInt64{Int64: user.ID, Valid: true},
		Action:       "user.update",
		ResourceType: sql.NullString{String: "user", Valid: true},
		ResourceID:   sql.NullString{String: user.Username, Valid: true},
		IPAddress:    sql.NullString{String: c.RealIP(), Valid: true},
		UserAgent:    sql.NullString{String: c.Request().UserAgent(), Valid: true},
	})

	return SuccessResponse(c, http.StatusOK, "User profile updated successfully", user.ToResponse())
}

// ChangePassword handles password change for local auth users
// PUT /api/me/password
func ChangePassword(c echo.Context) error {
	userClaims, ok := c.Get("user_claims").(*service.Claims)
	if !ok {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	var req model.ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return ErrorResponse(c, http.StatusBadRequest, "Old password and new password are required", "MISSING_FIELDS", "")
	}

	// Get user
	user, err := model.GetUserByID(userClaims.UserID)
	if err != nil {
		return ErrorResponse(c, http.StatusNotFound, "User not found", "USER_NOT_FOUND", err.Error())
	}

	// Check if user is local auth
	if user.AuthProvider != "local" {
		return ErrorResponse(c, http.StatusBadRequest, "Cannot change password for OAuth users", "OAUTH_USER", "")
	}

	// Verify old password
	if !user.PasswordHash.Valid {
		return ErrorResponse(c, http.StatusBadRequest, "Password not set", "NO_PASSWORD", "")
	}

	// Authenticate with old password
	_, err = service.AuthenticateUser(user.Username, req.OldPassword)
	if err != nil {
		return ErrorResponse(c, http.StatusUnauthorized, "Invalid old password", "INVALID_OLD_PASSWORD", "")
	}

	// Hash new password
	newPasswordHash, err := helper.HashPassword(req.NewPassword)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to hash password", "HASH_FAILED", err.Error())
	}

	// Update password
	err = model.UpdateUserPassword(user.ID, newPasswordHash)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to update password", "UPDATE_FAILED", err.Error())
	}

	// Blacklist current access token (immediate logout)
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 {
			accessToken := parts[1]
			expiresAt := time.Unix(userClaims.ExpiresAt.Unix(), 0)
			err := model.BlacklistToken(accessToken, userClaims.UserID, "password_change", expiresAt)
			if err != nil {
				log.Printf("⚠️ Failed to blacklist token: %v", err)
			} else {
				log.Printf("✅ Access token blacklisted after password change for user ID: %d", userClaims.UserID)
			}
		}
	}

	// Revoke all existing refresh tokens for security
	log.Printf("🔍 DEBUG: Revoking all sessions for user ID: %d", user.ID)
	err = service.RevokeAllUserSessions(user.ID)
	if err != nil {
		log.Printf("❌ ERROR: Failed to revoke sessions: %v", err)
	} else {
		log.Printf("✅ SUCCESS: All sessions revoked for user ID: %d", user.ID)
	}

	// Log password change
	err = model.LogAction(&model.AuditLog{
		UserID:       sql.NullInt64{Int64: user.ID, Valid: true},
		Action:       "user.password_change",
		ResourceType: sql.NullString{String: "user", Valid: true},
		ResourceID:   sql.NullString{String: user.Username, Valid: true},
		IPAddress:    sql.NullString{String: c.RealIP(), Valid: true},
		UserAgent:    sql.NullString{String: c.Request().UserAgent(), Valid: true},
	})
	if err != nil {
		log.Printf("⚠️ Failed to log password change audit: %v", err)
	}

	return SuccessResponse(c, http.StatusOK, "Password changed successfully. Please login again.", nil)
}
