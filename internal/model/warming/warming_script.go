package warming

import (
	"database/sql"
	"fmt"
	"gowa-yourself/database"
	"strings"
	"time"
)

// WarmingScript represents warming_scripts table
type WarmingScript struct {
	ID          int64
	Title       string
	Description sql.NullString
	Category    sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WarmingScriptResponse for JSON response
type WarmingScriptResponse struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// CreateWarmingScriptRequest for POST request
type CreateWarmingScriptRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// UpdateWarmingScriptRequest for PUT request
type UpdateWarmingScriptRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// CreateWarmingScript inserts new warming script
func CreateWarmingScript(req *CreateWarmingScriptRequest) (*WarmingScript, error) {
	query := `
		INSERT INTO warming_scripts (title, description, category, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, title, description, category, created_at, updated_at
	`

	script := &WarmingScript{}
	err := database.AppDB.QueryRow(
		query,
		req.Title,
		nullString(req.Description),
		nullString(req.Category),
	).Scan(
		&script.ID,
		&script.Title,
		&script.Description,
		&script.Category,
		&script.CreatedAt,
		&script.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create warming script: %w", err)
	}

	return script, nil
}

// GetAllWarmingScripts retrieves all scripts with optional filters
func GetAllWarmingScripts(q, category string) ([]WarmingScript, error) {
	query := `
		SELECT id, title, description, category, created_at, updated_at
		FROM warming_scripts
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	// Filter by search query (title or description)
	if strings.TrimSpace(q) != "" {
		query += fmt.Sprintf(" AND (title ILIKE $%d OR description ILIKE $%d)", argCount, argCount)
		args = append(args, "%"+q+"%")
		argCount++
	}

	// Filter by category
	if strings.TrimSpace(category) != "" {
		query += fmt.Sprintf(" AND category = $%d", argCount)
		args = append(args, category)
		argCount++
	}

	query += " ORDER BY created_at DESC"

	rows, err := database.AppDB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query warming scripts: %w", err)
	}
	defer rows.Close()

	var scripts []WarmingScript
	for rows.Next() {
		var script WarmingScript
		err := rows.Scan(
			&script.ID,
			&script.Title,
			&script.Description,
			&script.Category,
			&script.CreatedAt,
			&script.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan warming script: %w", err)
		}
		scripts = append(scripts, script)
	}

	return scripts, nil
}

// GetWarmingScriptByID retrieves single script by ID
func GetWarmingScriptByID(id int) (*WarmingScript, error) {
	query := `
		SELECT id, title, description, category, created_at, updated_at
		FROM warming_scripts
		WHERE id = $1
	`

	script := &WarmingScript{}
	err := database.AppDB.QueryRow(query, id).Scan(
		&script.ID,
		&script.Title,
		&script.Description,
		&script.Category,
		&script.CreatedAt,
		&script.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("warming script not found")
		}
		return nil, fmt.Errorf("failed to get warming script: %w", err)
	}

	return script, nil
}

// UpdateWarmingScript updates existing script
func UpdateWarmingScript(id int, req *UpdateWarmingScriptRequest) error {
	query := `
		UPDATE warming_scripts
		SET title = $1, description = $2, category = $3, updated_at = NOW()
		WHERE id = $4
	`

	result, err := database.AppDB.Exec(
		query,
		req.Title,
		nullString(req.Description),
		nullString(req.Category),
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update warming script: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("warming script not found")
	}

	return nil
}

// DeleteWarmingScript deletes script by ID (CASCADE to warming_script_lines)
func DeleteWarmingScript(id int) error {
	query := `DELETE FROM warming_scripts WHERE id = $1`

	result, err := database.AppDB.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete warming script: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("warming script not found")
	}

	return nil
}

// ToWarmingScriptResponse converts WarmingScript to response format
func ToWarmingScriptResponse(script WarmingScript) WarmingScriptResponse {
	resp := WarmingScriptResponse{
		ID:        script.ID,
		Title:     script.Title,
		CreatedAt: script.CreatedAt,
		UpdatedAt: script.UpdatedAt,
	}

	if script.Description.Valid {
		resp.Description = script.Description.String
	}

	if script.Category.Valid {
		resp.Category = script.Category.String
	}

	return resp
}

// Helper function to convert string to sql.NullString
func nullString(s string) sql.NullString {
	if strings.TrimSpace(s) == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
