package warming

import (
	"database/sql"
	"fmt"
	"gowa-yourself/database"
	"time"
)

// WarmingScriptLine represents warming_script_lines table
type WarmingScriptLine struct {
	ID                int64
	ScriptID          int64
	SequenceOrder     int
	ActorRole         string // ACTOR_A or ACTOR_B
	MessageContent    string
	TypingDurationSec int
	CreatedAt         time.Time
}

// WarmingScriptLineResponse for JSON response
type WarmingScriptLineResponse struct {
	ID                int64     `json:"id"`
	ScriptID          int64     `json:"scriptId"`
	SequenceOrder     int       `json:"sequenceOrder"`
	ActorRole         string    `json:"actorRole"`
	MessageContent    string    `json:"messageContent"`
	TypingDurationSec int       `json:"typingDurationSec"`
	CreatedAt         time.Time `json:"createdAt"`
}

// CreateWarmingScriptLineRequest for POST request
type CreateWarmingScriptLineRequest struct {
	SequenceOrder     int    `json:"sequenceOrder"`
	ActorRole         string `json:"actorRole"`
	MessageContent    string `json:"messageContent"`
	TypingDurationSec int    `json:"typingDurationSec"`
}

// UpdateWarmingScriptLineRequest for PUT request
type UpdateWarmingScriptLineRequest struct {
	SequenceOrder     int    `json:"sequenceOrder"`
	ActorRole         string `json:"actorRole"`
	MessageContent    string `json:"messageContent"`
	TypingDurationSec int    `json:"typingDurationSec"`
}

// ReorderScriptLinesRequest for POST /scripts/:scriptId/lines/reorder
type ReorderScriptLinesRequest struct {
	Lines []struct {
		ID            int64 `json:"id"`
		SequenceOrder int   `json:"sequenceOrder"`
	} `json:"lines"`
}

// CreateWarmingScriptLine inserts new script line
func CreateWarmingScriptLine(scriptID int64, req *CreateWarmingScriptLineRequest) (*WarmingScriptLine, error) {
	query := `
		INSERT INTO warming_script_lines 
		(script_id, sequence_order, actor_role, message_content, typing_duration_sec, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, script_id, sequence_order, actor_role, message_content, typing_duration_sec, created_at
	`

	line := &WarmingScriptLine{}
	err := database.AppDB.QueryRow(
		query,
		scriptID,
		req.SequenceOrder,
		req.ActorRole,
		req.MessageContent,
		req.TypingDurationSec,
	).Scan(
		&line.ID,
		&line.ScriptID,
		&line.SequenceOrder,
		&line.ActorRole,
		&line.MessageContent,
		&line.TypingDurationSec,
		&line.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create warming script line: %w", err)
	}

	return line, nil
}

// GetAllWarmingScriptLines retrieves all lines for a script (ordered by sequence)
func GetAllWarmingScriptLines(scriptID int64) ([]WarmingScriptLine, error) {
	query := `
		SELECT id, script_id, sequence_order, actor_role, message_content, typing_duration_sec, created_at
		FROM warming_script_lines
		WHERE script_id = $1
		ORDER BY sequence_order ASC
	`

	rows, err := database.AppDB.Query(query, scriptID)
	if err != nil {
		return nil, fmt.Errorf("failed to query warming script lines: %w", err)
	}
	defer rows.Close()

	var lines []WarmingScriptLine
	for rows.Next() {
		var line WarmingScriptLine
		err := rows.Scan(
			&line.ID,
			&line.ScriptID,
			&line.SequenceOrder,
			&line.ActorRole,
			&line.MessageContent,
			&line.TypingDurationSec,
			&line.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan warming script line: %w", err)
		}
		lines = append(lines, line)
	}

	return lines, nil
}

// GetWarmingScriptLineByID retrieves single line by ID
func GetWarmingScriptLineByID(scriptID int64, lineID int64) (*WarmingScriptLine, error) {
	query := `
		SELECT id, script_id, sequence_order, actor_role, message_content, typing_duration_sec, created_at
		FROM warming_script_lines
		WHERE id = $1 AND script_id = $2
	`

	line := &WarmingScriptLine{}
	err := database.AppDB.QueryRow(query, lineID, scriptID).Scan(
		&line.ID,
		&line.ScriptID,
		&line.SequenceOrder,
		&line.ActorRole,
		&line.MessageContent,
		&line.TypingDurationSec,
		&line.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("warming script line not found")
		}
		return nil, fmt.Errorf("failed to get warming script line: %w", err)
	}

	return line, nil
}

