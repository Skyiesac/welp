package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Skyiesac/welp/providers"
	"github.com/manifoldco/promptui"
)

func printSetupInstructions() {
	fmt.Println("\n⚠️  No working AI provider configured.")
	fmt.Println("Set up one of the following, then run `welp setup` again:")
	fmt.Println("  1. GitHub Copilot: run `gh auth login` or export GH_TOKEN")
	fmt.Println("  2. Ollama (local): https://ollama.ai")
	fmt.Println("  3. LocalAI (local): https://localai.io")
	fmt.Println("  4. Claude Desktop: https://claude.ai/")
	fmt.Println("  5. API keys for Anthropic, OpenAI, or Gemini")
}

func finalizeSetup(config *providers.Config, providerName string) bool {
	provider, err := providers.CreateProvider(providerName)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return false
	}
	if err := provider.ValidateAPIKeyWithConfig(config); err != nil {
		fmt.Printf("\n⚠️  %s is not ready: %v\n", provider.GetName(), err)
		printSetupInstructions()
		return false
	}

	config.DefaultProvider = providerName
	if err := saveConfig(config); err != nil {
		fmt.Printf("Error saving configuration: %v\n", err)
		return false
	}

	fmt.Printf("\n✓ Default provider set to %s\n", providerName)
	fmt.Printf("✓ Configuration saved to %s\n", getConfigPath())
	fmt.Println("\n✓ Setup complete! You can now use welp.")
	return true
}

// runSetup handles the interactive setup process
func runSetup() {
	config := loadConfig()
	if config.Providers == nil {
		config.Providers = make(map[string]string)
	}

	// fmt.Println("\n=== welp Setup ===\n")
	if _, err := os.Executable(); err == nil {
		fmt.Println("\n(Run `go build -o welp` and copy it to ~/.local/bin/welp to update your installed binary)")
		fmt.Println()
	}
	fmt.Println("Scanning for available AI tools...")
	fmt.Println()

	selectedProvider := ""

	// 1. Check for GitHub Copilot first (most common)
	fmt.Println("🔍 Checking for GitHub Copilot...")
	if providers.IsGitHubCopilotAvailable() {
		fmt.Println("✓ GitHub Copilot API configuration detected!")
		if promptYesNo("  Use GitHub Copilot for welp? (y/n): ") {
			selectedProvider = "copilot"
			// Ask for model selection for copilot
			if config.ProviderModels == nil {
				config.ProviderModels = make(map[string]string)
			}
			model := promptCopilotModel()
			if model != "" {
				config.ProviderModels["copilot"] = model
			}
		}
	} else {
		fmt.Println("✗ GitHub Copilot API configuration not found")
		fmt.Println("  Authenticate first:")
		fmt.Println("    gh auth login")
		fmt.Println("  Or export a token:")
		fmt.Println("    export GH_TOKEN=<your-github-token>")
		fmt.Println()
	}

	// 2. Check for Claude Desktop
	if selectedProvider == "" {
		fmt.Println("🔍 Checking for Claude Desktop...")
		if isClaudeDesktopFound() {
			fmt.Println("✓ Claude Desktop detected!")
			if promptYesNo("  Use Claude Desktop for welp? (y/n): ") {
				selectedProvider = "claude-desktop"
			}
		} else {
			fmt.Println("✗ Claude Desktop not found")
			fmt.Println("  Install from: https://claude.ai/")
			fmt.Println()
		}
	}

	// 3. Check for LocalAI
	if selectedProvider == "" {
		fmt.Println("🔍 Checking for LocalAI...")
		if isLocalAIFound() {
			fmt.Println("✓ LocalAI detected (running on localhost:8080)!")
			if promptYesNo("  Use LocalAI for welp? (y/n): ") {
				selectedProvider = "localai"
			}
		} else {
			fmt.Println("✗ LocalAI not found")
			fmt.Println("  Install from: https://localai.io/")
			fmt.Println()
		}
	}

	// 4. Check for Ollama
	if selectedProvider == "" {
		fmt.Println("🔍 Checking for Ollama...")
		if isOllamaFound() {
			fmt.Println("✓ Ollama detected (running on localhost:11434)!")
			if promptYesNo("  Use Ollama for welp? (y/n): ") {
				selectedProvider = "ollama"
				// Ask for model selection for Ollama
				if config.ProviderModels == nil {
					config.ProviderModels = make(map[string]string)
				}
				model := promptOllamaModel()
				if model != "" {
					config.ProviderModels["ollama"] = model
				}
			}
		} else {
			fmt.Println("✗ Ollama not found")
			fmt.Println("  Install from: https://ollama.ai/")
			fmt.Println()
		}
	}

	if selectedProvider != "" {
		finalizeSetup(config, selectedProvider)
		return
	}

	// 5. Ask for API keys if no local provider was selected
	fmt.Println("🔑 No local AI source selected. Configuring API keys...")
	fmt.Println()

	apiProviders := []string{"anthropic", "openai", "gemini"}
	addedAny := false
	firstAdded := ""

	for _, provider := range apiProviders {
		for {
			fmt.Printf("Enter API key for %s (press Enter to skip): ", strings.ToUpper(provider))
			key := readInput()

			if key == "" {
				break
			}

			// Validate the API key
			if err := ValidateAPIKey(provider, key); err != nil {
				fmt.Printf("❌ Invalid API key: %v\n", err)
				fmt.Printf("   Please try again or press Enter to skip.\n")
				continue
			}

			// Valid key - save it
			config.Providers[provider] = key
			fmt.Printf("✓ %s API key saved\n", strings.ToUpper(provider))
			if !addedAny {
				firstAdded = provider
			}
			addedAny = true
			break
		}
	}

	if !addedAny {
		printSetupInstructions()
		return
	}

	finalizeSetup(config, firstAdded)
}

