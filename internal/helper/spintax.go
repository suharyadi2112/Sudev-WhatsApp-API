package helper

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func RenderSpintax(text string) string {
	result := RenderDynamicVariables(text)

	for {
		start := strings.Index(result, "{")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		spintax := result[start+1 : end]
		options := strings.Split(spintax, "|")
		chosen := options[rand.Intn(len(options))]

		result = result[:start] + chosen + result[end+1:]
	}
	return result
}

func RenderDynamicVariables(text string) string {
	now := time.Now()

	hour := now.Hour()
	var timeGreeting string
	switch {
	case hour >= 5 && hour < 10:
		timeGreeting = "Pagi"
	case hour >= 10 && hour < 15:
		timeGreeting = "Siang"
	case hour >= 15 && hour < 18:
		timeGreeting = "Sore"
	default:
		timeGreeting = "Malam"
	}

	dayNames := []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	dayName := dayNames[now.Weekday()]

	monthNames := []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	date := fmt.Sprintf("%d %s %d", now.Day(), monthNames[now.Month()], now.Year())

	result := text
	result = strings.ReplaceAll(result, "{TIME_GREETING}", timeGreeting)
	result = strings.ReplaceAll(result, "{DAY_NAME}", dayName)
	result = strings.ReplaceAll(result, "{DATE}", date)

	return result
}
