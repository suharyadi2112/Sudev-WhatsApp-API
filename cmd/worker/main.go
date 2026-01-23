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
	WorkerDB     *sql.DB
	WorkerDriver string // "mysql" or "postgres"
)

func initDB() {
	dbURL := os.Getenv("OUTBOX_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("APP_DATABASE_URL")
	}

	if dbURL == "" {
		log.Fatal("Neither OUTBOX_DATABASE_URL nor APP_DATABASE_URL is set")
	}

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
		log.Fatalf("Failed to open database (%s): %v", driver, err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database (%s): %v", driver, err)
	}

	WorkerDB = db
	WorkerDriver = driver
	log.Printf("Successfully connected to Worker Blast Outbox Database (%s)", driver)
}

func main() {
	// 1. Load configuration
	if err := godotenv.Load(); err != nil {
		_ = godotenv.Load("../../.env")
	}

	// 2. Initialize database
	initDB()
	defer WorkerDB.Close()

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
