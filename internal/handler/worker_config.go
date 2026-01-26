package handler

import (
	"database/sql"
	"gowa-yourself/internal/model"
	"gowa-yourself/internal/service"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type WorkerConfigRequest struct {
	WorkerName         string `json:"worker_name"`
	Circle             string `json:"circle"`
	Application        string `json:"application"`
	MessageType        string `json:"message_type"`
	IntervalMinSeconds int    `json:"interval_min_seconds"`
	IntervalSeconds    int    `json:"interval_seconds"` // Alias for backward compatibility
	IntervalMaxSeconds int    `json:"interval_max_seconds"`
	Enabled            *bool  `json:"enabled"`
	WebhookURL         string `json:"webhook_url"`
	WebhookSecret      string `json:"webhook_secret"`
	AllowMedia         *bool  `json:"allow_media"`
	UserID             int    `json:"user_id"` // Used for admin override
}

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

	var req WorkerConfigRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	// Validation
	if req.WorkerName == "" || req.Circle == "" || req.Application == "" {
		return ErrorResponse(c, http.StatusBadRequest, "worker_name, circle, and application are required", "VALIDATION_ERROR", "")
	}

	if req.MessageType != "direct" && req.MessageType != "group" {
		req.MessageType = "direct" // Default
	}

	if req.IntervalMinSeconds < 1 && req.IntervalSeconds > 0 {
		req.IntervalMinSeconds = req.IntervalSeconds
	}

	if req.IntervalMinSeconds < 1 {
		req.IntervalMinSeconds = 10 // Default
	}

	// Map to model
	config := model.WorkerConfig{
		WorkerName:         req.WorkerName,
		Circle:             req.Circle,
		Application:        req.Application,
		MessageType:        req.MessageType,
		IntervalSeconds:    req.IntervalMinSeconds,
		IntervalMaxSeconds: req.IntervalMaxSeconds,
		Enabled:            true,  // Default
		AllowMedia:         false, // Default to false
		WebhookURL:         sql.NullString{String: req.WebhookURL, Valid: req.WebhookURL != ""},
		WebhookSecret:      sql.NullString{String: req.WebhookSecret, Valid: req.WebhookSecret != ""},
	}

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}

	if req.AllowMedia != nil {
		config.AllowMedia = *req.AllowMedia
	}

	// Set user_id from authenticated user (admin can override)
	isAdmin := claims.Role == "admin"
	if isAdmin && req.UserID != 0 {
		config.UserID = req.UserID
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

	var req WorkerConfigRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	// Validation
	if req.WorkerName == "" || req.Circle == "" || req.Application == "" {
		return ErrorResponse(c, http.StatusBadRequest, "worker_name, circle, and application are required", "VALIDATION_ERROR", "")
	}

	if req.MessageType != "direct" && req.MessageType != "group" {
		req.MessageType = "direct"
	}

	if req.IntervalMinSeconds < 1 && req.IntervalSeconds > 0 {
		req.IntervalMinSeconds = req.IntervalSeconds
	}

	if req.IntervalMinSeconds < 1 {
		req.IntervalMinSeconds = 10
	}

	// Map to model
	config := model.WorkerConfig{
		ID:                 id,
		UserID:             existingConfig.UserID, // Preserve original user_id
		WorkerName:         req.WorkerName,
		Circle:             req.Circle,
		Application:        req.Application,
		MessageType:        req.MessageType,
		IntervalSeconds:    req.IntervalMinSeconds,
		IntervalMaxSeconds: req.IntervalMaxSeconds,
		Enabled:            existingConfig.Enabled, // Default to existing
		AllowMedia:         existingConfig.AllowMedia,
		WebhookURL:         sql.NullString{String: req.WebhookURL, Valid: req.WebhookURL != ""},
		WebhookSecret:      sql.NullString{String: req.WebhookSecret, Valid: req.WebhookSecret != ""},
	}

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}

	if req.AllowMedia != nil {
		config.AllowMedia = *req.AllowMedia
	}

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
