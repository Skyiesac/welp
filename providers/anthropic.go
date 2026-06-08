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

// AnthropicProvider implements AIProvider for Anthropic API
type AnthropicProvider struct{}

func (a *AnthropicProvider) GetName() string {
	return "Anthropic"
}

func (a *AnthropicProvider) ValidateAPIKey() error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}
	return nil
}

func (a *AnthropicProvider) ValidateAPIKeyWithConfig(config *Config) error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = config.Providers["anthropic"]
	}
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY not configured")
	}
	return nil
}

func (a *AnthropicProvider) StreamResponse(errorText, context string, sysCtx SystemContext) error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	return a.stream(apiKey, errorText, context, sysCtx)
}

func (a *AnthropicProvider) StreamResponseWithConfig(errorText, context string, sysCtx SystemContext, config *Config) error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = config.Providers["anthropic"]
	}
	return a.stream(apiKey, errorText, context, sysCtx)
}

func (a *AnthropicProvider) stream(apiKey, errorText, context string, sysCtx SystemContext) error {
	prompt := BuildPrompt(errorText, context, sysCtx)

	requestBody := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"stream":     true,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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

	return parseAnthropicStream(resp.Body)
}

func parseAnthropicStream(body io.Reader) error {
	scanner := bufio.NewScanner(body)
	printer := NewColorPrinter()

	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data: "))
		if bytes.Equal(data, []byte("[DONE]")) {
			break
		}

		var event map[string]interface{}
		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}

		if delta, ok := event["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				printer.Write(text)
			}
		}
	}

	printer.Flush()
	fmt.Println()
	return scanner.Err()
}
