package warming

import (
	"errors"
	"fmt"
	"strings"
	"time"

	warmingModel "gowa-yourself/internal/model/warming"
)

var (
	ErrRoomNameRequired     = errors.New("name is required")
	ErrRoomSenderRequired   = errors.New("sender_instance_id is required")
	ErrRoomReceiverRequired = errors.New("receiver_instance_id is required")
	ErrRoomScriptRequired   = errors.New("script_id is required")
	ErrRoomIntervalInvalid  = errors.New("interval_max_seconds must be >= interval_min_seconds")
	ErrRoomNotFound         = errors.New("warming room not found")
	ErrRoomAlreadyActive    = errors.New("room is already active")
	ErrRoomNotActive        = errors.New("room is not active")
	ErrRoomSameInstance     = errors.New("sender and receiver cannot be the same instance")
)

// CreateWarmingRoomService creates new room with validation
func CreateWarmingRoomService(req *warmingModel.CreateWarmingRoomRequest) (*warmingModel.WarmingRoom, error) {
	// Validate name
	if strings.TrimSpace(req.Name) == "" {
		return nil, ErrRoomNameRequired
	}

	// Validate sender
	if strings.TrimSpace(req.SenderInstanceID) == "" {
		return nil, ErrRoomSenderRequired
	}

	// Validate receiver
	if strings.TrimSpace(req.ReceiverInstanceID) == "" {
		return nil, ErrRoomReceiverRequired
	}

	// Validate not same instance
	if req.SenderInstanceID == req.ReceiverInstanceID {
		return nil, ErrRoomSameInstance
	}

	// Validate script
	if req.ScriptID <= 0 {
		return nil, ErrRoomScriptRequired
	}

	// Validate interval
	if req.IntervalMinSeconds <= 0 {
		req.IntervalMinSeconds = 5 // Default
	}
	if req.IntervalMaxSeconds <= 0 {
		req.IntervalMaxSeconds = 15 // Default
	}
	if req.IntervalMaxSeconds < req.IntervalMinSeconds {
		return nil, ErrRoomIntervalInvalid
	}

	// Check if script exists
	_, err := warmingModel.GetWarmingScriptByID(int(req.ScriptID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errors.New("script not found")
		}
		return nil, fmt.Errorf("failed to verify script: %w", err)
	}

	// Create in database
	room, err := warmingModel.CreateWarmingRoom(req)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	return room, nil
}

// GetAllWarmingRoomsService retrieves all rooms with optional filter
func GetAllWarmingRoomsService(status string) ([]warmingModel.WarmingRoom, error) {
	// Validate status if provided
	if status != "" {
		validStatuses := map[string]bool{
			"STOPPED":  true,
			"ACTIVE":   true,
			"PAUSED":   true,
			"FINISHED": true,
		}
		if !validStatuses[status] {
			return nil, fmt.Errorf("invalid status: must be STOPPED, ACTIVE, PAUSED, or FINISHED")
		}
	}

	rooms, err := warmingModel.GetAllWarmingRooms(status)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	return rooms, nil
}

// GetWarmingRoomByIDService retrieves single room by ID
func GetWarmingRoomByIDService(id string) (*warmingModel.WarmingRoom, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("invalid room ID")
	}

	room, err := warmingModel.GetWarmingRoomByID(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrRoomNotFound
		}
		return nil, fmt.Errorf("service: %w", err)
	}

	return room, nil
}

// UpdateWarmingRoomService updates existing room with validation
func UpdateWarmingRoomService(id string, req *warmingModel.UpdateWarmingRoomRequest) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("invalid room ID")
	}

	// Validate name
	if strings.TrimSpace(req.Name) == "" {
		return ErrRoomNameRequired
	}

	// Validate script
	if req.ScriptID <= 0 {
		return ErrRoomScriptRequired
	}

	// Check if script exists
	_, err := warmingModel.GetWarmingScriptByID(int(req.ScriptID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.New("script not found")
		}
		return fmt.Errorf("failed to verify script: %w", err)
	}

	// Validate interval
	if req.IntervalMinSeconds <= 0 {
		req.IntervalMinSeconds = 5
	}
	if req.IntervalMaxSeconds <= 0 {
		req.IntervalMaxSeconds = 15
	}
	if req.IntervalMaxSeconds < req.IntervalMinSeconds {
		return ErrRoomIntervalInvalid
	}

	// Update in database
	err = warmingModel.UpdateWarmingRoom(id, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrRoomNotFound
		}
		return fmt.Errorf("service: %w", err)
	}

	return nil
}

// DeleteWarmingRoomService deletes room by ID
func DeleteWarmingRoomService(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("invalid room ID")
	}

	err := warmingModel.DeleteWarmingRoom(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrRoomNotFound
		}
		return fmt.Errorf("service: %w", err)
	}

	return nil
}

// UpdateRoomStatusService updates room status with validation
func UpdateRoomStatusService(id string, newStatus string) error {
	// Validate status
	validStatuses := map[string]bool{
		"STOPPED":  true,
		"ACTIVE":   true,
		"PAUSED":   true,
		"FINISHED": true,
	}
	if !validStatuses[newStatus] {
		return fmt.Errorf("invalid status: must be STOPPED, ACTIVE, PAUSED, or FINISHED")
	}

	room, err := GetWarmingRoomByIDService(id)
	if err != nil {
		return err
	}

	// Validate status transitions
	if room.Status == newStatus {
		return fmt.Errorf("room is already in %s status", newStatus)
	}

	// Calculate next_run_at for ACTIVE status
	var nextRunAt *time.Time
	if newStatus == "ACTIVE" {
		nextRun := time.Now().Add(time.Duration(room.IntervalMinSeconds) * time.Second)
		nextRunAt = &nextRun
	}

	err = warmingModel.UpdateRoomStatus(id, newStatus, nextRunAt)
	if err != nil {
		return fmt.Errorf("failed to update room status: %w", err)
	}

	return nil
}

// RestartRoomService restarts a room from beginning
func RestartRoomService(id string) error {
	if id == "" {
		return errors.New("invalid room ID")
	}

	// Check if room exists
	_, err := warmingModel.GetWarmingRoomByID(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrRoomNotFound
		}
		return fmt.Errorf("failed to verify room: %w", err)
	}

	// Restart room
	err = warmingModel.RestartRoom(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrRoomNotFound
		}
		return fmt.Errorf("service: %w", err)
	}

	return nil
}
