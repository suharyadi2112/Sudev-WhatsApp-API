package handler

import (
	"gowa-yourself/internal/service"
	"gowa-yourself/internal/ws"
	"log"

	"github.com/labstack/echo/v4"
)

// GET /listen/:instanceId - WebSocket endpoint untuk listen incoming messages
func ListenMessages(hub *ws.Hub) echo.HandlerFunc {
	return func(c echo.Context) error {
		instanceID := c.Param("instanceId")

		// Validasi instanceID
		if instanceID == "" {
			return ErrorResponse(c, 400, "instanceId is required", "VALIDATION_ERROR", "")
		}

		// Validasi session exists dan connected
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

		// Upgrade ke WebSocket
		conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return ErrorResponse(c, 500, "Failed to upgrade WebSocket", "UPGRADE_FAILED", err.Error())
		}

		// Buat client dan register
		client := ws.NewClient(hub, conn)
		client.InstanceID = instanceID

		hub.Register(client)

		log.Printf("Client connected to listen instance: %s", instanceID)

		// Jalankan pump
		go client.WritePump()
		go client.ReadPump()

		return nil
	}
}
