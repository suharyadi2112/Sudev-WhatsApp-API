package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"gowa-yourself/config"
	"gowa-yourself/database"
	"gowa-yourself/internal/helper"
	"gowa-yourself/internal/model"
	"gowa-yourself/internal/ws"

	"go.mau.fi/whatsmeow/store"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var (
	sessions     = make(map[string]*model.Session)
	sessionsLock sync.RWMutex

	// Track instances yang sedang logout
	loggingOut     = make(map[string]bool)
	loggingOutLock sync.RWMutex
	Realtime       ws.RealtimePublisher

	// Track reconnect untuk staggered activation
	reconnectTracker     = make(map[string]time.Time) // instanceID -> waktu disconnect
	reconnectTrackerLock sync.RWMutex
	lastReconnectTime    time.Time
	lastReconnectLock    sync.RWMutex

	ErrInstanceNotFound       = errors.New("instance not found")
	ErrInstanceStillConnected = errors.New("instance still connected")
)

// Event handler untuk handle connection events
func eventHandler(instanceID string) func(evt interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {

		case *events.Connected:
			loggingOutLock.RLock()
			isLoggingOut := loggingOut[instanceID]
			loggingOutLock.RUnlock()
			if isLoggingOut {
				fmt.Println("⚠ Ignoring reconnect during logout:", instanceID)
				return
			}

			// Cek apakah ini reconnect setelah disconnect
			reconnectTrackerLock.Lock()
			disconnectTime, wasDisconnected := reconnectTracker[instanceID]
			delete(reconnectTracker, instanceID) // Hapus dari tracker
			reconnectTrackerLock.Unlock()

			// Hitung delay untuk staggered activation
			var activationDelay time.Duration
			if wasDisconnected {
				// Cek apakah ada device lain yang baru reconnect
				lastReconnectLock.Lock()
				timeSinceLastReconnect := time.Since(lastReconnectTime)

				// Jika ada device lain reconnect dalam 5 detik terakhir,
				// berarti kemungkinan internet baru pulih (mass reconnect)
				if timeSinceLastReconnect < 5*time.Second && !lastReconnectTime.IsZero() {
					// Tambahkan delay 3-8 detik untuk device ini
					activationDelay = time.Duration(rand.Intn(6)+3) * time.Second
					fmt.Printf("⏳ Staggered reconnect: delaying activation for %s by %v (disconnected at: %v)\n",
						instanceID, activationDelay, disconnectTime.Format("15:04:05"))
				}

				lastReconnectTime = time.Now()
				lastReconnectLock.Unlock()
			}

			sessionsLock.Lock()
			session, exists := sessions[instanceID]
			if exists {
				session.IsConnected = true
				if session.Client.Store.ID != nil {
					session.JID = session.Client.Store.ID.String()
				}

				fmt.Println("✓ Connected! Instance:", instanceID, "JID:", session.JID)
			}
			sessionsLock.Unlock()

			// Tunggu delay sebelum aktivasi (jika ada)
			if activationDelay > 0 {
				time.Sleep(activationDelay)
			}

			if exists {
				// Kirim presence saat connected, untuk status online di hp
				if err := session.Client.SendPresence(context.Background(), types.PresenceAvailable); err != nil {
					fmt.Println("⚠ Failed to send presence for instance:", instanceID, err)
				} else {
					fmt.Println("✓ Presence sent (Available) for instance:", instanceID)
				}
			}

			if exists && session.Client.Store.ID != nil {
				// Ambil phoneNumber dari JID (mis. "6285148107612:38@s.whatsapp.net")
				jid := session.Client.Store.ID
				phoneNumber := jid.User // biasanya sudah format 6285xxxx

				platform := "" // kalau ada field ini; kalau tidak bisa kosong
				if err := model.UpdateInstanceOnConnected(
					instanceID,
					jid.String(),
					phoneNumber,
					platform,
				); err != nil {
					fmt.Println("Warning: failed to update instance on connected:", err)
				}

				// Setelah DB update, kirim event WS
				if Realtime != nil {
					now := time.Now().UTC()
					data := ws.InstanceStatusChangedData{
						InstanceID:     instanceID,
						PhoneNumber:    phoneNumber,
						Status:         "online",
						IsConnected:    true,
						ConnectedAt:    &now,
						DisconnectedAt: nil,
					}

					evt := ws.WsEvent{
						Event:     ws.EventInstanceStatusChanged,
						Timestamp: now,
						Data:      data,
					}

					Realtime.Publish(evt)
				}

				// ✅ FIX: Stop heartbeat lama jika ada (prevent multiple goroutines)
				if session.HeartbeatCancel != nil {
					session.HeartbeatCancel()
					fmt.Println("⏹ Stopped previous heartbeat for:", instanceID)
				}

				// ✅ FIX: Start heartbeat goroutine dengan context cancellation
				ctx, cancel := context.WithCancel(context.Background())
				session.HeartbeatCancel = cancel // Simpan cancel function

				go func(ctx context.Context, instID string) {
					ticker := time.NewTicker(5 * time.Minute)
					defer ticker.Stop()

					for {
						select {
						case <-ctx.Done():
							// Context cancelled - stop goroutine
							fmt.Println("⏹ Heartbeat stopped (cancelled) for:", instID)
							return

						case <-ticker.C:
							// Send heartbeat
							sessionsLock.RLock()
							sess, ok := sessions[instID]
							sessionsLock.RUnlock()

							if !ok || !sess.IsConnected {
								fmt.Println("⏹ Heartbeat stopped (disconnected) for:", instID)
								return
							}

							if err := sess.Client.SendPresence(context.Background(), types.PresenceAvailable); err != nil {
								fmt.Println("⚠ Heartbeat failed for:", instID, err)
							} else {
								fmt.Println("💓 Heartbeat sent for:", instID)
							}
						}
					}
				}(ctx, instanceID)

			}

		case *events.PairSuccess:
			fmt.Println("✓ Pair Success! Instance:", instanceID)

		case *events.LoggedOut:
			sessionsLock.Lock()
			session, exists := sessions[instanceID]
			if exists {
				session.IsConnected = false
				fmt.Println("✗ Logged out! Instance:", instanceID)

				// Delete device store dari whatsapp-db
				if session.Client.Store != nil && session.Client.Store.ID != nil {
					err := database.Container.DeleteDevice(context.Background(), session.Client.Store)
					if err != nil {
						fmt.Println("⚠ Failed to delete device store:", err)
					} else {
						fmt.Println("✓ Device store deleted for:", instanceID)
					}
				}

				// Disconnect client
				session.Client.Disconnect()
			}
			sessionsLock.Unlock()

			// Update DB status
			if err := model.UpdateInstanceOnLoggedOut(instanceID); err != nil {
				fmt.Println("Warning: failed to update instance on logged out:", err)
			} else {
				// Kirim event WS
				if Realtime != nil {
					now := time.Now().UTC()

					inst, err := model.GetInstanceByInstanceID(instanceID)
					if err != nil {
						fmt.Printf("Failed to get instance by instance ID %s: %v\n", instanceID, err)
					}

					data := ws.InstanceStatusChangedData{
						InstanceID:     instanceID,
						PhoneNumber:    inst.PhoneNumber.String,
						Status:         "logged_out",
						IsConnected:    false,
						ConnectedAt:    &inst.ConnectedAt.Time,
						DisconnectedAt: &now,
					}

					evt := ws.WsEvent{
						Event:     ws.EventInstanceStatusChanged,
						Timestamp: now,
						Data:      data,
					}

					Realtime.Publish(evt)
				}
			}

			// Hapus session dari memory
			sessionsLock.Lock()
			delete(sessions, instanceID)
			sessionsLock.Unlock()

			fmt.Println("✓ Session cleanup completed for:", instanceID)

		case *events.StreamReplaced:
			fmt.Println("⚠ Stream replaced! Instance:", instanceID)

		case *events.Disconnected:
			loggingOutLock.RLock()
			isLoggingOut := loggingOut[instanceID]
			loggingOutLock.RUnlock()
			if !isLoggingOut {
				fmt.Println("⚠ Disconnected! Instance:", instanceID)

				// Catat waktu disconnect untuk tracking
				reconnectTrackerLock.Lock()
				reconnectTracker[instanceID] = time.Now()
				reconnectTrackerLock.Unlock()

				sessionsLock.Lock()
				if session, exists := sessions[instanceID]; exists {
					session.IsConnected = false
				}
				sessionsLock.Unlock()

				if err := model.UpdateInstanceOnDisconnected(instanceID); err != nil {
					fmt.Println("Warning: failed to update instance on disconnected:", err)
				}
			}

		//Handle incoming messages
		case *events.Message:
			msgTime := v.Info.Timestamp

			// Filter pesan lama (History Sync)
			// Jika pesan lebih tua dari 2 menit, skip
			if time.Since(msgTime) > 2*time.Minute {
				// fmt.Println("Ignoring old message from history sync")
				return
			}

			messageText := v.Message.GetConversation()

			// Handle extended text message (reply, link preview, etc)
			if messageText == "" && v.Message.ExtendedTextMessage != nil {
				messageText = v.Message.GetExtendedTextMessage().GetText()
			}

			// Handle image caption
			if messageText == "" && v.Message.ImageMessage != nil {
				messageText = v.Message.GetImageMessage().GetCaption()
			}

			// Handle video caption
			if messageText == "" && v.Message.VideoMessage != nil {
				messageText = v.Message.GetVideoMessage().GetCaption()
			}

			fmt.Printf("📨 Received message from %s: %s\n", v.Info.Sender, messageText)

			// Debug logging for sender investigation
			fmt.Printf("🔍 DEBUG - Full sender: %s\n", v.Info.Sender.String())
			fmt.Printf("🔍 DEBUG - User: %s, Server: %s\n", v.Info.Sender.User, v.Info.Sender.Server)
			fmt.Printf("🔍 DEBUG - IsGroup: %v, IsFromMe: %v\n", v.Info.IsGroup, v.Info.IsFromMe)

			senderNumber := v.Info.Sender.User

			// If message from linked device (@lid), resolve to real phone number
			if v.Info.Sender.Server == "lid" {
				session, err := GetSession(instanceID)
				if err == nil && session.Client != nil {
					ctx := context.Background()

					// Convert LID to Phone Number using whatsmeow's LID store
					phoneJID, err := session.Client.Store.LIDs.GetPNForLID(ctx, v.Info.Sender)
					if err == nil && phoneJID.User != "" {
						senderNumber = phoneJID.User
						log.Printf("✅ Resolved LID %s to phone number: %s", v.Info.Sender.User, senderNumber)
					} else {
						log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
						log.Printf("⚠️ HUMAN_VS_BOT: Could not resolve LID to phone number")
						log.Printf("👤 Contact Name: %s", v.Info.PushName)
						log.Printf("🔑 LID (Use this for whitelisting): %s", v.Info.Sender.User)
						log.Printf("💡 To enable auto-reply, set whitelisted_number = '%s'", v.Info.Sender.User)
						log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
					}
				}
			}

			if err := HandleIncomingMessage(instanceID, senderNumber, messageText, v.Info.Chat, v.Info.ID, v.Info.Sender.String()); err != nil {
				log.Printf("[HUMAN_VS_BOT] Error handling incoming message: %v", err)
			}

			// Siapkan Payload (dipakai WS & Webhook)
			payload := map[string]interface{}{
				"instance_id": instanceID,
				"from":        v.Info.Sender.String(),
				"from_me":     v.Info.IsFromMe,
				"message":     messageText,
				"timestamp":   v.Info.Timestamp.Unix(),
				"is_group":    v.Info.IsGroup,
				"message_id":  v.Info.ID,
				"push_name":   v.Info.PushName,
			}

			// Broadcast ke WebSocket (jika diaktifkan)
			if config.EnableWebsocketIncomingMessage && Realtime != nil {
				Realtime.BroadcastToInstance(instanceID, map[string]interface{}{
					"event": "incoming_message",
					"data":  payload,
				})
				fmt.Printf("✓ Message broadcasted to WebSocket listeners for instance: %s\n", instanceID)
			}

			//Broadcast ke Webhook (jika diaktifkan)
			if config.EnableWebhook {
				SendIncomingMessageWebhook(instanceID, payload)
				fmt.Printf("✓ Webhook dispatched for instance: %s\n", instanceID)
			}

		}

	}
}

