// internal/handler/system_file_upload.go
package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"gowa-yourself/internal/helper"
	"gowa-yourself/internal/model"
	"gowa-yourself/internal/service"

	"github.com/labstack/echo/v4"
)

// SystemDir is the directory for system-wide images
const SystemDir = "./uploads/system"

// UpdateSystemIdentityFull handles both text settings and file uploads in one request (Admin Only)
// POST /api/system/identity
func UpdateSystemIdentityFull(c echo.Context) error {
	userClaims, ok := c.Get("user_claims").(*service.Claims)
	if !ok || userClaims.Role != "admin" {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"success": false,
			"message": "Admin access required",
		})
	}

	// 1. Get current identity from DB
	identity, err := model.GetSystemIdentity()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to fetch current settings",
		})
	}

	// 2. Map text fields from form-data (Only update if provided/not empty)
	if val := c.FormValue("company_name"); val != "" {
		identity.CompanyName = val
	}
	if val := c.FormValue("company_short_name"); val != "" {
		identity.CompanyShortName = val
	}
	if val := c.FormValue("company_description"); val != "" {
		identity.CompanyDescription = val
	}
	if val := c.FormValue("company_address"); val != "" {
		identity.CompanyAddress = val
	}
	if val := c.FormValue("company_phone"); val != "" {
		identity.CompanyPhone = val
	}
	if val := c.FormValue("company_email"); val != "" {
		identity.CompanyEmail = val
	}
	if val := c.FormValue("company_website"); val != "" {
		identity.CompanyWebsite = val
	}

	// 3. Handle optional file uploads (logo, ico, second_logo)
	fileKeys := []string{"logo", "ico", "second_logo"}
	for _, key := range fileKeys {
		file, err := c.FormFile(key)
		if err != nil {
			if err == http.ErrMissingFile {
				continue // Skip if this specific file wasn't uploaded
			}
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Error reading file %s", key),
			})
		}

		// Validate and Process File
		if err := helper.ValidateImageFile(file); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("File %s: %s", key, err.Error()),
			})
		}

		src, err := file.Open()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Failed to open file %s", key),
			})
		}

		if err := helper.CheckMagicBytes(src); err != nil {
			src.Close()
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("File %s: %s", key, err.Error()),
			})
		}

		compressedData, err := helper.CompressAndResize(src, file)
		src.Close()
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Processing failed for %s: %s", key, err.Error()),
			})
		}

		// Create system directory
		_ = os.MkdirAll(SystemDir, 0755)

		// Use fixed filename (will overwrite old file)
		filename := fmt.Sprintf("%s.webp", key)
		filePath := filepath.Join(SystemDir, filename)
		imageURL := fmt.Sprintf("/uploads/system/%s", filename)

		// Delete old file if exists (will be overwritten anyway)
		var oldPath string
		switch key {
		case "logo":
			oldPath = identity.LogoURL
			identity.LogoURL = imageURL
		case "ico":
			oldPath = identity.IcoURL
			identity.IcoURL = imageURL
		case "second_logo":
			oldPath = identity.SecondLogoURL
			identity.SecondLogoURL = imageURL
		}

		if oldPath != "" {
			helper.DeleteFile(filepath.Join(".", oldPath))
		}

		// Save new file
		if err := os.WriteFile(filePath, compressedData, 0644); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Failed to save file %s", key),
			})
		}
	}

	// 4. Save everything back to database using existing model logic
	if err := model.UpdateSystemIdentitySettings(identity); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to save settings to database",
		})
	}

	// 5. Log audit
	_ = model.LogAction(&model.AuditLog{
		UserID:       sql.NullInt64{Int64: userClaims.UserID, Valid: true},
		Action:       "system.identity.update_full",
		ResourceType: sql.NullString{String: "system", Valid: true},
		ResourceID:   sql.NullString{String: "identity", Valid: true},
		Details: map[string]interface{}{
			"updated_by": userClaims.Username,
			"timestamp":  helper.GetTimestamp(),
		},
		IPAddress: sql.NullString{String: c.RealIP(), Valid: true},
		UserAgent: sql.NullString{String: c.Request().UserAgent(), Valid: true},
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "System identity updated successfully",
		"data":    identity,
	})
}

// GetSystemIdentityHandler returns the global identity settings
// GET /api/system/identity
func GetSystemIdentityHandler(c echo.Context) error {
	identity, err := model.GetSystemIdentity()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to get system identity",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    identity,
	})
}
