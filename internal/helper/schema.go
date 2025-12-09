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

	// WhatsApp Warming System Schema
	warmingSchema := `
        -- =====================================================
        -- Table: warming_scripts
        -- Purpose: Header/template untuk naskah percakapan
        -- =====================================================
        CREATE TABLE IF NOT EXISTS warming_scripts (
            id              SERIAL PRIMARY KEY,
            title           VARCHAR(255) NOT NULL,
            description     TEXT,
            category        VARCHAR(100),
            created_at      TIMESTAMP(6) WITH TIME ZONE NOT NULL DEFAULT NOW(),
            updated_at      TIMESTAMP(6) WITH TIME ZONE NOT NULL DEFAULT NOW()
        );

        COMMENT ON TABLE warming_scripts IS 'Master template untuk naskah percakapan warming';
        COMMENT ON COLUMN warming_scripts.title IS 'Judul template, contoh: Jual Beli Motor, Tanya Kabar';
        COMMENT ON COLUMN warming_scripts.category IS 'Kategori untuk grouping script, contoh: casual, business, support';

        -- =====================================================
        -- Table: warming_script_lines
        -- Purpose: Menyimpan urutan dialog untuk setiap script
        -- =====================================================
        CREATE TABLE IF NOT EXISTS warming_script_lines (
            id                      SERIAL PRIMARY KEY,
            script_id               INT NOT NULL,
            sequence_order          INT NOT NULL,
            actor_role              VARCHAR(20) NOT NULL CHECK (actor_role IN ('ACTOR_A', 'ACTOR_B')),
            message_content         TEXT NOT NULL,
            typing_duration_sec     INT NOT NULL DEFAULT 3,
            created_at              TIMESTAMP(6) WITH TIME ZONE NOT NULL DEFAULT NOW(),
            
            CONSTRAINT fk_script_lines_script 
                FOREIGN KEY (script_id) 
                REFERENCES warming_scripts(id) 
                ON DELETE CASCADE,
            
            CONSTRAINT unique_script_sequence 
                UNIQUE (script_id, sequence_order)
        );

        COMMENT ON TABLE warming_script_lines IS 'Detail baris dialog untuk setiap script';
        COMMENT ON COLUMN warming_script_lines.sequence_order IS 'Urutan chat: 1, 2, 3, dst';
        COMMENT ON COLUMN warming_script_lines.actor_role IS 'Siapa yang mengirim pesan: ACTOR_A (sender) atau ACTOR_B (receiver)';
        COMMENT ON COLUMN warming_script_lines.message_content IS 'Teks format Spintax, contoh: {Halo|Pagi}, barang {ready|ada}?';
        COMMENT ON COLUMN warming_script_lines.typing_duration_sec IS 'Simulasi lama waktu "sedang mengetik..." sebelum pesan dikirim';

        CREATE INDEX IF NOT EXISTS idx_script_lines_script_id ON warming_script_lines(script_id);
        CREATE INDEX IF NOT EXISTS idx_script_lines_script_sequence ON warming_script_lines(script_id, sequence_order);

        -- =====================================================
        -- Table: warming_rooms
        -- Purpose: Wadah eksekusi yang memasangkan 2 instance
        -- =====================================================
        CREATE TABLE IF NOT EXISTS warming_rooms (
            id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name                    VARCHAR(255) NOT NULL,
            sender_instance_id      VARCHAR(255) NOT NULL,
            receiver_instance_id    VARCHAR(255) NOT NULL,
            script_id               INT NOT NULL,
            current_sequence        INT NOT NULL DEFAULT 0,
            status                  VARCHAR(20) NOT NULL DEFAULT 'STOPPED' 
                                    CHECK (status IN ('STOPPED', 'ACTIVE', 'PAUSED', 'FINISHED')),
            interval_min_seconds    INT NOT NULL DEFAULT 5,
            interval_max_seconds    INT NOT NULL DEFAULT 15,
            next_run_at             TIMESTAMP(6) WITH TIME ZONE,
            last_run_at             TIMESTAMP(6) WITH TIME ZONE,
            created_at              TIMESTAMP(6) WITH TIME ZONE NOT NULL DEFAULT NOW(),
            updated_at              TIMESTAMP(6) WITH TIME ZONE NOT NULL DEFAULT NOW(),
            
            CONSTRAINT fk_rooms_script 
                FOREIGN KEY (script_id) 
                REFERENCES warming_scripts(id) 
                ON DELETE RESTRICT,
            
            CONSTRAINT check_interval_range 
                CHECK (interval_max_seconds >= interval_min_seconds)
        );

        COMMENT ON TABLE warming_rooms IS 'Room eksekusi yang memasangkan dua instance WhatsApp untuk menjalankan script';
        COMMENT ON COLUMN warming_rooms.sender_instance_id IS 'ID instance WhatsApp pertama (Aktor A / Pengirim)';
        COMMENT ON COLUMN warming_rooms.receiver_instance_id IS 'ID instance WhatsApp kedua (Aktor B / Penerima)';
        COMMENT ON COLUMN warming_rooms.current_sequence IS 'Pointer: sedang di baris naskah nomor berapa';
        COMMENT ON COLUMN warming_rooms.status IS 'Status room: STOPPED, ACTIVE, PAUSED, FINISHED';
        COMMENT ON COLUMN warming_rooms.interval_min_seconds IS 'Jeda waktu minimal antar pesan (detik)';
        COMMENT ON COLUMN warming_rooms.interval_max_seconds IS 'Jeda waktu maksimal antar pesan (detik, untuk variasi acak)';
        COMMENT ON COLUMN warming_rooms.next_run_at IS 'Waktu jadwal eksekusi berikutnya (penting untuk Cron/Worker)';

        -- Indexes untuk performa query worker
        CREATE INDEX IF NOT EXISTS idx_rooms_status ON warming_rooms(status);
        CREATE INDEX IF NOT EXISTS idx_rooms_next_run ON warming_rooms(next_run_at);
        
        -- Composite index untuk query worker: WHERE status = 'ACTIVE' AND next_run_at <= NOW()
        CREATE INDEX IF NOT EXISTS idx_rooms_status_next_run ON warming_rooms(status, next_run_at);
        
        CREATE INDEX IF NOT EXISTS idx_rooms_script_id ON warming_rooms(script_id);
        CREATE INDEX IF NOT EXISTS idx_rooms_sender_instance ON warming_rooms(sender_instance_id);
        CREATE INDEX IF NOT EXISTS idx_rooms_receiver_instance ON warming_rooms(receiver_instance_id);

        -- =====================================================
        -- Table: warming_logs
        -- Purpose: History eksekusi warming untuk audit trail
        -- =====================================================
        CREATE TABLE IF NOT EXISTS warming_logs (
            id                      BIGSERIAL PRIMARY KEY,
            room_id                 UUID NOT NULL,
            script_line_id          INT,
            sender_instance_id      VARCHAR(255) NOT NULL,
            receiver_instance_id    VARCHAR(255) NOT NULL,
            message_content         TEXT NOT NULL,
            status                  VARCHAR(20) NOT NULL CHECK (status IN ('SUCCESS', 'FAILED')),
            error_message           TEXT,
            executed_at             TIMESTAMP(6) WITH TIME ZONE NOT NULL DEFAULT NOW(),
            
            CONSTRAINT fk_logs_room 
                FOREIGN KEY (room_id) 
                REFERENCES warming_rooms(id) 
                ON DELETE CASCADE,
            
            CONSTRAINT fk_logs_script_line 
                FOREIGN KEY (script_line_id) 
                REFERENCES warming_script_lines(id) 
                ON DELETE SET NULL
        );

        COMMENT ON TABLE warming_logs IS 'History eksekusi warming untuk audit trail dan debugging';
        COMMENT ON COLUMN warming_logs.room_id IS 'Reference ke room yang menjalankan eksekusi';
        COMMENT ON COLUMN warming_logs.script_line_id IS 'Reference ke baris naskah yang dieksekusi (nullable jika line sudah dihapus)';
        COMMENT ON COLUMN warming_logs.sender_instance_id IS 'Snapshot ID pengirim saat eksekusi';
        COMMENT ON COLUMN warming_logs.receiver_instance_id IS 'Snapshot ID penerima saat eksekusi';
        COMMENT ON COLUMN warming_logs.message_content IS 'Pesan final yang terkirim (hasil render Spintax)';
        COMMENT ON COLUMN warming_logs.status IS 'Status eksekusi: SUCCESS atau FAILED';
        COMMENT ON COLUMN warming_logs.error_message IS 'Detail error jika status FAILED';

        -- Indexes untuk query history dan monitoring
        CREATE INDEX IF NOT EXISTS idx_logs_room_id ON warming_logs(room_id);
        CREATE INDEX IF NOT EXISTS idx_logs_executed_at ON warming_logs(executed_at);
        CREATE INDEX IF NOT EXISTS idx_logs_status ON warming_logs(status);
        
        -- Composite index untuk query monitoring: WHERE room_id = ? ORDER BY executed_at DESC
        CREATE INDEX IF NOT EXISTS idx_logs_room_executed ON warming_logs(room_id, executed_at DESC);
    `
	if _, err := db.Exec(warmingSchema); err != nil {
		log.Fatalf("failed to init warming schema: %v", err)
	}

	log.Println("schema created/ensured successfully (including warming system)")
}
