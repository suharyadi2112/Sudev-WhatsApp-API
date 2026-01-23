package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
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

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[%s] Worker shutting down...", w.config.WorkerName)
			return
		default:
			w.runCycle()

			// Sleep for the configured interval
			time.Sleep(time.Duration(w.config.IntervalSeconds) * time.Second)
		}
	}
}

func (w *WorkerInstance) Stop() {
	w.cancel()
}

func (w *WorkerInstance) runCycle() {
	// 1. Fetch a pending message for this application
	filter := fmt.Sprintf("application = '%s'", w.config.Application)
	msg, err := FetchPendingOutbox(w.ctx, filter)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("[%s] Error fetching outbox: %v", w.config.WorkerName, err)
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
		log.Printf("[%s] Error fetching instances: %v", w.config.WorkerName, err)
		return
	}

	if len(instances) == 0 {
		log.Printf("[%s] No used instances found in circle: %s", w.config.WorkerName, w.config.Circle)
		return
	}

	// 4. Select Instance (Round-Robin)
	selectedInstance := instances[w.counter%len(instances)]
	w.counter++

	// 5. Send Message
	var success bool
	var apiMsg string

	if w.config.MessageType == "group" {
		success, apiMsg, err = w.client.SendGroupMessage(selectedInstance.InstanceID, destination, msg.Messages)
	} else {
		success, apiMsg, err = w.client.SendMessage(selectedInstance.InstanceID, destination, msg.Messages)
	}

	if err != nil {
		log.Printf("[%s] Error calling API: %v", w.config.WorkerName, err)
		return
	}

	if success {
		log.Printf("[%s] Success! Sent ID %d via instance %s (%s)", w.config.WorkerName, msg.ID, selectedInstance.InstanceID, selectedInstance.PhoneNumber)
		if err := UpdateOutboxSuccess(w.ctx, msg.ID, selectedInstance.PhoneNumber); err != nil {
			log.Printf("[%s] CRITICAL: Failed to update status to success for ID %d: %v", w.config.WorkerName, msg.ID, err)
		}

		// Optional: delay after success to prevent mass-ban
		time.Sleep(time.Duration(rand.Intn(2)+1) * time.Second)
	} else {
		log.Printf("[%s] Failed sending ID %d: %s", w.config.WorkerName, msg.ID, apiMsg)
		if err := UpdateOutboxFailed(w.ctx, msg.ID, apiMsg); err != nil {
			log.Printf("[%s] CRITICAL: Failed to update status to failed for ID %d: %v", w.config.WorkerName, msg.ID, err)
		}
	}
}
