// internal/helper/file_upload.go
package helper

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// File upload configuration
const (
	MaxAvatarSizeMB    = 1
	MaxAvatarSizeBytes = MaxAvatarSizeMB * 1024 * 1024
	UploadDir          = "./uploads"
	AvatarDir          = "./uploads/avatars"
	SystemDir          = "./uploads/system"
)

var (
	AllowedExtensions = []string{".jpg", ".jpeg", ".png", ".webp", ".ico"}
	AllowedMIMETypes  = []string{"image/jpeg", "image/png", "image/webp", "image/x-icon", "image/vnd.microsoft.icon"}
)

// Magic bytes for file type detection
var magicBytes = map[string][]byte{
	"jpeg": {0xFF, 0xD8, 0xFF},
	"png":  {0x89, 0x50, 0x4E, 0x47},
	"webp": {0x52, 0x49, 0x46, 0x46},
	"ico":  {0x00, 0x00, 0x01, 0x00},
}

// CreateUploadDirectory ensures upload directories exist
func CreateUploadDirectory() error {
	// Create main upload directory
	if err := os.MkdirAll(UploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Create avatars subdirectory
	if err := os.MkdirAll(AvatarDir, 0755); err != nil {
		return fmt.Errorf("failed to create avatars directory: %w", err)
	}

	return nil
}

// ValidateImageFile performs basic validation on uploaded file
func ValidateImageFile(fileHeader *multipart.FileHeader) error {
	// Check file size
	if fileHeader.Size > MaxAvatarSizeBytes {
		return fmt.Errorf("file too large: max size is %dMB", MaxAvatarSizeMB)
	}

	if fileHeader.Size == 0 {
		return fmt.Errorf("file is empty")
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !contains(AllowedExtensions, ext) {
		return fmt.Errorf("invalid file type: only JPG, PNG, WebP, and ICO are allowed")
	}

	// Check MIME type
	contentType := fileHeader.Header.Get("Content-Type")
	if !contains(AllowedMIMETypes, contentType) {
		return fmt.Errorf("invalid MIME type: %s", contentType)
	}

	return nil
}

// CheckMagicBytes validates file signature (magic bytes)
func CheckMagicBytes(file multipart.File) error {
	// Read first 512 bytes for magic byte detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset file pointer: %w", err)
	}

	// Check magic bytes
	validMagicBytes := false
	for _, magic := range magicBytes {
		if n >= len(magic) && bytesEqual(buffer[:len(magic)], magic) {
			validMagicBytes = true
			break
		}
	}

	if !validMagicBytes {
		return fmt.Errorf("invalid file signature: file may be corrupted or not a valid image")
	}

	return nil
}

// SanitizeFilename removes dangerous characters from filename
func SanitizeFilename(filename string) string {
	// Get base filename (remove path)
	filename = filepath.Base(filename)

	// Remove path traversal attempts
	filename = strings.ReplaceAll(filename, "..", "")
	filename = strings.ReplaceAll(filename, "/", "")
	filename = strings.ReplaceAll(filename, "\\", "")

	// Only allow alphanumeric, underscore, hyphen, and dot
	reg := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	filename = reg.ReplaceAllString(filename, "")

	return filename
}

// GetUserUploadDir returns the upload directory for a specific user
func GetUserUploadDir(userID int64) string {
	return filepath.Join(AvatarDir, fmt.Sprintf("%d", userID))
}

// GenerateSecureFilename generates a filename for user avatar (always overwrites)
func GenerateSecureFilename(userID int64, fileType string) string {
	// Use fixed filename to ensure overwriting
	// fileType can be "avatar", "logo", etc.
	if fileType == "" {
		fileType = "avatar"
	}
	return fmt.Sprintf("%s.webp", fileType)
}

// GetUserAvatarPath returns the full path for user's avatar
func GetUserAvatarPath(userID int64) string {
	userDir := GetUserUploadDir(userID)
	return filepath.Join(userDir, "avatar.webp")
}

// GetUserAvatarURL returns the URL for user's avatar
func GetUserAvatarURL(userID int64) string {
	return fmt.Sprintf("/uploads/avatars/%d/avatar.webp", userID)
}

// SaveFile saves uploaded file to disk
func SaveFile(src multipart.File, destPath string) error {
	// Create destination file
	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Set file permissions (read-only for others)
	if err := dst.Chmod(0644); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Copy file content
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// DeleteFile removes a file from disk
func DeleteFile(filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to delete
	}

	// Delete file
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetFileExtension returns file extension
func GetFileExtension(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}

// GetFileSize returns file size in bytes
func GetFileSize(file multipart.File) (int64, error) {
	// Get current position
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	// Seek to end to get size
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	// Reset to original position
	if _, err := file.Seek(currentPos, io.SeekStart); err != nil {
		return 0, err
	}

	return size, nil
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GetTimestamp returns current Unix timestamp
func GetTimestamp() int64 {
	return time.Now().Unix()
}