// UpdateWarmingScriptLine updates existing line
func UpdateWarmingScriptLine(scriptID int64, lineID int64, req *UpdateWarmingScriptLineRequest) error {
	query := `
		UPDATE warming_script_lines
		SET sequence_order = $1, actor_role = $2, message_content = $3, typing_duration_sec = $4
		WHERE id = $5 AND script_id = $6
	`

	result, err := database.AppDB.Exec(
		query,
		req.SequenceOrder,
		req.ActorRole,
		req.MessageContent,
		req.TypingDurationSec,
		lineID,
		scriptID,
	)
	if err != nil {
		return fmt.Errorf("failed to update warming script line: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("warming script line not found")
	}

	return nil
}

// DeleteWarmingScriptLine deletes line by ID
func DeleteWarmingScriptLine(scriptID int64, lineID int64) error {
	query := `DELETE FROM warming_script_lines WHERE id = $1 AND script_id = $2`

	result, err := database.AppDB.Exec(query, lineID, scriptID)
	if err != nil {
		return fmt.Errorf("failed to delete warming script line: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("warming script line not found")
	}

	return nil
}

// ToWarmingScriptLineResponse converts WarmingScriptLine to response format
func ToWarmingScriptLineResponse(line WarmingScriptLine) WarmingScriptLineResponse {
	return WarmingScriptLineResponse{
		ID:                line.ID,
		ScriptID:          line.ScriptID,
		SequenceOrder:     line.SequenceOrder,
		ActorRole:         line.ActorRole,
		MessageContent:    line.MessageContent,
		TypingDurationSec: line.TypingDurationSec,
		CreatedAt:         line.CreatedAt,
	}
}

// ReorderScriptLines updates sequence order for multiple lines in a transaction
func ReorderScriptLines(scriptID int64, req *ReorderScriptLinesRequest) error {
	if len(req.Lines) == 0 {
		return fmt.Errorf("no lines provided for reordering")
	}

	// Start transaction
	tx, err := database.AppDB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// PHASE 1: Set all sequences to temporary negative values to avoid unique constraint conflicts
	// This prevents conflicts when swapping sequences (e.g., 1→2 and 2→1)
	tempQuery := `
		UPDATE warming_script_lines
		SET sequence_order = $1
		WHERE id = $2 AND script_id = $3
	`

	for i, line := range req.Lines {
		// Use negative index as temporary value (e.g., -1, -2, -3, ...)
		tempSequence := -(i + 1)

		result, err := tx.Exec(tempQuery, tempSequence, line.ID, scriptID)
		if err != nil {
			return fmt.Errorf("failed to set temporary sequence for line %d: %w", line.ID, err)
		}

		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected for line %d: %w", line.ID, err)
		}

		if rows == 0 {
			return fmt.Errorf("line %d not found in script %d", line.ID, scriptID)
		}
	}

	// PHASE 2: Update to final sequence orders
	finalQuery := `
		UPDATE warming_script_lines
		SET sequence_order = $1
		WHERE id = $2 AND script_id = $3
	`

	for _, line := range req.Lines {
		result, err := tx.Exec(finalQuery, line.SequenceOrder, line.ID, scriptID)
		if err != nil {
			return fmt.Errorf("failed to update line %d to final sequence: %w", line.ID, err)
		}

		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected for line %d: %w", line.ID, err)
		}

		if rows == 0 {
			return fmt.Errorf("line %d not found in script %d", line.ID, scriptID)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetNextAvailableScriptLine retrieves next available script line after current sequence (for worker)
func GetNextAvailableScriptLine(scriptID int64, currentSequence int) (*WarmingScriptLine, error) {
	query := `
		SELECT id, script_id, sequence_order, actor_role, message_content, typing_duration_sec, created_at
		FROM warming_script_lines
		WHERE script_id = $1 AND sequence_order > $2
		ORDER BY sequence_order ASC
		LIMIT 1
	`

	line := &WarmingScriptLine{}
	err := database.AppDB.QueryRow(query, scriptID, currentSequence).Scan(
		&line.ID,
		&line.ScriptID,
		&line.SequenceOrder,
		&line.ActorRole,
		&line.MessageContent,
		&line.TypingDurationSec,
		&line.CreatedAt,
	)

	return line, err
}
