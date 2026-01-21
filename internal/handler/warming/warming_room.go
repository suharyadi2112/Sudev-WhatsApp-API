package warming

import (
	"errors"
	"net/http"
	"strings"

	"gowa-yourself/internal/handler"
	warmingModel "gowa-yourself/internal/model/warming"
	warmingService "gowa-yourself/internal/service/warming"

	"github.com/labstack/echo/v4"
)

// CreateWarmingRoom handles POST /warming/rooms
func CreateWarmingRoom(c echo.Context) error {
	var req warmingModel.CreateWarmingRoomRequest
	if err := c.Bind(&req); err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	// Extract user ID from JWT context
	userID, ok := c.Get("user_id").(int64)
	if !ok {
		return handler.ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	room, err := warmingService.CreateWarmingRoomService(&req, userID)
	if err != nil {
		if errors.Is(err, warmingService.ErrRoomNameRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "NAME_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrRoomSenderRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "SENDER_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrRoomReceiverRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "RECEIVER_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrRoomScriptRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "SCRIPT_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrRoomIntervalInvalid) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "INTERVAL_INVALID", "")
		}
		if errors.Is(err, warmingService.ErrRoomSameInstance) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "SAME_INSTANCE", "")
		}
		if strings.Contains(err.Error(), "script not found") {
			return handler.ErrorResponse(c, http.StatusNotFound, "Script not found", "SCRIPT_NOT_FOUND", "")
		}

		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to create room", "CREATE_FAILED", err.Error())
	}

	resp := warmingModel.ToWarmingRoomResponse(*room)
	return handler.SuccessResponse(c, http.StatusOK, "Room created successfully", resp)
}

// GetAllWarmingRooms handles GET /warming/rooms
func GetAllWarmingRooms(c echo.Context) error {
	status := c.QueryParam("status")

	// Extract user context from JWT
	userID, ok := c.Get("user_id").(int64)
	if !ok {
		return handler.ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	role, ok := c.Get("role").(string)
	if !ok {
		role = "user"
	}
	isAdmin := role == "admin"

	rooms, err := warmingService.GetAllWarmingRoomsService(status, userID, isAdmin)
	if err != nil {
		if strings.Contains(err.Error(), "invalid status") {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "INVALID_STATUS", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to get rooms", "GET_FAILED", err.Error())
	}

	var responses []warmingModel.WarmingRoomResponse
	for _, room := range rooms {
		responses = append(responses, warmingModel.ToWarmingRoomResponse(room))
	}

	return handler.SuccessResponse(c, http.StatusOK, "Rooms retrieved successfully", map[string]interface{}{
		"total": len(responses),
		"rooms": responses,
	})
}

// GetWarmingRoomByID handles GET /warming/rooms/:id
func GetWarmingRoomByID(c echo.Context) error {
	id := c.Param("id")

	room, err := warmingService.GetWarmingRoomByIDService(id)
	if err != nil {
		if errors.Is(err, warmingService.ErrRoomNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Room not found", "NOT_FOUND", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to get room", "GET_FAILED", err.Error())
	}

	resp := warmingModel.ToWarmingRoomResponse(*room)
	return handler.SuccessResponse(c, http.StatusOK, "Room retrieved successfully", resp)
}

// UpdateWarmingRoom handles PUT /warming/rooms/:id
func UpdateWarmingRoom(c echo.Context) error {
	id := c.Param("id")

	var req warmingModel.UpdateWarmingRoomRequest
	if err := c.Bind(&req); err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	err := warmingService.UpdateWarmingRoomService(id, &req)
	if err != nil {
		if errors.Is(err, warmingService.ErrRoomNameRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "NAME_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrRoomIntervalInvalid) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "INTERVAL_INVALID", "")
		}
		if errors.Is(err, warmingService.ErrRoomNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Room not found", "NOT_FOUND", "")
		}

		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to update room", "UPDATE_FAILED", err.Error())
	}

	return handler.SuccessResponse(c, http.StatusOK, "Room updated successfully", map[string]interface{}{
		"id": id,
	})
}

// DeleteWarmingRoom handles DELETE /warming/rooms/:id
func DeleteWarmingRoom(c echo.Context) error {
	id := c.Param("id")

	err := warmingService.DeleteWarmingRoomService(id)
	if err != nil {
		if errors.Is(err, warmingService.ErrRoomNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Room not found", "NOT_FOUND", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete room", "DELETE_FAILED", err.Error())
	}

	return handler.SuccessResponse(c, http.StatusOK, "Room deleted successfully", map[string]interface{}{
		"id": id,
	})
}

// UpdateRoomStatus handles PATCH /warming/rooms/:id/status
func UpdateRoomStatus(c echo.Context) error {
	id := c.Param("id")

	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	err := warmingService.UpdateRoomStatusService(id, req.Status)
	if err != nil {
		if errors.Is(err, warmingService.ErrRoomNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Room not found", "NOT_FOUND", "")
		}
		if strings.Contains(err.Error(), "invalid status") {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "INVALID_STATUS", "")
		}
		if strings.Contains(err.Error(), "already in") {
			return handler.ErrorResponse(c, http.StatusConflict, err.Error(), "ALREADY_IN_STATUS", "")
		}

		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to update room status", "UPDATE_STATUS_FAILED", err.Error())
	}

	return handler.SuccessResponse(c, http.StatusOK, "Room status updated successfully", map[string]interface{}{
		"id":     id,
		"status": req.Status,
	})
}

// RestartWarmingRoom handles POST /warming/rooms/:id/restart
func RestartWarmingRoom(c echo.Context) error {
	id := c.Param("id")

	err := warmingService.RestartRoomService(id)
	if err != nil {
		if errors.Is(err, warmingService.ErrRoomNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Room not found", "NOT_FOUND", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to restart room", "RESTART_FAILED", err.Error())
	}

	return handler.SuccessResponse(c, http.StatusOK, "Room restarted successfully", map[string]interface{}{
		"id":               id,
		"current_sequence": 0,
		"status":           "ACTIVE",
	})
}
