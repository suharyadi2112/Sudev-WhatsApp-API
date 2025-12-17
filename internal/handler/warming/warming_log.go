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

	logs, err := warmingService.GetAllWarmingLogsService(roomID, status, limit)
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

	log, err := warmingService.GetWarmingLogByIDService(id)
	if err != nil {
		if errors.Is(err, warmingService.ErrLogNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Log not found", "NOT_FOUND", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to get log", "GET_FAILED", err.Error())
	}

	resp := warmingModel.ToWarmingLogResponse(*log)
	return handler.SuccessResponse(c, http.StatusOK, "Log retrieved successfully", resp)
}
