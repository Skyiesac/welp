package providers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OllamaProvider implements AIProvider for Ollama (local LLM)
type OllamaProvider struct {
	Model string // Default model, can be configured
}

func (o *OllamaProvider) GetName() string {
	return "Ollama"
}

func (o *OllamaProvider) ValidateAPIKey() error {
	// Ollama doesn't require API keys, but needs to be running
	if !isOllamaRunning() {
		return fmt.Errorf("Ollama is not running. Start it with: ollama serve")
	}
	return nil
}

func (o *OllamaProvider) ValidateAPIKeyWithConfig(config *Config) error {
	return o.ValidateAPIKey()
}

func (o *OllamaProvider) StreamResponse(errorText, context string, sysCtx SystemContext) error {
	return o.stream(errorText, context, sysCtx)
}

func (o *OllamaProvider) StreamResponseWithConfig(errorText, context string, sysCtx SystemContext, config *Config) error {
	return o.stream(errorText, context, sysCtx)
}

func (o *OllamaProvider) stream(errorText, context string, sysCtx SystemContext) error {
	if o.Model == "" {
		// Try to use commonly available models
		model, err := getAvailableOllamaModel()
		if err != nil {
			return fmt.Errorf("no models available in Ollama. Pull a model first: ollama pull neural-chat")
		}
		o.Model = model
	}

	prompt := BuildPrompt(errorText, context, sysCtx)

	requestBody := map[string]interface{}{
		"model":  o.Model,
		"prompt": prompt,
		"stream": true,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "http://localhost:11434/api/generate", bytes.NewBuffer(body))
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
		return fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return parseOllamaStream(resp.Body)
}

func parseOllamaStream(body io.Reader) error {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Bytes()

		var event map[string]interface{}
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		if response, ok := event["response"].(string); ok {
			fmt.Print(response)
		}
	}

	fmt.Println()
	return scanner.Err()
}

func isOllamaRunning() bool {
	client := &http.Client{}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func getAvailableOllamaModel() (string, error) {
	client := &http.Client{}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if models, ok := result["models"].([]interface{}); ok && len(models) > 0 {
		if firstModel, ok := models[0].(map[string]interface{}); ok {
			if name, ok := firstModel["name"].(string); ok {
				return name, nil
			}
		}
	}

	return "", fmt.Errorf("no models available")
}
