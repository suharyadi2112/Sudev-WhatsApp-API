package warming

import (
	"fmt"
	"gowa-yourself/database"
	"log"

	"github.com/google/uuid"
)

// PauseConflictingRooms pauses other ACTIVE HUMAN_VS_BOT rooms with the same whitelisted number
// This is called before activating a room to prevent unique constraint violations
func PauseConflictingRooms(roomID uuid.UUID, whitelistedNumber string) error {
	if whitelistedNumber == "" {
		return nil // No conflict possible if no whitelisted number
	}

	query := `
		UPDATE warming_rooms 
		SET status = 'PAUSED', updated_at = NOW()
		WHERE room_type = 'HUMAN_VS_BOT'
		  AND status = 'ACTIVE'
		  AND whitelisted_number = $1
		  AND id != $2
	`

	result, err := database.AppDB.Exec(query, whitelistedNumber, roomID)
	if err != nil {
		return fmt.Errorf("failed to pause conflicting rooms: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("⚠️ Auto-paused %d conflicting room(s) with whitelisted number: %s", rowsAffected, whitelistedNumber)
	}

	return nil
}

// UpdateWarmingRoomStatus updates room status with conflict resolution
func UpdateWarmingRoomStatus(id, status string) error {
	roomID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid room ID format: %w", err)
	}

	// If activating a HUMAN_VS_BOT room, pause conflicting rooms first
	if status == "ACTIVE" {
		// Get room details to check if it's HUMAN_VS_BOT and get whitelisted number
		room, err := GetWarmingRoomByID(id)
		if err != nil {
			return fmt.Errorf("failed to get room details: %w", err)
		}

		if room.RoomType == "HUMAN_VS_BOT" && room.WhitelistedNumber.Valid {
			if err := PauseConflictingRooms(roomID, room.WhitelistedNumber.String); err != nil {
				return err
			}
		}
	}

	// Update the room status
	query := `
		UPDATE warming_rooms 
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := database.AppDB.Exec(query, status, roomID)
	if err != nil {
		return fmt.Errorf("failed to update room status: %w", err)
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
