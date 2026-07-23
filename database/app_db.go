package database

import (
	"database/sql"
	"log"

	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

var AppDB *sql.DB
var OutboxDB *sql.DB
var OutboxDriver = "postgres"

// Inisialisasi koneksi ke database custom (bukan whatsmeow)
func InitAppDB(appDbURL string) {
	db, err := sql.Open("postgres", appDbURL)
	if err != nil {
		log.Fatal("Failed to connect app DB:", err)
	}
	AppDB = db
	err = AppDB.Ping()
	if err != nil {
		log.Fatal("Failed to ping app DB:", err)
	}
	log.Println("App DB (custom) connected successfully")
}

// InitOutboxDB inisialisasi koneksi ke database outbox (bisa sama atau beda dengan AppDB)
func InitOutboxDB(outboxURL string) {
	if outboxURL == "" {
		log.Println("OUTBOX_DATABASE_URL not set, falling back to AppDB for outbox features")
		OutboxDB = AppDB
		OutboxDriver = "postgres"
		return
	}

	driver := "postgres"
	if strings.HasPrefix(outboxURL, "mysql://") {
		driver = "mysql"
		// convert mysql://user:pass@tcp(host:port)/db to user:pass@tcp(host:port)/db
		outboxURL = strings.TrimPrefix(outboxURL, "mysql://")
		if strings.Contains(outboxURL, "?") {
			if !strings.Contains(outboxURL, "parseTime=true") {
				outboxURL += "&parseTime=true"
			}
		} else {
			outboxURL += "?parseTime=true"
		}
	}

	db, err := sql.Open(driver, outboxURL)
	if err != nil {
		log.Printf("⚠️ Warning: Failed to open Outbox DB (%s): %v. Falling back to AppDB.", driver, err)
		OutboxDB = AppDB
		OutboxDriver = "postgres"
		return
	}

	if err := db.Ping(); err != nil {
		log.Printf("⚠️ Warning: Failed to ping Outbox DB (%s): %v. Falling back to AppDB.", driver, err)
		OutboxDB = AppDB
		OutboxDriver = "postgres"
		return
	}

	OutboxDB = db
	OutboxDriver = driver
	log.Printf("Outbox DB (%s) connected successfully", driver)
}
