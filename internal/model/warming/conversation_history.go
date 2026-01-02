package warming

import (
	"fmt"
	"log"
	"time"

	"gowa-yourself/database"
	"gowa-yourself/internal/service/ai"

	"github.com/google/uuid"
)

// ConversationHistory represents a conversation log entry
type ConversationHistory struct {
	ID        int64
	RoomID    uuid.UUID
	Sender    string // "human" or "bot"
	Message   string
	CreatedAt time.Time
}

// GetConversationHistory retrieves last N messages for a room
func GetConversationHistory(roomID uuid.UUID, limit int) ([]ai.ConversationMessage, error) {
	query := `
		SELECT sender_type, message_content, executed_at 
		FROM warming_logs 
		WHERE room_id = $1 
		AND sender_type IN ('human', 'bot')
		ORDER BY executed_at DESC 
		LIMIT $2
	`

	rows, err := database.AppDB.Query(query, roomID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversation history: %w", err)
	}
	defer rows.Close()

	var messages []ai.ConversationMessage
	for rows.Next() {
		var msg ai.ConversationMessage
		var executedAt time.Time

		if err := rows.Scan(&msg.Sender, &msg.Message, &executedAt); err != nil {
			log.Printf("Warning: failed to scan conversation message: %v", err)
			continue
		}

		messages = append(messages, msg)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// SaveHumanMessage saves incoming human message to conversation history
func SaveHumanMessage(roomID uuid.UUID, instanceID, sender, message string) error {
	// This will be saved via CreateWarmingLog with sender_type='human'
	// Just a wrapper for clarity
	return CreateWarmingLog(roomID, 0, instanceID, sender, message, "SUCCESS", "", "human")
}
