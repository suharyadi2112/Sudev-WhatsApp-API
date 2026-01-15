// internal/model/audit_log.go
package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gowa-yourself/database"
)

// AuditLog represents an audit trail entry for security and compliance
type AuditLog struct {
	ID           int64
	UserID       sql.NullInt64
	Action       string
	ResourceType sql.NullString
	ResourceID   sql.NullString
	Details      map[string]interface{}
	IPAddress    sql.NullString
	UserAgent    sql.NullString
	CreatedAt    time.Time
}

// LogAction creates an audit log entry
func LogAction(log *AuditLog) error {
	db := database.AppDB

	// DEBUG: Check database connection
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Convert details map to JSONB
	var detailsJSON interface{}
	if log.Details != nil && len(log.Details) > 0 {
		jsonBytes, err := json.Marshal(log.Details)
		if err != nil {
			return err
		}
		detailsJSON = jsonBytes
	} else {
		// If details is nil or empty, use NULL instead of empty JSON
		detailsJSON = nil
	}

	query := `
		INSERT INTO audit_logs (user_id, action, resource_type, resource_id, details, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	// DEBUG: Log query parameters
	fmt.Printf("🔍 DEBUG [audit_log.go]: Executing INSERT query\n")
	fmt.Printf("🔍 DEBUG [audit_log.go]: UserID=%v, Action=%s, ResourceType=%v, ResourceID=%v\n",
		log.UserID, log.Action, log.ResourceType, log.ResourceID)
	fmt.Printf("🔍 DEBUG [audit_log.go]: IPAddress=%v, UserAgent=%v, Details=%v\n",
		log.IPAddress, log.UserAgent, detailsJSON)

	err := db.QueryRow(
		query,
		log.UserID,
		log.Action,
		log.ResourceType,
		log.ResourceID,
		detailsJSON,
		log.IPAddress,
		log.UserAgent,
	).Scan(&log.ID, &log.CreatedAt)

	if err != nil {
		fmt.Printf("❌ DEBUG [audit_log.go]: Query failed: %v\n", err)
		return err
	}

	fmt.Printf("✅ DEBUG [audit_log.go]: Audit log inserted with ID=%d\n", log.ID)
	return nil
}

// GetUserAuditLogs retrieves audit logs for a specific user
func GetUserAuditLogs(userID int64, limit int) ([]AuditLog, error) {
	db := database.AppDB

	if limit <= 0 {
		limit = 100 // Default limit
	}

	query := `
		SELECT id, user_id, action, resource_type, resource_id, details, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// GetResourceAuditLogs retrieves audit logs for a specific resource
func GetResourceAuditLogs(resourceType, resourceID string, limit int) ([]AuditLog, error) {
	db := database.AppDB

	if limit <= 0 {
		limit = 100 // Default limit
	}

	query := `
		SELECT id, user_id, action, resource_type, resource_id, details, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE resource_type = $1 AND resource_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := db.Query(query, resourceType, resourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// GetRecentAuditLogs retrieves recent audit logs (for admin dashboard)
func GetRecentAuditLogs(limit int) ([]AuditLog, error) {
	db := database.AppDB

	if limit <= 0 {
		limit = 100 // Default limit
	}

	query := `
		SELECT id, user_id, action, resource_type, resource_id, details, ip_address, user_agent, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// scanAuditLogs is a helper function to scan rows into AuditLog slice
func scanAuditLogs(rows *sql.Rows) ([]AuditLog, error) {
	var logs []AuditLog

	for rows.Next() {
		var log AuditLog
		var detailsJSON []byte

		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Action,
			&log.ResourceType,
			&log.ResourceID,
			&detailsJSON,
			&log.IPAddress,
			&log.UserAgent,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSONB details
		if len(detailsJSON) > 0 {
			if err := json.Unmarshal(detailsJSON, &log.Details); err != nil {
				return nil, err
			}
		}

		logs = append(logs, log)
	}

	return logs, nil
}
