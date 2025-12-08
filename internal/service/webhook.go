package service

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"gowa-yourself/internal/model"
)

type WebhookPayload struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

func SendIncomingMessageWebhook(instanceID string, data map[string]interface{}) {
	inst, err := model.GetInstanceByInstanceID(instanceID)
	if err != nil || !inst.WebhookURL.Valid || inst.WebhookURL.String == "" {
		return
	}

	payload := WebhookPayload{
		Event:     "incoming_message",
		Timestamp: time.Now().UTC(),
		Data:      data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook: marshal error: %v", err)
		return
	}

	req, err := http.NewRequest("POST", inst.WebhookURL.String, bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook: new request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	go func() {
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("webhook: send error: %v", err)
			return
		}
		_ = resp.Body.Close()
	}()
}
