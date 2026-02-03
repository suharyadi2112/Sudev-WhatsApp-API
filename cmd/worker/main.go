package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/joho/godotenv"
	_ "github.com/lib/pq" // Postgres driver
)

var (
	ConfigDB     *sql.DB
	ConfigDriver string // "mysql" or "postgres"

	OutboxDB     *sql.DB
	OutboxDriver string // "mysql" or "postgres"
)

func initDB() {
	// 1. Initialise ConfigDB (always APP_DATABASE_URL)
	appDBURL := os.Getenv("APP_DATABASE_URL")
	if appDBURL == "" {
		log.Fatal("APP_DATABASE_URL is not set")
	}
	ConfigDB, ConfigDriver = connectDB(appDBURL, "Config (Postgres)")

	// 2. Initialise OutboxDB (OUTBOX_DATABASE_URL or fallback)
	outboxDBURL := os.Getenv("OUTBOX_DATABASE_URL")
	if outboxDBURL == "" {
		log.Println("OUTBOX_DATABASE_URL not set, falling back to APP_DATABASE_URL for outbox")
		OutboxDB, OutboxDriver = ConfigDB, ConfigDriver
	} else {
		OutboxDB, OutboxDriver = connectDB(outboxDBURL, "Outbox (External)")
	}
}

func connectDB(dbURL, label string) (*sql.DB, string) {
	driver := "postgres"
	if strings.HasPrefix(dbURL, "mysql://") {
		driver = "mysql"
		dbURL = strings.TrimPrefix(dbURL, "mysql://")
		if strings.Contains(dbURL, "?") {
			dbURL += "&parseTime=true"
		} else {
			dbURL += "?parseTime=true"
		}
	} else if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		driver = "postgres"
	}

	db, err := sql.Open(driver, dbURL)
	if err != nil {
		log.Fatalf("Failed to open %s database (%s): %v", label, driver, err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping %s database (%s): %v", label, driver, err)
	}

	log.Printf("Successfully connected to %s (%s)", label, driver)
	return db, driver
}

func main() {
	// 1. Load configuration
	if err := godotenv.Load(); err != nil {
		_ = godotenv.Load("../../.env")
	}

	// 2. Initialize database
	initDB()
	defer ConfigDB.Close()
	if OutboxDB != ConfigDB {
		defer OutboxDB.Close()
	}

	// 3. Worker Configuration
	apiBaseURL := os.Getenv("OUTBOX_API_BASEURL")
	if apiBaseURL == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "2121"
		}
		apiBaseURL = "http://localhost:" + port
	}

	apiUser := os.Getenv("OUTBOX_API_USER")
	apiPass := os.Getenv("OUTBOX_API_PASS")
	if apiUser == "" || apiPass == "" {
		log.Fatal("OUTBOX_API_USER or OUTBOX_API_PASS is not set")
	}

	// 4. Initialize API Client
	client := NewSudevwaClient(apiBaseURL, apiUser, apiPass)

	// 5. Start Worker Manager
	manager := NewWorkerManager(client)
	manager.Start()

	// 6. Wait for termination signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down worker...")
	manager.Stop()
	log.Println("Worker shutdown complete.")
}
