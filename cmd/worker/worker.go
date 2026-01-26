package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type WorkerInstance struct {
	config  WorkerConfig
	client  *SudevwaClient
	ctx     context.Context
	cancel  context.CancelFunc
	counter int // Round-robin counter
}

func NewWorkerInstance(config WorkerConfig, client *SudevwaClient) *WorkerInstance {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerInstance{
		config: config,
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (w *WorkerInstance) Start() {
	log.Printf("[%s] Worker started", w.config.WorkerName)
	LogWorkerEvent(w.config.ID, w.config.WorkerName, "INFO", "Worker started")

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[%s] Worker shutting down...", w.config.WorkerName)
			return
		default:
			w.runCycle()

			// Calculate sleep duration
			sleepSeconds := w.config.IntervalSeconds
			if w.config.IntervalMaxSeconds > w.config.IntervalSeconds {
				rangeSec := w.config.IntervalMaxSeconds - w.config.IntervalSeconds + 1
				sleepSeconds = w.config.IntervalSeconds + rand.Intn(rangeSec)
			}

			// Sleep for the calculated interval
			time.Sleep(time.Duration(sleepSeconds) * time.Second)
		}
	}
}

func (w *WorkerInstance) Stop() {
	LogWorkerEvent(w.config.ID, w.config.WorkerName, "INFO", "Worker stopping")
	w.cancel()
}

func (w *WorkerInstance) runCycle() {
	// 1. Claim a pending message for this application (atomicly sets status to 3)
	filter := fmt.Sprintf("application = '%s'", w.config.Application)
	msg, err := ClaimPendingOutbox(w.ctx, filter)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("[%s] Error claiming outbox: %v", w.config.WorkerName, err)
		}
		return
	}

	log.Printf("[%s] Processing message ID: %d to %s", w.config.WorkerName, msg.ID, msg.Destination)

	// 2. Validate and Normalize Destination
	destination := msg.Destination
	if w.config.MessageType == "group" {
		// Group ID normalization: append @g.us if missing
		if !strings.Contains(destination, "@") {
			destination = destination + "@g.us"
			log.Printf("[%s] Normalized Group ID: %s", w.config.WorkerName, destination)
		}
	} else {
		// Direct Message normalization
		cleaned := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, destination)

		if strings.HasPrefix(cleaned, "0") {
			cleaned = "62" + cleaned[1:]
		}

		if !strings.HasPrefix(cleaned, "62") || len(cleaned) < 10 {
			log.Printf("[%s] Invalid phone number format: %s", w.config.WorkerName, destination)
			UpdateOutboxFailed(w.ctx, msg.ID, "Invalid phone number format")
			return
		}
		destination = cleaned
	}

	// 3. Get Instances for this Circle
	instances, err := w.client.GetInstances(w.config.Circle)
	if err != nil {
		msg := fmt.Sprintf("Error fetching instances: %v", err)
		log.Printf("[%s] %s", w.config.WorkerName, msg)
		LogWorkerEvent(w.config.ID, w.config.WorkerName, "ERROR", msg)
		return
	}

	if len(instances) == 0 {
		msg := fmt.Sprintf("No used instances found in circle: %s", w.config.Circle)
		log.Printf("[%s] %s", w.config.WorkerName, msg)
		LogWorkerEvent(w.config.ID, w.config.WorkerName, "WARN", msg)
		return
	}

	// 4. Select Instance (Round-Robin)
	selectedInstance := instances[w.counter%len(instances)]
	w.counter++

	// 5. Send Message
	var success bool
	var apiMsg string

	if w.config.AllowMedia && msg.File.Valid && msg.File.String != "" {
		// Media Message (File with Caption from Messages)
		if w.config.MessageType == "group" {
			success, apiMsg, err = w.client.SendGroupMediaURL(selectedInstance.InstanceID, destination, msg.File.String, msg.Messages)
		} else {
			success, apiMsg, err = w.client.SendMediaURL(selectedInstance.InstanceID, destination, msg.File.String, msg.Messages)
		}
	} else {
		// Text Message
		if w.config.MessageType == "group" {
			success, apiMsg, err = w.client.SendGroupMessage(selectedInstance.InstanceID, destination, msg.Messages)
		} else {
			success, apiMsg, err = w.client.SendMessage(selectedInstance.InstanceID, destination, msg.Messages)
		}
	}

	if err != nil {
		msg := fmt.Sprintf("Error calling API (Instance %s): %v", selectedInstance.InstanceID, err)
		log.Printf("[%s] %s", w.config.WorkerName, msg)
		LogWorkerEvent(w.config.ID, w.config.WorkerName, "ERROR", msg)
		return
	}

	if success {
		log.Printf("[%s] Success! Sent ID %d via instance %s (%s)", w.config.WorkerName, msg.ID, selectedInstance.InstanceID, selectedInstance.PhoneNumber)
		if err := UpdateOutboxSuccess(w.ctx, msg.ID, selectedInstance.PhoneNumber); err != nil {
			log.Printf("[%s] CRITICAL: Failed to update status to success for ID %d: %v", w.config.WorkerName, msg.ID, err)
		}

		// Trigger Webhook
		go w.sendWebhook(msg, 1, "success", selectedInstance.PhoneNumber, "")

		// Optional: delay after success to prevent mass-ban
		time.Sleep(time.Duration(rand.Intn(2)+1) * time.Second)
	} else {
		log.Printf("[%s] Failed sending ID %d: %s", w.config.WorkerName, msg.ID, apiMsg)
		if err := UpdateOutboxFailed(w.ctx, msg.ID, apiMsg); err != nil {
			log.Printf("[%s] CRITICAL: Failed to update status to failed for ID %d: %v", w.config.WorkerName, msg.ID, err)
		}

		// Log significant failures (like 401 or specific API errors)
		if strings.Contains(strings.ToLower(apiMsg), "unauthorized") || strings.Contains(strings.ToLower(apiMsg), "forbidden") {
			LogWorkerEvent(w.config.ID, w.config.WorkerName, "ERROR", fmt.Sprintf("API Authorization Error: %s", apiMsg))
		}

		// Trigger Webhook
		go w.sendWebhook(msg, 2, "failed", "", apiMsg)
	}
}

func (w *WorkerInstance) sendWebhook(msg *OutboxMessage, status int, statusText string, fromNumber string, errorMsg string) {
	webhookURL := w.config.WebhookURL.String
	if webhookURL == "" {
		return
	}

	payload := map[string]interface{}{
		"event":     "outbox.processed",
		"timestamp": time.Now().UTC(),
		"data": map[string]interface{}{
			"id_outbox":   msg.ID,
			"status":      status,
			"status_text": statusText,
			"destination": msg.Destination,
			"from_number": fromNumber,
			"application": msg.Application,
			"table_id":    msg.TableID.String,
			"error_msg":   errorMsg,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[%s] Webhook marshal error: %v", w.config.WorkerName, err)
		return
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[%s] Webhook request error: %v", w.config.WorkerName, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Add HMAC signature if secret is provided
	secret := w.config.WebhookSecret.String
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-SUDEVWA-Signature", signature)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] Webhook send error: %v", w.config.WorkerName, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[%s] Webhook returned error status %d: %s", w.config.WorkerName, resp.StatusCode, string(respBody))
	} else {
		log.Printf("[%s] Webhook sent successfully to %s", w.config.WorkerName, webhookURL)
	}
}
