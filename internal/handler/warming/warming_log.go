package warming

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"gowa-yourself/internal/handler"
	warmingModel "gowa-yourself/internal/model/warming"
	warmingService "gowa-yourself/internal/service/warming"

	"github.com/labstack/echo/v4"
)

// GetAllWarmingLogs handles GET /warming/logs
func GetAllWarmingLogs(c echo.Context) error {
	roomID := c.QueryParam("roomId")
	status := c.QueryParam("status")
	limitStr := c.QueryParam("limit")

	limit := 100 // Default
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil {
			limit = parsedLimit
		}
	}

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

	logs, err := warmingService.GetAllWarmingLogsService(roomID, status, limit, userID, isAdmin)
	if err != nil {
		if strings.Contains(err.Error(), "invalid status") {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "INVALID_STATUS", "")
		}
		if strings.Contains(err.Error(), "invalid room ID") {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "INVALID_ROOM_ID", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to get logs", "GET_FAILED", err.Error())
	}

	var responses []warmingModel.WarmingLogResponse
	for _, log := range logs {
		responses = append(responses, warmingModel.ToWarmingLogResponse(log))
	}

	return handler.SuccessResponse(c, http.StatusOK, "Logs retrieved successfully", map[string]interface{}{
		"total": len(responses),
		"logs":  responses,
	})
}

// GetWarmingLogByID handles GET /warming/logs/:id
func GetWarmingLogByID(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid log ID", "INVALID_ID", err.Error())
	}

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

	log, err := warmingService.GetWarmingLogByIDService(id, userID, isAdmin)
	if err != nil {
		if errors.Is(err, warmingService.ErrLogNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Log not found", "NOT_FOUND", "")
		}
		if strings.Contains(err.Error(), "forbidden") {
			return handler.ErrorResponse(c, http.StatusForbidden, err.Error(), "FORBIDDEN", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to get log", "GET_FAILED", err.Error())
	}

	resp := warmingModel.ToWarmingLogResponse(*log)
	return handler.SuccessResponse(c, http.StatusOK, "Log retrieved successfully", resp)
}
