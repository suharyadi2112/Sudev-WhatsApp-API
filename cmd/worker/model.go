package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SQLPlaceholders returns correct placeholders ($1, $2 or ?, ?) based on driver
func SQLPlaceholders(query string) string {
	if WorkerDriver != "postgres" {
		// Convert $1, $2, etc to ?
		// Simple replacement for our specific queries
		newQuery := query
		for i := 10; i >= 1; i-- { // Replace from highest to lowest to avoid $10 becoming ?0
			old := fmt.Sprintf("$%d", i)
			newQuery = strings.ReplaceAll(newQuery, old, "?")
		}
		return newQuery
	}
	return query
}

type OutboxMessage struct {
	ID              int64          `json:"id_outbox"`
	Destination     string         `json:"destination"`
	Messages        string         `json:"messages"`
	Status          int            `json:"status"`
	Application     string         `json:"application"`
	InsertDateTime  time.Time      `json:"insertDateTime"`
	SendingDateTime sql.NullTime   `json:"sendingDateTime"`
	FromNumber      sql.NullString `json:"from_number"`
	MsgError        sql.NullString `json:"msg_error"`
}

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

func FetchWorkerConfigs(ctx context.Context) ([]WorkerConfig, error) {
	query := `
		SELECT id, user_id, worker_name, circle, application, message_type,
		       interval_seconds, enabled, created_at, updated_at
		FROM outbox_worker_config
		WHERE enabled = true
	`

	rows, err := WorkerDB.QueryContext(ctx, query)
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

func FetchPendingOutbox(ctx context.Context, filter string) (*OutboxMessage, error) {
	query := `
		SELECT id_outbox, destination, messages, status, application, insertDateTime 
		FROM outbox 
		WHERE status = 0 
	`
	if filter != "" {
		query += fmt.Sprintf(" AND (%s) ", filter)
	}
	query += " ORDER BY insertDateTime ASC LIMIT 1 "

	row := WorkerDB.QueryRowContext(ctx, query)

	var msg OutboxMessage
	err := row.Scan(&msg.ID, &msg.Destination, &msg.Messages, &msg.Status, &msg.Application, &msg.InsertDateTime)
	if err != nil {
		return nil, err
	}

	return &msg, nil
}

func UpdateOutboxSuccess(ctx context.Context, id int64, fromNumber string) error {
	query := `
		UPDATE outbox 
		SET status = 1, sendingDateTime = NOW(), from_number = $1, msg_error = NULL 
		WHERE id_outbox = $2
	`
	res, err := WorkerDB.ExecContext(ctx, SQLPlaceholders(query), fromNumber, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows affected for id %d", id)
	}
	return nil
}

func UpdateOutboxFailed(ctx context.Context, id int64, errorMsg string) error {
	query := `
		UPDATE outbox 
		SET status = 2, msg_error = $1 
		WHERE id_outbox = $2
	`
	res, err := WorkerDB.ExecContext(ctx, SQLPlaceholders(query), errorMsg, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows affected for id %d", id)
	}
	return nil
}

func UpdateOutboxStatus(ctx context.Context, id int64, status int, errorMsg string) error {
	query := `
		UPDATE outbox 
		SET status = $1, msg_error = $2 
		WHERE id_outbox = $3
	`
	res, err := WorkerDB.ExecContext(ctx, SQLPlaceholders(query), status, errorMsg, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows affected for id %d", id)
	}
	return nil
}
