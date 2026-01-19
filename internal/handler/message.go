package handler

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"strconv"
	"time"

	"gowa-yourself/internal/helper"
	"gowa-yourself/internal/service"

	"gowa-yourself/internal/model"

	"github.com/labstack/echo/v4"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// Request body untuk send message
type SendMessageRequest struct {
	To      string `json:"to" validate:"required"`
	Message string `json:"message" validate:"required"`
}

// POST /send/:instanceId
func SendMessage(c echo.Context) error {
	instanceID := c.Param("instanceId")

	var req SendMessageRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, 400, "Invalid request body", "INVALID_REQUEST", err.Error())
	}

	if req.To == "" || req.Message == "" {
		return ErrorResponse(c, 400, "Field 'to' and 'message' are required", "VALIDATION_ERROR", "")
	}

	session, err := service.GetSession(instanceID)
	if err != nil {
		return ErrorResponse(c, 404, "Session not found", "SESSION_NOT_FOUND", "Please login first")
	}

	// Validate instance used flag
	if err := model.ValidateInstanceUsed(instanceID); err != nil {
		if errors.Is(err, model.ErrInstanceNotAvailable) {
			return ErrorResponse(c, 403, "Instance is blocked from sending messages. Please check the used flag", "INSTANCE_NOT_AVAILABLE",
				"This instance is currently blocked. Please activate the instance first by setting 'used' flag to true via PATCH /api/instances/:instanceId")
		}
		return ErrorResponse(c, 500, "Failed to validate instance", "VALIDATION_ERROR", err.Error())
	}

	if !session.IsConnected {
		return ErrorResponse(c, 400, "Session is not connected", "NOT_CONNECTED", "Please check /status endpoint")
	}

	if !session.Client.IsConnected() {
		return ErrorResponse(c, 400, "WhatsApp connection lost", "CONNECTION_LOST", "Please reconnect")
	}

	if session.Client.Store.ID == nil {
		return ErrorResponse(c, 400, "Not logged in", "NOT_LOGGED_IN", "Please scan QR code first")
	}

	recipient, err := helper.FormatPhoneNumber(req.To)
	if err != nil {
		return ErrorResponse(c, 400, "Invalid phone number", "INVALID_PHONE", err.Error())
	}

	if !helper.ShouldSkipValidation(req.To) {
		isRegistered, err := session.Client.IsOnWhatsApp(context.Background(), []string{recipient.User})
		if err != nil {
			return ErrorResponse(c, 500, "Failed to verify phone number", "VERIFICATION_FAILED", err.Error())
		}

		if len(isRegistered) == 0 || !isRegistered[0].IsIn {
			return ErrorResponse(c, 400, "Phone number is not registered on WhatsApp", "PHONE_NOT_REGISTERED",
				"Please check the number or ask recipient to install WhatsApp")
		}
	}

	// Simulasi typing yang lebih natural
	messageLength := len(req.Message)
	baseDelay := 2      // detik minimum
	typingSpeed := 0.15 // detik per karakter (simulasi kecepatan mengetik)
	calculatedDelay := baseDelay + int(float64(messageLength)*typingSpeed)

	// Tambahkan variasi random ±20%
	variationRange := int(float64(calculatedDelay) * 0.4)
	if variationRange < 1 {
		variationRange = 1 // Pastikan minimal 1 untuk menghindari panic
	}
	variation := rand.Intn(variationRange) - int(float64(calculatedDelay)*0.2)
	finalDelay := calculatedDelay + variation

	// Batasi delay (min 3 detik, max 30 detik)
	if finalDelay > 30 {
		finalDelay = 30
	}
	if finalDelay < 3 {
		finalDelay = 3
	}

	// Override dengan env variable jika ada
	minDelayStr := os.Getenv("SUDEVWA_TYPING_DELAY_MIN")
	maxDelayStr := os.Getenv("SUDEVWA_TYPING_DELAY_MAX")
	if minDelayStr != "" && maxDelayStr != "" {
		min, _ := strconv.Atoi(minDelayStr)
		max, _ := strconv.Atoi(maxDelayStr)
		if max >= min && min > 0 {
			rangeVal := max - min + 1
			if rangeVal > 0 {
				finalDelay = rand.Intn(rangeVal) + min
			}
		}
	}

	// Kirim status Typing
	_ = session.Client.SendChatPresence(context.Background(), recipient, types.ChatPresenceComposing, types.ChatPresenceMediaText)

	// Tunggu sebagian waktu (70%)
	time.Sleep(time.Duration(finalDelay*70/100) * time.Second)

	// Pause sejenak (30% kemungkinan untuk pesan > 50 karakter)
	if messageLength > 50 && rand.Intn(100) < 30 {
		_ = session.Client.SendChatPresence(context.Background(), recipient, types.ChatPresencePaused, types.ChatPresenceMediaText)
		time.Sleep(time.Duration(rand.Intn(2)+1) * time.Second)
		_ = session.Client.SendChatPresence(context.Background(), recipient, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	}

	// Tunggu sisa waktu (30%)
	time.Sleep(time.Duration(finalDelay*30/100) * time.Second)

	msg := &waE2E.Message{
		Conversation: &req.Message,
	}

	resp, err := session.Client.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		return ErrorResponse(c, 500, "Failed to send message", "SEND_FAILED", err.Error())
	}

	return SuccessResponse(c, 200, "Message sent successfully", map[string]interface{}{
		"messageId": resp.ID,
		"timestamp": resp.Timestamp.Unix(),
		"to":        req.To,
		"verified":  true,
	})
}

