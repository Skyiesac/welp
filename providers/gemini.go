package providers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// GeminiProvider implements AIProvider for Google Gemini API
type GeminiProvider struct{}

func (g *GeminiProvider) GetName() string {
	return "Gemini"
}

func (g *GeminiProvider) ValidateAPIKey() error {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}
	return nil
}

func (g *GeminiProvider) ValidateAPIKeyWithConfig(config *Config) error {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = config.Providers["gemini"]
	}
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY not configured")
	}
	return nil
}

func (g *GeminiProvider) StreamResponse(errorText, context string, sysCtx SystemContext) error {
	apiKey := os.Getenv("GEMINI_API_KEY")
	return g.stream(apiKey, errorText, context, sysCtx)
}

func (g *GeminiProvider) StreamResponseWithConfig(errorText, context string, sysCtx SystemContext, config *Config) error {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = config.Providers["gemini"]
	}
	return g.stream(apiKey, errorText, context, sysCtx)
}

func (g *GeminiProvider) stream(apiKey, errorText, context string, sysCtx SystemContext) error {
	prompt := BuildPrompt(errorText, context, sysCtx)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{
						"text": prompt,
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 1024,
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:streamGenerateContent?key=%s", apiKey)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return parseGeminiStream(resp.Body)
}

func parseGeminiStream(body io.Reader) error {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event map[string]interface{}
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		if candidates, ok := event["candidates"].([]interface{}); ok && len(candidates) > 0 {
			if candidate, ok := candidates[0].(map[string]interface{}); ok {
				if content, ok := candidate["content"].(map[string]interface{}); ok {
					if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
						if part, ok := parts[0].(map[string]interface{}); ok {
							if text, ok := part["text"].(string); ok {
								fmt.Print(text)
							}
						}
					}
				}
			}
		}
	}

	fmt.Println()
	return scanner.Err()
}
