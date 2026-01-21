package warming

import (
	"errors"
	"fmt"
	"strings"

	warmingModel "gowa-yourself/internal/model/warming"
)

var (
	ErrLogNotFound = errors.New("warming log not found")
)

// GetAllWarmingLogsService retrieves logs with filters
func GetAllWarmingLogsService(roomID, status string, limit int, userID int64, isAdmin bool) ([]warmingModel.WarmingLog, error) {
	// Validate status if provided
	if status != "" {
		validStatuses := map[string]bool{
			"SUCCESS": true,
			"FAILED":  true,
		}
		if !validStatuses[status] {
			return nil, fmt.Errorf("invalid status: must be SUCCESS or FAILED")
		}
	}

	// Validate limit
	if limit < 0 {
		limit = 0
	}
	if limit > 1000 {
		limit = 1000 // Max 1000 records
	}
	if limit == 0 {
		limit = 100 // Default 100
	}

	logs, err := warmingModel.GetAllWarmingLogs(roomID, status, limit, userID, isAdmin)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	return logs, nil
}

// GetWarmingLogByIDService retrieves single log by ID with RBAC
func GetWarmingLogByIDService(id int64, userID int64, isAdmin bool) (*warmingModel.WarmingLog, error) {
	if id <= 0 {
		return nil, errors.New("invalid log ID")
	}

	log, err := warmingModel.GetWarmingLogByID(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrLogNotFound
		}
		return nil, fmt.Errorf("service: %w", err)
	}

	// RBAC: Check ownership for non-admin users
	if !isAdmin {
		if log.CreatedBy.Valid && log.CreatedBy.Int64 != userID {
			return nil, errors.New("forbidden: you do not own this log")
		}
	}

	return log, nil
}
