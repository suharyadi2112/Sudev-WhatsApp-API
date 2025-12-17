package warming

import (
	"errors"
	"fmt"
	"strings"

	warmingModel "gowa-yourself/internal/model/warming"
)

var (
	ErrScriptLineActorRoleInvalid       = errors.New("actor_role must be ACTOR_A or ACTOR_B")
	ErrScriptLineMessageContentRequired = errors.New("message_content is required")
	ErrScriptLineSequenceOrderInvalid   = errors.New("sequence_order must be greater than 0")
	ErrScriptLineTypingDurationInvalid  = errors.New("typing_duration_sec must be greater than 0")
	ErrScriptLineNotFound               = errors.New("warming script line not found")
)

// CreateWarmingScriptLineService creates new script line with validation
func CreateWarmingScriptLineService(scriptID int64, req *warmingModel.CreateWarmingScriptLineRequest) (*warmingModel.WarmingScriptLine, error) {
	// Validate sequence order
	if req.SequenceOrder <= 0 {
		return nil, ErrScriptLineSequenceOrderInvalid
	}

	// Validate actor role
	if req.ActorRole != "ACTOR_A" && req.ActorRole != "ACTOR_B" {
		return nil, ErrScriptLineActorRoleInvalid
	}

	// Validate message content
	if strings.TrimSpace(req.MessageContent) == "" {
		return nil, ErrScriptLineMessageContentRequired
	}

	// Validate typing duration (default to 3 if not provided or invalid)
	if req.TypingDurationSec <= 0 {
		req.TypingDurationSec = 3 // Default value
	}

	// Check if script exists
	_, err := warmingModel.GetWarmingScriptByID(int(scriptID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errors.New("script not found")
		}
		return nil, fmt.Errorf("failed to verify script: %w", err)
	}

	// Create in database
	line, err := warmingModel.CreateWarmingScriptLine(scriptID, req)
	if err != nil {
		// Check for unique constraint violation (duplicate sequence_order)
		if strings.Contains(err.Error(), "unique_script_sequence") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("sequence_order %d already exists for this script", req.SequenceOrder)
		}
		return nil, fmt.Errorf("service: %w", err)
	}

	return line, nil
}

// GetAllWarmingScriptLinesService retrieves all lines for a script
func GetAllWarmingScriptLinesService(scriptID int64) ([]warmingModel.WarmingScriptLine, error) {
	// Check if script exists
	_, err := warmingModel.GetWarmingScriptByID(int(scriptID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errors.New("script not found")
		}
		return nil, fmt.Errorf("failed to verify script: %w", err)
	}

	lines, err := warmingModel.GetAllWarmingScriptLines(scriptID)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	return lines, nil
}

// GetWarmingScriptLineByIDService retrieves single line by ID
func GetWarmingScriptLineByIDService(scriptID int64, lineID int64) (*warmingModel.WarmingScriptLine, error) {
	if lineID <= 0 {
		return nil, errors.New("invalid line ID")
	}

	line, err := warmingModel.GetWarmingScriptLineByID(scriptID, lineID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrScriptLineNotFound
		}
		return nil, fmt.Errorf("service: %w", err)
	}

	return line, nil
}

// UpdateWarmingScriptLineService updates existing line with validation
func UpdateWarmingScriptLineService(scriptID int64, lineID int64, req *warmingModel.UpdateWarmingScriptLineRequest) error {
	if lineID <= 0 {
		return errors.New("invalid line ID")
	}

	// Validate sequence order
	if req.SequenceOrder <= 0 {
		return ErrScriptLineSequenceOrderInvalid
	}

	// Validate actor role
	if req.ActorRole != "ACTOR_A" && req.ActorRole != "ACTOR_B" {
		return ErrScriptLineActorRoleInvalid
	}

	// Validate message content
	if strings.TrimSpace(req.MessageContent) == "" {
		return ErrScriptLineMessageContentRequired
	}

	// Validate typing duration
	if req.TypingDurationSec <= 0 {
		req.TypingDurationSec = 3 // Default value
	}

	// Update in database
	err := warmingModel.UpdateWarmingScriptLine(scriptID, lineID, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrScriptLineNotFound
		}
		// Check for unique constraint violation
		if strings.Contains(err.Error(), "unique_script_sequence") || strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("sequence_order %d already exists for this script", req.SequenceOrder)
		}
		return fmt.Errorf("service: %w", err)
	}

	return nil
}

// DeleteWarmingScriptLineService deletes line by ID
func DeleteWarmingScriptLineService(scriptID int64, lineID int64) error {
	if lineID <= 0 {
		return errors.New("invalid line ID")
	}

	err := warmingModel.DeleteWarmingScriptLine(scriptID, lineID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrScriptLineNotFound
		}
		return fmt.Errorf("service: %w", err)
	}

	return nil
}

// GenerateWarmingScriptLinesService generates conversation lines based on template
func GenerateWarmingScriptLinesService(scriptID int64, category string, lineCount int) ([]warmingModel.WarmingScriptLine, error) {
	// Validate line count
	if lineCount <= 0 || lineCount > 100 {
		return nil, errors.New("line_count must be between 1 and 100")
	}

	// Check if script exists
	_, err := warmingModel.GetWarmingScriptByID(int(scriptID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errors.New("script not found")
		}
		return nil, fmt.Errorf("failed to verify script: %w", err)
	}

	// Get existing lines to determine starting sequence
	existingLines, err := warmingModel.GetAllWarmingScriptLines(scriptID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing lines: %w", err)
	}

	// Find max sequence (not count, to support gaps)
	startSequence := 1
	for _, line := range existingLines {
		if line.SequenceOrder >= startSequence {
			startSequence = line.SequenceOrder + 1
		}
	}

	// Generate conversation lines from template
	templateLines := GenerateConversationLines(category, lineCount)

	// Create lines in database
	var createdLines []warmingModel.WarmingScriptLine
	for i, templateLine := range templateLines {
		req := &warmingModel.CreateWarmingScriptLineRequest{
			SequenceOrder:     startSequence + i,
			ActorRole:         templateLine.ActorRole,
			MessageContent:    templateLine.MessageOptions[0], // Already selected in generator
			TypingDurationSec: RandomTypingDuration(),
		}

		line, err := warmingModel.CreateWarmingScriptLine(scriptID, req)
		if err != nil {
			return nil, fmt.Errorf("failed to create line %d: %w", i+1, err)
		}

		createdLines = append(createdLines, *line)
	}

	return createdLines, nil
}
