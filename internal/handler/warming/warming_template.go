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

// CreateWarmingTemplate handles POST /warming/templates
func CreateWarmingTemplate(c echo.Context) error {
	var req warmingModel.CreateWarmingTemplateRequest
	if err := c.Bind(&req); err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	template, err := warmingService.CreateWarmingTemplateService(&req)
	if err != nil {
		if errors.Is(err, warmingService.ErrTemplateCategoryRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "CATEGORY_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrTemplateNameRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "NAME_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrTemplateStructureInvalid) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "STRUCTURE_INVALID", "")
		}
		if strings.Contains(err.Error(), "actorRole") || strings.Contains(err.Error(), "messageOptions") {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "VALIDATION_ERROR", "")
		}
		if strings.Contains(err.Error(), "already exists") {
			return handler.ErrorResponse(c, http.StatusConflict, err.Error(), "DUPLICATE_TEMPLATE", "")
		}

		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to create template", "CREATE_FAILED", err.Error())
	}

	resp := warmingModel.ToWarmingTemplateResponse(*template)
	return handler.SuccessResponse(c, http.StatusOK, "Template created successfully", resp)
}

// GetAllWarmingTemplates handles GET /warming/templates
func GetAllWarmingTemplates(c echo.Context) error {
	category := c.QueryParam("category")

	templates, err := warmingService.GetAllWarmingTemplatesService(category)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to get templates", "GET_FAILED", err.Error())
	}

	var responses []warmingModel.WarmingTemplateResponse
	for _, template := range templates {
		responses = append(responses, warmingModel.ToWarmingTemplateResponse(template))
	}

	return handler.SuccessResponse(c, http.StatusOK, "Templates retrieved successfully", map[string]interface{}{
		"total":     len(responses),
		"templates": responses,
	})
}

// GetWarmingTemplateByID handles GET /warming/templates/:id
func GetWarmingTemplateByID(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid template ID", "INVALID_ID", err.Error())
	}

	template, err := warmingService.GetWarmingTemplateByIDService(id)
	if err != nil {
		if errors.Is(err, warmingService.ErrTemplateNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Template not found", "NOT_FOUND", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to get template", "GET_FAILED", err.Error())
	}

	resp := warmingModel.ToWarmingTemplateResponse(*template)
	return handler.SuccessResponse(c, http.StatusOK, "Template retrieved successfully", resp)
}

// UpdateWarmingTemplate handles PUT /warming/templates/:id
func UpdateWarmingTemplate(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid template ID", "INVALID_ID", err.Error())
	}

	var req warmingModel.UpdateWarmingTemplateRequest
	if err := c.Bind(&req); err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid request body", "BAD_REQUEST", err.Error())
	}

	err = warmingService.UpdateWarmingTemplateService(id, &req)
	if err != nil {
		if errors.Is(err, warmingService.ErrTemplateCategoryRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "CATEGORY_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrTemplateNameRequired) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "NAME_REQUIRED", "")
		}
		if errors.Is(err, warmingService.ErrTemplateStructureInvalid) {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "STRUCTURE_INVALID", "")
		}
		if errors.Is(err, warmingService.ErrTemplateNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Template not found", "NOT_FOUND", "")
		}
		if strings.Contains(err.Error(), "actorRole") || strings.Contains(err.Error(), "messageOptions") {
			return handler.ErrorResponse(c, http.StatusBadRequest, err.Error(), "VALIDATION_ERROR", "")
		}
		if strings.Contains(err.Error(), "already exists") {
			return handler.ErrorResponse(c, http.StatusConflict, err.Error(), "DUPLICATE_TEMPLATE", "")
		}

		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to update template", "UPDATE_FAILED", err.Error())
	}

	return handler.SuccessResponse(c, http.StatusOK, "Template updated successfully", map[string]interface{}{
		"id": id,
	})
}

// DeleteWarmingTemplate handles DELETE /warming/templates/:id
func DeleteWarmingTemplate(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return handler.ErrorResponse(c, http.StatusBadRequest, "Invalid template ID", "INVALID_ID", err.Error())
	}

	err = warmingService.DeleteWarmingTemplateService(id)
	if err != nil {
		if errors.Is(err, warmingService.ErrTemplateNotFound) {
			return handler.ErrorResponse(c, http.StatusNotFound, "Template not found", "NOT_FOUND", "")
		}
		return handler.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete template", "DELETE_FAILED", err.Error())
	}

	return handler.SuccessResponse(c, http.StatusOK, "Template deleted successfully", map[string]interface{}{
		"id": id,
	})
}
