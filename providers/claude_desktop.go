package providers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ClaudeDesktopProvider implements AIProvider for Claude Desktop CLI
type ClaudeDesktopProvider struct {
	CLIPath string
}

func (c *ClaudeDesktopProvider) GetName() string {
	return "Claude Desktop"
}

func (c *ClaudeDesktopProvider) ValidateAPIKey() error {
	// Find Claude Desktop CLI
	path, err := findClaudeCLI()
	if err != nil {
		return fmt.Errorf("Claude Desktop CLI not found. Install Claude Desktop from https://claude.ai/")
	}
	c.CLIPath = path
	return nil
}

func (c *ClaudeDesktopProvider) ValidateAPIKeyWithConfig(config *Config) error {
	return c.ValidateAPIKey()
}

func (c *ClaudeDesktopProvider) StreamResponse(errorText, context string, sysCtx SystemContext) error {
	return c.stream(errorText, context, sysCtx)
}

func (c *ClaudeDesktopProvider) StreamResponseWithConfig(errorText, context string, sysCtx SystemContext, config *Config) error {
	return c.stream(errorText, context, sysCtx)
}

func (c *ClaudeDesktopProvider) stream(errorText, context string, sysCtx SystemContext) error {
	if c.CLIPath == "" {
		path, err := findClaudeCLI()
		if err != nil {
			return err
		}
		c.CLIPath = path
	}

	prompt := BuildPrompt(errorText, context, sysCtx)

	// Use the Claude Desktop CLI to get response
	// The CLI is invoked as: claude <prompt>
	cmd := exec.Command(c.CLIPath)
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set the prompt as environment variable or pass via stdin
	cmd.Env = append(os.Environ(), "CLAUDE_PROMPT="+prompt)

	// Execute the command
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to call Claude Desktop: %w", err)
	}

	return nil
}

func findClaudeCLI() (string, error) {
	// Try to find claude in PATH first
	if path, err := exec.LookPath("claude"); err == nil {
		return path, nil
	}

	home, _ := os.UserHomeDir()

	// Possible installation paths
	paths := []string{
		// Linux
		filepath.Join(home, ".local", "share", "claude-desktop", "bin", "claude"),
		// macOS
		"/Applications/Claude.app/Contents/MacOS/claude",
		// Alternative Linux location
		filepath.Join(home, ".claude", "bin", "claude"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("Claude Desktop CLI not found in any known location")
}
