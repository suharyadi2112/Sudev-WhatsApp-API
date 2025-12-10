package warming

import (
	"database/sql"
	"fmt"
	"gowa-yourself/database"
	"time"

	"github.com/google/uuid"
)

// WarmingRoom represents warming_rooms table
type WarmingRoom struct {
	ID                 uuid.UUID
	Name               string
	SenderInstanceID   string
	ReceiverInstanceID string
	ScriptID           int64
	CurrentSequence    int
	Status             string // STOPPED, ACTIVE, PAUSED, FINISHED
	IntervalMinSeconds int
	IntervalMaxSeconds int
	NextRunAt          sql.NullTime
	LastRunAt          sql.NullTime
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// WarmingRoomResponse for JSON response
type WarmingRoomResponse struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	SenderInstanceID   string     `json:"senderInstanceId"`
	ReceiverInstanceID string     `json:"receiverInstanceId"`
	ScriptID           int64      `json:"scriptId"`
	CurrentSequence    int        `json:"currentSequence"`
	Status             string     `json:"status"`
	IntervalMinSeconds int        `json:"intervalMinSeconds"`
	IntervalMaxSeconds int        `json:"intervalMaxSeconds"`
	NextRunAt          *time.Time `json:"nextRunAt"`
	LastRunAt          *time.Time `json:"lastRunAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// CreateWarmingRoomRequest for POST request
type CreateWarmingRoomRequest struct {
	Name               string `json:"name"`
	SenderInstanceID   string `json:"senderInstanceId"`
	ReceiverInstanceID string `json:"receiverInstanceId"`
	ScriptID           int64  `json:"scriptId"`
	IntervalMinSeconds int    `json:"intervalMinSeconds"`
	IntervalMaxSeconds int    `json:"intervalMaxSeconds"`
}

// UpdateWarmingRoomRequest for PUT request
type UpdateWarmingRoomRequest struct {
	Name               string `json:"name"`
	IntervalMinSeconds int    `json:"intervalMinSeconds"`
	IntervalMaxSeconds int    `json:"intervalMaxSeconds"`
}

// CreateWarmingRoom inserts new room
func CreateWarmingRoom(req *CreateWarmingRoomRequest) (*WarmingRoom, error) {
	query := `
		INSERT INTO warming_rooms 
		(name, sender_instance_id, receiver_instance_id, script_id, 
		 interval_min_seconds, interval_max_seconds, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, name, sender_instance_id, receiver_instance_id, script_id, 
		          current_sequence, status, interval_min_seconds, interval_max_seconds,
		          next_run_at, last_run_at, created_at, updated_at
	`

	room := &WarmingRoom{}
	err := database.AppDB.QueryRow(
		query,
		req.Name,
		req.SenderInstanceID,
		req.ReceiverInstanceID,
		req.ScriptID,
		req.IntervalMinSeconds,
		req.IntervalMaxSeconds,
	).Scan(
		&room.ID,
		&room.Name,
		&room.SenderInstanceID,
		&room.ReceiverInstanceID,
		&room.ScriptID,
		&room.CurrentSequence,
		&room.Status,
		&room.IntervalMinSeconds,
		&room.IntervalMaxSeconds,
		&room.NextRunAt,
		&room.LastRunAt,
		&room.CreatedAt,
		&room.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create warming room: %w", err)
	}

	return room, nil
}

// GetAllWarmingRooms retrieves all rooms with optional status filter
func GetAllWarmingRooms(status string) ([]WarmingRoom, error) {
	var query string
	var args []interface{}

	if status != "" {
		query = `
			SELECT id, name, sender_instance_id, receiver_instance_id, script_id,
			       current_sequence, status, interval_min_seconds, interval_max_seconds,
			       next_run_at, last_run_at, created_at, updated_at
			FROM warming_rooms
			WHERE status = $1
			ORDER BY created_at DESC
		`
		args = append(args, status)
	} else {
		query = `
			SELECT id, name, sender_instance_id, receiver_instance_id, script_id,
			       current_sequence, status, interval_min_seconds, interval_max_seconds,
			       next_run_at, last_run_at, created_at, updated_at
			FROM warming_rooms
			ORDER BY created_at DESC
		`
	}

	rows, err := database.AppDB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query warming rooms: %w", err)
	}
	defer rows.Close()

	var rooms []WarmingRoom
	for rows.Next() {
		var room WarmingRoom
		err := rows.Scan(
			&room.ID,
			&room.Name,
			&room.SenderInstanceID,
			&room.ReceiverInstanceID,
			&room.ScriptID,
			&room.CurrentSequence,
			&room.Status,
			&room.IntervalMinSeconds,
			&room.IntervalMaxSeconds,
			&room.NextRunAt,
			&room.LastRunAt,
			&room.CreatedAt,
			&room.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan warming room: %w", err)
		}
		rooms = append(rooms, room)
	}

	return rooms, nil
}

// GetWarmingRoomByID retrieves single room by ID
func GetWarmingRoomByID(id string) (*WarmingRoom, error) {
	roomID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid room ID format: %w", err)
	}

	query := `
		SELECT id, name, sender_instance_id, receiver_instance_id, script_id,
		       current_sequence, status, interval_min_seconds, interval_max_seconds,
		       next_run_at, last_run_at, created_at, updated_at
		FROM warming_rooms
		WHERE id = $1
	`

	room := &WarmingRoom{}
	err = database.AppDB.QueryRow(query, roomID).Scan(
		&room.ID,
		&room.Name,
		&room.SenderInstanceID,
		&room.ReceiverInstanceID,
		&room.ScriptID,
		&room.CurrentSequence,
		&room.Status,
		&room.IntervalMinSeconds,
		&room.IntervalMaxSeconds,
		&room.NextRunAt,
		&room.LastRunAt,
		&room.CreatedAt,
		&room.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("warming room not found")
		}
		return nil, fmt.Errorf("failed to get warming room: %w", err)
	}

	return room, nil
}

// UpdateWarmingRoom updates existing room
func UpdateWarmingRoom(id string, req *UpdateWarmingRoomRequest) error {
	roomID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid room ID format: %w", err)
	}

	query := `
		UPDATE warming_rooms
		SET name = $1, interval_min_seconds = $2, interval_max_seconds = $3, updated_at = NOW()
		WHERE id = $4
	`

	result, err := database.AppDB.Exec(
		query,
		req.Name,
		req.IntervalMinSeconds,
		req.IntervalMaxSeconds,
		roomID,
	)
	if err != nil {
		return fmt.Errorf("failed to update warming room: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("warming room not found")
	}

	return nil
}

// DeleteWarmingRoom deletes room by ID (CASCADE to warming_logs)
func DeleteWarmingRoom(id string) error {
	roomID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid room ID format: %w", err)
	}

	query := `DELETE FROM warming_rooms WHERE id = $1`

	result, err := database.AppDB.Exec(query, roomID)
	if err != nil {
		return fmt.Errorf("failed to delete warming room: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("warming room not found")
	}

	return nil
}

// UpdateRoomStatus updates room status and related fields
func UpdateRoomStatus(id string, status string, nextRunAt *time.Time) error {
	roomID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid room ID format: %w", err)
	}

	var query string
	var args []interface{}

	if nextRunAt != nil {
		query = `
			UPDATE warming_rooms
			SET status = $1, next_run_at = $2, updated_at = NOW()
			WHERE id = $3
		`
		args = []interface{}{status, nextRunAt, roomID}
	} else {
		query = `
			UPDATE warming_rooms
			SET status = $1, next_run_at = NULL, updated_at = NOW()
			WHERE id = $2
		`
		args = []interface{}{status, roomID}
	}

	result, err := database.AppDB.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update room status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("warming room not found")
	}

	return nil
}

// ToWarmingRoomResponse converts WarmingRoom to response format
func ToWarmingRoomResponse(room WarmingRoom) WarmingRoomResponse {
	resp := WarmingRoomResponse{
		ID:                 room.ID.String(),
		Name:               room.Name,
		SenderInstanceID:   room.SenderInstanceID,
		ReceiverInstanceID: room.ReceiverInstanceID,
		ScriptID:           room.ScriptID,
		CurrentSequence:    room.CurrentSequence,
		Status:             room.Status,
		IntervalMinSeconds: room.IntervalMinSeconds,
		IntervalMaxSeconds: room.IntervalMaxSeconds,
		CreatedAt:          room.CreatedAt,
		UpdatedAt:          room.UpdatedAt,
	}

	if room.NextRunAt.Valid {
		resp.NextRunAt = &room.NextRunAt.Time
	}

	if room.LastRunAt.Valid {
		resp.LastRunAt = &room.LastRunAt.Time
	}

	return resp
}

// GetActiveRoomsForWorker retrieves rooms ready for execution
func GetActiveRoomsForWorker(limit int) ([]WarmingRoom, error) {
	query := `
		SELECT id, name, sender_instance_id, receiver_instance_id, script_id,
		       current_sequence, status, interval_min_seconds, interval_max_seconds,
		       next_run_at, last_run_at, created_at, updated_at
		FROM warming_rooms
		WHERE status = 'ACTIVE' AND next_run_at <= NOW()
		ORDER BY next_run_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := database.AppDB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query active rooms: %w", err)
	}
	defer rows.Close()

	var rooms []WarmingRoom
	for rows.Next() {
		var room WarmingRoom
		err := rows.Scan(
			&room.ID,
			&room.Name,
			&room.SenderInstanceID,
			&room.ReceiverInstanceID,
			&room.ScriptID,
			&room.CurrentSequence,
			&room.Status,
			&room.IntervalMinSeconds,
			&room.IntervalMaxSeconds,
			&room.NextRunAt,
			&room.LastRunAt,
			&room.CreatedAt,
			&room.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan room: %w", err)
		}
		rooms = append(rooms, room)
	}

	return rooms, nil
}

// UpdateRoomProgress updates room current sequence and next run time
func UpdateRoomProgress(roomID uuid.UUID, sequence int, nextRunAt time.Time) error {
	query := `
		UPDATE warming_rooms
		SET current_sequence = $1, next_run_at = $2, last_run_at = NOW(), updated_at = NOW()
		WHERE id = $3
	`

	_, err := database.AppDB.Exec(query, sequence, nextRunAt, roomID)
	return err
}

// FinishRoom marks room as finished
func FinishRoom(roomID uuid.UUID) error {
	query := `
		UPDATE warming_rooms
		SET status = 'FINISHED', next_run_at = NULL, updated_at = NOW()
		WHERE id = $1
	`

	_, err := database.AppDB.Exec(query, roomID)
	return err
}

// RestartRoom resets room to start from beginning
func RestartRoom(id string) error {
	query := `
		UPDATE warming_rooms
		SET current_sequence = 0,
		    status = 'ACTIVE',
		    next_run_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`

	result, err := database.AppDB.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to restart room: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("room not found")
	}

	return nil
}
