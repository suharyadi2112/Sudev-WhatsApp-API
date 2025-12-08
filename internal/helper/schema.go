// internal/helper/schema.go
package helper

import (
	"log"

	"gowa-yourself/database"
)

func InitCustomSchema() {
	db := database.AppDB

	baseSchema := `
        CREATE TABLE IF NOT EXISTS instances (
            id                  SERIAL PRIMARY KEY,
            instance_id         VARCHAR(255)  NOT NULL UNIQUE,
            phone_number        VARCHAR(25),
            jid                 VARCHAR(255),

            status              VARCHAR(20)   NOT NULL DEFAULT 'created',
            is_connected        BOOLEAN       NOT NULL DEFAULT FALSE,

            name                VARCHAR(255),
            profile_picture     TEXT,
            about               TEXT,
            platform            VARCHAR(30),

            battery_level       INT,
            battery_charging    BOOLEAN       NOT NULL DEFAULT FALSE,

            qr_code             TEXT,
            qr_expires_at       TIMESTAMP(6) WITH TIME ZONE,

            created_at          TIMESTAMP(6) WITH TIME ZONE NOT NULL DEFAULT NOW(),
            connected_at        TIMESTAMP(6) WITH TIME ZONE,
            disconnected_at     TIMESTAMP(6) WITH TIME ZONE,
            last_seen           TIMESTAMP(6) WITH TIME ZONE,

            session_data        BYTEA
        );

        CREATE INDEX IF NOT EXISTS idx_instances_instance_id ON instances(instance_id);
        CREATE INDEX IF NOT EXISTS idx_instances_phone_number ON instances(phone_number);
        CREATE INDEX IF NOT EXISTS idx_instances_status ON instances(status);
    `
	if _, err := db.Exec(baseSchema); err != nil {
		log.Fatalf("failed to init base schema: %v", err)
	}

	alterSchema := `
        ALTER TABLE instances
        ADD COLUMN IF NOT EXISTS circle VARCHAR(255);

        ALTER TABLE instances
        ADD COLUMN IF NOT EXISTS webhook_url TEXT,
        ADD COLUMN IF NOT EXISTS webhook_secret TEXT;

        CREATE INDEX IF NOT EXISTS idx_instances_circle ON instances(circle);
    `
	if _, err := db.Exec(alterSchema); err != nil {
		log.Fatalf("failed to alter schema (add circle/webhook): %v", err)
	}

	log.Println("schema created/ensured successfully")
}
