package handler

import (
	"gowa-yourself/internal/model"
	"gowa-yourself/internal/service"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// getClaims is a helper to get user claims from context
func getClaims(c echo.Context) *service.Claims {
	claims, ok := c.Get("user_claims").(*service.Claims)
	if !ok {
		return nil
	}
	return claims
}

// GetWorkerConfigs retrieves blast outbox configurations based on user permissions
func GetWorkerConfigs(c echo.Context) error {
	claims := getClaims(c)
	if claims == nil {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	isAdmin := claims.Role == "admin"
	configs, err := model.GetWorkerConfigs(c.Request().Context(), int(claims.UserID), isAdmin)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve worker configs", "INTERNAL_ERROR", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Worker configs retrieved successfully", configs)
}

// GetWorkerConfig retrieves a single worker configuration by ID
func GetWorkerConfig(c echo.Context) error {
	claims := getClaims(c)
	if claims == nil {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid config ID", "BAD_REQUEST", "")
	}

	config, err := model.GetWorkerConfigByID(c.Request().Context(), id)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve worker config", "INTERNAL_ERROR", err.Error())
	}

	if config == nil {
		return ErrorResponse(c, http.StatusNotFound, "Worker config not found", "NOT_FOUND", "")
	}

	// Authorization: user can only view own configs, admin can view all
	isAdmin := claims.Role == "admin"
	if !isAdmin && config.UserID != int(claims.UserID) {
		return ErrorResponse(c, http.StatusForbidden, "Access denied", "FORBIDDEN", "")
	}

	return SuccessResponse(c, http.StatusOK, "Worker config retrieved successfully", config)
}

// CreateWorkerConfig creates a new worker configuration
func CreateWorkerConfig(c echo.Context) error {
	claims := getClaims(c)
	if claims == nil {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	var config model.WorkerConfig
	if err := c.Bind(&config); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	// Validation
	if config.WorkerName == "" || config.Circle == "" || config.Application == "" {
		return ErrorResponse(c, http.StatusBadRequest, "worker_name, circle, and application are required", "VALIDATION_ERROR", "")
	}

	if config.MessageType != "direct" && config.MessageType != "group" {
		return ErrorResponse(c, http.StatusBadRequest, "message_type must be 'direct' or 'group'", "VALIDATION_ERROR", "")
	}

	if config.IntervalSeconds < 1 {
		return ErrorResponse(c, http.StatusBadRequest, "interval_seconds must be at least 1", "VALIDATION_ERROR", "")
	}

	// Set user_id from authenticated user (admin can override)
	isAdmin := claims.Role == "admin"
	if isAdmin && config.UserID != 0 {
		// Admin can create config for other users
	} else {
		config.UserID = int(claims.UserID)
	}

	if err := model.CreateWorkerConfig(c.Request().Context(), &config); err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to create worker config", "INTERNAL_ERROR", err.Error())
	}

	return SuccessResponse(c, http.StatusCreated, "Worker config created successfully", config)
}

// UpdateWorkerConfig updates an existing worker configuration
func UpdateWorkerConfig(c echo.Context) error {
	claims := getClaims(c)
	if claims == nil {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid config ID", "BAD_REQUEST", "")
	}

	// Check if config exists and user has permission
	existingConfig, err := model.GetWorkerConfigByID(c.Request().Context(), id)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve worker config", "INTERNAL_ERROR", err.Error())
	}

	if existingConfig == nil {
		return ErrorResponse(c, http.StatusNotFound, "Worker config not found", "NOT_FOUND", "")
	}

	// Authorization: user can only update own configs, admin can update all
	isAdmin := claims.Role == "admin"
	if !isAdmin && existingConfig.UserID != int(claims.UserID) {
		return ErrorResponse(c, http.StatusForbidden, "Access denied", "FORBIDDEN", "")
	}

	var config model.WorkerConfig
	if err := c.Bind(&config); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	// Validation
	if config.WorkerName == "" || config.Circle == "" || config.Application == "" {
		return ErrorResponse(c, http.StatusBadRequest, "worker_name, circle, and application are required", "VALIDATION_ERROR", "")
	}

	if config.MessageType != "direct" && config.MessageType != "group" {
		return ErrorResponse(c, http.StatusBadRequest, "message_type must be 'direct' or 'group'", "VALIDATION_ERROR", "")
	}

	if config.IntervalSeconds < 1 {
		return ErrorResponse(c, http.StatusBadRequest, "interval_seconds must be at least 1", "VALIDATION_ERROR", "")
	}

	config.ID = id
	config.UserID = existingConfig.UserID // Preserve original user_id

	if err := model.UpdateWorkerConfig(c.Request().Context(), &config); err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to update worker config", "INTERNAL_ERROR", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Worker config updated successfully", config)
}

// DeleteWorkerConfig deletes a worker configuration
func DeleteWorkerConfig(c echo.Context) error {
	claims := getClaims(c)
	if claims == nil {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid config ID", "BAD_REQUEST", "")
	}

	// Check if config exists and user has permission
	existingConfig, err := model.GetWorkerConfigByID(c.Request().Context(), id)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve worker config", "INTERNAL_ERROR", err.Error())
	}

	if existingConfig == nil {
		return ErrorResponse(c, http.StatusNotFound, "Worker config not found", "NOT_FOUND", "")
	}

	// Authorization: user can only delete own configs, admin can delete all
	isAdmin := claims.Role == "admin"
	if !isAdmin && existingConfig.UserID != int(claims.UserID) {
		return ErrorResponse(c, http.StatusForbidden, "Access denied", "FORBIDDEN", "")
	}

	if err := model.DeleteWorkerConfig(c.Request().Context(), id); err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to delete worker config", "INTERNAL_ERROR", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Worker config deleted successfully", nil)
}

// ToggleWorkerConfig toggles the enabled status of a worker configuration
func ToggleWorkerConfig(c echo.Context) error {
	claims := getClaims(c)
	if claims == nil {
		return ErrorResponse(c, http.StatusUnauthorized, "Unauthorized", "UNAUTHORIZED", "")
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid config ID", "BAD_REQUEST", "")
	}

	// Check if config exists and user has permission
	existingConfig, err := model.GetWorkerConfigByID(c.Request().Context(), id)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve worker config", "INTERNAL_ERROR", err.Error())
	}

	if existingConfig == nil {
		return ErrorResponse(c, http.StatusNotFound, "Worker config not found", "NOT_FOUND", "")
	}

	// Authorization: user can only toggle own configs, admin can toggle all
	isAdmin := claims.Role == "admin"
	if !isAdmin && existingConfig.UserID != int(claims.UserID) {
		return ErrorResponse(c, http.StatusForbidden, "Access denied", "FORBIDDEN", "")
	}

	if err := model.ToggleWorkerConfig(c.Request().Context(), id); err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to toggle worker config", "INTERNAL_ERROR", err.Error())
	}

	// Get updated config
	updatedConfig, _ := model.GetWorkerConfigByID(c.Request().Context(), id)

	return SuccessResponse(c, http.StatusOK, "Worker config toggled successfully", updatedConfig)
}

// GetAvailableCircles returns list of available circles from instances
func GetAvailableCircles(c echo.Context) error {
	circles, err := model.GetAvailableCircles(c.Request().Context())
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve available circles", "INTERNAL_ERROR", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Available circles retrieved successfully", circles)
}

// GetAvailableApplications returns list of available applications from outbox
func GetAvailableApplications(c echo.Context) error {
	applications, err := model.GetAvailableApplications(c.Request().Context())
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve available applications", "INTERNAL_ERROR", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Available applications retrieved successfully", applications)
}
