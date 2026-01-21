// internal/helper/image_processor.go
package helper

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	_ "github.com/mat/besticon/ico"
)

const (
	MaxAvatarDimension    = 1024
	MinAvatarDimension    = 100
	TargetFileSizeKB      = 500
	TargetFileSizeBytes   = TargetFileSizeKB * 1024
	MaxDecompressedSizeMB = 50
	MaxDecompressedSize   = MaxDecompressedSizeMB * 1024 * 1024
)

// CompressAndResize processes uploaded image: validates, resizes, and compresses to WebP
func CompressAndResize(file multipart.File, fileHeader *multipart.FileHeader) ([]byte, error) {
	// Validate image content (deep validation)
	if err := ValidateImageContent(file); err != nil {
		return nil, err
	}

	// Decode image
	img, err := decodeImage(file, fileHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Validate decompressed size (prevent decompression bombs)
	if err := ValidateDecompressedSize(img); err != nil {
		return nil, err
	}

	// Validate dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width < MinAvatarDimension || height < MinAvatarDimension {
		return nil, fmt.Errorf("image too small: minimum %dx%d pixels", MinAvatarDimension, MinAvatarDimension)
	}

	// Resize if needed (maintain aspect ratio)
	if width > MaxAvatarDimension || height > MaxAvatarDimension {
		img = imaging.Fit(img, MaxAvatarDimension, MaxAvatarDimension, imaging.Lanczos)
	}

	// Convert to WebP with iterative quality reduction to meet size target
	webpData, err := convertToWebPWithSizeLimit(img)
	if err != nil {
		return nil, err
	}

	return webpData, nil
}

// ValidateImageContent performs deep validation on image file
func ValidateImageContent(file multipart.File) error {
	// Read file content for malicious pattern detection
	buffer := make([]byte, 8192) // Read first 8KB
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset file pointer: %w", err)
	}

	// Detect malicious content
	if DetectMaliciousContent(buffer[:n]) {
		return errors.New("malicious content detected in file")
	}

	return nil
}

// DetectMaliciousContent scans for embedded scripts or malicious patterns
func DetectMaliciousContent(data []byte) bool {
	content := strings.ToLower(string(data))

	maliciousPatterns := []string{
		"<?php",
		"<script",
		"eval(",
		"base64_decode",
		"system(",
		"exec(",
		"shell_exec",
		"passthru",
		"<iframe",
		"javascript:",
		"onerror=",
		"onload=",
	}

	for _, pattern := range maliciousPatterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}

	return false
}

// ValidateDecompressedSize prevents decompression bomb attacks
func ValidateDecompressedSize(img image.Image) error {
	bounds := img.Bounds()
	pixels := bounds.Dx() * bounds.Dy()

	// RGBA = 4 bytes per pixel
	decompressedSize := pixels * 4

	if decompressedSize > MaxDecompressedSize {
		return fmt.Errorf("decompression bomb detected: image too large when decompressed (%d MB)", decompressedSize/(1024*1024))
	}

	return nil
}

// convertToWebPWithSizeLimit converts image to WebP with iterative quality reduction
func convertToWebPWithSizeLimit(img image.Image) ([]byte, error) {
	qualities := []float32{85, 75, 60, 50, 40}

	for _, quality := range qualities {
		var buf bytes.Buffer

		// Encode to WebP
		if err := webp.Encode(&buf, img, &webp.Options{
			Lossless: false,
			Quality:  quality,
		}); err != nil {
			return nil, fmt.Errorf("failed to encode WebP: %w", err)
		}

		// Check size
		if buf.Len() <= TargetFileSizeBytes {
			return buf.Bytes(), nil
		}
	}

	// If still too large after quality 40, return error
	return nil, fmt.Errorf("unable to compress image to %dKB", TargetFileSizeKB)
}

// decodeImage decodes image from multipart file
func decodeImage(file multipart.File, fileHeader *multipart.FileHeader) (image.Image, error) {
	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}

	// Try to decode based on content type
	contentType := fileHeader.Header.Get("Content-Type")

	switch contentType {
	case "image/jpeg":
		return jpeg.Decode(file)
	case "image/png":
		return png.Decode(file)
	case "image/webp":
		return webp.Decode(file)
	case "image/x-icon", "image/vnd.microsoft.icon":
		// Fallback for ICO if direct decode fails, standard image.Decode will handle it
		// now that the driver is registered
		img, _, err := image.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("failed to decode ICO file: %w", err)
		}
		return img, nil
	default:
		// Fallback: try generic decode
		img, _, err := image.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("unsupported image format or corrupted file")
		}
		return img, nil
	}
}
