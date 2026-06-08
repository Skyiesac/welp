package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	githubModelsBaseURL = "https://models.github.ai/inference"
	githubModelsModel   = "openai/gpt-4o"
)

// CopilotProvider implements AIProvider using the GitHub Models API.
// Works with Copilot Free, Pro, and paid tiers — no session token exchange needed.
type CopilotProvider struct {
	oauthToken string
	httpClient *http.Client
	model      string
}

func (c *CopilotProvider) GetName() string {
	return "GitHub Copilot"
}

func (c *CopilotProvider) ValidateAPIKey() error {
	token, err := readGitHubOAuthToken()
	if err != nil {
		return fmt.Errorf("GitHub Copilot: %w", err)
	}
	c.oauthToken = token
	c.httpClient = &http.Client{Timeout: 60 * time.Second}
	return nil
}

func (c *CopilotProvider) ValidateAPIKeyWithConfig(config *Config) error {
	return c.ValidateAPIKey()
}

func (c *CopilotProvider) StreamResponse(errorText, context string, sysCtx SystemContext) error {
	return c.stream(errorText, context, sysCtx)
}

func (c *CopilotProvider) StreamResponseWithConfig(errorText, context string, sysCtx SystemContext, config *Config) error {
	// Read model from config if available
	if config != nil && config.ProviderModels != nil {
		if model, ok := config.ProviderModels["copilot"]; ok && model != "" {
			c.model = model
		}
	}
	// Use default if not set
	if c.model == "" {
		c.model = githubModelsModel
	}
	return c.stream(errorText, context, sysCtx)
}

func (c *CopilotProvider) stream(errorText, context string, sysCtx SystemContext) error {
	prompt := BuildPrompt(errorText, context, sysCtx)

	// Create a cancellable context for the streaming request
	ctx, cancel := contextWithSignalCancel()
	defer cancel()

	return c.streamFromModelsAPI(ctx, prompt)
}

// streamFromModelsAPI calls the official GitHub Models inference endpoint.
func (c *CopilotProvider) streamFromModelsAPI(ctx context.Context, prompt string) error {
	model := c.model
	if model == "" {
		model = githubModelsModel
	}
	payload := map[string]interface{}{
		"model":      model,
		"stream":     true,
		"max_tokens": 150,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a terminal error fixer. Reply only in one line each. No markdown. No extra text.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}
	// fmt.Print(payload)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", githubModelsBaseURL+"/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.oauthToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Check if it was a context cancellation (Ctrl+C)
		if ctx.Err() == context.Canceled {
			fmt.Println("\n\n⏹️  Request cancelled by user")
			return nil
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub Models API error (status %d): %s", resp.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	printer := NewColorPrinter()

	for scanner.Scan() {
		// Check for cancellation signal
		select {
		case <-ctx.Done():
			fmt.Println("\n\n⏹️  Request cancelled by user")
			return nil
		default:
		}

		line := scanner.Text()
		if line == "" || line == "data: [DONE]" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			printer.Write(chunk.Choices[0].Delta.Content)
		}
	}

	printer.Flush()
	fmt.Println()
	return scanner.Err()
}

// IsGitHubCopilotAvailable reports whether a GitHub token is available.
func IsGitHubCopilotAvailable() bool {
	_, err := readGitHubOAuthToken()
	return err == nil
}

// contextWithSignalCancel creates a context that cancels on SIGINT (Ctrl+C)
func contextWithSignalCancel() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		_ = sig // Use sig to avoid unused variable warning
		cancel()
	}()

	return ctx, cancel
}

// FetchAvailableModels fetches the list of available models from the GitHub Models catalog
func FetchAvailableModels() (map[string]string, error) {
	token, err := readGitHubOAuthToken()
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://models.github.ai/catalog/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog API error (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var models []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	if err := json.Unmarshal(body, &models); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, m := range models {
		if m.ID != "" && m.Name != "" {
			result[m.ID] = m.Name
		}
	}

	return result, nil
}

// FetchAvailableCopilotModels fetches available models and returns as slice for setup UI
func FetchAvailableCopilotModels() ([]string, error) {
	token, err := readGitHubOAuthToken()
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://models.github.ai/catalog/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog API error (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var models []struct {
		ID string `json:"id"`
	}

	if err := json.Unmarshal(body, &models); err != nil {
		return nil, err
	}

	var result []string
	for _, m := range models {
		if m.ID != "" {
			result = append(result, m.ID)
		}
	}

	return result, nil
}

// readGitHubOAuthToken reads a token from env vars, hosts.yml, or gh CLI.
func readGitHubOAuthToken() (string, error) {
	// 1. env vars
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if token := strings.TrimSpace(os.Getenv(key)); token != "" {
			return token, nil
		}
	}
	// 2. hosts.yml
	if token, err := readOAuthTokenFromHostsFile(); err == nil {
		return token, nil
	}
	// 3. gh CLI keyring (handles keyring-stored tokens)
	if token, err := readGitHubTokenFromCLI(); err == nil {
		return token, nil
	}
	return "", fmt.Errorf("no GitHub token found (set GH_TOKEN or run `gh auth login`)")
}

func readGitHubTokenFromCLI() (string, error) {
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh not found in PATH")
	}
	out, err := exec.Command(ghPath, "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token failed: %w", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("empty token from gh auth token")
	}
	return token, nil
}

func readOAuthTokenFromHostsFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(home, ".config/gh/hosts.yml")
	file, err := os.Open(configPath)
	if err != nil {
		return "", fmt.Errorf("cannot read GitHub CLI config: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "oauth_token:") {
			token := strings.TrimSpace(strings.TrimPrefix(line, "oauth_token:"))
			token = strings.Trim(token, "\"'")
			if token != "" {
				return token, nil
			}
		}
	}
	return "", fmt.Errorf("oauth_token not found in GitHub CLI config")
}
