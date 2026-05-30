package providers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	return c.stream(errorText, context, sysCtx)
}

func (c *CopilotProvider) stream(errorText, context string, sysCtx SystemContext) error {
	prompt := BuildPrompt(errorText, context, sysCtx)
	return c.streamFromModelsAPI(prompt)
}

// streamFromModelsAPI calls the official GitHub Models inference endpoint.
func (c *CopilotProvider) streamFromModelsAPI(prompt string) error {
	payload := map[string]interface{}{
    "model":      githubModelsModel,
    "stream":     true,
    "max_tokens": 150,
    "messages": []map[string]string{
        {
            "role":    "system",
            "content": "You are a terminal error fixer. Reply ONLY in the format: Cause/Fix/Why. One line each. No markdown. No extra text.",
        },
        {
            "role":    "user",
            "content": prompt,
        },
    },
}
//   fmt.Print(payload)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", githubModelsBaseURL+"/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.oauthToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub Models API error (status %d): %s", resp.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
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
			fmt.Print(chunk.Choices[0].Delta.Content)
		}
	}

	fmt.Println()
	return scanner.Err()
}

// IsGitHubCopilotAvailable reports whether a GitHub token is available.
func IsGitHubCopilotAvailable() bool {
	_, err := readGitHubOAuthToken()
	return err == nil
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