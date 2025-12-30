package service

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gowa-yourself/config"
	"gowa-yourself/internal/helper"
	warmingModel "gowa-yourself/internal/model/warming"
	"gowa-yourself/internal/ws"
)

var (
	lastReplyTime sync.Map // map[roomID]time.Time
)

func HandleIncomingMessage(instanceID, sender, messageText string) error {
	if !config.WarmingAutoReplyEnabled {
		return nil
	}

	room, err := warmingModel.GetActiveHumanRoomBySender(sender)
	if err != nil {
		log.Printf("[HUMAN_VS_BOT] Error checking room: %v", err)
		return fmt.Errorf("failed to check human room: %w", err)
	}

	if room == nil {
		return nil
	}

	// Rate limiting: check cooldown
	roomKey := room.ID.String()
	if lastTime, ok := lastReplyTime.Load(roomKey); ok {
		elapsed := time.Since(lastTime.(time.Time))
		cooldown := time.Duration(config.WarmingAutoReplyCooldown) * time.Second

		if elapsed < cooldown {
			remaining := cooldown - elapsed
			log.Printf("[HUMAN_VS_BOT] Rate limit: ignoring message from %s (cooldown: %v remaining)", sender, remaining.Round(time.Second))
			return nil
		}
	}

	// Update last reply time immediately to prevent race condition
	lastReplyTime.Store(roomKey, time.Now())

	go processAutoReply(room, instanceID, sender)
	return nil
}

func processAutoReply(room *warmingModel.WarmingRoom, instanceID, sender string) {
	delay := calculateDelay(room)

	time.Sleep(delay)

	reply, lineID, err := getScriptReply(room)
	if err != nil {
		log.Printf("[HUMAN_VS_BOT] Error getting script reply: %v", err)
		return
	}

	if reply == "" {
		return
	}

	if err := sendReply(instanceID, sender, reply, lineID, room); err != nil {
		log.Printf("[HUMAN_VS_BOT] Error sending reply: %v", err)
		return
	}
}

func getScriptReply(room *warmingModel.WarmingRoom) (string, int64, error) {
	scriptLine, err := warmingModel.GetNextAvailableScriptLine(room.ScriptID, room.CurrentSequence)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get script line: %w", err)
	}

	if scriptLine == nil {
		if err := warmingModel.FinishRoom(room.ID); err != nil {
			log.Printf("[HUMAN_VS_BOT] Error finishing room: %v", err)
		}
		return "", 0, nil
	}

	processedMessage := helper.RenderSpintax(scriptLine.MessageContent)

	nextSeq := room.CurrentSequence + 1
	if err := warmingModel.UpdateRoomProgress(room.ID, nextSeq, time.Now().Add(time.Minute)); err != nil {
		log.Printf("[HUMAN_VS_BOT] Error updating room progress: %v", err)
	}

	return processedMessage, scriptLine.ID, nil
}

func calculateDelay(room *warmingModel.WarmingRoom) time.Duration {
	min := room.ReplyDelayMin
	max := room.ReplyDelayMax

	if max <= min {
		return time.Duration(min) * time.Second
	}

	range_ := max - min + 1
	randomOffset := time.Now().UnixNano() % int64(range_)
	randomDelay := min + int(randomOffset)

	return time.Duration(randomDelay) * time.Second
}

func sendReply(instanceID, recipient, message string, lineID int64, room *warmingModel.WarmingRoom) error {
	if !room.SendRealMessage {
		warmingModel.CreateWarmingLog(room.ID, lineID, instanceID, recipient, message, "SUCCESS", "dry-run mode")
		publishHumanVsBotEvent(room, lineID, instanceID, recipient, message, "SUCCESS", "dry-run mode")
		return nil
	}

	success, errMsg := SendWarmingMessageToPhone(instanceID, recipient, message)

	if !success {
		warmingModel.CreateWarmingLog(room.ID, lineID, instanceID, recipient, message, "FAILED", errMsg)
		publishHumanVsBotEvent(room, lineID, instanceID, recipient, message, "FAILED", errMsg)
		return errors.New(errMsg)
	}

	warmingModel.CreateWarmingLog(room.ID, lineID, instanceID, recipient, message, "SUCCESS", "")
	publishHumanVsBotEvent(room, lineID, instanceID, recipient, message, "SUCCESS", "")

	return nil
}

func publishHumanVsBotEvent(room *warmingModel.WarmingRoom, lineID int64, senderID, receiverID, message, status, errorMsg string) {
	if Realtime == nil {
		return
	}

	event := ws.WsEvent{
		Event:     ws.EventWarmingMessage,
		Timestamp: time.Now().UTC(),
		Data: ws.WarmingMessageData{
			RoomID:             room.ID.String(),
			RoomName:           room.Name,
			SenderInstanceID:   senderID,
			ReceiverInstanceID: receiverID,
			Message:            message,
			SequenceOrder:      room.CurrentSequence,
			ActorRole:          "BOT",
			Status:             status,
			ErrorMessage:       errorMsg,
			Timestamp:          time.Now().UTC(),
		},
	}

	Realtime.Publish(event)
}
