// internal/handler/file_upload.go
package handler

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"gowa-yourself/internal/helper"
	"gowa-yourself/internal/model"
	"gowa-yourself/internal/service"

	"github.com/labstack/echo/v4"
)

// UploadAvatar handles avatar upload
// POST /api/me/avatar
func UploadAvatar(c echo.Context) error {
	// Get user from context
	userClaims, ok := c.Get("user_claims").(*service.Claims)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Unauthorized",
			"error": map[string]string{
				"code": "UNAUTHORIZED",
			},
		})
	}

	// Get uploaded file
	file, err := c.FormFile("avatar")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "No file uploaded",
			"error": map[string]string{
				"code": "NO_FILE",
			},
		})
	}

	// Validate file (basic validation)
	if err := helper.ValidateImageFile(file); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": err.Error(),
			"error": map[string]string{
				"code": "INVALID_FILE",
			},
		})
	}

	// Open file
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to open uploaded file",
			"error": map[string]string{
				"code": "FILE_OPEN_ERROR",
			},
		})
	}
	defer src.Close()

	// Check magic bytes (file signature validation)
	if err := helper.CheckMagicBytes(src); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": err.Error(),
			"error": map[string]string{
				"code": "INVALID_FILE_SIGNATURE",
			},
		})
	}

	// Process image: validate, compress, convert to WebP
	log.Printf("📸 Processing avatar upload for user %d (original size: %d bytes)", userClaims.UserID, file.Size)

	compressedData, err := helper.CompressAndResize(src, file)
	if err != nil {
		log.Printf("❌ Image processing failed: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Image processing failed: %s", err.Error()),
			"error": map[string]string{
				"code": "PROCESSING_FAILED",
			},
		})
	}

	log.Printf("✅ Image compressed: %d bytes → %d bytes", file.Size, len(compressedData))

	// Create user-specific directory
	userDir := helper.GetUserUploadDir(userClaims.UserID)
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to create user directory",
			"error": map[string]string{
				"code": "DIRECTORY_ERROR",
			},
		})
	}

	// Get file path (always overwrites existing avatar)
	filePath := helper.GetUserAvatarPath(userClaims.UserID)
	avatarURL := helper.GetUserAvatarURL(userClaims.UserID)

	// Delete old avatar if exists (will be overwritten anyway, but good practice)
	if _, err := os.Stat(filePath); err == nil {
		log.Printf("🗑️ Overwriting existing avatar: %s", filePath)
	}

	// Save compressed file
	if err := saveCompressedFile(filePath, compressedData); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to save file",
			"error": map[string]string{
				"code": "SAVE_ERROR",
			},
		})
	}

	// Get user for database update
	user, err := model.GetUserByID(userClaims.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to get user data",
			"error": map[string]string{
				"code": "DATABASE_ERROR",
			},
		})
	}

	// Update user avatar_url in database
	user.AvatarURL = sql.NullString{String: avatarURL, Valid: true}
	err = model.UpdateUser(user)
	if err != nil {
		// Rollback: delete uploaded file
		helper.DeleteFile(filePath)

		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to update user profile",
			"error": map[string]string{
				"code": "DATABASE_ERROR",
			},
		})
	}

	// Log upload to audit_logs
	_ = model.LogAction(&model.AuditLog{
		UserID:       sql.NullInt64{Int64: userClaims.UserID, Valid: true},
		Action:       "avatar.upload",
		ResourceType: sql.NullString{String: "user", Valid: true},
		ResourceID:   sql.NullString{String: userClaims.Username, Valid: true},
		Details: map[string]interface{}{
			"original_filename": file.Filename,
			"original_size":     file.Size,
			"compressed_size":   len(compressedData),
			"format":            "webp",
			"saved_as":          filepath.Base(filePath),
		},
		IPAddress: sql.NullString{String: c.RealIP(), Valid: true},
		UserAgent: sql.NullString{String: c.Request().UserAgent(), Valid: true},
	})

	log.Printf("✅ Avatar uploaded successfully for user %d: %s", userClaims.UserID, avatarURL)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Avatar uploaded successfully",
		"data": map[string]interface{}{
			"avatar_url":      avatarURL,
			"original_size":   file.Size,
			"compressed_size": len(compressedData),
			"format":          "webp",
		},
	})
}

// saveCompressedFile saves compressed byte data to file
func saveCompressedFile(filePath string, data []byte) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Set file permissions
	if err := file.Chmod(0644); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Write data
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
