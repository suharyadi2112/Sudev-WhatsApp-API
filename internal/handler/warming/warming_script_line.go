package warming

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gowa-yourself/internal/handler"
	warmingModel "gowa-yourself/internal/model/warming"
	warmingService "gowa-yourself/internal/service/warming"

	"github.com/labstack/echo/v4"
)

// CreateWarmingScriptLine handles POST /warming/scripts/:scriptId/lines
func CreateWarmingScriptLine(c echo.Context) error {
	scriptIDParam := c.Param("scriptId")
	scriptID, err := strconv.ParseInt(scriptIDParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid script ID", "INVALID_SCRIPT_ID", err.Error())
	}

	var req warmingModel.CreateWarmingScriptLineRequest
	if err := c.Bind(&req); err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	line, err := warmingService.CreateWarmingScriptLineService(scriptID, &req)
	if err != nil {
		// Handle validation errors
		if errors.Is(err, warmingService.ErrScriptLineActorRoleInvalid) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "ACTOR_ROLE_INVALID", "")
		}
		if errors.Is(err, warmingService.ErrScriptLineMessageContentRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "MESSAGE_CONTENT_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrScriptLineSequenceOrderInvalid) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "SEQUENCE_ORDER_INVALID", "")
		}
		if strings.Contains(err.Error(), "script not found") {
			return handler.ErrorResponse(c, http.StatusNotFound, "Script not found", "SCRIPT_NOT_FOUND", "")
		}
		if strings.Contains(err.Error(), "already exists") {
			return handler.ErrorResponse(c, http.StatusConflict, err.Error(), "DUPLICATE_SEQUENCE", "")
		}

		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to create script line", "CREATE_FAILED", err.Error())
	}

	resp := warmingModel.ToWarmingScriptLineResponse(*line)
	return handler.SuccessResponse(c, http.StatusOK, "Script line created successfully", resp)
}

// GetAllWarmingScriptLines handles GET /warming/scripts/:scriptId/lines
func GetAllWarmingScriptLines(c echo.Context) error {
	scriptIDParam := c.Param("scriptId")
	scriptID, err := strconv.ParseInt(scriptIDParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid script ID", "INVALID_SCRIPT_ID", err.Error())
	}

	lines, err := warmingService.GetAllWarmingScriptLinesService(scriptID)
	if err != nil {
		if strings.Contains(err.Error(), "script not found") {
			return handler.ErrorResponse(c, http.StatusNotFound, "Script not found", "SCRIPT_NOT_FOUND", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to get script lines", "GET_FAILED", err.Error())
	}

	// Convert to response format
	var responses []warmingModel.WarmingScriptLineResponse
	for _, line := range lines {
		responses = append(responses, warmingModel.ToWarmingScriptLineResponse(line))
	}

	return handler.SuccessResponse(c, http.StatusOK, "Script lines retrieved successfully", map[string]interface{}{
		"total": len(responses),
		"lines": responses,
	})
}

// GetWarmingScriptLineByID handles GET /warming/scripts/:scriptId/lines/:id
func GetWarmingScriptLineByID(c echo.Context) error {
	scriptIDParam := c.Param("scriptId")
	scriptID, err := strconv.ParseInt(scriptIDParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid script ID", "INVALID_SCRIPT_ID", err.Error())
	}

	lineIDParam := c.Param("id")
	lineID, err := strconv.ParseInt(lineIDParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid line ID", "INVALID_LINE_ID", err.Error())
	}

	line, err := warmingService.GetWarmingScriptLineByIDService(scriptID, lineID)
	if err != nil {
		if errors.Is(err, warmingService.ErrScriptLineNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Script line not found", "NOT_FOUND", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to get script line", "GET_FAILED", err.Error())
	}

	resp := warmingModel.ToWarmingScriptLineResponse(*line)
	return handler.SuccessResponse(c, http.StatusOK, "Script line retrieved successfully", resp)
}

// UpdateWarmingScriptLine handles PUT /warming/scripts/:scriptId/lines/:id
func UpdateWarmingScriptLine(c echo.Context) error {
	scriptIDParam := c.Param("scriptId")
	scriptID, err := strconv.ParseInt(scriptIDParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid script ID", "INVALID_SCRIPT_ID", err.Error())
	}

	lineIDParam := c.Param("id")
	lineID, err := strconv.ParseInt(lineIDParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid line ID", "INVALID_LINE_ID", err.Error())
	}

	var req warmingModel.UpdateWarmingScriptLineRequest
	if err := c.Bind(&req); err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	err = warmingService.UpdateWarmingScriptLineService(scriptID, lineID, &req)
	if err != nil {
		// Handle validation errors
		if errors.Is(err, warmingService.ErrScriptLineActorRoleInvalid) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "ACTOR_ROLE_INVALID", "")
		}
		if errors.Is(err, warmingService.ErrScriptLineMessageContentRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "MESSAGE_CONTENT_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrScriptLineSequenceOrderInvalid) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "SEQUENCE_ORDER_INVALID", "")
		}
		if errors.Is(err, warmingService.ErrScriptLineNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Script line not found", "NOT_FOUND", "")
		}
		if strings.Contains(err.Error(), "already exists") {
			return handler.ErrorResponse(c, http.StatusConflict, err.Error(), "DUPLICATE_SEQUENCE", "")
		}

		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to update script line", "UPDATE_FAILED", err.Error())
	}

	return handler.SuccessResponse(c, http.StatusOK, "Script line updated successfully", map[string]interface{}{
		"id": lineID,
	})
}

