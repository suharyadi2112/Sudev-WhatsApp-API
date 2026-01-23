package main

import (
	"context"
	"log"
	"sync"
	"time"
)

type WorkerManager struct {
	client  *SudevwaClient
	workers map[int]*WorkerInstance
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewWorkerManager(client *SudevwaClient) *WorkerManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerManager{
		client:  client,
		workers: make(map[int]*WorkerInstance),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (m *WorkerManager) Start() {
	log.Println("Worker Blast Outbox Manager started. Polling for configurations...")

	// Initial load
	m.reloadConfigs()

	// Periodic reload every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.reloadConfigs()
			case <-m.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (m *WorkerManager) reloadConfigs() {
	log.Println("Reloading blast outbox configurations from database...")

	configs, err := FetchWorkerConfigs(m.ctx)
	if err != nil {
		log.Printf("Error fetching worker configs: %v", err)
		// If no table exists yet or other error, we might want to check if any active workers exist
		// but for now we just return
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Keep track of configs we found to identify ones that need to be removed
	activeConfigIDs := make(map[int]bool)

	for _, config := range configs {
		activeConfigIDs[config.ID] = true

		if existingWorker, exists := m.workers[config.ID]; exists {
			// Update existing worker if config changed (e.g., interval)
			if existingWorker.config.IntervalSeconds != config.IntervalSeconds ||
				existingWorker.config.Circle != config.Circle ||
				existingWorker.config.Application != config.Application ||
				existingWorker.config.MessageType != config.MessageType {

				log.Printf("Config changed for worker ID %d (%s). Restarting...", config.ID, config.WorkerName)
				existingWorker.Stop()

				newWorker := NewWorkerInstance(config, m.client)
				m.workers[config.ID] = newWorker
				go newWorker.Start()
			}
		} else {
			// Start new worker
			log.Printf("Starting new worker ID %d: %s (Application: %s, Circle: %s, Interval: %ds)",
				config.ID, config.WorkerName, config.Application, config.Circle, config.IntervalSeconds)

			newWorker := NewWorkerInstance(config, m.client)
			m.workers[config.ID] = newWorker
			go newWorker.Start()
		}
	}

	// Stop workers whose configs are no longer in DB or are disabled
	for id, worker := range m.workers {
		if !activeConfigIDs[id] {
			log.Printf("Worker ID %d (%s) is no longer active. Stopping...", id, worker.config.WorkerName)
			worker.Stop()
			delete(m.workers, id)
		}
	}
}

func (m *WorkerManager) Stop() {
	m.cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, worker := range m.workers {
		worker.Stop()
	}
	log.Println("Worker Manager stopped.")
}
