package handler

import (
	"gowa-yourself/internal/model"
	"gowa-yourself/internal/service"
	"net/http"

	"github.com/labstack/echo/v4"
)

// GetInstanceTimeline handles GET /api/instances/:instanceId/timeline
func GetInstanceTimeline(c echo.Context) error {
	instanceID := c.Param("instanceId")
	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")

	events, err := model.GetInstanceTimeline(instanceID, startDate, endDate)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to get timeline", "GET_FAILED", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Timeline retrieved successfully", map[string]interface{}{
		"instanceId": instanceID,
		"total":      len(events),
		"events":     events,
	})
}

// GetGlobalTimeline handles GET /api/timeline
func GetGlobalTimeline(c echo.Context) error {
	userID, ok := c.Get("user_id").(int64)
	if !ok {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")

	var role string
	userClaims, ok := c.Get("user_claims").(*service.Claims)
	if ok && userClaims != nil {
		role = userClaims.Role
	}

	events, err := model.GetGlobalTimeline(userID, role, startDate, endDate)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to get global timeline", "GET_FAILED", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Global timeline retrieved successfully", map[string]interface{}{
		"total":  len(events),
		"events": events,
	})
}