// POST /send/by-number/:phoneNumber
func SendMessageByNumber(c echo.Context) error {
	phoneNumber := c.Param("phoneNumber")

	var req SendMessageRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, 400, "Invalid request body", "INVALID_REQUEST", err.Error())
	}

	if req.To == "" || req.Message == "" {
		return ErrorResponse(c, 400, "Field 'to' and 'message' are required", "VALIDATION_ERROR", "")
	}

	//Cari instance aktif berdasarkan nomor pengirim (phoneNumber)
	inst, err := model.GetActiveInstanceByPhoneNumber(phoneNumber)
	if err != nil {
		if errors.Is(err, model.ErrNoActiveInstance) {
			return ErrorResponse(c, 404,
				"No active instance for this phone number",
				"NO_ACTIVE_INSTANCE",
				"Please login / scan QR for this number",
			)
		}
		return ErrorResponse(c, 500,
			"Failed to get instance for this phone number",
			"DB_ERROR",
			err.Error(),
		)
	}

	// 1.5) Check Permission
	userClaims, _ := c.Get("user_claims").(*service.Claims)
	if userClaims != nil && userClaims.Role != "admin" {
		_, err := model.CheckUserInstancePermission(userClaims.UserID, inst.InstanceID)
		if err != nil {
			return ErrorResponse(c, 403, "Insufficient permission to use this phone number", "FORBIDDEN", "")
		}
	}

	// 2) Ambil session dari memory berdasarkan instance_id
	session, err := service.GetSession(inst.InstanceID)
	if err != nil {
		return ErrorResponse(c, 404, "Session not found", "SESSION_NOT_FOUND", "Please login / reconnect first")
	}

	// Validate instance used flag
	if err := model.ValidateInstanceUsed(inst.InstanceID); err != nil {
		if errors.Is(err, model.ErrInstanceNotAvailable) {
			return ErrorResponse(c, 403, "Instance is blocked from sending messages. Please check the used flag", "INSTANCE_NOT_AVAILABLE",
				"This instance is currently blocked. Please activate the instance first by setting 'used' flag to true via PATCH /api/instances/:instanceId")
		}
		return ErrorResponse(c, 500, "Failed to validate instance", "VALIDATION_ERROR", err.Error())
	}

	// 3) Validasi koneksi sama seperti fungsi lama
	if !session.IsConnected || !session.Client.IsConnected() || session.Client.Store.ID == nil {
		return ErrorResponse(c, 400, "WhatsApp session is not connected", "NOT_CONNECTED", "Please scan QR or reconnect")
	}

	// 4) Format recipient & cek registered
	recipient, err := helper.FormatPhoneNumber(req.To)
	if err != nil {
		return ErrorResponse(c, 400, "Invalid phone number", "INVALID_PHONE", err.Error())
	}

	if !helper.ShouldSkipValidation(req.To) {
		isRegistered, err := session.Client.IsOnWhatsApp(context.Background(), []string{recipient.User})
		if err != nil {
			return ErrorResponse(c, 500, "Failed to verify phone number", "VERIFICATION_FAILED", err.Error())
		}
		if len(isRegistered) == 0 || !isRegistered[0].IsIn {
			return ErrorResponse(c, 400, "Phone number is not registered on WhatsApp", "PHONE_NOT_REGISTERED",
				"Please check the number or ask recipient to install WhatsApp")
		}
	}

	// Simulasi typing yang lebih natural
	messageLength := len(req.Message)
	baseDelay := 2      // detik minimum
	typingSpeed := 0.15 // detik per karakter (simulasi kecepatan mengetik)
	calculatedDelay := baseDelay + int(float64(messageLength)*typingSpeed)

	// Tambahkan variasi random ±20%
	variationRange := int(float64(calculatedDelay) * 0.4)
	if variationRange < 1 {
		variationRange = 1 // Pastikan minimal 1 untuk menghindari panic
	}
	variation := rand.Intn(variationRange) - int(float64(calculatedDelay)*0.2)
	finalDelay := calculatedDelay + variation

	// Batasi delay (min 3 detik, max 30 detik)
	if finalDelay > 30 {
		finalDelay = 30
	}
	if finalDelay < 3 {
		finalDelay = 3
	}

	// Override dengan env variable jika ada
	minDelayStr := os.Getenv("SUDEVWA_TYPING_DELAY_MIN")
	maxDelayStr := os.Getenv("SUDEVWA_TYPING_DELAY_MAX")
	if minDelayStr != "" && maxDelayStr != "" {
		min, _ := strconv.Atoi(minDelayStr)
		max, _ := strconv.Atoi(maxDelayStr)
		if max >= min && min > 0 {
			rangeVal := max - min + 1
			if rangeVal > 0 {
				finalDelay = rand.Intn(rangeVal) + min
			}
		}
	}

	// Kirim status Typing
	_ = session.Client.SendChatPresence(context.Background(), recipient, types.ChatPresenceComposing, types.ChatPresenceMediaText)

	// Tunggu sebagian waktu (70%)
	time.Sleep(time.Duration(finalDelay*70/100) * time.Second)

	// Pause sejenak (30% kemungkinan untuk pesan > 50 karakter)
	if messageLength > 50 && rand.Intn(100) < 30 {
		_ = session.Client.SendChatPresence(context.Background(), recipient, types.ChatPresencePaused, types.ChatPresenceMediaText)
		time.Sleep(time.Duration(rand.Intn(2)+1) * time.Second)
		_ = session.Client.SendChatPresence(context.Background(), recipient, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	}

	// Tunggu sisa waktu (30%)
	time.Sleep(time.Duration(finalDelay*30/100) * time.Second)

	// 5) Kirim pesan
	msg := &waE2E.Message{
		Conversation: &req.Message,
	}
	resp, err := session.Client.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		return ErrorResponse(c, 500, "Failed to send message", "SEND_FAILED", err.Error())
	}

	return SuccessResponse(c, 200, "Message sent successfully", map[string]interface{}{
		"messageId": resp.ID,
		"timestamp": resp.Timestamp.Unix(),
		"from":      phoneNumber, // nomor pengirim
		"to":        req.To,
		"verified":  true,
	})
}
