package model

import (
	"fmt"
	"gowa-yourself/database"
)

func UpdateInstanceWebhook(instanceID, url, secret string) error {
	res, err := database.AppDB.Exec(`
        UPDATE instances
        SET webhook_url   = $1,
            webhook_secret = $2
        WHERE instance_id = $3
    `, url, secret, instanceID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		// instance_id tidak ditemukan
		return fmt.Errorf("instance %s not found", instanceID)
	}

	return nil
}
