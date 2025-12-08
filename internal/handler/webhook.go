// internal/handler/webhook.go
package handler

import (
	"net/http"

	"gowa-yourself/internal/model"

	"github.com/labstack/echo/v4"
)

type WebhookConfigRequest struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

// POST /api/instances/:instanceId/webhook
func SetWebhookConfig(c echo.Context) error {
	instanceID := c.Param("instanceId")

	if instanceID == "" {
		return ErrorResponse(c, http.StatusBadRequest,
			"instanceId is required", "VALIDATION_ERROR", "")
	}

	var req WebhookConfigRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest,
			"Invalid request body", "INVALID_REQUEST", err.Error())
	}

	if req.URL == "" {
		return ErrorResponse(c, http.StatusBadRequest,
			"Field 'url' is required", "VALIDATION_ERROR", "")
	}

	// validasi URL minimal ada http
	if !(len(req.URL) > 7 && (req.URL[:7] == "http://" || req.URL[:8] == "https://")) {
		return ErrorResponse(c, http.StatusBadRequest,
			"webhook url must start with http:// or https://", "INVALID_URL", "")
	}

	// Update ke DB
	if err := model.UpdateInstanceWebhook(instanceID, req.URL, req.Secret); err != nil {
		if err.Error() == "instance_not_found" {
			return ErrorResponse(c, http.StatusNotFound,
				"Instance not found", "INSTANCE_NOT_FOUND", "")
		}

		return ErrorResponse(c, http.StatusInternalServerError,
			"Failed to update webhook config", "WEBHOOK_UPDATE_FAILED", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Webhook config updated", map[string]interface{}{
		"instanceId": instanceID,
		"webhookUrl": req.URL,
		"hasSecret":  req.Secret != "",
	})
}
