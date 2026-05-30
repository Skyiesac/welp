package providers

import (
	"fmt"
	"strings"
)

// SystemContext holds system information for the prompt
type SystemContext struct {
	OS          string
	Shell       string
	CWD         string
	HistoryCmds []string
}

// Config stores API keys and the preferred provider
type Config struct {
	DefaultProvider string            `json:"default_provider,omitempty"`
	Providers       map[string]string `json:"providers"`
}

// AIProvider is the interface that all AI providers must implement
type AIProvider interface {
	// GetName returns the provider name
	GetName() string
	// ValidateAPIKey checks if the API key is set
	ValidateAPIKey() error
	// ValidateAPIKeyWithConfig checks if the API key is set (using config)
	ValidateAPIKeyWithConfig(config *Config) error
	// StreamResponse streams the AI response to stdout
	StreamResponse(errorText, context string, sysCtx SystemContext) error
	// StreamResponseWithConfig streams the AI response (using config)
	StreamResponseWithConfig(errorText, context string, sysCtx SystemContext, config *Config) error
}

// CreateProvider creates the appropriate AI provider based on the provider name
func CreateProvider(providerName string) (AIProvider, error) {
	switch strings.ToLower(providerName) {
	case "anthropic":
		return &AnthropicProvider{}, nil
	case "openai":
		return &OpenAIProvider{}, nil
	case "gemini":
		return &GeminiProvider{}, nil
	case "ollama":
		return &OllamaProvider{}, nil
	case "localai":
		return &LocalAIProvider{URL: "http://localhost:8080"}, nil
	case "claude-desktop":
		return &ClaudeDesktopProvider{}, nil
	case "copilot":
		return &CopilotProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: anthropic, openai, gemini, ollama, localai, claude-desktop, copilot)", providerName)
	}
}
// func BuildPrompt(errorText, context string, sysCtx SystemContext) string {
//     return fmt.Sprintf(`You are an expert terminal error fixer. A developer got this error and needs an immediate fix.

// RULES (non-negotiable):
// - Never ask clarifying questions. Ever.
// - If you are uncertain, give the 2-3 most likely fixes ranked by probability
// - Assume the most common/likely cause of this error
// - Always give a concrete runnable command, even if guessing
// - Format: one line root cause → fix command(s) → one line why

// RESPONSE FORMAT:
// Cause: <what went wrong in one line>
// Fix:
//   <command 1>
//   <command 2 if needed>
// Why: <one line explanation>`,
//         sysCtx.OS,
//         sysCtx.Shell,
//         sysCtx.CWD,
//         "unknown",
//         formatRecentCommands(sysCtx.HistoryCmds),
//         errorText,
//         buildContextBlock(context),
//     )
// }
func BuildPrompt(errorText, context string, sysCtx SystemContext) string {
	return fmt.Sprintf(`OS: %s | Shell: %s | CWD: %s | Recent: %s

ERROR:
%s
%s

RULES (non-negotiable):
- Never ask clarifying questions. Ever.
- Assume the most common/likely cause of this error
- Always give a concrete runnable command, even if guessing
-In next line,  give a follow up concrete command in case first fails or as a follow up depending on the situation

FORMAT:
Fix:
  <command 1>
  <command 2 if needed>
Why: <one line explanation>`,

		sysCtx.OS,
		sysCtx.Shell,
		sysCtx.CWD,
		formatRecentCommands(sysCtx.HistoryCmds),
		errorText,
		buildContextBlock(context),
	)
}
func formatRecentCommands(cmds []string) string {
	if len(cmds) == 0 {
		return "none"
	}
	return strings.Join(cmds, "; ")
}

func buildContextBlock(context string) string {
    if context == "" {
        return "EXTRA CONTEXT: none — make reasonable assumptions based on the error alone"
    }
    return fmt.Sprintf("EXTRA CONTEXT: %s", context)
}