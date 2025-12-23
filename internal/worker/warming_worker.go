package worker

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	warmingModel "gowa-yourself/internal/model/warming"
	"gowa-yourself/internal/service"
	"gowa-yourself/internal/ws"
)

// StartWarmingWorker runs the warming worker in background
func StartWarmingWorker(hub ws.RealtimePublisher) {
	log.Println("🤖 Warming Worker started")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := processActiveRooms(hub); err != nil {
			log.Printf("❌ Worker error: %v", err)
		}
	}
}

// processActiveRooms finds and executes active rooms
func processActiveRooms(hub ws.RealtimePublisher) error {
	rooms, err := warmingModel.GetActiveRoomsForWorker(10)
	if err != nil {
		return fmt.Errorf("failed to get active rooms: %w", err)
	}

	for _, room := range rooms {
		if err := executeRoom(room, hub); err != nil {
			log.Printf("❌ Failed to execute room %s: %v", room.ID, err)
		}
	}

	return nil
}

func executeRoom(room warmingModel.WarmingRoom, hub ws.RealtimePublisher) error {
	line, err := warmingModel.GetNextAvailableScriptLine(room.ScriptID, room.CurrentSequence)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("✅ Room %s: Script finished - all lines executed", room.Name)

			if hub != nil {
				finishedLine := warmingModel.WarmingScriptLine{
					SequenceOrder: room.CurrentSequence,
					ActorRole:     "SYSTEM",
				}
				publishWarmingMessageEvent(
					hub,
					room,
					finishedLine,
					room.SenderInstanceID,
					room.ReceiverInstanceID,
					"Script completed - all dialog sequences finished",
					"FINISHED",
					"",
				)
			}

			return warmingModel.FinishRoom(room.ID)
		}
		return fmt.Errorf("failed to get script line: %w", err)
	}

	message := renderSpintax(line.MessageContent)

	var senderID, receiverID string
	if line.ActorRole == "ACTOR_A" {
		senderID = room.SenderInstanceID
		receiverID = room.ReceiverInstanceID
	} else {
		senderID = room.ReceiverInstanceID
		receiverID = room.SenderInstanceID
	}

	// Send WhatsApp message
	success, errMsg := sendWhatsAppMessage(senderID, receiverID, message, room.SendRealMessage)

	// Log execution
	logStatus := "SUCCESS"
	if !success {
		logStatus = "FAILED"
	}

	if err := warmingModel.CreateWarmingLog(room.ID, line.ID, senderID, receiverID, message, logStatus, errMsg); err != nil {
		log.Printf("⚠️ Failed to create log: %v", err)
	}

	// Publish warming message event to WebSocket for real-time display
	if hub != nil {
		publishWarmingMessageEvent(hub, room, *line, senderID, receiverID, message, logStatus, errMsg)
	}

	nextRunAt := calculateNextRun(room.IntervalMinSeconds, room.IntervalMaxSeconds)

	if success {
		if err := warmingModel.UpdateRoomProgress(room.ID, line.SequenceOrder, nextRunAt); err != nil {
			return fmt.Errorf("failed to update room: %w", err)
		}
		log.Printf("✅ Room %s: Sent message (sequence %d)", room.Name, line.SequenceOrder)
	} else {
		// Check for critical connection errors
		errMsgLow := strings.ToLower(errMsg)
		if strings.Contains(errMsgLow, "not connected") ||
			strings.Contains(errMsgLow, "session not found") ||
			strings.Contains(errMsgLow, "not logged in") {

			log.Printf("⛔ Room %s PAUSED due to connection error: %s", room.Name, errMsg)

			// Publish failure event with PAUSED status
			if hub != nil {
				publishWarmingMessageEvent(hub, room, *line, senderID, receiverID, "Room PAUSED: "+errMsg, "PAUSED", errMsg)
			}

			// Pause the room
			if err := warmingModel.UpdateRoomStatus(room.ID.String(), "PAUSED", nil); err != nil {
				log.Printf("⚠️ Failed to pause room %s: %v", room.Name, err)
			}
			return nil
		}

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

func renderDynamicVariables(text string) string {
	now := time.Now()

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

	dayNames := []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	dayName := dayNames[now.Weekday()]

	monthNames := []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	date := fmt.Sprintf("%d %s %d", now.Day(), monthNames[now.Month()], now.Year())

	result := text
	result = strings.ReplaceAll(result, "{TIME_GREETING}", timeGreeting)
	result = strings.ReplaceAll(result, "{DAY_NAME}", dayName)
	result = strings.ReplaceAll(result, "{DATE}", date)

	return result
}

func sendWhatsAppMessage(senderID, receiverID, message string, sendReal bool) (bool, string) {
	if !sendReal {
		log.Printf("🧪 [SIMULATION] %s → %s: %s", senderID, receiverID, message)
		time.Sleep(100 * time.Millisecond)
		return true, ""
	}

	log.Printf("📤 [REAL] Sending: %s → %s: %s", senderID, receiverID, message)

	// Use shared service function
	success, errMsg := service.SendWarmingMessage(senderID, receiverID, message)

	if success {
		log.Printf("✅ Message sent successfully: %s → %s", senderID, receiverID)
	} else {
		log.Printf("❌ Failed to send: %s", errMsg)
	}

	return success, errMsg
}

// calculateNextRun calculates next run time with random interval
func calculateNextRun(minSec, maxSec int) time.Time {
	interval := minSec
	if maxSec > minSec {
		rangeVal := maxSec - minSec + 1
		if rangeVal > 0 {
			interval = minSec + rand.Intn(rangeVal)
		}
	}
	return time.Now().Add(time.Duration(interval) * time.Second)
}

func publishWarmingMessageEvent(hub ws.RealtimePublisher, room warmingModel.WarmingRoom, line warmingModel.WarmingScriptLine, senderID, receiverID, message, status, errorMsg string) {
	event := ws.WsEvent{
		Event:     ws.EventWarmingMessage,
		Timestamp: time.Now().UTC(),
		Data: ws.WarmingMessageData{
			RoomID:             room.ID.String(),
			RoomName:           room.Name,
			SenderInstanceID:   senderID,
			ReceiverInstanceID: receiverID,
			Message:            message,
			SequenceOrder:      line.SequenceOrder,
			ActorRole:          line.ActorRole,
			Status:             status,
			ErrorMessage:       errorMsg,
			Timestamp:          time.Now().UTC(),
		},
	}

	hub.Publish(event)

	if status == "FINISHED" {
		log.Printf("🎉 Published script finished event: room=%s", room.Name)
	} else {
		log.Printf("📡 Published warming message event: room=%s, sequence=%d, status=%s", room.Name, line.SequenceOrder, status)
	}
}
