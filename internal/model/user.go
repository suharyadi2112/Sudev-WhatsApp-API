// internal/model/user.go
package model

import (
	"database/sql"
	"errors"
	"time"

	"gowa-yourself/database"
)

// User represents a user account in the system
type User struct {
	ID              int64
	Username        string
	Email           string
	PasswordHash    sql.NullString // Nullable for OAuth users
	FullName        sql.NullString
	AvatarURL       sql.NullString
	AuthProvider    string // 'local' or 'google'
	OAuthProviderID sql.NullString
	Role            string // 'admin', 'user', 'viewer'
	IsActive        bool
	EmailVerified   bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastLoginAt     sql.NullTime
}

// UserResponse is the JSON response format for user data (without sensitive fields)
type UserResponse struct {
	ID            int64     `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	FullName      string    `json:"full_name,omitempty"`
	AvatarURL     string    `json:"avatar_url,omitempty"`
	AuthProvider  string    `json:"auth_provider"`
	Role          string    `json:"role"`
	IsActive      bool      `json:"is_active"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
	LastLoginAt   time.Time `json:"last_login_at,omitempty"`
}

// CreateUserRequest is the request payload for creating a new user
type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name,omitempty"`
	Role     string `json:"role,omitempty"` // Optional, defaults to 'user'
}

// UpdateUserRequest is the request payload for updating user profile
type UpdateUserRequest struct {
	FullName  *string `json:"full_name,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

// ChangePasswordRequest is the request payload for changing password
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// CreateUser inserts a new user into the database
func CreateUser(user *User) error {
	db := database.AppDB

	query := `
		INSERT INTO users (username, email, password_hash, full_name, avatar_url, 
			auth_provider, oauth_provider_id, role, is_active, email_verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	err := db.QueryRow(
		query,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.FullName,
		user.AvatarURL,
		user.AuthProvider,
		user.OAuthProviderID,
		user.Role,
		user.IsActive,
		user.EmailVerified,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		// Check for unique constraint violation
		if err.Error() == "pq: duplicate key value violates unique constraint \"users_username_key\"" {
			return errors.New("username already exists")
		}
		if err.Error() == "pq: duplicate key value violates unique constraint \"users_email_key\"" {
			return errors.New("email already exists")
		}
		return err
	}

	return nil
}

// GetUserByUsername retrieves a user by username
func GetUserByUsername(username string) (*User, error) {
	db := database.AppDB

	query := `
		SELECT id, username, email, password_hash, full_name, avatar_url,
			auth_provider, oauth_provider_id, role, is_active, email_verified,
			created_at, updated_at, last_login_at
		FROM users
		WHERE username = $1
	`

	user := &User{}
	err := db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.AvatarURL,
		&user.AuthProvider,
		&user.OAuthProviderID,
		&user.Role,
		&user.IsActive,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func GetUserByEmail(email string) (*User, error) {
	db := database.AppDB

	query := `
		SELECT id, username, email, password_hash, full_name, avatar_url,
			auth_provider, oauth_provider_id, role, is_active, email_verified,
			created_at, updated_at, last_login_at
		FROM users
		WHERE email = $1
	`

	user := &User{}
	err := db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.AvatarURL,
		&user.AuthProvider,
		&user.OAuthProviderID,
		&user.Role,
		&user.IsActive,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func GetUserByID(id int64) (*User, error) {
	db := database.AppDB

	query := `
		SELECT id, username, email, password_hash, full_name, avatar_url,
			auth_provider, oauth_provider_id, role, is_active, email_verified,
			created_at, updated_at, last_login_at
		FROM users
		WHERE id = $1
	`

	user := &User{}
	err := db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.AvatarURL,
		&user.AuthProvider,
		&user.OAuthProviderID,
		&user.Role,
		&user.IsActive,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

// UpdateUser updates user information
func UpdateUser(user *User) error {
	db := database.AppDB

	query := `
		UPDATE users
		SET full_name = $1, avatar_url = $2, updated_at = NOW()
		WHERE id = $3
	`

	result, err := db.Exec(query, user.FullName, user.AvatarURL, user.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// UpdateUserPassword updates user password hash
func UpdateUserPassword(userID int64, newPasswordHash string) error {
	db := database.AppDB

	query := `
		UPDATE users
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2 AND auth_provider = 'local'
	`

	result, err := db.Exec(query, newPasswordHash, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("cannot change password for OAuth users or user not found")
	}

	return nil
}

// UpdateLastLogin updates the last login timestamp
func UpdateLastLogin(userID int64) error {
	db := database.AppDB

	query := `UPDATE users SET last_login_at = NOW() WHERE id = $1`

	_, err := db.Exec(query, userID)
	return err
}

// DeleteUser soft deletes a user by setting is_active to false
func DeleteUser(id int64) error {
	db := database.AppDB

	query := `UPDATE users SET is_active = false, updated_at = NOW() WHERE id = $1`

	result, err := db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// ListUsers retrieves all users (for admin)
func ListUsers() ([]User, error) {
	db := database.AppDB

	query := `
		SELECT id, username, email, password_hash, full_name, avatar_url,
			auth_provider, oauth_provider_id, role, is_active, email_verified,
			created_at, updated_at, last_login_at
		FROM users
		ORDER BY created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.PasswordHash,
			&user.FullName,
			&user.AvatarURL,
			&user.AuthProvider,
			&user.OAuthProviderID,
			&user.Role,
			&user.IsActive,
			&user.EmailVerified,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.LastLoginAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// ToResponse converts User to UserResponse (removes sensitive data)
func (u *User) ToResponse() UserResponse {
	resp := UserResponse{
		ID:            u.ID,
		Username:      u.Username,
		Email:         u.Email,
		AuthProvider:  u.AuthProvider,
		Role:          u.Role,
		IsActive:      u.IsActive,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
	}

	if u.FullName.Valid {
		resp.FullName = u.FullName.String
	}
	if u.AvatarURL.Valid {
		resp.AvatarURL = u.AvatarURL.String
	}
	if u.LastLoginAt.Valid {
		resp.LastLoginAt = u.LastLoginAt.Time
	}

	return resp
}
