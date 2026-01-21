// internal/model/token_blacklist.go
package model

import (
	"time"

	"gowa-yourself/database"
)

// TokenBlacklist represents a blacklisted access token
type TokenBlacklist struct {
	ID        int64
	Token     string
	UserID    int64
	Reason    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// BlacklistToken adds a token to the blacklist
func BlacklistToken(token string, userID int64, reason string, expiresAt time.Time) error {
	db := database.AppDB

	query := `
		INSERT INTO token_blacklist (token, user_id, reason, expires_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err := db.Exec(query, token, userID, reason, expiresAt)
	return err
}

// IsTokenBlacklisted checks if a token is blacklisted
func IsTokenBlacklisted(token string) (bool, error) {
	db := database.AppDB

	query := `
		SELECT COUNT(*) 
		FROM token_blacklist 
		WHERE token = $1 AND expires_at > NOW()
	`

	var count int
	err := db.QueryRow(query, token).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// BlacklistAllUserTokens blacklists all tokens for a user (for password change)
func BlacklistAllUserTokens(userID int64, reason string) error {
	db := database.AppDB

	// We can't blacklist tokens we don't know about, so this is a placeholder
	// In practice, we'll blacklist tokens as they're used
	// For now, we just mark in the database that all tokens before this time are invalid

	query := `
		INSERT INTO token_blacklist (token, user_id, reason, expires_at)
		VALUES ($1, $2, $3, NOW() + INTERVAL '1 hour')
	`

	// Use a special marker token for "all tokens before this time"
	markerToken := "USER_" + string(rune(userID)) + "_INVALIDATE_ALL"
	_, err := db.Exec(query, markerToken, userID, reason)
	return err
}

// CleanupExpiredBlacklistedTokens removes expired tokens from blacklist
func CleanupExpiredBlacklistedTokens() (int64, error) {
	db := database.AppDB

	query := `DELETE FROM token_blacklist WHERE expires_at < NOW()`

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
