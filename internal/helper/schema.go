// internal/helper/schema.go
package helper

import (
	"database/sql"
	"encoding/json"
	"log"

	"gowa-yourself/database"
)

func InitCustomSchema() {
	db := database.AppDB

	baseSchema := `
        CREATE TABLE IF NOT EXISTS instances (
            id                  SERIAL PRIMARY KEY,
            instance_id         VARCHAR(255) UNIQUE NOT NULL,
            phone_number        VARCHAR(50),
            jid                 VARCHAR(255),
            status              VARCHAR(50) NOT NULL DEFAULT 'disconnected',
            is_connected        BOOLEAN NOT NULL DEFAULT false,
            name                VARCHAR(255),
            profile_picture     TEXT,
            about               TEXT,
            platform            VARCHAR(50),
            battery_level       INT,
            battery_charging    BOOLEAN,
            qr_code             TEXT,
            qr_expires_at       TIMESTAMP,
            created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
            connected_at        TIMESTAMP,
            disconnected_at     TIMESTAMP,
            last_seen           TIMESTAMP,

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

        ALTER TABLE instances
        ADD COLUMN IF NOT EXISTS used BOOLEAN NOT NULL DEFAULT false,
        ADD COLUMN IF NOT EXISTS keterangan TEXT;

        CREATE INDEX IF NOT EXISTS idx_instances_circle ON instances(circle);
        CREATE INDEX IF NOT EXISTS idx_instances_used ON instances(used);
    `
	if _, err := db.Exec(alterSchema); err != nil {
		log.Fatalf("failed to alter schema: %v", err)
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

        COMMENT ON TABLE warming_scripts IS 'Header/template untuk naskah percakapan warming';
        COMMENT ON COLUMN warming_scripts.title IS 'Judul naskah, contoh: Percakapan Jual Beli Motor';
        COMMENT ON COLUMN warming_scripts.description IS 'Deskripsi singkat naskah';
        COMMENT ON COLUMN warming_scripts.category IS 'Kategori naskah untuk grouping, contoh: casual, business';

        -- =====================================================
        -- Table: warming_script_lines
        -- Purpose: Urutan dialog percakapan untuk setiap script
        -- =====================================================
        CREATE TABLE IF NOT EXISTS warming_script_lines (
            id                      SERIAL PRIMARY KEY,
            script_id               INT NOT NULL,
            sequence_order          INT NOT NULL,
            actor_role              VARCHAR(20) NOT NULL CHECK (actor_role IN ('ACTOR_A', 'ACTOR_B')),
            message_content         TEXT NOT NULL,
            typing_duration_sec     INT NOT NULL DEFAULT 3,
            created_at              TIMESTAMP(6) WITH TIME ZONE NOT NULL DEFAULT NOW(),
            
            CONSTRAINT fk_lines_script 
                FOREIGN KEY (script_id) 
                REFERENCES warming_scripts(id) 
                ON DELETE CASCADE,
            
            CONSTRAINT unique_script_sequence 
                UNIQUE (script_id, sequence_order)
        );

        COMMENT ON TABLE warming_script_lines IS 'Urutan dialog percakapan untuk setiap script';
        COMMENT ON COLUMN warming_script_lines.script_id IS 'Reference ke warming_scripts';
        COMMENT ON COLUMN warming_script_lines.sequence_order IS 'Urutan dialog (1, 2, 3, ...)';
        COMMENT ON COLUMN warming_script_lines.actor_role IS 'Peran aktor: ACTOR_A (pengirim) atau ACTOR_B (penerima)';
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

        COMMENT ON TABLE warming_rooms IS 'Wadah eksekusi yang memasangkan 2 instance untuk menjalankan script tertentu';
        COMMENT ON COLUMN warming_rooms.id IS 'UUID untuk room ID';
        COMMENT ON COLUMN warming_rooms.name IS 'Nama room untuk identifikasi mudah';
        COMMENT ON COLUMN warming_rooms.sender_instance_id IS 'Instance ID pengirim (ACTOR_A)';
        COMMENT ON COLUMN warming_rooms.receiver_instance_id IS 'Instance ID penerima (ACTOR_B)';
        COMMENT ON COLUMN warming_rooms.script_id IS 'Reference ke warming_scripts yang akan dijalankan';
        COMMENT ON COLUMN warming_rooms.current_sequence IS 'Sequence terakhir yang dieksekusi (untuk resume)';
        COMMENT ON COLUMN warming_rooms.status IS 'Status room: STOPPED, ACTIVE, PAUSED, FINISHED';
        COMMENT ON COLUMN warming_rooms.interval_min_seconds IS 'Interval minimum antar pesan (detik)';
        COMMENT ON COLUMN warming_rooms.interval_max_seconds IS 'Interval maksimum antar pesan (detik)';
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

	// Add send_real_message column if not exists (migration for existing tables)
	alterWarmingSchema := `
		ALTER TABLE warming_rooms 
		ADD COLUMN IF NOT EXISTS send_real_message BOOLEAN NOT NULL DEFAULT false;

		COMMENT ON COLUMN warming_rooms.send_real_message IS 'true = kirim WA asli, false = simulasi saja (dry-run mode)';
		
		-- HUMAN_VS_BOT feature columns
		ALTER TABLE warming_rooms
		ADD COLUMN IF NOT EXISTS room_type VARCHAR(20) NOT NULL DEFAULT 'BOT_VS_BOT'
			CHECK (room_type IN ('BOT_VS_BOT', 'HUMAN_VS_BOT')),
		ADD COLUMN IF NOT EXISTS whitelisted_number VARCHAR(50),
		ADD COLUMN IF NOT EXISTS reply_delay_min INT NOT NULL DEFAULT 10,
		ADD COLUMN IF NOT EXISTS reply_delay_max INT NOT NULL DEFAULT 60,
		ADD COLUMN IF NOT EXISTS use_ai BOOLEAN NOT NULL DEFAULT false,
		ADD COLUMN IF NOT EXISTS ai_context TEXT;
		
		COMMENT ON COLUMN warming_rooms.room_type IS 'BOT_VS_BOT: automated script exchange, HUMAN_VS_BOT: auto-reply to human';
		COMMENT ON COLUMN warming_rooms.whitelisted_number IS 'Phone number allowed to trigger auto-reply (format: 6281234567890)';
		COMMENT ON COLUMN warming_rooms.reply_delay_min IS 'Minimum delay in seconds before replying (HUMAN_VS_BOT mode)';
		COMMENT ON COLUMN warming_rooms.reply_delay_max IS 'Maximum delay in seconds before replying (HUMAN_VS_BOT mode)';
		COMMENT ON COLUMN warming_rooms.use_ai IS 'Use AI (OpenAI/Gemini) for generating replies instead of script';
		COMMENT ON COLUMN warming_rooms.ai_context IS 'Context/personality for AI replies (e.g., "casual friend", "professional colleague")';
		
		-- Indexes for HUMAN_VS_BOT queries
		CREATE INDEX IF NOT EXISTS idx_rooms_type ON warming_rooms(room_type);
		CREATE INDEX IF NOT EXISTS idx_rooms_whitelist ON warming_rooms(whitelisted_number);
	`
	if _, err := db.Exec(alterWarmingSchema); err != nil {
		log.Fatalf("failed to alter warming schema: %v", err)
	}

	// Warming Templates (Dynamic Templates)
	templatesSchema := `
		CREATE TABLE IF NOT EXISTS warming_templates (
			id SERIAL PRIMARY KEY,
			category VARCHAR(100) NOT NULL,
			name VARCHAR(255) NOT NULL,
			structure JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			CONSTRAINT unique_category_name UNIQUE (category, name)
		);
		CREATE INDEX IF NOT EXISTS idx_warming_templates_category ON warming_templates(category);
		COMMENT ON TABLE warming_templates IS 'Template percakapan dinamis untuk auto-generate dialog';
		COMMENT ON COLUMN warming_templates.category IS 'Kategori template: casual, business, customer_service';
		COMMENT ON COLUMN warming_templates.structure IS 'Array JSON dari dialog lines dengan message options';
	`
	if _, err := db.Exec(templatesSchema); err != nil {
		log.Fatalf("failed to create warming_templates table: %v", err)
	}

	// Auto-seed initial templates if table is empty
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM warming_templates").Scan(&count); err != nil {
		log.Printf("Warning: failed to check warming_templates count: %v", err)
	} else if count == 0 {
		log.Println("Seeding initial warming templates...")
		seedInitialTemplates(db)
	}

	log.Println("schema created/ensured successfully (including warming system)")
}

