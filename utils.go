package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/Skyiesac/welp/providers"
)

// collectContextInParallel gathers OS, shell, cwd, and history concurrently
func collectContextInParallel() providers.SystemContext {
	var wg sync.WaitGroup
	sysCtx := providers.SystemContext{}

	wg.Add(4)

	go func() {
		defer wg.Done()
		sysCtx.OS = getDistro()
	}()

	go func() {
		defer wg.Done()
		sysCtx.Shell = getShell()
	}()

	go func() {
		defer wg.Done()
		sysCtx.CWD = getCWD()
	}()

	go func() {
		defer wg.Done()
		sysCtx.RecentCommands = loadRecentCommands()
	}()

	wg.Wait()
	return sysCtx
}

func getShell() string {
	shell := os.Getenv("SHELL")
	if shell != "" {
		return shell
	}
	return "unknown"
}

func getCWD() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return cwd
}

func loadRecentCommands() []providers.RecentCommand {
	historyCmds := getLastShellCommands()
	storedCmds := loadStoredRecentCommands()
	outputByCommand := map[string]string{}
	for _, cmd := range storedCmds {
		if cmd.Command == "" || cmd.Output == "" {
			continue
		}
		outputByCommand[cmd.Command] = cmd.Output
	}

	if len(historyCmds) == 0 {
		return storedCmds
	}

	cmds := make([]providers.RecentCommand, 0, len(historyCmds))
	for _, command := range historyCmds {
		cmds = append(cmds, providers.RecentCommand{
			Command: command,
			Output:  outputByCommand[command],
		})
	}
	return cmds
}

func loadStoredRecentCommands() []providers.RecentCommand {
	path := getRecentCommandsPath()
	if path == "" {
		return []providers.RecentCommand{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return []providers.RecentCommand{}
	}

	var cmds []providers.RecentCommand
	if err := json.Unmarshal(data, &cmds); err != nil {
		return []providers.RecentCommand{}
	}
	return cmds
}

func getLastShellCommands() []string {
	histFile := os.Getenv("HISTFILE")
	if histFile == "" {
		histFile = os.Getenv("HOME") + "/.bash_history"
	}

	data, err := os.ReadFile(histFile)
	if err != nil {
		return []string{}
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var cmds []string
	for i := len(lines) - 1; i >= 0 && len(cmds) < 3; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "#") {
			cmds = append([]string{line}, cmds...)
		}
	}
	return cmds
}

func saveRecentCommands(cmds []providers.RecentCommand) {
	path := getRecentCommandsPath()
	if path == "" {
		return
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	data, err := json.MarshalIndent(cmds, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(path, data, 0600)
}

func appendRecentCommand(cmds []providers.RecentCommand, current providers.RecentCommand) []providers.RecentCommand {
	cmds = append(cmds, current)
	if len(cmds) > 4 {
		cmds = cmds[len(cmds)-4:]
	}
	return cmds
}

func getRecentCommandsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".welp", "recent-commands.json")
}

func shouldStoreOutput(command string, output string) bool {
	if output == "" {
		return false
	}
	if len(output) > 20000 && looksLikeInstallCommand(command) {
		return false
	}
	return true
}

func looksLikeInstallCommand(command string) bool {
	command = strings.ToLower(command)
	installKeywords := []string{" install ", " install", "install ", "install", "upgrade", "add ", "add", "get ", " get "}
	for _, keyword := range installKeywords {
		if strings.Contains(command, keyword) {
			return true
		}
	}
	return false
}

func getDistro() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			val := strings.TrimPrefix(line, "PRETTY_NAME=")
			return strings.Trim(val, "\"")
		}
	}
	return "unknown"
}

// readStdin reads all input from stdin
func readStdin() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	var lines []string

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}

// ValidateAPIKey validates an API key for a given provider
func ValidateAPIKey(provider, key string) error {
	key = strings.TrimSpace(key)

	// Check if key is empty
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Check for only spaces or special characters that don't look like a key
	if isOnlySpaces(key) {
		return fmt.Errorf("API key cannot contain only spaces")
	}

	// Check minimum length (most API keys are at least 20 characters)
	if len(key) < 8 {
		return fmt.Errorf("API key is too short (minimum 8 characters)")
	}

	// Provider-specific validation
	switch strings.ToLower(provider) {
	case "anthropic":
		if !strings.HasPrefix(key, "sk-ant-") {
			return fmt.Errorf("Anthropic API key should start with 'sk-ant-'")
		}
		if len(key) < 15 {
			return fmt.Errorf("Anthropic API key appears to be invalid (too short)")
		}
		// Check for mostly letters only (should have mix of chars)
		if isMostlyLetters(key) {
			return fmt.Errorf("Anthropic API key appears to contain only letters - please verify")
		}

	case "openai":
		if !strings.HasPrefix(key, "sk-") {
			return fmt.Errorf("OpenAI API key should start with 'sk-'")
		}
		if len(key) < 20 {
			return fmt.Errorf("OpenAI API key appears to be invalid (too short)")
		}
		// Check for mostly letters only
		if isMostlyLetters(key) {
			return fmt.Errorf("OpenAI API key appears to contain only letters - please verify")
		}

	case "gemini":
		// Gemini keys are typically long alphanumeric strings, no strict prefix
		if len(key) < 30 {
			return fmt.Errorf("Gemini API key appears to be invalid (too short)")
		}
		// Check if it's just random letters
		if isOnlyLetters(key) {
			return fmt.Errorf("Gemini API key should not contain only letters")
		}
	}

	return nil
}

// isOnlySpaces checks if a string contains only whitespace
func isOnlySpaces(s string) bool {
	return strings.TrimSpace(s) == ""
}

// isOnlyLetters checks if a string contains only letters (no numbers, no special chars)
func isOnlyLetters(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// isMostlyLetters checks if a string is mostly letters (>80% letters)
func isMostlyLetters(s string) bool {
	if len(s) == 0 {
		return false
	}
	letterCount := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letterCount++
		}
	}
	percentage := float64(letterCount) / float64(len(s))
	return percentage > 0.8
}
