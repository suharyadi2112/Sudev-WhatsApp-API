package warming

import (
	"errors"
	"fmt"
	"strings"

	warmingModel "gowa-yourself/internal/model/warming"
)

var (
	ErrWarmingScriptTitleRequired   = errors.New("title is required")
	ErrWarmingScriptTitleTooLong    = errors.New("title must be less than 255 characters")
	ErrWarmingScriptCategoryTooLong = errors.New("category must be less than 100 characters")
	ErrWarmingScriptNotFound        = errors.New("warming script not found")
)

// CreateWarmingScriptService creates new warming script with validation
func CreateWarmingScriptService(req *warmingModel.CreateWarmingScriptRequest) (*warmingModel.WarmingScript, error) {
	// Validate title
	if strings.TrimSpace(req.Title) == "" {
		return nil, ErrWarmingScriptTitleRequired
	}

	if len(req.Title) > 255 {
		return nil, ErrWarmingScriptTitleTooLong
	}

	// Validate category (optional)
	if len(req.Category) > 100 {
		return nil, ErrWarmingScriptCategoryTooLong
	}

	// Create in database
	script, err := warmingModel.CreateWarmingScript(req)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	return script, nil
}

// GetAllWarmingScriptsService retrieves all scripts with filters
func GetAllWarmingScriptsService(q, category string) ([]warmingModel.WarmingScript, error) {
	scripts, err := warmingModel.GetAllWarmingScripts(q, category)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	return scripts, nil
}

// GetWarmingScriptByIDService retrieves single script by ID
func GetWarmingScriptByIDService(id int) (*warmingModel.WarmingScript, error) {
	if id <= 0 {
		return nil, errors.New("invalid script ID")
	}

	script, err := warmingModel.GetWarmingScriptByID(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrWarmingScriptNotFound
		}
		return nil, fmt.Errorf("service: %w", err)
	}

	return script, nil
}

// UpdateWarmingScriptService updates existing script with validation
func UpdateWarmingScriptService(id int, req *warmingModel.UpdateWarmingScriptRequest) error {
	if id <= 0 {
		return errors.New("invalid script ID")
	}

	// Validate title
	if strings.TrimSpace(req.Title) == "" {
		return ErrWarmingScriptTitleRequired
	}

	if len(req.Title) > 255 {
		return ErrWarmingScriptTitleTooLong
	}

	// Validate category (optional)
	if len(req.Category) > 100 {
		return ErrWarmingScriptCategoryTooLong
	}

	// Update in database
	err := warmingModel.UpdateWarmingScript(id, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrWarmingScriptNotFound
		}
		return fmt.Errorf("service: %w", err)
	}

	return nil
}

// DeleteWarmingScriptService deletes script by ID
func DeleteWarmingScriptService(id int) error {
	if id <= 0 {
		return errors.New("invalid script ID")
	}

	err := warmingModel.DeleteWarmingScript(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrWarmingScriptNotFound
		}
		return fmt.Errorf("service: %w", err)
	}

	return nil
}
