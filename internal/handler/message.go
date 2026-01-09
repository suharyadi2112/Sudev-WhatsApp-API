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

type CheckNumberRequest struct {
	Phone string `json:"phone" validate:"required"`
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

// POST /check/:instanceId
func CheckNumber(c echo.Context) error {
	instanceID := c.Param("instanceId")

	var req CheckNumberRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, 400, "Invalid request body", "INVALID_REQUEST", err.Error())
	}

	session, err := service.GetSession(instanceID)
	if err != nil {
		return ErrorResponse(c, 404, "Session not found", "SESSION_NOT_FOUND", "")
	}

	if !session.Client.IsConnected() {
		return ErrorResponse(c, 400, "Session is not connected", "NOT_CONNECTED", "")
	}

	recipient, err := helper.FormatPhoneNumber(req.Phone)
	if err != nil {
		return ErrorResponse(c, 400, "Invalid phone number", "INVALID_PHONE", err.Error())
	}

	willSkipValidation := helper.ShouldSkipValidation(req.Phone)

	isRegistered, err := session.Client.IsOnWhatsApp(context.Background(), []string{recipient.User})
	if err != nil {
		return ErrorResponse(c, 500, "Failed to check phone number", "CHECK_FAILED", err.Error())
	}

	if len(isRegistered) == 0 {
		return ErrorResponse(c, 400, "Unable to verify number", "VERIFICATION_ERROR", "")
	}

	return SuccessResponse(c, 200, "Phone number checked", map[string]interface{}{
		"phone":              req.Phone,
		"isRegistered":       isRegistered[0].IsIn,
		"jid":                isRegistered[0].JID.String(),
		"willSkipValidation": willSkipValidation,
		"note":               getValidationNote(isRegistered[0].IsIn, willSkipValidation),
	})
}

// Helper function to provide user-friendly note about validation behavior
func getValidationNote(isRegistered, willSkip bool) string {
	if willSkip {
		if isRegistered {
			return "Number is registered. Validation will be skipped when sending (ALLOW_9_DIGIT_PHONE_NUMBER=true)"
		}
		return "Number appears unregistered, but validation will be skipped when sending (ALLOW_9_DIGIT_PHONE_NUMBER=true). Message will be attempted anyway."
	}
	if isRegistered {
		return "Number is registered and will pass validation when sending"
	}
	return "Number is not registered. Message sending will be blocked unless ALLOW_9_DIGIT_PHONE_NUMBER=true is set"
}

// GET /contacts/:instanceId?page=1&limit=50
func GetContactList(c echo.Context) error {
	instanceID := c.Param("instanceId")

	session, err := service.GetSession(instanceID)
	if err != nil {
		return ErrorResponse(c, 404, "Session not found", "SESSION_NOT_FOUND", "Please login first")
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

	// Parse pagination params (default: page=1, limit=50, max=100)
	page := 1
	limit := 50

	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	contacts, err := session.Client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return ErrorResponse(c, 500, "Failed to retrieve contact list", "FETCH_FAILED", err.Error())
	}

	type ContactInfo struct {
		JID     string `json:"jid"`
		Name    string `json:"name"`
		IsGroup bool   `json:"isGroup"`
	}

	// Build contact list with name fallback
	allContacts := make([]ContactInfo, 0, len(contacts))
	for jid, contact := range contacts {
		contactInfo := ContactInfo{
			JID:     jid.String(),
			Name:    contact.FullName,
			IsGroup: jid.Server == "g.us",
		}

		if contactInfo.Name == "" {
			if contact.BusinessName != "" {
				contactInfo.Name = contact.BusinessName
			} else if contact.PushName != "" {
				contactInfo.Name = contact.PushName
			} else {
				contactInfo.Name = jid.User
			}
		}

		allContacts = append(allContacts, contactInfo)
	}

	// Calculate pagination
	totalContacts := len(allContacts)
	totalPages := (totalContacts + limit - 1) / limit

	startIndex := (page - 1) * limit
	endIndex := startIndex + limit

	// Handle out of range page
	if startIndex >= totalContacts {
		return SuccessResponse(c, 200, "Contact list retrieved successfully", map[string]interface{}{
			"total":       totalContacts,
			"page":        page,
			"limit":       limit,
			"totalPages":  totalPages,
			"contacts":    []ContactInfo{},
			"hasNextPage": false,
			"hasPrevPage": page > 1,
		})
	}

	if endIndex > totalContacts {
		endIndex = totalContacts
	}

	paginatedContacts := allContacts[startIndex:endIndex]

	return SuccessResponse(c, 200, "Contact list retrieved successfully", map[string]interface{}{
		"total":       totalContacts,
		"page":        page,
		"limit":       limit,
		"totalPages":  totalPages,
		"contacts":    paginatedContacts,
		"hasNextPage": page < totalPages,
		"hasPrevPage": page > 1,
	})
}
