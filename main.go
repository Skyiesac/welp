package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"welp/providers"
)

const banner = `
╔══════════════════════════════════════╗
║          welp - CLI Tool           ║
║   Error Analysis and Fixing Utility  ║
╚══════════════════════════════════════╝
`

func hasEnvironmentKeys() bool {
	if os.Getenv("ANTHROPIC_API_KEY") != "" ||
		os.Getenv("OPENAI_API_KEY") != "" ||
		os.Getenv("GEMINI_API_KEY") != "" {
		return true
	}
	return len(DetectAvailableCredentials()) > 0
}

func isConfigured(config *providers.Config) bool {
	if config.DefaultProvider != "" || len(config.Providers) > 0 {
		return true
	}
	return len(DetectAvailableCredentials()) > 0
}

func main() {
	fmt.Print(banner)

	if len(os.Args) > 1 && os.Args[1] == "setup" {
		runSetup()
		os.Exit(0)
	}

	contextFlag := flag.String("context", "", "Additional context for error analysis")
	providerFlag := flag.String("provider", "", "AI provider to use")
	flag.Parse()

	// remaining args after flags = the command to run
	cmdArgs := flag.Args()

	config := loadConfig()
	detectedSources := DetectAvailableCredentials()

	if !isConfigured(config) {
		fmt.Println("No AI provider configured. Run: welp setup")
		os.Exit(1)
	}

	if len(detectedSources) > 0 {
		PrintDetectedCredentials(detectedSources)
	}

	// resolve provider
	if *providerFlag == "" {
		if os.Getenv("ANTHROPIC_API_KEY") != "" {
			*providerFlag = "anthropic"
		} else if os.Getenv("OPENAI_API_KEY") != "" {
			*providerFlag = "openai"
		} else if os.Getenv("GEMINI_API_KEY") != "" {
			*providerFlag = "gemini"
		} else if config.DefaultProvider != "" {
			*providerFlag = config.DefaultProvider
		} else if len(detectedSources) > 0 {
			*providerFlag = detectedSources[0].ProviderName
		} else {
			fmt.Fprintln(os.Stderr, "No AI provider configured. Run 'welp setup'.")
			os.Exit(1)
		}
	}

	var errorText string

	if len(cmdArgs) > 0 {
		// mode 1: welp <command> [args...]
		// run the command, capture stdout+stderr, use output as error text
		fmt.Printf("Running: %s\n\n", strings.Join(cmdArgs, " "))

		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdin = os.Stdin

		// capture both stdout and stderr
		out, _ := cmd.CombinedOutput()
		outputStr := strings.TrimSpace(string(out))

		// print the command output so user sees it
		if outputStr != "" {
			fmt.Println(outputStr)
			fmt.Println()
		}

		// only proceed if there was actual output to analyse
		if outputStr == "" {
			fmt.Println("Command produced no output.")
			os.Exit(0)
		}

		errorText = outputStr
	} else {
		// mode 2: <command> 2>&1 | welp  (legacy pipe mode)
		var err error
		errorText, err = readStdin()
		if err != nil || strings.TrimSpace(errorText) == "" {
			fmt.Fprintln(os.Stderr, "Usage:")
			fmt.Fprintln(os.Stderr, "  welp <command> [args...]     # recommended")
			fmt.Fprintln(os.Stderr, "  <command> 2>&1 | welp        # pipe mode")
			os.Exit(1)
		}
	}

	sysCtx := collectContextInParallel()

	provider, err := providers.CreateProvider(*providerFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := provider.ValidateAPIKeyWithConfig(config); err != nil {
		if len(detectedSources) > 1 {
			fmt.Fprintf(os.Stderr, "⚠️  %s failed: %v\n", provider.GetName(), err)
			for _, source := range detectedSources {
				if source.ProviderName == *providerFlag {
					continue
				}
				altProvider, err := providers.CreateProvider(source.ProviderName)
				if err != nil {
					continue
				}
				if err := altProvider.ValidateAPIKeyWithConfig(config); err == nil {
					fmt.Printf("✓ Using %s instead\n\n", source.Description)
					provider = altProvider
					goto providerValid
				}
			}
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "No working AI provider found. Run 'welp setup' to configure.")
		os.Exit(1)
	}

providerValid:
	if err := provider.StreamResponseWithConfig(errorText, *contextFlag, sysCtx, config); err != nil {
		fmt.Fprintf(os.Stderr, "Error calling %s API: %v\n", provider.GetName(), err)
		os.Exit(1)
	}

	os.Exit(0)
}