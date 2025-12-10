package warming

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	warmingModel "gowa-yourself/internal/model/warming"
)

var (
	ErrTemplateCategoryRequired = errors.New("category is required")
	ErrTemplateNameRequired     = errors.New("name is required")
	ErrTemplateStructureInvalid = errors.New("structure must be valid JSON array")
	ErrTemplateNotFound         = errors.New("warming template not found")
)

// CreateWarmingTemplateService creates new template with validation
func CreateWarmingTemplateService(req *warmingModel.CreateWarmingTemplateRequest) (*warmingModel.WarmingTemplate, error) {
	// Validate category
	if strings.TrimSpace(req.Category) == "" {
		return nil, ErrTemplateCategoryRequired
	}

	// Validate name
	if strings.TrimSpace(req.Name) == "" {
		return nil, ErrTemplateNameRequired
	}

	// Validate structure is valid JSON array
	var lines []TemplateLine
	if err := json.Unmarshal(req.Structure, &lines); err != nil {
		return nil, ErrTemplateStructureInvalid
	}

	// Validate each line has required fields
	for i, line := range lines {
		if line.ActorRole != "ACTOR_A" && line.ActorRole != "ACTOR_B" {
			return nil, fmt.Errorf("line %d: actorRole must be ACTOR_A or ACTOR_B", i+1)
		}
		if len(line.MessageOptions) == 0 {
			return nil, fmt.Errorf("line %d: messageOptions cannot be empty", i+1)
		}
		// Validate messageType is required
		if strings.TrimSpace(line.MessageType) == "" {
			return nil, fmt.Errorf("line %d: messageType is required (QUESTION, ANSWER, ANSWER_AND_QUESTION, or STATEMENT)", i+1)
		}
		// Validate messageType value
		validTypes := map[string]bool{
			"QUESTION":            true,
			"ANSWER":              true,
			"ANSWER_AND_QUESTION": true,
			"STATEMENT":           true,
			"GREETING":            true,
		}
		if !validTypes[line.MessageType] {
			return nil, fmt.Errorf("line %d: messageType must be QUESTION, ANSWER, ANSWER_AND_QUESTION, STATEMENT, or GREETING", i+1)
		}
	}

	// Create in database
	template, err := warmingModel.CreateWarmingTemplate(req)
	if err != nil {
		// Check for unique constraint violation
		if strings.Contains(err.Error(), "unique_category_name") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("template with category '%s' and name '%s' already exists", req.Category, req.Name)
		}
		return nil, fmt.Errorf("service: %w", err)
	}

	return template, nil
}

// GetAllWarmingTemplatesService retrieves all templates with optional filter
func GetAllWarmingTemplatesService(category string) ([]warmingModel.WarmingTemplate, error) {
	templates, err := warmingModel.GetAllWarmingTemplates(category)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	return templates, nil
}

// GetWarmingTemplateByIDService retrieves single template by ID
func GetWarmingTemplateByIDService(id int64) (*warmingModel.WarmingTemplate, error) {
	if id <= 0 {
		return nil, errors.New("invalid template ID")
	}

	template, err := warmingModel.GetWarmingTemplateByID(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("service: %w", err)
	}

	return template, nil
}

// UpdateWarmingTemplateService updates existing template with validation
func UpdateWarmingTemplateService(id int64, req *warmingModel.UpdateWarmingTemplateRequest) error {
	if id <= 0 {
		return errors.New("invalid template ID")
	}

	// Validate category
	if strings.TrimSpace(req.Category) == "" {
		return ErrTemplateCategoryRequired
	}

	// Validate name
	if strings.TrimSpace(req.Name) == "" {
		return ErrTemplateNameRequired
	}

	// Validate structure is valid JSON array
	var lines []TemplateLine
	if err := json.Unmarshal(req.Structure, &lines); err != nil {
		return ErrTemplateStructureInvalid
	}

	// Validate each line
	for i, line := range lines {
		if line.ActorRole != "ACTOR_A" && line.ActorRole != "ACTOR_B" {
			return fmt.Errorf("line %d: actorRole must be ACTOR_A or ACTOR_B", i+1)
		}
		if len(line.MessageOptions) == 0 {
			return fmt.Errorf("line %d: messageOptions cannot be empty", i+1)
		}
		// Validate messageType is required
		if strings.TrimSpace(line.MessageType) == "" {
			return fmt.Errorf("line %d: messageType is required (QUESTION, ANSWER, ANSWER_AND_QUESTION, or STATEMENT)", i+1)
		}
		// Validate messageType value
		validTypes := map[string]bool{
			"QUESTION":            true,
			"ANSWER":              true,
			"ANSWER_AND_QUESTION": true,
			"STATEMENT":           true,
			"GREETING":            true,
		}
		if !validTypes[line.MessageType] {
			return fmt.Errorf("line %d: messageType must be QUESTION, ANSWER, ANSWER_AND_QUESTION, STATEMENT, or GREETING", i+1)
		}
	}

	// Update in database
	err := warmingModel.UpdateWarmingTemplate(id, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrTemplateNotFound
		}
		// Check for unique constraint violation
		if strings.Contains(err.Error(), "unique_category_name") || strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("template with category '%s' and name '%s' already exists", req.Category, req.Name)
		}
		return fmt.Errorf("service: %w", err)
	}

	return nil
}

// DeleteWarmingTemplateService deletes template by ID
func DeleteWarmingTemplateService(id int64) error {
	if id <= 0 {
		return errors.New("invalid template ID")
	}

	err := warmingModel.DeleteWarmingTemplate(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrTemplateNotFound
		}
		return fmt.Errorf("service: %w", err)
	}

	return nil
}
