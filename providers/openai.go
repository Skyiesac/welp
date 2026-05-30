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

// OpenAIProvider implements AIProvider for OpenAI API
type OpenAIProvider struct{}

func (o *OpenAIProvider) GetName() string {
	return "OpenAI"
}

func (o *OpenAIProvider) ValidateAPIKey() error {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}
	return nil
}

func (o *OpenAIProvider) ValidateAPIKeyWithConfig(config *Config) error {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = config.Providers["openai"]
	}
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY not configured")
	}
	return nil
}

func (o *OpenAIProvider) StreamResponse(errorText, context string, sysCtx SystemContext) error {
	apiKey := os.Getenv("OPENAI_API_KEY")
	return o.stream(apiKey, errorText, context, sysCtx)
}

func (o *OpenAIProvider) StreamResponseWithConfig(errorText, context string, sysCtx SystemContext, config *Config) error {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = config.Providers["openai"]
	}
	return o.stream(apiKey, errorText, context, sysCtx)
}

func (o *OpenAIProvider) stream(apiKey, errorText, context string, sysCtx SystemContext) error {
	prompt := BuildPrompt(errorText, context, sysCtx)

	requestBody := map[string]interface{}{
		"model":       "gpt-4-turbo",
		"max_tokens":  1024,
		"stream":      true,
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

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

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

	return parseOpenAIStream(resp.Body)
}

func parseOpenAIStream(body io.Reader) error {
	scanner := bufio.NewScanner(body)
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

		if choices, ok := event["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if text, ok := delta["content"].(string); ok {
						fmt.Print(text)
					}
				}
			}
		}
	}

	fmt.Println()
	return scanner.Err()
}
