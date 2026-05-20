package handler

import (
	"gowa-yourself/internal/model"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// parseAndValidateDate validates a date string in YYYY-MM-DD or YYYY-MM-DD HH:MM:SS format
func parseAndValidateDate(dateStr string) (time.Time, error) {
	// Try full timestamp first
	if t, err := time.Parse("2006-01-02 15:04:05", dateStr); err == nil {
		return t, nil
	}
	// Fallback to date only
	return time.Parse("2006-01-02", dateStr)
}

// CreateAttendance handles POST /api/attendance
func CreateAttendance(c echo.Context) error {
	userID, ok := c.Get("user_id").(int64)
	if !ok {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	var req model.CreateAttendanceRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	if req.AttendanceType == "" || req.Title == "" {
		return ErrorResponse(c, http.StatusBadRequest, "attendance_type and title are required", "VALIDATION_ERROR", "")
	}

	attendance, err := model.CreateAttendance(userID, &req)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to create attendance", "CREATE_FAILED", err.Error())
	}

	return SuccessResponse(c, http.StatusCreated, "Attendance created successfully", attendance)
}

// GetAttendances handles GET /api/attendance
func GetAttendances(c echo.Context) error {
	userID, ok := c.Get("user_id").(int64)
	if !ok {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	role, ok := c.Get("role").(string)
	if !ok {
		role = "user"
	}
	isAdmin := role == "admin"

	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")
	attendanceType := c.QueryParam("attendance_type")
	phoneNumber := c.QueryParam("phone_number")
	simLabel := c.QueryParam("sim_label")
	instanceID := c.QueryParam("instance_id")
	if instanceID == "" {
		instanceID = c.QueryParam("instanceId")
	}

	// Validate date format and validity
	if startDate != "" {
		if _, err := parseAndValidateDate(startDate); err != nil {
			return ErrorResponse(c, http.StatusBadRequest, "Invalid start_date", "INVALID_DATE", err.Error())
		}
	}

	if endDate != "" {
		if _, err := parseAndValidateDate(endDate); err != nil {
			return ErrorResponse(c, http.StatusBadRequest, "Invalid end_date", "INVALID_DATE", err.Error())
		}
	}

	attendances, err := model.GetAttendances(userID, isAdmin, startDate, endDate, attendanceType, phoneNumber, simLabel, instanceID)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to get attendances", "GET_FAILED", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Attendances retrieved successfully", map[string]interface{}{
		"total":       len(attendances),
		"attendances": attendances,
	})
}

// UpdateAttendance handles PATCH /api/attendance/:id
func UpdateAttendance(c echo.Context) error {
	userID, ok := c.Get("user_id").(int64)
	if !ok {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid attendance ID", "INVALID_ID", err.Error())
	}

	var req model.UpdateAttendanceRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	err = model.UpdateAttendance(id, userID, &req)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to update attendance", "UPDATE_FAILED", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Attendance updated successfully", nil)
}

// DeleteAttendance handles DELETE /api/attendance/:id
func DeleteAttendance(c echo.Context) error {
	userID, ok := c.Get("user_id").(int64)
	if !ok {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid attendance ID", "INVALID_ID", err.Error())
	}

	err = model.DeleteAttendance(id, userID)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to delete attendance", "DELETE_FAILED", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Attendance deleted successfully", nil)
}
