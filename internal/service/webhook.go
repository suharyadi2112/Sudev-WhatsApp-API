package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"gowa-yourself/internal/model"
)

// ✅ FIX: Tambahkan struct untuk webhook config dengan TTL
type WebhookConfig struct {
	URL       string
	Secret    string
	ExpiresAt time.Time // ← Tambahkan expiry time
}

// ✅ FIX: Cache untuk webhook config (menghindari N+1 query)
var (
	webhookCache      = make(map[string]*WebhookConfig)
	webhookCacheMutex sync.RWMutex
	webhookCacheTTL   = 5 * time.Minute // Cache valid selama 5 menit
)

type WebhookPayload struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// ✅ FIX: Function untuk get webhook config dengan caching + TTL
func GetWebhookConfig(instanceID string) (*WebhookConfig, error) {
	// Cek cache dulu
	webhookCacheMutex.RLock()
	config, exists := webhookCache[instanceID]
	webhookCacheMutex.RUnlock()

	// ✅ IMPROVEMENT: Cek apakah cache masih valid (belum expired) dan memiliki URL yang tidak kosong
	if exists && config != nil && config.URL != "" && time.Now().Before(config.ExpiresAt) {
		return config, nil
	}

	// Cache miss atau expired atau sebelumnya kosong - load langsung dari DB
	inst, err := model.GetInstanceByInstanceID(instanceID)
	if err != nil {
		log.Printf("❌ Gagal membaca instance %s dari DB: %v", instanceID, err)
		return nil, err
	}

	log.Printf("🔍 DB Read untuk %s: WebhookURL='%s' (Valid: %v, Phone: %s)", instanceID, inst.WebhookURL.String, inst.WebhookURL.Valid, inst.PhoneNumber.String)

	// Buat config object dengan expiry time
	config = &WebhookConfig{
		URL:       inst.WebhookURL.String,
		Secret:    inst.WebhookSecret.String,
		ExpiresAt: time.Now().Add(webhookCacheTTL), // ← Set expiry
	}

	// Hanya cache jika URL terisi, agar jika user baru mengisi di DB langsung terbaca
	if config.URL != "" {
		webhookCacheMutex.Lock()
		webhookCache[instanceID] = config
		webhookCacheMutex.Unlock()
		log.Printf("✅ Webhook config cached for instance: %s (expires in %v)", instanceID, webhookCacheTTL)
	}

	return config, nil
}

// ✅ FIX: Function untuk invalidate cache (dipanggil saat webhook config diupdate)
func InvalidateWebhookCache(instanceID string) {
	webhookCacheMutex.Lock()
	delete(webhookCache, instanceID)
	webhookCacheMutex.Unlock()
	log.Printf("🗑️ Webhook cache invalidated for instance: %s", instanceID)
}

// Global HTTP Client dengan Connection Pooling (Keep-Alive Reuse)
var webhookHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// ✅ FIX: Refactored function - sekarang pakai cache & full logging
func SendIncomingMessageWebhook(instanceID string, data map[string]interface{}) {
	// Get webhook config dari cache (bukan DB!)
	config, err := GetWebhookConfig(instanceID)
	if err != nil || config == nil || config.URL == "" {
		return
	}

	payload := WebhookPayload{
		Event:     "incoming_message",
		Timestamp: time.Now().UTC(),
		Data:      data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Webhook: marshal error: %v", err)
		return
	}

	req, err := http.NewRequest("POST", config.URL, bytes.NewReader(body))
	if err != nil {
		log.Printf("❌ Webhook: new request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// If webhook_secret is set, add HMAC signature header
	if config.Secret != "" {
		mac := hmac.New(sha256.New, []byte(config.Secret))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))

		req.Header.Set("X-SUDEVWA-Signature", signature)
	}

	go func() {
		resp, err := webhookHTTPClient.Do(req)
		if err != nil {
			log.Printf("❌ Webhook: Gagal kirim ke %s: %v", config.URL, err)
			return
		}
		defer resp.Body.Close()
		log.Printf("✅ Webhook: Terkirim ke %s [%s]", config.URL, resp.Status)
	}()
}
