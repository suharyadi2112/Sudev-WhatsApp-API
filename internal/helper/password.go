// internal/helper/password.go
package helper

import (
	"golang.org/x/crypto/bcrypt"
)

// HashPassword generates a bcrypt hash from plain text password
// Cost factor is set to bcrypt.DefaultCost (10) for balance between security and performance
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// VerifyPassword compares a bcrypt hashed password with plain text password
// Returns nil if password matches, error otherwise
func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