// seedInitialTemplates populates warming_templates with initial conversation templates
func seedInitialTemplates(db *sql.DB) {
	type templateLine struct {
		ActorRole      string   `json:"actorRole"`
		MessageOptions []string `json:"messageOptions"`
	}

	templates := []struct {
		Category string
		Name     string
		Lines    []templateLine
	}{
		{
			Category: "casual",
			Name:     "Percakapan Santai 1",
			Lines: []templateLine{
				{ActorRole: "ACTOR_A", MessageOptions: []string{
					"{Halo|Hai|Pagi|Siang|Sore} {bro|kak|mas|mbak}",
					"{Gimana|Apa} kabar {nih|bro|kak}?",
					"Lagi {sibuk|ngapain} {gak|nggak|ga}?",
				}},
				{ActorRole: "ACTOR_B", MessageOptions: []string{
					"{Halo|Hai} juga, {baik|alhamdulillah baik|aman} kok",
					"{Santai|Biasa} aja {nih|bro|kak}",
					"Lagi {nganggur|free|luang} {nih|bro}",
				}},
				{ActorRole: "ACTOR_A", MessageOptions: []string{
					"{Syukur|Alhamdulillah|Oke} deh",
					"{Wah|Oh} {asyik|mantap|sip} dong",
					"{Bagus|Mantap} {lah|dong|nih}",
				}},
				{ActorRole: "ACTOR_B", MessageOptions: []string{
					"{Iya|Yoi|Betul} {nih|bro|kak}",
					"{Gimana|Apa kabar} {kamu|lo|elu}?",
					"Lagi {ngapain|sibuk apa} {sekarang|nih}?",
				}},
				{ActorRole: "ACTOR_A", MessageOptions: []string{
					"{Baik|Aman|Sehat} {aja|kok|alhamdulillah}",
					"Lagi {kerja|di kantor|WFH} {nih|bro}",
					"{Biasa|Standar} aja {sih|nih|kak}",
				}},
			},
		},
		{
			Category: "business",
			Name:     "Jual Beli Produk",
			Lines: []templateLine{
				{ActorRole: "ACTOR_A", MessageOptions: []string{
					"{Halo|Hai|Permisi} {kak|mas|mbak}, {mau tanya|nanya} dong",
					"{Permisi|Maaf} {ganggu|numpang tanya}",
					"{Halo|Hai}, {ada|ready} {barang|produk|stock} {gak|ga|nggak}?",
				}},
				{ActorRole: "ACTOR_B", MessageOptions: []string{
					"{Halo|Hai} {kak|mas|mbak}, {ada|iya} {kok|dong}",
					"{Iya|Ya} {kak|mas|mbak}, {mau cari|nyari|butuh} apa?",
					"{Silakan|Monggo} {kak|mas|mbak}, {mau tanya|nanya} apa?",
				}},
				{ActorRole: "ACTOR_A", MessageOptions: []string{
					"{Mau|Nyari|Cari} {produk|barang} {A|B|C} {nih|dong}",
					"{Ada|Ready} {stock|barang} {gak|ga|nggak}?",
					"{Harga|Berapa} {untuk|buat} {produk|barang} {ini|itu}?",
				}},
				{ActorRole: "ACTOR_B", MessageOptions: []string{
					"{Ada|Ready|Stock ada} {kak|mas|mbak}",
					"{Harga|Untuk harga} {sekitar|kisaran} {100|200|300}rb {kak|mas}",
					"{Mau|Minat} {ambil|order} {berapa|jumlah berapa} {kak|mas}?",
				}},
				{ActorRole: "ACTOR_A", MessageOptions: []string{
					"{Ambil|Order|Mau} {1|2|3} {aja|dulu} {deh|kak}",
					"{Oke|Ok|Siap}, {kirim|transfer} {kemana|ke mana}?",
					"{Bisa|Boleh} {COD|cash|transfer} {gak|ga|nggak}?",
				}},
				{ActorRole: "ACTOR_B", MessageOptions: []string{
					"{Bisa|Boleh} {kak|mas|mbak}, {COD|transfer} {aja|dulu}",
					"{Oke|Siap} {kak|mas}, {total|totalnya} {jadi|sekitar} {100|200|300}rb",
					"{Alamat|Lokasi} {dimana|di mana} {kak|mas}?",
				}},
			},
		},
		{
			Category: "customer_service",
			Name:     "Customer Support",
			Lines: []templateLine{
				{ActorRole: "ACTOR_A", MessageOptions: []string{
					"{Halo|Hai} {admin|cs|kak}, {mau tanya|nanya} dong",
					"{Permisi|Maaf}, {ada|bisa} {bantuan|bantu} {gak|ga}?",
					"{Halo|Hai}, {saya|aku} {mau|butuh} {info|informasi}",
				}},
				{ActorRole: "ACTOR_B", MessageOptions: []string{
					"{Halo|Hai} {kak|mas|mbak}, {ada yang bisa|bisa} {dibantu|kami bantu}?",
					"{Selamat|Halo} {pagi|siang|sore} {kak|mas}, {silakan|monggo}",
					"{Iya|Ya} {kak|mas}, {ada|butuh} {bantuan|info} apa?",
				}},
				{ActorRole: "ACTOR_A", MessageOptions: []string{
					"{Mau tanya|Nanya} {tentang|soal} {produk|layanan} {nih|dong}",
					"{Cara|Gimana caranya} {order|pesan} {gimana|bagaimana}?",
					"{Ongkir|Biaya kirim} {ke|untuk} {Jakarta|Bandung|Surabaya} {berapa|brp}?",
				}},
				{ActorRole: "ACTOR_B", MessageOptions: []string{
					"{Untuk|Kalau} {produk|layanan} {kami|kita} {ada|tersedia} {A|B|C} {kak|mas}",
					"{Cara|Untuk} {order|pesan} {bisa|tinggal} {chat|hubungi} {kami|admin} {kak|mas}",
					"{Ongkir|Biaya kirim} {sekitar|kisaran} {10|20|30}rb {kak|mas}",
				}},
				{ActorRole: "ACTOR_A", MessageOptions: []string{
					"{Oh|Ooh} {gitu|begitu} {ya|yah}, {oke|ok|siap} {deh|kak}",
					"{Baik|Oke} {kak|mas}, {terima kasih|thanks|makasih} {ya|yah}",
					"{Siap|Ok} {kak|mas}, {nanti|tar} {saya|aku} {order|pesan}",
				}},
				{ActorRole: "ACTOR_B", MessageOptions: []string{
					"{Sama-sama|Terima kasih kembali} {kak|mas|mbak}",
					"{Siap|Oke} {kak|mas}, {ditunggu|tunggu} {ordernya|pesanannya} {ya|yah}",
					"{Senang|Terima kasih} {bisa|sudah} {membantu|bantu} {kak|mas}",
				}},
			},
		},
	}

	for _, tmpl := range templates {
		structureJSON, err := json.Marshal(tmpl.Lines)
		if err != nil {
			log.Printf("Failed to marshal template %s: %v", tmpl.Name, err)
			continue
		}

		_, err = db.Exec(
			"INSERT INTO warming_templates (category, name, structure) VALUES ($1, $2, $3)",
			tmpl.Category,
			tmpl.Name,
			structureJSON,
		)
		if err != nil {
			log.Printf("Failed to insert template %s: %v", tmpl.Name, err)
		} else {
			log.Printf("  ✓ Seeded template: %s (%s)", tmpl.Name, tmpl.Category)
		}
	}

	log.Println("Initial templates seeded successfully")
}
