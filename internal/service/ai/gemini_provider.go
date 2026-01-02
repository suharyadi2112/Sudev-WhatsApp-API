package ai

import (
	"context"
	"fmt"
	"strings"

	"gowa-yourself/config"

	"google.golang.org/genai"
)

// ConversationMessage represents a single message in conversation history
type ConversationMessage struct {
	Sender  string // "human" or "bot"
	Message string
}

// GenerateReply generates an AI response using Gemini (Official SDK)
func GenerateReply(systemPrompt string, conversationHistory []ConversationMessage, temperature float64, maxTokens int) (string, error) {
	// Validate API key
	if config.GeminiAPIKey == "" {
		return "", fmt.Errorf("Gemini API key not configured")
	}

	ctx := context.Background()

	// Create Gemini client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  config.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Build prompt from conversation history
	var contextParts []string
	if len(conversationHistory) > 0 {
		contextParts = append(contextParts, "Previous conversation:")
		for _, msg := range conversationHistory {
			role := "Customer"
			if msg.Sender == "bot" {
				role = "You"
			}
			contextParts = append(contextParts, fmt.Sprintf("%s: %s", role, msg.Message))
		}
		contextParts = append(contextParts, "\nPlease respond to the customer's last message:")
	}

	prompt := strings.Join(contextParts, "\n")
	if prompt == "" {
		prompt = "Please greet the customer."
	}

	systemInstruction := systemPrompt
	if systemInstruction == "" {
		systemInstruction = "You are a helpful customer service assistant. Be friendly, concise, and professional."
	}

	// Setup parameters and clean model name
	temp := float32(temperature)
	maxTok := int32(maxTokens)
	modelName := strings.TrimPrefix(config.GeminiDefaultModel, "models/")

	// Call Gemini API
	result, err := client.Models.GenerateContent(
		ctx,
		modelName,
		genai.Text(prompt),
		&genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{
					{Text: systemInstruction},
				},
			},
			Temperature:     &temp,
			MaxOutputTokens: maxTok,
		},
	)
	if err != nil {
		return "", fmt.Errorf("Gemini SDK Error: %w", err)
	}

	// Extract and return result
	responseText := result.Text()
	if responseText == "" {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return strings.TrimSpace(responseText), nil
}
