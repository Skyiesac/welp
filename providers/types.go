package providers

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// SystemContext holds system information for the prompt
type SystemContext struct {
	OS    string
	Shell string
	CWD   string
	// RecentCommands holds the last few commands from the shell history
	RecentCommands []RecentCommand
	// CurrentCommand is the command being analyzed (the one that produced the error)
	CurrentCommand string
	// CurrentOutput is the captured output of the current command (may be omitted for large install outputs)
	CurrentOutput string
}

// RecentCommand represents a historical command and an optional captured output
type RecentCommand struct {
	Command string
	Output  string
}

// Config stores API keys and the preferred provider
type Config struct {
	DefaultProvider string            `json:"default_provider,omitempty"`
	Providers       map[string]string `json:"providers"`
	ProviderModels  map[string]string `json:"provider_models,omitempty"`
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

func BuildPrompt(errorText, context string, sysCtx SystemContext) string {
	contextStr := ""
	if context != "" {
		contextStr = fmt.Sprintf("\nCONTEXT: %s", context)
	}

	return fmt.Sprintf(`OS: %s | Shell: %s | CWD: %s

COMMAND: %s
ERROR:
%s%s

RULES (non-negotiable):
- Never ask clarifying questions, no preamable , no note. Ever.
- Assume the most common/likely cause of this error. The "Why:" explanation MUST be a single sentence on exactly one line.
- Always give a concrete runnable command, even if guessing
- In next line, give a follow up concrete command in case first fails or as a follow up depending on the situation

FORMAT Template:
Fix:
	<command 1>
	<command 2 if needed>
Why: <strictly one line explanation>`,

		sysCtx.OS,
		sysCtx.Shell,
		sysCtx.CWD,
		sysCtx.CurrentCommand,
		sysCtx.CurrentOutput,
		contextStr,
	)
}

func formatRecentCommands(cmds []RecentCommand) string {
	if len(cmds) == 0 {
		return "none"
	}
	var parts []string
	for _, c := range cmds {
		if c.Output != "" {
			out := strings.ReplaceAll(c.Output, "\n", " ")
			if len(out) > 200 {
				out = out[:200] + "..."
			}
			parts = append(parts, fmt.Sprintf("%s => %s", c.Command, out))
		} else {
			parts = append(parts, c.Command)
		}
	}
	return strings.Join(parts, "; ")
}

// PrintWithColors prints text with colored "Fix:" and "Why:" keywords
func PrintWithColors(text string) {
	printer := NewColorPrinter()
	printer.Write(text)
	printer.Flush()
}

type ColorPrinter struct {
	buffer   string
	fixColor *color.Color
	whyColor *color.Color
}

func NewColorPrinter() *ColorPrinter {
	return &ColorPrinter{
		fixColor: color.New(color.FgHiGreen, color.Bold),
		whyColor: color.New(color.FgHiCyan, color.Bold),
	}
}

func (p *ColorPrinter) Write(text string) {
	p.buffer += text
	p.print(false)
}

func (p *ColorPrinter) Flush() {
	p.print(true)
}

func (p *ColorPrinter) print(flush bool) {
	for len(p.buffer) > 0 {
		if strings.HasPrefix(p.buffer, "Fix:") {
			p.fixColor.Print("Fix:")
			p.buffer = p.buffer[4:]
			continue
		}
		if strings.HasPrefix(p.buffer, "Why:") {
			p.whyColor.Print("Why:")
			p.buffer = p.buffer[4:]
			continue
		}

		fixIdx := strings.Index(p.buffer, "Fix:")
		whyIdx := strings.Index(p.buffer, "Why:")
		nextIdx := -1
		if fixIdx != -1 && (whyIdx == -1 || fixIdx < whyIdx) {
			nextIdx = fixIdx
		} else if whyIdx != -1 {
			nextIdx = whyIdx
		}

		if nextIdx == -1 {
			printLen := len(p.buffer)
			if !flush {
				printLen -= colorKeywordOverlap(p.buffer)
			}
			if printLen <= 0 {
				return
			}
			fmt.Print(p.buffer[:printLen])
			p.buffer = p.buffer[printLen:]
			continue
		}

		if nextIdx > 0 {
			fmt.Print(p.buffer[:nextIdx])
			p.buffer = p.buffer[nextIdx:]
		}
	}
}

func colorKeywordOverlap(text string) int {
	maxOverlap := 0
	for _, keyword := range []string{"Fix:", "Why:"} {
		for i := 1; i < len(keyword) && i <= len(text); i++ {
			if strings.HasSuffix(text, keyword[:i]) && i > maxOverlap {
				maxOverlap = i
			}
		}
	}
	return maxOverlap
}
