package model

import (
	"gowa-yourself/database"
)

// IncrementMessageCount increments the daily message count for an instance
func IncrementMessageCount(instanceID string) error {
	query := `
		INSERT INTO instance_message_stats (instance_id, stat_date, message_count, updated_at)
		VALUES ($1, CURRENT_DATE, 1, NOW())
		ON CONFLICT (instance_id, stat_date)
		DO UPDATE SET 
			message_count = instance_message_stats.message_count + 1,
			updated_at = EXCLUDED.updated_at
	`
	_, err := database.AppDB.Exec(query, instanceID)
	return err
}
