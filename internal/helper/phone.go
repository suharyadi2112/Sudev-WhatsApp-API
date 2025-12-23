package helper

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"go.mau.fi/whatsmeow/types"
)

// FormatPhoneNumber converts phone number to WhatsApp JID format
func FormatPhoneNumber(phone string) (types.JID, error) {
	// 🔥 REGEX: Hanya terima digit, +, -, (, ), spasi
	validFormat := regexp.MustCompile(`^[\d\s\+\-\(\)]+$`)
	if !validFormat.MatchString(phone) {
		return types.JID{}, fmt.Errorf("invalid phone number format: contains invalid characters")
	}

	// Hapus semua karakter kecuali digit
	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(phone, "")

	// Validate minimal input
	if len(cleaned) < 9 {
		return types.JID{}, fmt.Errorf("phone number too short")
	}

	// Auto-convert 0xxx → 62xxx
	if strings.HasPrefix(cleaned, "0") {
		cleaned = "62" + cleaned[1:]
	}

	// Auto-convert 8xxx → 62xxx (nomor tanpa 0 di depan)
	if len(cleaned) >= 9 && strings.HasPrefix(cleaned, "8") && !strings.HasPrefix(cleaned, "62") {
		cleaned = "62" + cleaned
	}

	// 🔥 ENFORCE: Must start with 62
	if !strings.HasPrefix(cleaned, "62") {
		return types.JID{}, fmt.Errorf("phone number must start with 62 (Indonesia). Example: 628123456789")
	}

	// Validate length (62 + 9-12 digit nomor Indonesia)
	if len(cleaned) < 11 || len(cleaned) > 15 {
		return types.JID{}, fmt.Errorf("invalid phone number length")
	}

	// 🔥 VALIDASI TAMBAHAN: Cek apakah digit kedua dan ketiga masuk akal untuk Indonesia
	// Indonesia: 628xxx (valid operator: 08xx)
	if len(cleaned) >= 3 {
		thirdDigit := cleaned[2]
		if thirdDigit != '8' && thirdDigit != '1' && thirdDigit != '2' && thirdDigit != '5' && thirdDigit != '9' {
			return types.JID{}, fmt.Errorf("invalid Indonesian phone number format")
		}
	}

	return types.JID{
		User:   cleaned,
		Server: types.DefaultUserServer,
	}, nil
}

// ShouldSkipValidation checks if the phone number should skip IsOnWhatsApp validation
func ShouldSkipValidation(phone string) bool {
	allow9Digit := os.Getenv("ALLOW_9_DIGIT_PHONE_NUMBER") == "true"
	if !allow9Digit {
		return false
	}

	cleaned := regexp.MustCompile(`[^\d]`).ReplaceAllString(phone, "")

	// 9 digits: 818280277
	if len(cleaned) == 9 {
		return true
	}

	// 10 digits with 0: 0818280277
	if len(cleaned) == 10 && strings.HasPrefix(cleaned, "0") {
		return true
	}

	// 11 digits with 62: 62818280277
	if len(cleaned) == 11 && strings.HasPrefix(cleaned, "62") {
		return true
	}

	return false
}

func ExtractPhoneFromJID(jid string) string {
	// "6285148107612:43@s.whatsapp.net" -> "6285148107612"
	atSplit := strings.SplitN(jid, "@", 2)
	if len(atSplit) == 0 {
		return jid
	}
	beforeAt := atSplit[0]
	colonSplit := strings.SplitN(beforeAt, ":", 2)
	return colonSplit[0]
}
