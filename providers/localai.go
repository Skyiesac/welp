package providers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// LocalAIProvider implements AIProvider for LocalAI (local LLM)
type LocalAIProvider struct {
	Model string // Default model
	URL   string // LocalAI server URL
}

func (l *LocalAIProvider) GetName() string {
	return "LocalAI"
}

func (l *LocalAIProvider) ValidateAPIKey() error {
	// LocalAI doesn't require API keys, but needs to be running
	if !isLocalAIRunning() {
		return fmt.Errorf("LocalAI is not running. Start it with: localai start")
	}
	return nil
}

func (l *LocalAIProvider) ValidateAPIKeyWithConfig(config *Config) error {
	return l.ValidateAPIKey()
}

func (l *LocalAIProvider) StreamResponse(errorText, context string, sysCtx SystemContext) error {
	return l.stream(errorText, context, sysCtx)
}

func (l *LocalAIProvider) StreamResponseWithConfig(errorText, context string, sysCtx SystemContext, config *Config) error {
	return l.stream(errorText, context, sysCtx)
}

func (l *LocalAIProvider) stream(errorText, context string, sysCtx SystemContext) error {
	if l.URL == "" {
		l.URL = "http://localhost:8080"
	}

	if l.Model == "" {
		l.Model = "gpt-3.5-turbo" // Default model name in LocalAI
	}

	prompt := BuildPrompt(errorText, context, sysCtx)

	// LocalAI uses OpenAI-compatible API
	requestBody := map[string]interface{}{
		"model": l.Model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"stream":      true,
		"temperature": 0.7,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", l.URL+"/v1/chat/completions", bytes.NewBuffer(body))
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
		return fmt.Errorf("LocalAI returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return parseLocalAIStream(resp.Body)
}

func parseLocalAIStream(body io.Reader) error {
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

		if choices, ok := event["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if content, ok := delta["content"].(string); ok {
						printer.Write(content)
					}
				}
			}
		}
	}

	printer.Flush()
	fmt.Println()
	return scanner.Err()
}

func isLocalAIRunning() bool {
	client := &http.Client{}
	resp, err := client.Get("http://localhost:8080/v1/models")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized
}
