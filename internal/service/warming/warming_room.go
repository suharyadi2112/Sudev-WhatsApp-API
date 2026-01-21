package warming

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gowa-yourself/internal/model"
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
func CreateWarmingRoomService(req *warmingModel.CreateWarmingRoomRequest, userID int64) (*warmingModel.WarmingRoom, error) {
	// Validate name
	if strings.TrimSpace(req.Name) == "" {
		return nil, ErrRoomNameRequired
	}

	// Set default room_type if not provided
	if req.RoomType == "" {
		req.RoomType = "BOT_VS_BOT"
	}

	// Validate room_type
	if req.RoomType != "BOT_VS_BOT" && req.RoomType != "HUMAN_VS_BOT" {
		return nil, errors.New("invalid room_type: must be 'BOT_VS_BOT' or 'HUMAN_VS_BOT'")
	}

	// BOT_VS_BOT specific validation
	if req.RoomType == "BOT_VS_BOT" {
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

		// Validate sender instance exists, online, and available
		senderInstance, err := model.GetInstanceByInstanceID(req.SenderInstanceID)
		if err != nil {
			return nil, fmt.Errorf("sender instance not found: %s", req.SenderInstanceID)
		}
		if senderInstance.Status != "online" {
			return nil, fmt.Errorf("sender instance '%s' is not online (status: %s)", req.SenderInstanceID, senderInstance.Status)
		}

		// Validate receiver instance exists, online, and available
		receiverInstance, err := model.GetInstanceByInstanceID(req.ReceiverInstanceID)
		if err != nil {
			return nil, fmt.Errorf("receiver instance not found: %s", req.ReceiverInstanceID)
		}
		if receiverInstance.Status != "online" {
			return nil, fmt.Errorf("receiver instance '%s' is not online (status: %s)", req.ReceiverInstanceID, receiverInstance.Status)
		}

	}

	// HUMAN_VS_BOT specific validation
	if req.RoomType == "HUMAN_VS_BOT" {
		// Validate sender (the bot that will auto-reply)
		if strings.TrimSpace(req.SenderInstanceID) == "" {
			return nil, ErrRoomSenderRequired
		}

		// Validate whitelisted number
		if strings.TrimSpace(req.WhitelistedNumber) == "" {
			return nil, errors.New("whitelisted_number is required for HUMAN_VS_BOT")
		}

		// Validate sender instance exists, online, and available
		senderInstance, err := model.GetInstanceByInstanceID(req.SenderInstanceID)
		if err != nil {
			return nil, fmt.Errorf("sender instance not found: %s", req.SenderInstanceID)
		}
		if senderInstance.Status != "online" {
			return nil, fmt.Errorf("sender instance '%s' is not online (status: %s)", req.SenderInstanceID, senderInstance.Status)
		}

		// Set default reply delays if not provided
		if req.ReplyDelayMin <= 0 {
			req.ReplyDelayMin = 10
		}
		if req.ReplyDelayMax <= 0 {
			req.ReplyDelayMax = 60
		}
		if req.ReplyDelayMax < req.ReplyDelayMin {
			return nil, errors.New("reply_delay_max must be >= reply_delay_min")
		}

		// Receiver not needed for HUMAN_VS_BOT (human is the receiver)
		req.ReceiverInstanceID = ""
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
	room, err := warmingModel.CreateWarmingRoom(req, userID)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	return room, nil
}

// GetAllWarmingRoomsService retrieves all rooms with optional filter
func GetAllWarmingRoomsService(status string, userID int64, isAdmin bool) ([]warmingModel.WarmingRoom, error) {
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

	rooms, err := warmingModel.GetAllWarmingRooms(status, userID, isAdmin)
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

	// Get existing room to check room_type
	existingRoom, err := warmingModel.GetWarmingRoomByID(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrRoomNotFound
		}
		return fmt.Errorf("failed to get existing room: %w", err)
	}

	// Prevent room_type changes (immutable after creation)
	if req.RoomType != "" && req.RoomType != existingRoom.RoomType {
		return errors.New("room_type cannot be changed after creation. Please create a new room instead")
	}

	// Use existing room_type if not provided in request
	if req.RoomType == "" {
		req.RoomType = existingRoom.RoomType
	}

	// Validate room_type
	if req.RoomType != "BOT_VS_BOT" && req.RoomType != "HUMAN_VS_BOT" {
		return errors.New("invalid room_type: must be 'BOT_VS_BOT' or 'HUMAN_VS_BOT'")
	}

	// HUMAN_VS_BOT specific validation
	if req.RoomType == "HUMAN_VS_BOT" {
		// Validate whitelisted number
		if strings.TrimSpace(req.WhitelistedNumber) == "" {
			return errors.New("whitelisted_number is required for HUMAN_VS_BOT")
		}

		// Validate reply delays
		if req.ReplyDelayMin <= 0 {
			req.ReplyDelayMin = 10
		}
		if req.ReplyDelayMax <= 0 {
			req.ReplyDelayMax = 60
		}
		if req.ReplyDelayMax < req.ReplyDelayMin {
			return errors.New("reply_delay_max must be >= reply_delay_min")
		}
	}

	// Validate script
	if req.ScriptID <= 0 {
		return ErrRoomScriptRequired
	}

	// Check if script exists
	_, err = warmingModel.GetWarmingScriptByID(int(req.ScriptID))
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
