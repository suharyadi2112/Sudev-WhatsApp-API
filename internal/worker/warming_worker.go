package worker

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	warmingModel "gowa-yourself/internal/model/warming"
)

// StartWarmingWorker runs the warming worker in background
func StartWarmingWorker() {
	log.Println("🤖 Warming Worker started")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := processActiveRooms(); err != nil {
			log.Printf("❌ Worker error: %v", err)
		}
	}
}

// processActiveRooms finds and executes active rooms
func processActiveRooms() error {
	rooms, err := warmingModel.GetActiveRoomsForWorker(10)
	if err != nil {
		return fmt.Errorf("failed to get active rooms: %w", err)
	}

	for _, room := range rooms {
		if err := executeRoom(room); err != nil {
			log.Printf("❌ Failed to execute room %s: %v", room.ID, err)
		}
	}

	return nil
}

// executeRoom executes next line for a room
func executeRoom(room warmingModel.WarmingRoom) error {
	// Get next available script line (supports gaps in sequence)
	line, err := warmingModel.GetNextAvailableScriptLine(room.ScriptID, room.CurrentSequence)
	if err != nil {
		if err == sql.ErrNoRows {
			// Script finished (no more lines)
			return warmingModel.FinishRoom(room.ID)
		}
		return fmt.Errorf("failed to get script line: %w", err)
	}

	// Render spintax
	message := renderSpintax(line.MessageContent)

	// Determine sender/receiver based on actor role
	var senderID, receiverID string
	if line.ActorRole == "ACTOR_A" {
		senderID = room.SenderInstanceID
		receiverID = room.ReceiverInstanceID
	} else {
		senderID = room.ReceiverInstanceID
		receiverID = room.SenderInstanceID
	}

	// Send WhatsApp message
	success, errMsg := sendWhatsAppMessage(senderID, receiverID, message)

	// Log execution
	logStatus := "SUCCESS"
	if !success {
		logStatus = "FAILED"
	}

	if err := warmingModel.CreateWarmingLog(room.ID, line.ID, senderID, receiverID, message, logStatus, errMsg); err != nil {
		log.Printf("⚠️ Failed to create log: %v", err)
	}

	// Update room progress (even on failure for retry)
	nextRunAt := calculateNextRun(room.IntervalMinSeconds, room.IntervalMaxSeconds)

	if success {
		// Success: Move to next sequence
		if err := warmingModel.UpdateRoomProgress(room.ID, line.SequenceOrder, nextRunAt); err != nil {
			return fmt.Errorf("failed to update room: %w", err)
		}
		log.Printf("✅ Room %s: Sent message (sequence %d)", room.Name, line.SequenceOrder)
	} else {
		// Failed: Retry same sequence after interval
		if err := warmingModel.UpdateRoomProgress(room.ID, room.CurrentSequence, nextRunAt); err != nil {
			return fmt.Errorf("failed to update room: %w", err)
		}
		log.Printf("❌ Room %s: Failed to send message - %s (will retry)", room.Name, errMsg)
	}

	return nil
}

// renderSpintax renders spintax format {option1|option2} and dynamic variables
func renderSpintax(text string) string {
	// First, render dynamic variables
	result := renderDynamicVariables(text)

	// Then, render spintax
	for {
		start := strings.Index(result, "{")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		spintax := result[start+1 : end]
		options := strings.Split(spintax, "|")
		chosen := options[rand.Intn(len(options))]

		result = result[:start] + chosen + result[end+1:]
	}
	return result
}

// renderDynamicVariables replaces dynamic variables with actual values
func renderDynamicVariables(text string) string {
	now := time.Now()

	// TIME_GREETING based on hour
	hour := now.Hour()
	var timeGreeting string
	switch {
	case hour >= 5 && hour < 10:
		timeGreeting = "Pagi"
	case hour >= 10 && hour < 15:
		timeGreeting = "Siang"
	case hour >= 15 && hour < 18:
		timeGreeting = "Sore"
	default:
		timeGreeting = "Malam"
	}

	// DAY_NAME in Indonesian
	dayNames := []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	dayName := dayNames[now.Weekday()]

	// DATE in Indonesian format
	monthNames := []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	date := fmt.Sprintf("%d %s %d", now.Day(), monthNames[now.Month()], now.Year())

	// Replace variables
	result := text
	result = strings.ReplaceAll(result, "{TIME_GREETING}", timeGreeting)
	result = strings.ReplaceAll(result, "{DAY_NAME}", dayName)
	result = strings.ReplaceAll(result, "{DATE}", date)

	return result
}

// sendWhatsAppMessage sends message via WhatsApp API
func sendWhatsAppMessage(senderID, receiverID, message string) (bool, string) {
	// TODO: Integrate with your WhatsApp API
	// For now, just simulate success
	log.Printf("📤 Sending: %s → %s: %s", senderID, receiverID, message)

	// Simulate API call
	time.Sleep(100 * time.Millisecond)

	// Return success (replace with actual API call)
	return true, ""
}

// calculateNextRun calculates next run time with random interval
func calculateNextRun(minSec, maxSec int) time.Time {
	interval := minSec
	if maxSec > minSec {
		interval = minSec + rand.Intn(maxSec-minSec+1)
	}
	return time.Now().Add(time.Duration(interval) * time.Second)
}
