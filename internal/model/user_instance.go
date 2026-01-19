// internal/model/user_instance.go
package model

import (
	"database/sql"
	"errors"
	"time"

	"gowa-yourself/database"
)

// UserInstance represents the relationship between a user and an instance
type UserInstance struct {
	ID              int64
	UserID          int64
	InstanceID      string
	PermissionLevel string // Legacy field (not used for authorization)
	CreatedAt       time.Time
}

var (
	ErrNoPermission           = errors.New("user does not have permission for this instance")
	ErrInsufficientPermission = errors.New("insufficient permission level")
)

// AssignInstanceToUser creates a user-instance relationship
func AssignInstanceToUser(userID int64, instanceID string, permission string) error {
	db := database.AppDB

	query := `
		INSERT INTO user_instances (user_id, instance_id, permission_level)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, instance_id) 
		DO UPDATE SET permission_level = $3
	`

	_, err := db.Exec(query, userID, instanceID, permission)
	return err
}

// GetUserInstances retrieves all instance IDs that a user has access to
func GetUserInstances(userID int64) ([]string, error) {
	db := database.AppDB

	query := `
		SELECT instance_id 
		FROM user_instances 
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instanceIDs []string
	for rows.Next() {
		var instanceID string
		if err := rows.Scan(&instanceID); err != nil {
			return nil, err
		}
		instanceIDs = append(instanceIDs, instanceID)
	}

	return instanceIDs, nil
}

// GetInstanceUsers retrieves all users who have access to an instance
func GetInstanceUsers(instanceID string) ([]UserInstance, error) {
	db := database.AppDB

	query := `
		SELECT id, user_id, instance_id, permission_level, created_at
		FROM user_instances
		WHERE instance_id = $1
		ORDER BY created_at ASC
	`

	rows, err := db.Query(query, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userInstances []UserInstance
	for rows.Next() {
		var ui UserInstance
		if err := rows.Scan(&ui.ID, &ui.UserID, &ui.InstanceID, &ui.PermissionLevel, &ui.CreatedAt); err != nil {
			return nil, err
		}
		userInstances = append(userInstances, ui)
	}

	return userInstances, nil
}

// CheckUserInstancePermission checks if a user has permission for an instance
// Returns the permission level if user has access, error otherwise
func CheckUserInstancePermission(userID int64, instanceID string) (string, error) {
	db := database.AppDB

	query := `
		SELECT permission_level 
		FROM user_instances 
		WHERE user_id = $1 AND instance_id = $2
	`

	var permissionLevel string
	err := db.QueryRow(query, userID, instanceID).Scan(&permissionLevel)

	if err == sql.ErrNoRows {
		return "", ErrNoPermission
	}
	if err != nil {
		return "", err
	}

	return permissionLevel, nil
}

// RemoveUserInstance removes a user's access to an instance
func RemoveUserInstance(userID int64, instanceID string) error {
	db := database.AppDB

	query := `DELETE FROM user_instances WHERE user_id = $1 AND instance_id = $2`

	result, err := db.Exec(query, userID, instanceID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNoPermission
	}

	return nil
}

// UpdateInstanceCreatedBy updates the created_by field in instances table
func UpdateInstanceCreatedBy(instanceID string, userID int64) error {
	db := database.AppDB

	query := `UPDATE instances SET created_by = $1 WHERE instance_id = $2`

	_, err := db.Exec(query, userID, instanceID)
	return err
}
