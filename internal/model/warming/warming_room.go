package warming

import (
	"database/sql"
	"fmt"
	"gowa-yourself/database"
	"strings"
	"time"

	"github.com/google/uuid"
)

// WarmingRoom represents warming_rooms table
type WarmingRoom struct {
	ID                 uuid.UUID
	Name               string
	SenderInstanceID   string
	ReceiverInstanceID string
	ScriptID           int64 // Mandatory: required for all room types
	CurrentSequence    int
	Status             string // STOPPED, ACTIVE, PAUSED, FINISHED
	IntervalMinSeconds int
	IntervalMaxSeconds int
	SendRealMessage    bool
	RoomType           string
	WhitelistedNumber  sql.NullString
	ReplyDelayMin      int
	ReplyDelayMax      int
	// AI Configuration
	AIEnabled        bool
	AIProvider       string
	AIModel          string
	AISystemPrompt   string
	AITemperature    float64
	AIMaxTokens      int
	FallbackToScript bool
	NextRunAt        sql.NullTime
	LastRunAt        sql.NullTime
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// WarmingRoomResponse for JSON response
type WarmingRoomResponse struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	SenderInstanceID   string `json:"senderInstanceId"`
	ReceiverInstanceID string `json:"receiverInstanceId"`
	ScriptID           int64  `json:"scriptId"`
	CurrentSequence    int    `json:"currentSequence"`
	Status             string `json:"status"`
	IntervalMinSeconds int    `json:"intervalMinSeconds"`
	IntervalMaxSeconds int    `json:"intervalMaxSeconds"`
	SendRealMessage    bool   `json:"sendRealMessage"`
	RoomType           string `json:"roomType"`
	WhitelistedNumber  string `json:"whitelistedNumber,omitempty"`
	ReplyDelayMin      int    `json:"replyDelayMin"`
	ReplyDelayMax      int    `json:"replyDelayMax"`
	// AI Configuration (New)
	AIEnabled        bool       `json:"aiEnabled"`
	AIProvider       string     `json:"aiProvider,omitempty"`
	AIModel          string     `json:"aiModel,omitempty"`
	AISystemPrompt   string     `json:"aiSystemPrompt,omitempty"`
	AITemperature    float64    `json:"aiTemperature,omitempty"`
	AIMaxTokens      int        `json:"aiMaxTokens,omitempty"`
	FallbackToScript bool       `json:"fallbackToScript"`
	NextRunAt        *time.Time `json:"nextRunAt"`
	LastRunAt        *time.Time `json:"lastRunAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// CreateWarmingRoomRequest for POST request
type CreateWarmingRoomRequest struct {
	Name               string `json:"name"`
	SenderInstanceID   string `json:"senderInstanceId"`
	ReceiverInstanceID string `json:"receiverInstanceId"`
	ScriptID           int64  `json:"scriptId"`
	IntervalMinSeconds int    `json:"intervalMinSeconds"`
	IntervalMaxSeconds int    `json:"intervalMaxSeconds"`
	SendRealMessage    bool   `json:"sendRealMessage"`
	RoomType           string `json:"roomType,omitempty"`
	WhitelistedNumber  string `json:"whitelistedNumber,omitempty"`
	ReplyDelayMin      int    `json:"replyDelayMin,omitempty"`
	ReplyDelayMax      int    `json:"replyDelayMax,omitempty"`
	// AI Configuration
	AIEnabled        bool    `json:"aiEnabled,omitempty"`
	AIProvider       string  `json:"aiProvider,omitempty"`
	AIModel          string  `json:"aiModel,omitempty"`
	AISystemPrompt   string  `json:"aiSystemPrompt,omitempty"`
	AITemperature    float64 `json:"aiTemperature,omitempty"`
	AIMaxTokens      int     `json:"aiMaxTokens,omitempty"`
	FallbackToScript bool    `json:"fallbackToScript,omitempty"`
}

// UpdateWarmingRoomRequest for PUT request
type UpdateWarmingRoomRequest struct {
	Name               string `json:"name"`
	ScriptID           int64  `json:"scriptId"`
	IntervalMinSeconds int    `json:"intervalMinSeconds"`
	IntervalMaxSeconds int    `json:"intervalMaxSeconds"`
	SendRealMessage    bool   `json:"sendRealMessage"`
	RoomType           string `json:"roomType,omitempty"`
	WhitelistedNumber  string `json:"whitelistedNumber,omitempty"`
	ReplyDelayMin      int    `json:"replyDelayMin,omitempty"`
	ReplyDelayMax      int    `json:"replyDelayMax,omitempty"`
	// AI Configuration
	AIEnabled        *bool    `json:"aiEnabled,omitempty"`
	AIProvider       string   `json:"aiProvider,omitempty"`
	AIModel          string   `json:"aiModel,omitempty"`
	AISystemPrompt   string   `json:"aiSystemPrompt,omitempty"`
	AITemperature    *float64 `json:"aiTemperature,omitempty"`
	AIMaxTokens      *int     `json:"aiMaxTokens,omitempty"`
	FallbackToScript *bool    `json:"fallbackToScript,omitempty"`
}

// CheckDuplicateWhitelistedNumber checks if whitelisted number is already used in another active HUMAN_VS_BOT room
func CheckDuplicateWhitelistedNumber(whitelistedNumber string, excludeRoomID *uuid.UUID) (bool, error) {
	if whitelistedNumber == "" {
		return false, nil
	}

	var query string
	var args []interface{}

	if excludeRoomID != nil {
		query = `
			SELECT COUNT(*) FROM warming_rooms
			WHERE room_type = 'HUMAN_VS_BOT'
			  AND whitelisted_number = $1
			  AND status IN ('ACTIVE', 'PAUSED')
			  AND id != $2
		`
		args = []interface{}{whitelistedNumber, excludeRoomID}
	} else {
		query = `
			SELECT COUNT(*) FROM warming_rooms
			WHERE room_type = 'HUMAN_VS_BOT'
			  AND whitelisted_number = $1
			  AND status IN ('ACTIVE', 'PAUSED')
		`
		args = []interface{}{whitelistedNumber}
	}

	var count int
	err := database.AppDB.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check duplicate whitelisted number: %w", err)
	}

	return count > 0, nil
}

// CreateWarmingRoom inserts new room
func CreateWarmingRoom(req *CreateWarmingRoomRequest) (*WarmingRoom, error) {
	// Normalize whitelisted number for HUMAN_VS_BOT rooms (08xxx -> 628xxx)
	if req.RoomType == "HUMAN_VS_BOT" && req.WhitelistedNumber != "" {
		// Use existing FormatPhoneNumber logic to normalize
		cleaned := strings.ReplaceAll(req.WhitelistedNumber, " ", "")
		cleaned = strings.ReplaceAll(cleaned, "-", "")
		cleaned = strings.TrimPrefix(cleaned, "+")

		// Convert 08xxx to 628xxx
		if strings.HasPrefix(cleaned, "08") {
			cleaned = "62" + cleaned[1:]
		} else if strings.HasPrefix(cleaned, "8") && !strings.HasPrefix(cleaned, "62") {
			cleaned = "62" + cleaned
		}

		req.WhitelistedNumber = cleaned

		// Check for duplicate whitelisted number
		isDuplicate, err := CheckDuplicateWhitelistedNumber(req.WhitelistedNumber, nil)
		if err != nil {
			return nil, err
		}
		if isDuplicate {
			return nil, fmt.Errorf("whitelisted number %s is already used in another active HUMAN_VS_BOT room", req.WhitelistedNumber)
		}
	}

	query := `
		INSERT INTO warming_rooms 
		(name, sender_instance_id, receiver_instance_id, script_id,
		 interval_min_seconds, interval_max_seconds, send_real_message,
		 room_type, whitelisted_number, reply_delay_min, reply_delay_max,
		 ai_enabled, ai_provider, ai_model, ai_system_prompt, ai_temperature, ai_max_tokens, fallback_to_script,
		 created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW(), NOW())
		RETURNING id, name, sender_instance_id, receiver_instance_id, script_id,
		          current_sequence, status, interval_min_seconds, interval_max_seconds, send_real_message,
		          room_type, whitelisted_number, reply_delay_min, reply_delay_max,
		          ai_enabled, ai_provider, ai_model, ai_system_prompt, ai_temperature, ai_max_tokens, fallback_to_script,
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
		req.SendRealMessage,
		req.RoomType,
		sql.NullString{String: req.WhitelistedNumber, Valid: req.WhitelistedNumber != ""},
		req.ReplyDelayMin,
		req.ReplyDelayMax,
		req.AIEnabled,
		req.AIProvider,
		req.AIModel,
		req.AISystemPrompt,
		req.AITemperature,
		req.AIMaxTokens,
		req.FallbackToScript,
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
		&room.SendRealMessage,
		&room.RoomType,
		&room.WhitelistedNumber,
		&room.ReplyDelayMin,
		&room.ReplyDelayMax,
		&room.AIEnabled,
		&room.AIProvider,
		&room.AIModel,
		&room.AISystemPrompt,
		&room.AITemperature,
		&room.AIMaxTokens,
		&room.FallbackToScript,
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
			       current_sequence, status, interval_min_seconds, interval_max_seconds, send_real_message,
			       room_type, whitelisted_number, reply_delay_min, reply_delay_max,
			       ai_enabled, ai_provider, ai_model, ai_system_prompt, ai_temperature, ai_max_tokens, fallback_to_script,
			       next_run_at, last_run_at, created_at, updated_at
			FROM warming_rooms
			WHERE status = $1
			ORDER BY created_at DESC
		`
		args = append(args, status)
	} else {
		query = `
			SELECT id, name, sender_instance_id, receiver_instance_id, script_id,
			       current_sequence, status, interval_min_seconds, interval_max_seconds, send_real_message,
			       room_type, whitelisted_number, reply_delay_min, reply_delay_max,
			       ai_enabled, ai_provider, ai_model, ai_system_prompt, ai_temperature, ai_max_tokens, fallback_to_script,
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
			&room.SendRealMessage,
			&room.RoomType,
			&room.WhitelistedNumber,
			&room.ReplyDelayMin,
			&room.ReplyDelayMax,
			&room.AIEnabled,
			&room.AIProvider,
			&room.AIModel,
			&room.AISystemPrompt,
			&room.AITemperature,
			&room.AIMaxTokens,
			&room.FallbackToScript,
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
		       current_sequence, status, interval_min_seconds, interval_max_seconds, send_real_message,
		       room_type, whitelisted_number, reply_delay_min, reply_delay_max,
		       ai_enabled, ai_provider, ai_model, ai_system_prompt, ai_temperature, ai_max_tokens, fallback_to_script,
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
		&room.SendRealMessage,
		&room.RoomType,
		&room.WhitelistedNumber,
		&room.ReplyDelayMin,
		&room.ReplyDelayMax,
		&room.AIEnabled,
		&room.AIProvider,
		&room.AIModel,
		&room.AISystemPrompt,
		&room.AITemperature,
		&room.AIMaxTokens,
		&room.FallbackToScript,
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

	// Check for duplicate whitelisted number if updating HUMAN_VS_BOT room
	if req.RoomType == "HUMAN_VS_BOT" && req.WhitelistedNumber != "" {
		isDuplicate, err := CheckDuplicateWhitelistedNumber(req.WhitelistedNumber, &roomID)
		if err != nil {
			return err
		}
		if isDuplicate {
			return fmt.Errorf("whitelisted number %s is already used in another active HUMAN_VS_BOT room", req.WhitelistedNumber)
		}
	}

	query := `
		UPDATE warming_rooms
		SET name = $1, script_id = $2, interval_min_seconds = $3, interval_max_seconds = $4, send_real_message = $5,
		    room_type = $6, whitelisted_number = $7, reply_delay_min = $8, reply_delay_max = $9,
		    ai_enabled = $10, ai_provider = $11, ai_model = $12, ai_system_prompt = $13,
		    ai_temperature = $14, ai_max_tokens = $15, fallback_to_script = $16,
		    updated_at = NOW()
		WHERE id = $17
	`

	// Handle nullable AI fields with defaults
	aiEnabled := false
	if req.AIEnabled != nil {
		aiEnabled = *req.AIEnabled
	}

	aiTemperature := 0.7
	if req.AITemperature != nil {
		aiTemperature = *req.AITemperature
	}

	aiMaxTokens := 150
	if req.AIMaxTokens != nil {
		aiMaxTokens = *req.AIMaxTokens
	}

	fallbackToScript := true
	if req.FallbackToScript != nil {
		fallbackToScript = *req.FallbackToScript
	}

	result, err := database.AppDB.Exec(
		query,
		req.Name,
		req.ScriptID,
		req.IntervalMinSeconds,
		req.IntervalMaxSeconds,
		req.SendRealMessage,
		req.RoomType,
		sql.NullString{String: req.WhitelistedNumber, Valid: req.WhitelistedNumber != ""},
		req.ReplyDelayMin,
		req.ReplyDelayMax,
		aiEnabled,
		req.AIProvider,
		req.AIModel,
		req.AISystemPrompt,
		aiTemperature,
		aiMaxTokens,
		fallbackToScript,
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

	// If activating a HUMAN_VS_BOT room, check for duplicate whitelisted number
	if status == "ACTIVE" {
		room, err := GetWarmingRoomByID(id)
		if err != nil {
			return fmt.Errorf("failed to get room: %w", err)
		}

		if room.RoomType == "HUMAN_VS_BOT" && room.WhitelistedNumber.Valid && room.WhitelistedNumber.String != "" {
			isDuplicate, err := CheckDuplicateWhitelistedNumber(room.WhitelistedNumber.String, &roomID)
			if err != nil {
				return err
			}
			if isDuplicate {
				return fmt.Errorf("cannot activate room: whitelisted number %s is already used in another active HUMAN_VS_BOT room", room.WhitelistedNumber.String)
			}
		}
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

func ToWarmingRoomResponse(room WarmingRoom) WarmingRoomResponse {
	var nextRunAt *time.Time
	if room.NextRunAt.Valid {
		nextRunAt = &room.NextRunAt.Time
	}

	var lastRunAt *time.Time
	if room.LastRunAt.Valid {
		lastRunAt = &room.LastRunAt.Time
	}

	var whitelistedNumber string
	if room.WhitelistedNumber.Valid {
		whitelistedNumber = room.WhitelistedNumber.String
	}

	return WarmingRoomResponse{
		ID:                 room.ID.String(),
		Name:               room.Name,
		SenderInstanceID:   room.SenderInstanceID,
		ReceiverInstanceID: room.ReceiverInstanceID,
		ScriptID:           room.ScriptID,
		CurrentSequence:    room.CurrentSequence,
		Status:             room.Status,
		IntervalMinSeconds: room.IntervalMinSeconds,
		IntervalMaxSeconds: room.IntervalMaxSeconds,
		SendRealMessage:    room.SendRealMessage,
		RoomType:           room.RoomType,
		WhitelistedNumber:  whitelistedNumber,
		ReplyDelayMin:      room.ReplyDelayMin,
		ReplyDelayMax:      room.ReplyDelayMax,
		// AI Configuration (New)
		AIEnabled:        room.AIEnabled,
		AIProvider:       room.AIProvider,
		AIModel:          room.AIModel,
		AISystemPrompt:   room.AISystemPrompt,
		AITemperature:    room.AITemperature,
		AIMaxTokens:      room.AIMaxTokens,
		FallbackToScript: room.FallbackToScript,
		NextRunAt:        nextRunAt,
		LastRunAt:        lastRunAt,
		CreatedAt:        room.CreatedAt,
		UpdatedAt:        room.UpdatedAt,
	}
}

// GetActiveRoomsForWorker retrieves rooms ready for execution
func GetActiveRoomsForWorker(limit int) ([]WarmingRoom, error) {
	query := `
		SELECT id, name, sender_instance_id, receiver_instance_id, script_id,
		       current_sequence, status, interval_min_seconds, interval_max_seconds, send_real_message,
		       room_type, whitelisted_number, reply_delay_min, reply_delay_max,
		       ai_enabled, ai_provider, ai_model, ai_system_prompt, ai_temperature, ai_max_tokens, fallback_to_script,
		       next_run_at, last_run_at, created_at, updated_at
		FROM warming_rooms
		WHERE status = 'ACTIVE' 
		  AND room_type != 'HUMAN_VS_BOT'
		  AND next_run_at <= NOW()
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
			&room.SendRealMessage,
			&room.RoomType,
			&room.WhitelistedNumber,
			&room.ReplyDelayMin,
			&room.ReplyDelayMax,
			&room.AIEnabled,
			&room.AIProvider,
			&room.AIModel,
			&room.AISystemPrompt,
			&room.AITemperature,
			&room.AIMaxTokens,
			&room.FallbackToScript,
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

func GetActiveHumanRoomBySender(senderNumber string) (*WarmingRoom, error) {
	query := `
		SELECT id, name, sender_instance_id, receiver_instance_id, script_id,
		       current_sequence, status, interval_min_seconds, interval_max_seconds, send_real_message,
		       room_type, whitelisted_number, reply_delay_min, reply_delay_max,
		       ai_enabled, ai_provider, ai_model, ai_system_prompt, ai_temperature, ai_max_tokens, fallback_to_script,
		       next_run_at, last_run_at, created_at, updated_at
		FROM warming_rooms
		WHERE room_type = 'HUMAN_VS_BOT' 
		  AND status = 'ACTIVE' 
		  AND whitelisted_number = $1
		LIMIT 1
	`

	room := &WarmingRoom{}
	err := database.AppDB.QueryRow(query, senderNumber).Scan(
		&room.ID,
		&room.Name,
		&room.SenderInstanceID,
		&room.ReceiverInstanceID,
		&room.ScriptID,
		&room.CurrentSequence,
		&room.Status,
		&room.IntervalMinSeconds,
		&room.IntervalMaxSeconds,
		&room.SendRealMessage,
		&room.RoomType,
		&room.WhitelistedNumber,
		&room.ReplyDelayMin,
		&room.ReplyDelayMax,
		&room.AIEnabled,
		&room.AIProvider,
		&room.AIModel,
		&room.AISystemPrompt,
		&room.AITemperature,
		&room.AIMaxTokens,
		&room.FallbackToScript,
		&room.NextRunAt,
		&room.LastRunAt,
		&room.CreatedAt,
		&room.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active human room: %w", err)
	}

	return room, nil
}
