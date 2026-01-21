// internal/service/auth_service.go
package service

import (
	"database/sql"
	"errors"
	"os"
	"strconv"
	"time"

	"gowa-yourself/internal/helper"
	"gowa-yourself/internal/model"

	"github.com/golang-jwt/jwt/v5"
)

// JWT configuration
var (
	jwtSecret               []byte
	accessTokenExpiry       time.Duration
	refreshTokenExpiry      time.Duration
	maxRefreshTokensPerUser int
)

// InitAuthConfig initializes authentication configuration from environment variables
func InitAuthConfig(secret string) {
	jwtSecret = []byte(secret)

	// Access token expiry (default: 1 hour)
	accessExp := os.Getenv("JWT_ACCESS_TOKEN_EXPIRY")
	if accessExp == "" {
		accessExp = "1h"
	}
	accessTokenExpiry, _ = time.ParseDuration(accessExp)

	// Refresh token expiry (default: 7 days)
	refreshExp := os.Getenv("JWT_REFRESH_TOKEN_EXPIRY")
	if refreshExp == "" {
		refreshExp = "168h" // 7 days
	}
	refreshTokenExpiry, _ = time.ParseDuration(refreshExp)

	// Max refresh tokens per user (default: 5)
	maxTokens := os.Getenv("MAX_REFRESH_TOKENS_PER_USER")
	if maxTokens == "" {
		maxRefreshTokensPerUser = 5
	} else {
		maxRefreshTokensPerUser, _ = strconv.Atoi(maxTokens)
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// RegisterUser creates a new user account
func RegisterUser(req model.CreateUserRequest) (*model.User, error) {
	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, errors.New("username, email, and password are required")
	}

	// Check if user already exists
	existingUser, _ := model.GetUserByUsername(req.Username)
	if existingUser != nil {
		return nil, errors.New("username already exists")
	}

	existingUser, _ = model.GetUserByEmail(req.Email)
	if existingUser != nil {
		return nil, errors.New("email already exists")
	}

	// Hash password
	passwordHash, err := helper.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// Set default role if not provided
	role := req.Role
	if role == "" {
		role = "user"
	}

	// Validate role
	if role != "admin" && role != "user" && role != "viewer" {
		return nil, errors.New("invalid role")
	}

	// Create user
	user := &model.User{
		Username:      req.Username,
		Email:         req.Email,
		PasswordHash:  sql.NullString{String: passwordHash, Valid: true},
		FullName:      sql.NullString{String: req.FullName, Valid: req.FullName != ""},
		AuthProvider:  "local",
		Role:          role,
		IsActive:      true,
		EmailVerified: false, // Email verification can be added later
	}

	err = model.CreateUser(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// AuthenticateUser validates username/password and returns user if valid
func AuthenticateUser(username, password string) (*model.User, error) {
	// Get user by username
	user, err := model.GetUserByUsername(username)
	if err != nil {
		if err == model.ErrUserNotFound {
			return nil, model.ErrInvalidCredentials
		}
		return nil, err
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.New("user account is disabled")
	}

	// Check auth provider - OAuth users cannot login with password
	if user.AuthProvider != "local" {
		return nil, errors.New("please use 'Sign in with " + user.AuthProvider + "' for this account")
	}

	// Verify password
	if !user.PasswordHash.Valid {
		return nil, errors.New("password not set for this account")
	}

	err = helper.VerifyPassword(user.PasswordHash.String, password)
	if err != nil {
		return nil, model.ErrInvalidCredentials
	}

	// Update last login timestamp
	_ = model.UpdateLastLogin(user.ID)

	return user, nil
}

// GenerateAccessToken generates a JWT access token for a user
func GenerateAccessToken(user *model.User) (string, error) {
	expirationTime := time.Now().Add(accessTokenExpiry)

	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// GenerateRefreshTokenForUser generates a refresh token and stores it in database
func GenerateRefreshTokenForUser(user *model.User, ipAddress, userAgent string) (string, error) {
	// Check token count and enforce limit
	tokenCount, err := model.GetUserTokenCount(user.ID)
	if err != nil {
		return "", err
	}

	// If user has too many tokens, delete the oldest one
	if tokenCount >= maxRefreshTokensPerUser {
		_ = model.DeleteOldestUserToken(user.ID)
	}

	// Generate random token
	tokenString, err := model.GenerateRefreshToken()
	if err != nil {
		return "", err
	}

	// Create refresh token record
	refreshToken := &model.RefreshToken{
		UserID:    user.ID,
		Token:     tokenString,
		ExpiresAt: time.Now().Add(refreshTokenExpiry),
		IPAddress: sql.NullString{String: ipAddress, Valid: ipAddress != ""},
		UserAgent: sql.NullString{String: userAgent, Valid: userAgent != ""},
	}

	err = model.CreateRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// RefreshAccessToken validates refresh token and generates new access token
func RefreshAccessToken(refreshTokenString string) (string, *model.User, error) {
	// Get refresh token from database
	refreshToken, err := model.GetRefreshToken(refreshTokenString)
	if err != nil {
		return "", nil, err
	}

	// Get user
	user, err := model.GetUserByID(refreshToken.UserID)
	if err != nil {
		return "", nil, err
	}

	// Check if user is still active
	if !user.IsActive {
		return "", nil, errors.New("user account is disabled")
	}

	// Generate new access token
	accessToken, err := GenerateAccessToken(user)
	if err != nil {
		return "", nil, err
	}

	return accessToken, user, nil
}

// ValidateAccessToken validates JWT access token and returns claims
func ValidateAccessToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// RevokeUserSession revokes a refresh token (logout)
func RevokeUserSession(refreshToken string) error {
	return model.RevokeRefreshToken(refreshToken)
}

// RevokeAllUserSessions revokes all refresh tokens for a user
func RevokeAllUserSessions(userID int64) error {
	return model.RevokeAllUserTokens(userID)
}
