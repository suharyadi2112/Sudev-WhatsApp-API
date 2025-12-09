package warming

import (
	"errors"
	"net/http"
	"strconv"

	warmingModel "gowa-yourself/internal/model/warming"
	warmingService "gowa-yourself/internal/service/warming"

	"github.com/labstack/echo/v4"
)

// SuccessResponse helper (copied from handler package)
func SuccessResponse(c echo.Context, code int, message string, data interface{}) error {
	return c.JSON(code, map[string]interface{}{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// ErrorResponse helper (copied from handler package)
func ErrorResponse(c echo.Context, code int, message, errorCode, detail string) error {
	resp := map[string]interface{}{
		"success": false,
		"message": message,
		"error":   errorCode,
	}
	if detail != "" {
		resp["detail"] = detail
	}
	return c.JSON(code, resp)
}

// CreateWarmingScript handles POST /warming/scripts
func CreateWarmingScript(c echo.Context) error {
	var req warmingModel.CreateWarmingScriptRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	script, err := warmingService.CreateWarmingScriptService(&req)
	if err != nil {
		// Handle validation errors
		if errors.Is(err, warmingService.ErrWarmingScriptTitleRequired) {
			return ErrorResponse(c, http.StatusBadRequest, err.Error(), "TITLE_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrWarmingScriptTitleTooLong) {
			return ErrorResponse(c, http.StatusBadRequest, err.Error(), "TITLE_TOO_LONG", "")
		}
		if errors.Is(err, warmingService.ErrWarmingScriptCategoryTooLong) {
			return ErrorResponse(c, http.StatusBadRequest, err.Error(), "CATEGORY_TOO_LONG", "")
		}

		return ErrorResponse(c, http.StatusInternalServerError, "Failed to create warming script", "CREATE_FAILED", err.Error())
	}

	resp := warmingModel.ToWarmingScriptResponse(*script)
	return SuccessResponse(c, http.StatusOK, "Warming script created successfully", resp)
}

// GetAllWarmingScripts handles GET /warming/scripts
func GetAllWarmingScripts(c echo.Context) error {
	q := c.QueryParam("q")
	category := c.QueryParam("category")

	scripts, err := warmingService.GetAllWarmingScriptsService(q, category)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to get warming scripts", "GET_FAILED", err.Error())
	}

	// Convert to response format
	var responses []warmingModel.WarmingScriptResponse
	for _, script := range scripts {
		responses = append(responses, warmingModel.ToWarmingScriptResponse(script))
	}

	return SuccessResponse(c, http.StatusOK, "Warming scripts retrieved successfully", map[string]interface{}{
		"total":   len(responses),
		"scripts": responses,
	})
}

// GetWarmingScriptByID handles GET /warming/scripts/:id
func GetWarmingScriptByID(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid script ID", "INVALID_ID", err.Error())
	}

	script, err := warmingService.GetWarmingScriptByIDService(id)
	if err != nil {
		if errors.Is(err, warmingService.ErrWarmingScriptNotFound) {
			return ErrorResponse(c, http.StatusNotFound, "Warming script not found", "NOT_FOUND", "")
		}
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to get warming script", "GET_FAILED", err.Error())
	}

	resp := warmingModel.ToWarmingScriptResponse(*script)
	return SuccessResponse(c, http.StatusOK, "Warming script retrieved successfully", resp)
}

// UpdateWarmingScript handles PUT /warming/scripts/:id
func UpdateWarmingScript(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid script ID", "INVALID_ID", err.Error())
	}

	var req warmingModel.UpdateWarmingScriptRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	err = warmingService.UpdateWarmingScriptService(id, &req)
	if err != nil {
		// Handle validation errors
		if errors.Is(err, warmingService.ErrWarmingScriptTitleRequired) {
			return ErrorResponse(c, http.StatusBadRequest, err.Error(), "TITLE_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrWarmingScriptTitleTooLong) {
			return ErrorResponse(c, http.StatusBadRequest, err.Error(), "TITLE_TOO_LONG", "")
		}
		if errors.Is(err, warmingService.ErrWarmingScriptCategoryTooLong) {
			return ErrorResponse(c, http.StatusBadRequest, err.Error(), "CATEGORY_TOO_LONG", "")
		}
		if errors.Is(err, warmingService.ErrWarmingScriptNotFound) {
			return ErrorResponse(c, http.StatusNotFound, "Warming script not found", "NOT_FOUND", "")
		}

		return ErrorResponse(c, http.StatusInternalServerError, "Failed to update warming script", "UPDATE_FAILED", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Warming script updated successfully", map[string]interface{}{
		"id": id,
	})
}

// DeleteWarmingScript handles DELETE /warming/scripts/:id
func DeleteWarmingScript(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return ErrorResponse(c, http.StatusBadRequest, "Invalid script ID", "INVALID_ID", err.Error())
	}

	err = warmingService.DeleteWarmingScriptService(id)
	if err != nil {
		if errors.Is(err, warmingService.ErrWarmingScriptNotFound) {
			return ErrorResponse(c, http.StatusNotFound, "Warming script not found", "NOT_FOUND", "")
		}
		return ErrorResponse(c, http.StatusInternalServerError, "Failed to delete warming script", "DELETE_FAILED", err.Error())
	}

	return SuccessResponse(c, http.StatusOK, "Warming script deleted successfully", map[string]interface{}{
		"id": id,
	})
}
