package model

import (
	"context"
	"database/sql"
	"gowa-yourself/database"
	"time"
)

// WorkerConfig represents a configuration for worker blast outbox processing
type WorkerConfig struct {
	ID              int       `json:"id"`
	UserID          int       `json:"user_id"`
	WorkerName      string    `json:"worker_name"`
	Circle          string    `json:"circle"`
	Application     string    `json:"application"`
	MessageType     string    `json:"message_type"` // "direct" or "group"
	IntervalSeconds int       `json:"interval_seconds"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// GetWorkerConfigs retrieves worker configs based on user permissions
func GetWorkerConfigs(ctx context.Context, userID int, isAdmin bool) ([]WorkerConfig, error) {
	var query string
	var args []interface{}

	if isAdmin {
		query = `
			SELECT id, user_id, worker_name, circle, application, message_type, 
			       interval_seconds, enabled, created_at, updated_at
			FROM outbox_worker_config
			ORDER BY created_at DESC
		`
	} else {
		query = `
			SELECT id, user_id, worker_name, circle, application, message_type,
			       interval_seconds, enabled, created_at, updated_at
			FROM outbox_worker_config
			WHERE user_id = $1
			ORDER BY created_at DESC
		`
		args = append(args, userID)
	}

	rows, err := database.AppDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []WorkerConfig
	for rows.Next() {
		var config WorkerConfig
		err := rows.Scan(
			&config.ID,
			&config.UserID,
			&config.WorkerName,
			&config.Circle,
			&config.Application,
			&config.MessageType,
			&config.IntervalSeconds,
			&config.Enabled,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// GetWorkerConfigByID retrieves a single worker config by ID
func GetWorkerConfigByID(ctx context.Context, id int) (*WorkerConfig, error) {
	query := `
		SELECT id, user_id, worker_name, circle, application, message_type,
		       interval_seconds, enabled, created_at, updated_at
		FROM outbox_worker_config
		WHERE id = $1
	`

	var config WorkerConfig
	err := database.AppDB.QueryRowContext(ctx, query, id).Scan(
		&config.ID,
		&config.UserID,
		&config.WorkerName,
		&config.Circle,
		&config.Application,
		&config.MessageType,
		&config.IntervalSeconds,
		&config.Enabled,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// CreateWorkerConfig creates a new worker configuration
func CreateWorkerConfig(ctx context.Context, config *WorkerConfig) error {
	query := `
		INSERT INTO outbox_worker_config 
		(user_id, worker_name, circle, application, message_type, interval_seconds, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	err := database.AppDB.QueryRowContext(
		ctx,
		query,
		config.UserID,
		config.WorkerName,
		config.Circle,
		config.Application,
		config.MessageType,
		config.IntervalSeconds,
		config.Enabled,
	).Scan(&config.ID)

	return err
}

// UpdateWorkerConfig updates an existing worker configuration
func UpdateWorkerConfig(ctx context.Context, config *WorkerConfig) error {
	query := `
		UPDATE outbox_worker_config
		SET worker_name = $1, circle = $2, application = $3, message_type = $4,
		    interval_seconds = $5, enabled = $6, updated_at = NOW()
		WHERE id = $7
	`

	_, err := database.AppDB.ExecContext(
		ctx,
		query,
		config.WorkerName,
		config.Circle,
		config.Application,
		config.MessageType,
		config.IntervalSeconds,
		config.Enabled,
		config.ID,
	)

	return err
}

// DeleteWorkerConfig deletes a worker configuration
func DeleteWorkerConfig(ctx context.Context, id int) error {
	query := `DELETE FROM outbox_worker_config WHERE id = $1`
	_, err := database.AppDB.ExecContext(ctx, query, id)
	return err
}

// ToggleWorkerConfig toggles the enabled status of a worker configuration
func ToggleWorkerConfig(ctx context.Context, id int) error {
	query := `
		UPDATE outbox_worker_config
		SET enabled = NOT enabled, updated_at = NOW()
		WHERE id = $1
	`
	_, err := database.AppDB.ExecContext(ctx, query, id)
	return err
}

// GetEnabledConfigs retrieves all enabled worker configurations
func GetEnabledConfigs(ctx context.Context) ([]WorkerConfig, error) {
	query := `
		SELECT id, user_id, worker_name, circle, application, message_type,
		       interval_seconds, enabled, created_at, updated_at
		FROM outbox_worker_config
		WHERE enabled = true
		ORDER BY id ASC
	`

	rows, err := database.AppDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []WorkerConfig
	for rows.Next() {
		var config WorkerConfig
		err := rows.Scan(
			&config.ID,
			&config.UserID,
			&config.WorkerName,
			&config.Circle,
			&config.Application,
			&config.MessageType,
			&config.IntervalSeconds,
			&config.Enabled,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// GetAvailableCircles retrieves distinct circles from instances table
func GetAvailableCircles(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT circle FROM instances WHERE used = true ORDER BY circle`

	rows, err := database.AppDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var circles []string
	for rows.Next() {
		var circle string
		if err := rows.Scan(&circle); err != nil {
			return nil, err
		}
		circles = append(circles, circle)
	}

	return circles, rows.Err()
}

// GetAvailableApplications retrieves distinct applications from outbox table
func GetAvailableApplications(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT application 
		FROM outbox 
		WHERE application IS NOT NULL AND application != ''
		ORDER BY application
	`

	rows, err := database.OutboxDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applications []string
	for rows.Next() {
		var app string
		if err := rows.Scan(&app); err != nil {
			return nil, err
		}
		applications = append(applications, app)
	}

	return applications, rows.Err()
}
