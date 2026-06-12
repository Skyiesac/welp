package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Skyiesac/welp/providers"
)

// CredentialSource represents where credentials come from
type CredentialSource struct {
	Type         string // "env", "config", "ollama", "localai", "claude-desktop", "chatgpt"
	ProviderName string
	Description  string
	Priority     int // Lower number = higher priority
}

// DetectAvailableCredentials checks all possible credential sources
func DetectAvailableCredentials() []CredentialSource {
	var sources []CredentialSource
	priority := 0

	// 1. Check environment variables first (highest priority)
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		sources = append(sources, CredentialSource{
			Type:         "env",
			ProviderName: "anthropic",
			Description:  "ANTHROPIC_API_KEY environment variable",
			Priority:     priority,
		})
		priority++
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		sources = append(sources, CredentialSource{
			Type:         "env",
			ProviderName: "openai",
			Description:  "OPENAI_API_KEY environment variable",
			Priority:     priority,
		})
		priority++
	}
	if os.Getenv("GEMINI_API_KEY") != "" {
		sources = append(sources, CredentialSource{
			Type:         "env",
			ProviderName: "gemini",
			Description:  "GEMINI_API_KEY environment variable",
			Priority:     priority,
		})
		priority++
	}

	// 2. Check local LLM models (Ollama first, then LocalAI)
	if isOllamaAvailable() {
		sources = append(sources, CredentialSource{
			Type:         "ollama",
			ProviderName: "ollama",
			Description:  "Ollama (running locally on localhost:11434)",
			Priority:     priority,
		})
		priority++
	}

	if isLocalAIAvailable() {
		sources = append(sources, CredentialSource{
			Type:         "localai",
			ProviderName: "localai",
			Description:  "LocalAI (running locally on localhost:8080)",
			Priority:     priority,
		})
		priority++
	}

	// 3. Check desktop AI apps
	if isClaudeDesktopAvailable() {
		sources = append(sources, CredentialSource{
			Type:         "claude-desktop",
			ProviderName: "claude-desktop",
			Description:  "Claude Desktop application (via CLI)",
			Priority:     priority,
		})
		priority++
	}

	// 4. Check GitHub Copilot HTTP API credentials
	if providers.IsGitHubCopilotAvailable() {
		sources = append(sources, CredentialSource{
			Type:         "github-copilot",
			ProviderName: "copilot",
			Description:  "GitHub Copilot (HTTP API)",
			Priority:     priority,
		})
		priority++
	}

	return sources
}

// isOllamaAvailable checks if Ollama is running locally
func isOllamaAvailable() bool {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// isLocalAIAvailable checks if LocalAI is running locally
func isLocalAIAvailable() bool {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get("http://localhost:8080/v1/models")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized
}

// isClaudeDesktopAvailable checks if Claude Desktop is installed and accessible
func isClaudeDesktopAvailable() bool {
	// Try to find Claude Desktop CLI
	paths := getClaudeDesktopPaths()
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			// Check if it's executable
			if isExecutable(path) {
				return true
			}
		}
	}

	// Also check if 'claude' command is in PATH
	_, err := exec.LookPath("claude")
	return err == nil
}

// getClaudeDesktopPaths returns possible paths for Claude Desktop executable
func getClaudeDesktopPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{}

	// Linux paths
	if runtime := os.Getenv("XDG_DATA_HOME"); runtime != "" {
		paths = append(paths, filepath.Join(runtime, "claude-desktop", "bin", "claude"))
	}
	paths = append(paths, filepath.Join(home, ".local", "share", "claude-desktop", "bin", "claude"))

	// macOS paths
	paths = append(paths, "/Applications/Claude.app/Contents/MacOS/claude")

	// Windows paths (even though we're on Linux, for completeness)
	paths = append(paths, filepath.Join(home, "AppData", "Local", "Claude", "bin", "claude.exe"))

	return paths
}

// isExecutable checks if a file is executable
func isExecutable(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fileInfo.Mode()&0111 != 0
}

// PrintDetectedCredentials displays available credential sources to user
func PrintDetectedCredentials(sources []CredentialSource) {
	if len(sources) == 0 {
		return
	}

	fmt.Println("\n✓ Detected available AI sources:")
	for _, source := range sources {
		fmt.Printf("  • %s - %s\n", source.ProviderName, source.Description)
	}
	fmt.Println()
}