// DeleteWarmingScriptLine handles DELETE /warming/scripts/:scriptId/lines/:id
func DeleteWarmingScriptLine(c echo.Context) error {
	scriptIDParam := c.Param("scriptId")
	scriptID, err := strconv.ParseInt(scriptIDParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid script ID", "INVALID_SCRIPT_ID", err.Error())
	}

	lineIDParam := c.Param("id")
	lineID, err := strconv.ParseInt(lineIDParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid line ID", "INVALID_LINE_ID", err.Error())
	}

	err = warmingService.DeleteWarmingScriptLineService(scriptID, lineID)
	if err != nil {
		if errors.Is(err, warmingService.ErrScriptLineNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Script line not found", "NOT_FOUND", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete script line", "DELETE_FAILED", err.Error())
	}

	return handler.SuccessResponse(c, http.StatusOK, "Script line deleted successfully", map[string]interface{}{
		"id": lineID,
	})
}

// GenerateWarmingScriptLines handles POST /warming/scripts/:scriptId/lines/generate
func GenerateWarmingScriptLines(c echo.Context) error {
	scriptIDParam := c.Param("scriptId")
	scriptID, err := strconv.ParseInt(scriptIDParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid script ID", "INVALID_SCRIPT_ID", err.Error())
	}

	var req struct {
		LineCount int    `json:"lineCount"`
		Category  string `json:"category"`
	}
	if err := c.Bind(&req); err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	// Default category to casual if not provided
	if req.Category == "" {
		req.Category = "casual"
	}

	// Validate category by checking if templates exist in database
	_, err = warmingService.GetConversationTemplatesFromDB(req.Category)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest,
			fmt.Sprintf("No templates found for category '%s'. Please create templates first or use existing categories.", req.Category),
			"INVALID_CATEGORY", "")
	}

	lines, err := warmingService.GenerateWarmingScriptLinesService(scriptID, req.Category, req.LineCount)
	if err != nil {
		if strings.Contains(err.Error(), "script not found") {
			return handler.ErrorResponse(c, http.StatusNotFound, "Script not found", "SCRIPT_NOT_FOUND", "")
		}
		if strings.Contains(err.Error(), "line_count") {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "INVALID_LINE_COUNT", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to generate script lines", "GENERATE_FAILED", err.Error())
	}

	// Convert to response format
	var responses []warmingModel.WarmingScriptLineResponse
	for _, line := range lines {
		responses = append(responses, warmingModel.ToWarmingScriptLineResponse(line))
	}

	return handler.SuccessResponse(c, http.StatusOK, fmt.Sprintf("%d script lines generated successfully", len(responses)), map[string]interface{}{
		"created":  len(responses),
		"category": req.Category,
		"lines":    responses,
	})
}
