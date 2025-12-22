package service

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// SendWarmingMessage sends a WhatsApp message for warming system
// Returns (success bool, error message string)
func SendWarmingMessage(senderInstanceID, receiverInstanceID, message string) (bool, string) {
	// Get sender session
	senderSession, err := GetSession(senderInstanceID)
	if err != nil {
		return false, fmt.Sprintf("sender session not found: %v", err)
	}

	if !senderSession.IsConnected || !senderSession.Client.IsConnected() {
		return false, "sender not connected"
	}

	if senderSession.Client.Store.ID == nil {
		return false, "sender not logged in"
	}

	// Get receiver session to get JID
	receiverSession, err := GetSession(receiverInstanceID)
	if err != nil {
		return false, fmt.Sprintf("receiver session not found: %v", err)
	}

	if receiverSession.JID == "" {
		return false, "receiver JID not found"
	}

	// Parse receiver JID
	recipientJID, err := types.ParseJID(receiverSession.JID)
	if err != nil {
		return false, fmt.Sprintf("invalid receiver JID: %v", err)
	}

	ctx := context.Background()

	// Check if receiver is registered on WhatsApp (same as handler.SendMessage)
	isRegistered, err := senderSession.Client.IsOnWhatsApp(ctx, []string{recipientJID.User})
	if err != nil {
		return false, fmt.Sprintf("failed to verify receiver number: %v", err)
	}

	if len(isRegistered) == 0 || !isRegistered[0].IsIn {
		return false, "receiver phone number is not registered on WhatsApp"
	}

	// Typing simulation (same as SendMessage handler)
	messageLength := len(message)
	baseDelay := 2
	typingSpeed := 0.15
	calculatedDelay := baseDelay + int(float64(messageLength)*typingSpeed)

	// Add random variation ±20%
	variationRange := int(float64(calculatedDelay) * 0.4)
	if variationRange < 1 {
		variationRange = 1 // Pastikan minimal 1 untuk menghindari panic
	}
	variation := rand.Intn(variationRange) - int(float64(calculatedDelay)*0.2)
	finalDelay := calculatedDelay + variation

	// Limit delay (min 3 sec, max 30 sec)
	if finalDelay > 30 {
		finalDelay = 30
	}
	if finalDelay < 3 {
		finalDelay = 3
	}

	// Override with env variable if exists
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

	// Send typing status
	_ = senderSession.Client.SendChatPresence(ctx, recipientJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)

	// Wait 70% of time
	time.Sleep(time.Duration(finalDelay*70/100) * time.Second)

	// Pause occasionally (30% chance for messages > 50 chars)
	if messageLength > 50 && rand.Intn(100) < 30 {
		_ = senderSession.Client.SendChatPresence(ctx, recipientJID, types.ChatPresencePaused, types.ChatPresenceMediaText)
		time.Sleep(time.Duration(rand.Intn(2)+1) * time.Second)
		_ = senderSession.Client.SendChatPresence(ctx, recipientJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	}

	// Wait remaining 30% of time
	time.Sleep(time.Duration(finalDelay*30/100) * time.Second)

	// Send message
	msg := &waE2E.Message{
		Conversation: &message,
	}

	_, err = senderSession.Client.SendMessage(ctx, recipientJID, msg)
	if err != nil {
		return false, fmt.Sprintf("failed to send message: %v", err)
	}

	return true, ""
}