// Load all devices from database and reconnect
func LoadAllDevices() error {
	devices, err := database.Container.GetAllDevices(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	fmt.Printf("Found %d saved devices in database\n", len(devices))

	for i, device := range devices {
		if device.ID == nil {
			continue
		}

		jid := device.ID.String()

		// 1) Ambil instanceID dari DB custom, JANGAN generate baru dari JID
		inst, err := model.GetInstanceByJID(jid)
		if err != nil {
			fmt.Printf("Failed to get instance for jid %s: %v\n", jid, err)
			continue
		}

		instanceID := inst.InstanceID
		if instanceID == "" {
			fmt.Printf("Empty instanceID for jid %s, skipping\n", jid)
			continue
		}

		// Tambahkan random delay antar reconnect (kecuali device pertama)
		if i > 0 {
			// Random delay 3-10 detik untuk menghindari pola bot farm
			delaySeconds := rand.Intn(8) + 3 // 3-10 detik
			fmt.Printf("⏳ Waiting %d seconds before reconnecting next device ...\n", delaySeconds)
			time.Sleep(time.Duration(delaySeconds) * time.Second)
		}

		// 2) Buat client WhatsMeow dan attach event handler dengan instanceID yang benar
		client := whatsmeow.NewClient(device, nil)
		client.AddEventHandler(eventHandler(instanceID))

		if err := client.Connect(); err != nil {
			fmt.Printf("Failed to connect device %s: %v\n", jid, err)
			continue
		}

		// 3) Simpan ke sessions map dengan key instanceID yang konsisten
		sessionsLock.Lock()
		sessions[instanceID] = &model.Session{
			ID:          instanceID,
			JID:         jid,
			Client:      client,
			IsConnected: client.IsConnected(),
		}
		sessionsLock.Unlock()

		// 4) Update status di DB bahwa instance ini berhasil re-connect
		//    (kalau client.IsConnected() == true)
		if client.IsConnected() {
			phoneNumber := helper.ExtractPhoneFromJID(jid) // mis. "6285148107612"

			if err := model.UpdateInstanceOnConnected(
				instanceID,
				jid,
				phoneNumber,
				"", // platform sementara kosong
			); err != nil {
				fmt.Printf("Warning: failed to update instance on reconnect %s: %v\n", instanceID, err)
			}
		}

		fmt.Printf("✓ Loaded and connected: %s (instance: %s)\n", jid, instanceID)
	}

	return nil
}

func CreateSession(instanceID string) (*model.Session, error) {
	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	// Cek apakah session sudah ada
	if _, exists := sessions[instanceID]; exists {
		return nil, fmt.Errorf("session already exists")
	}

	// Acak OS supaya tidak seragam
	osOptions := []string{"Windows", "macOS", "Linux"}
	randomOS := osOptions[rand.Intn(len(osOptions))]

	// Generate random suffix (4 digit hex) untuk identitas unik
	randomID := fmt.Sprintf("%04x", rand.Intn(0xffff))

	// Gabungkan OS dengan nama unik: "Windows (SUDEVWA-a1b2)"
	customOsName := fmt.Sprintf("%s (SUDEVWA-%s)", randomOS, randomID)

	// Set Global Device Props (akan dipake NewDevice)
	store.DeviceProps.Os = proto.String(customOsName)
	store.DeviceProps.PlatformType = waProto.DeviceProps_DESKTOP.Enum()
	store.DeviceProps.RequireFullSync = proto.Bool(false)

	// Buat device baru
	deviceStore := database.Container.NewDevice()

	// Create whatsmeow client
	clientLog := waLog.Stdout("Client", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	// Add event handler
	client.AddEventHandler(eventHandler(instanceID))

	// Simpan session
	session := &model.Session{
		ID:          instanceID,
		Client:      client,
		IsConnected: false,
	}

	sessions[instanceID] = session
	return session, nil
}

func GetSession(instanceID string) (*model.Session, error) {
	sessionsLock.RLock()
	defer sessionsLock.RUnlock()

	session, exists := sessions[instanceID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	return session, nil
}

// ambil semua session dalam instance
func GetAllSessions() map[string]*model.Session {
	sessionsLock.RLock()
	defer sessionsLock.RUnlock()

	result := make(map[string]*model.Session)
	for k, v := range sessions {
		result[k] = v
	}

	return result
}

func DeleteSession(instanceID string) error {
	// Mark sebagai sedang logout untuk prevent auto-reconnect
	loggingOutLock.Lock()
	loggingOut[instanceID] = true
	loggingOutLock.Unlock()

	// Ambil session
	sessionsLock.Lock()
	session, exists := sessions[instanceID]
	if !exists {
		sessionsLock.Unlock()

		// Clean up flag
		loggingOutLock.Lock()
		delete(loggingOut, instanceID)
		loggingOutLock.Unlock()

		return fmt.Errorf("session not found")
	}

	// Hapus dari map sessions (memory)
	delete(sessions, instanceID)
	sessionsLock.Unlock()

	// ✅ FIX: Stop heartbeat goroutine sebelum logout
	if session.HeartbeatCancel != nil {
		session.HeartbeatCancel()
		fmt.Printf("⏹ Heartbeat cancelled for instance: %s\n", instanceID)
	}

	// LOGOUT: Unlink device dari WhatsApp
	if session.Client != nil {
		err := session.Client.Logout(context.Background())
		if err != nil {
			fmt.Printf("Warning: Failed to logout from WhatsApp: %v\n", err)
		}
		session.Client.Disconnect()
	}

	// Update status instance di DB custom (tidak dihapus, hanya update status)
	err := model.UpdateInstanceStatus(instanceID, "logged_out", false, time.Now())
	if err != nil {
		fmt.Printf("Warning: Failed to update instance status in DB: %v\n", err)
	} else {
		if Realtime != nil {
			now := time.Now().UTC()

			inst, err := model.GetInstanceByInstanceID(instanceID)
			if err != nil {
				fmt.Printf("Failed to get instance by instance ID %s: %v\n", instanceID, err)
			}

			data := ws.InstanceStatusChangedData{
				InstanceID:     instanceID,
				PhoneNumber:    inst.PhoneNumber.String,
				Status:         "logged_out",
				IsConnected:    false,
				ConnectedAt:    &inst.ConnectedAt.Time,
				DisconnectedAt: &now,
			}

			evt := ws.WsEvent{
				Event:     ws.EventInstanceStatusChanged,
				Timestamp: now,
				Data:      data,
			}

			Realtime.Publish(evt)
		}
	}

	// Clean up flag
	loggingOutLock.Lock()
	delete(loggingOut, instanceID)
	loggingOutLock.Unlock()

	fmt.Println("✓ Device logged out, session cleared. Instance kept in DB:", instanceID)
	return nil
}

func DeleteInstance(instanceID string) error {
	inst, err := model.GetInstanceByInstanceID(instanceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInstanceNotFound
		}
		return fmt.Errorf("get instance: %w", err)
	}

	if inst.IsConnected || inst.Status == "online" {
		return ErrInstanceStillConnected
	}

	// Opsional: bersihkan session in-memory + store whatsmeow
	sess, err := GetSession(instanceID)
	if err == nil && sess.Client != nil {
		sess.Client.Disconnect()
		// Hapus data store whatsmeow
		_ = sess.Client.Store.Delete(context.Background())

		DeleteSessionFromMemory(instanceID)
	}

	if err := model.DeleteInstanceByInstanceID(instanceID); err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}

	return nil
}

// Hapus session whatsmeow
func DeleteSessionFromMemory(instanceID string) {
	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	if _, ok := sessions[instanceID]; ok {
		delete(sessions, instanceID)
		fmt.Println("Session removed from memory:", instanceID)
	}
}
