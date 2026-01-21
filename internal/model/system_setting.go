// internal/model/system_setting.go
package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gowa-yourself/database"
)

type SystemIdentity struct {
	LogoURL            string `json:"logo_url"`
	IcoURL             string `json:"ico_url"`
	SecondLogoURL      string `json:"second_logo_url"`
	CompanyName        string `json:"company_name"`
	CompanyShortName   string `json:"company_short_name"`
	CompanyDescription string `json:"company_description"`
	CompanyAddress     string `json:"company_address"`
	CompanyPhone       string `json:"company_phone"`
	CompanyEmail       string `json:"company_email"`
	CompanyWebsite     string `json:"company_website"`
}

type SystemSetting struct {
	ID        int64           `json:"id"`
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	UpdatedAt time.Time       `json:"updated_at"`
}

const (
	KeySystemIdentity = "system_identity"
)

// GetSystemIdentity retrieves the global identity images
func GetSystemIdentity() (*SystemIdentity, error) {
	db := database.AppDB
	var value json.RawMessage

	err := db.QueryRow("SELECT value FROM system_settings WHERE key = $1", KeySystemIdentity).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return default empty identity if not found
			return &SystemIdentity{}, nil
		}
		return nil, fmt.Errorf("failed to get system identity: %w", err)
	}

	var identity SystemIdentity
	if err := json.Unmarshal(value, &identity); err != nil {
		return nil, fmt.Errorf("failed to unmarshal system identity: %w", err)
	}

	return &identity, nil
}

// UpdateSystemIdentitySettings saves the entire identity struct to the database
func UpdateSystemIdentitySettings(identity *SystemIdentity) error {
	db := database.AppDB

	// Marshal to JSON
	newValue, err := json.Marshal(identity)
	if err != nil {
		return fmt.Errorf("failed to marshal system settings: %w", err)
	}

	// Save to database
	_, err = db.Exec(`
		INSERT INTO system_settings (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, updated_at = NOW()
	`, KeySystemIdentity, newValue)

	if err != nil {
		return fmt.Errorf("failed to save system settings: %w", err)
	}

	return nil
}
