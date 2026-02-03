package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// ConfigSQL returns correct placeholders for ConfigDB (always postgres)
func ConfigSQL(query string) string {
	return query
}

// OutboxSQL returns correct placeholders for OutboxDB (mysql or postgres)
func OutboxSQL(query string) string {
	if OutboxDriver != "postgres" {
		// Convert $1, $2, etc to ?
		newQuery := query
		for i := 10; i >= 1; i-- { // Replace from highest to lowest
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
	TableID         sql.NullString `json:"table_id"`
	File            sql.NullString `json:"file"`
	InsertDateTime  time.Time      `json:"insertDateTime"`
	SendingDateTime sql.NullTime   `json:"sendingDateTime"`
	FromNumber      sql.NullString `json:"from_number"`
	MsgError        sql.NullString `json:"msg_error"`
}

type WorkerConfig struct {
	ID                 int            `json:"id"`
	UserID             int            `json:"user_id"`
	WorkerName         string         `json:"worker_name"`
	Circle             string         `json:"circle"`
	Application        string         `json:"application"`
	MessageType        string         `json:"message_type"` // "direct" or "group"
	IntervalSeconds    int            `json:"interval_seconds"`
	IntervalMaxSeconds int            `json:"interval_max_seconds"`
	Enabled            bool           `json:"enabled"`
	AllowMedia         bool           `json:"allow_media"`
	WebhookURL         sql.NullString `json:"webhook_url"`
	WebhookSecret      sql.NullString `json:"webhook_secret"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

func FetchWorkerConfigs(ctx context.Context) ([]WorkerConfig, error) {
	query := `
		SELECT id, user_id, worker_name, circle, application, message_type,
		       interval_seconds, interval_max_seconds, enabled, allow_media, webhook_url, webhook_secret, created_at, updated_at
		FROM outbox_worker_config
		WHERE enabled = true
	`

	rows, err := ConfigDB.QueryContext(ctx, query)
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
			&config.IntervalMaxSeconds,
			&config.Enabled,
			&config.AllowMedia,
			&config.WebhookURL,
			&config.WebhookSecret,
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

func ClaimPendingOutbox(ctx context.Context, filter string) (*OutboxMessage, error) {
	// Atomic claim: Find one pending message (status 0), set it to processing (status 3), and return it.
	// Using FOR UPDATE SKIP LOCKED to prevent multiple workers from claiming the same row.
	var query string
	if OutboxDriver == "postgres" {
		query = `
			UPDATE outbox 
			SET status = 3
			WHERE id_outbox = (
				SELECT id_outbox 
				FROM outbox 
				WHERE status = 0 
		`
		if filter != "" {
			query += fmt.Sprintf(" AND (%s) ", filter)
		}
		query += `
				ORDER BY insertDateTime ASC 
				LIMIT 1 
				FOR UPDATE SKIP LOCKED
			)
			RETURNING id_outbox, destination, messages, status, application, table_id, file, insertDateTime
		`
	} else {
		// MySQL 8.0+ support
		// For MySQL, we might need a transaction block, but this is a simplified version
		query = `
			UPDATE outbox 
			SET status = 3
			WHERE status = 0 
		`
		if filter != "" {
			query += fmt.Sprintf(" AND (%s) ", filter)
		}
		query += `
			ORDER BY insertDateTime ASC 
			LIMIT 1
		`
		// Note: MySQL requires more careful handling for RETURN values,
		// usually done via a transaction and SELECT ... FOR UPDATE.
	}

	if OutboxDriver == "postgres" {
		row := OutboxDB.QueryRowContext(ctx, query)
		var msg OutboxMessage
		err := row.Scan(&msg.ID, &msg.Destination, &msg.Messages, &msg.Status, &msg.Application, &msg.TableID, &msg.File, &msg.InsertDateTime)
		if err != nil {
			return nil, err
		}
		return &msg, nil
	} else {
		// MySQL 8.0+ Atomic Claiming via Transaction
		tx, err := OutboxDB.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		// 1. Select and Lock
		selectQuery := `
			SELECT id_outbox, destination, messages, status, application, table_id, file, insertDateTime 
			FROM outbox 
			WHERE status = 0 
		`
		if filter != "" {
			selectQuery += fmt.Sprintf(" AND (%s) ", filter)
		}
		selectQuery += " ORDER BY insertDateTime ASC LIMIT 1 FOR UPDATE SKIP LOCKED "

		var msg OutboxMessage
		err = tx.QueryRowContext(ctx, selectQuery).Scan(
			&msg.ID, &msg.Destination, &msg.Messages, &msg.Status, &msg.Application, &msg.TableID, &msg.File, &msg.InsertDateTime,
		)
		if err != nil {
			return nil, err // Will include sql.ErrNoRows
		}

		// 2. Update status
		_, err = tx.ExecContext(ctx, "UPDATE outbox SET status = 3 WHERE id_outbox = ?", msg.ID)
		if err != nil {
			return nil, err
		}

		// 3. Commit
		if err := tx.Commit(); err != nil {
			return nil, err
		}

		msg.Status = 3
		return &msg, nil
	}
}

func FetchPendingOutbox(ctx context.Context, filter string) (*OutboxMessage, error) {
	query := `
		SELECT id_outbox, destination, messages, status, application, table_id, file, insertDateTime 
		FROM outbox 
		WHERE status = 0 
	`
	if filter != "" {
		query += fmt.Sprintf(" AND (%s) ", filter)
	}
	query += " ORDER BY insertDateTime ASC LIMIT 1 "

	row := OutboxDB.QueryRowContext(ctx, query)

	var msg OutboxMessage
	err := row.Scan(&msg.ID, &msg.Destination, &msg.Messages, &msg.Status, &msg.Application, &msg.TableID, &msg.File, &msg.InsertDateTime)
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
	res, err := OutboxDB.ExecContext(ctx, OutboxSQL(query), fromNumber, id)
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
	res, err := OutboxDB.ExecContext(ctx, OutboxSQL(query), errorMsg, id)
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
	res, err := OutboxDB.ExecContext(ctx, OutboxSQL(query), status, errorMsg, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows affected for id %d", id)
	}
	return nil
}

func LogWorkerEvent(workerID int, workerName, level, message string) {
	query := `
		INSERT INTO worker_system_logs (worker_id, worker_name, level, message)
		VALUES ($1, $2, $3, $4)
	`
	var wID interface{}
	if workerID > 0 {
		wID = workerID
	}
	_, err := ConfigDB.Exec(ConfigSQL(query), wID, workerName, level, message)
	if err != nil {
		log.Printf("CRITICAL: Failed to write system log to DB: %v", err)
	}
}