// readInput reads a line from stdin (for setup)
func readInput() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

// promptYesNo asks the user a yes/no question
func promptYesNo(question string) bool {
	fmt.Print(question)
	response := readInput()
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

// isClaudeDesktopFound checks if Claude Desktop CLI is available
func isClaudeDesktopFound() bool {
	_, err := exec.LookPath("claude")
	if err == nil {
		return true
	}

	// Check common installation paths
	home, _ := os.UserHomeDir()
	paths := []string{
		"claude",
		"/Applications/Claude.app/Contents/MacOS/claude",
		home + "/.local/share/claude-desktop/bin/claude",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// isLocalAIFound checks if LocalAI is running
func isLocalAIFound() bool {
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

// isOllamaFound checks if Ollama is running
func isOllamaFound() bool {
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

// promptCopilotModel asks the user to select a model with arrow key UI
func promptCopilotModel() string {
	fmt.Println("\n  Fetching available models from GitHub Models API...")
	models, err := providers.FetchAvailableCopilotModels()
	if err != nil {
		fmt.Printf("  ⚠️  Could not fetch models: %v\n", err)
		fmt.Println("  Using default model: openai/gpt-4o")
		return "openai/gpt-4o"
	}

	if len(models) == 0 {
		fmt.Println("  ⚠️  No models found")
		return "openai/gpt-4o"
	}

	// Find default model index (gpt-4o or first model)
	defaultIdx := 0
	for i, m := range models {
		if strings.Contains(m, "gpt-4o") && !strings.Contains(m, "gpt-4-turbo") {
			defaultIdx = i
			break
		}
	}

	// Use promptui for arrow-key selection
	prompt := promptui.Select{
		Label: "Select GitHub Models (use ↑↓ arrow keys, press Enter to select)",
		Items: models,
		Size:  10,
	}

	// Set cursor position to default
	prompt.CursorPos = defaultIdx

	idx, _, err := prompt.Run()

	// Handle cancellation (Ctrl+C)
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("  ⚠️  Selection cancelled, using default model: openai/gpt-4o")
			return "openai/gpt-4o"
		}
		fmt.Printf("  ⚠️  Selection error: %v\n", err)
		fmt.Println("  Using default model: openai/gpt-4o")
		return "openai/gpt-4o"
	}

	// Validate index
	if idx < 0 || idx >= len(models) {
		fmt.Printf("  ⚠️  Invalid selection, using default model: openai/gpt-4o\n")
		return "openai/gpt-4o"
	}

	selected := models[idx]
	fmt.Printf("  ✓ Model selected: %s\n", selected)
	return selected
}

// promptOllamaModel asks the user to select an Ollama model with arrow key UI
func promptOllamaModel() string {
	fmt.Println("\n  Fetching available Ollama models...")
	models, err := providers.GetAllAvailableOllamaModels()
	if err != nil {
		fmt.Printf("  ⚠️  Could not fetch models: %v\n", err)
		fmt.Println("  Using first available model")
		return ""
	}

	if len(models) == 0 {
		fmt.Println("  ⚠️  No models found")
		return ""
	}

	if len(models) == 1 {
		fmt.Printf("  ✓ Only one model available: %s\n", models[0])
		return models[0]
	}

	// Use promptui for arrow-key selection
	prompt := promptui.Select{
		Label: "Select Ollama model (use ↑↓ arrow keys, press Enter to select)",
		Items: models,
		Size:  10,
	}

	idx, _, err := prompt.Run()

	// Handle cancellation (Ctrl+C)
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("  ⚠️  Selection cancelled, using first available model")
			return models[0]
		}
		fmt.Printf("  ⚠️  Selection error: %v\n", err)
		fmt.Println("  Using first available model")
		return models[0]
	}

	// Validate index
	if idx < 0 || idx >= len(models) {
		fmt.Printf("  ⚠️  Invalid selection, using first available model\n")
		return models[0]
	}

	selected := models[idx]
	fmt.Printf("  ✓ Model selected: %s\n", selected)
	return selected
}
