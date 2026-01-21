package warming

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"gowa-yourself/database"
	"time"
)

// WarmingTemplate represents warming_templates table
type WarmingTemplate struct {
	ID        int64
	Category  string
	Name      string
	Structure json.RawMessage // JSONB stored as raw JSON
	CreatedBy sql.NullInt64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WarmingTemplateResponse for JSON response
type WarmingTemplateResponse struct {
	ID        int64           `json:"id"`
	Category  string          `json:"category"`
	Name      string          `json:"name"`
	Structure json.RawMessage `json:"structure"`
	CreatedBy *int64          `json:"createdBy,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// CreateWarmingTemplateRequest for POST request
type CreateWarmingTemplateRequest struct {
	Category  string          `json:"category"`
	Name      string          `json:"name"`
	Structure json.RawMessage `json:"structure"`
}

// UpdateWarmingTemplateRequest for PUT request
type UpdateWarmingTemplateRequest struct {
	Category  string          `json:"category"`
	Name      string          `json:"name"`
	Structure json.RawMessage `json:"structure"`
}

// CreateWarmingTemplate inserts new template
func CreateWarmingTemplate(req *CreateWarmingTemplateRequest, userID int64) (*WarmingTemplate, error) {
	query := `
		INSERT INTO warming_templates (category, name, structure, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, category, name, structure, created_by, created_at, updated_at
	`

	template := &WarmingTemplate{}
	err := database.AppDB.QueryRow(
		query,
		req.Category,
		req.Name,
		req.Structure,
		userID,
	).Scan(
		&template.ID,
		&template.Category,
		&template.Name,
		&template.Structure,
		&template.CreatedBy,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create warming template: %w", err)
	}

	return template, nil
}

// GetAllWarmingTemplates retrieves all templates with optional category filter
func GetAllWarmingTemplates(category string, userID int64, isAdmin bool) ([]WarmingTemplate, error) {
	var query string
	var args []interface{}

	if category != "" {
		query = `
			SELECT id, category, name, structure, created_by, created_at, updated_at
			FROM warming_templates
			WHERE category = $1
		`
		args = append(args, category)

		// RBAC: Filter by ownership for non-admin users
		if !isAdmin {
			query += " AND (created_by = $2 OR created_by IS NULL)"
			args = append(args, userID)
		}

		query += " ORDER BY category, name"
	} else {
		query = `
			SELECT id, category, name, structure, created_by, created_at, updated_at
			FROM warming_templates
		`

		// RBAC: Filter by ownership for non-admin users
		if !isAdmin {
			query += " WHERE (created_by = $1 OR created_by IS NULL)"
			args = append(args, userID)
		}

		query += " ORDER BY category, name"
	}

	rows, err := database.AppDB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query warming templates: %w", err)
	}
	defer rows.Close()

	var templates []WarmingTemplate
	for rows.Next() {
		var template WarmingTemplate
		err := rows.Scan(
			&template.ID,
			&template.Category,
			&template.Name,
			&template.Structure,
			&template.CreatedBy,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan warming template: %w", err)
		}
		templates = append(templates, template)
	}

	return templates, nil
}

// GetWarmingTemplateByID retrieves single template by ID
func GetWarmingTemplateByID(id int64) (*WarmingTemplate, error) {
	query := `
		SELECT id, category, name, structure, created_by, created_at, updated_at
		FROM warming_templates
		WHERE id = $1
	`

	template := &WarmingTemplate{}
	err := database.AppDB.QueryRow(query, id).Scan(
		&template.ID,
		&template.Category,
		&template.Name,
		&template.Structure,
		&template.CreatedBy,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("warming template not found")
		}
		return nil, fmt.Errorf("failed to get warming template: %w", err)
	}

	return template, nil
}

// UpdateWarmingTemplate updates existing template
func UpdateWarmingTemplate(id int64, req *UpdateWarmingTemplateRequest) error {
	query := `
		UPDATE warming_templates
		SET category = $1, name = $2, structure = $3, updated_at = NOW()
		WHERE id = $4
	`

	result, err := database.AppDB.Exec(
		query,
		req.Category,
		req.Name,
		req.Structure,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update warming template: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("warming template not found")
	}

	return nil
}

// DeleteWarmingTemplate deletes template by ID
func DeleteWarmingTemplate(id int64) error {
	query := `DELETE FROM warming_templates WHERE id = $1`

	result, err := database.AppDB.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete warming template: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("warming template not found")
	}

	return nil
}

// CheckTemplateOwnership validates if user owns the template (Admin bypass)
func CheckTemplateOwnership(templateID int64, userID int64) (bool, error) {
	var ownerID sql.NullInt64
	err := database.AppDB.QueryRow("SELECT created_by FROM warming_templates WHERE id = $1", templateID).Scan(&ownerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("template not found")
		}
		return false, err
	}

	// If created_by is NULL (public template), deny ownership
	if !ownerID.Valid {
		return false, nil
	}

	return ownerID.Int64 == userID, nil
}

// ToWarmingTemplateResponse converts WarmingTemplate to response format
func ToWarmingTemplateResponse(template WarmingTemplate) WarmingTemplateResponse {
	resp := WarmingTemplateResponse{
		ID:        template.ID,
		Category:  template.Category,
		Name:      template.Name,
		Structure: template.Structure,
		CreatedAt: template.CreatedAt,
		UpdatedAt: template.UpdatedAt,
	}

	if template.CreatedBy.Valid {
		createdBy := template.CreatedBy.Int64
		resp.CreatedBy = &createdBy
	}

	return resp
}
