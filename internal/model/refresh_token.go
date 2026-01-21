// internal/model/refresh_token.go
package model

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"gowa-yourself/database"
)

// RefreshToken represents a refresh token for maintaining user sessions
type RefreshToken struct {
	ID        int64
	UserID    int64
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
	Revoked   bool
	IPAddress sql.NullString
	UserAgent sql.NullString
}

var (
	ErrTokenNotFound = errors.New("refresh token not found")
	ErrTokenExpired  = errors.New("refresh token expired")
	ErrTokenRevoked  = errors.New("refresh token revoked")
)

// GenerateRefreshToken generates a random refresh token string
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateRefreshToken inserts a new refresh token into the database
func CreateRefreshToken(rt *RefreshToken) error {
	db := database.AppDB

	query := `
		INSERT INTO refresh_tokens (user_id, token, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	err := db.QueryRow(
		query,
		rt.UserID,
		rt.Token,
		rt.ExpiresAt,
		rt.IPAddress,
		rt.UserAgent,
	).Scan(&rt.ID, &rt.CreatedAt)

	return err
}

// GetRefreshToken retrieves a refresh token by token string
func GetRefreshToken(token string) (*RefreshToken, error) {
	db := database.AppDB

	query := `
		SELECT id, user_id, token, expires_at, created_at, revoked, ip_address, user_agent
		FROM refresh_tokens
		WHERE token = $1
	`

	rt := &RefreshToken{}
	err := db.QueryRow(query, token).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.Token,
		&rt.ExpiresAt,
		&rt.CreatedAt,
		&rt.Revoked,
		&rt.IPAddress,
		&rt.UserAgent,
	)

	if err == sql.ErrNoRows {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}

	// Check if token is revoked
	if rt.Revoked {
		return nil, ErrTokenRevoked
	}

	// Check if token is expired
	if time.Now().After(rt.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	return rt, nil
}

// RevokeRefreshToken marks a refresh token as revoked (logout)
func RevokeRefreshToken(token string) error {
	db := database.AppDB

	query := `UPDATE refresh_tokens SET revoked = true WHERE token = $1`

	result, err := db.Exec(query, token)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrTokenNotFound
	}

	return nil
}

// RevokeAllUserTokens revokes all refresh tokens for a specific user
func RevokeAllUserTokens(userID int64) error {
	db := database.AppDB

	query := `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1 AND revoked = false`

	result, err := db.Exec(query, userID)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("🔍 DEBUG [refresh_token.go]: Revoked %d tokens for user ID %d", rowsAffected, userID)

	return nil
}

// CleanupExpiredTokens removes expired tokens from the database
// This should be called periodically (e.g., daily cron job)
func CleanupExpiredTokens() (int64, error) {
	db := database.AppDB

	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW()`

	result, err := db.Exec(query)
	if err != nil {
		return 0, err
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsDeleted, nil
}

// GetUserTokenCount returns the number of active (non-revoked, non-expired) tokens for a user
func GetUserTokenCount(userID int64) (int, error) {
	db := database.AppDB

	query := `
		SELECT COUNT(*) 
		FROM refresh_tokens 
		WHERE user_id = $1 AND revoked = false AND expires_at > NOW()
	`

	var count int
	err := db.QueryRow(query, userID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// DeleteOldestUserToken deletes the oldest token for a user (for max token limit enforcement)
func DeleteOldestUserToken(userID int64) error {
	db := database.AppDB

	query := `
		DELETE FROM refresh_tokens
		WHERE id = (
			SELECT id FROM refresh_tokens
			WHERE user_id = $1 AND revoked = false
			ORDER BY created_at ASC
			LIMIT 1
		)
	`

	_, err := db.Exec(query, userID)
	return err
}
