package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		fmt.Print(banner)
		runSetup()
		os.Exit(0)
	}

	contextFlag := flag.String("context", "", "Additional context for error analysis")
	providerFlag := flag.String("provider", "", "AI provider to use")
	flag.Parse()

	cmdArgs := flag.Args()

	var errorText string

	if len(cmdArgs) > 0 {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdin = nil
		cmd.Stdout = os.Stdout // stream output live to terminal
		cmd.Stderr = os.Stderr // stream stderr live to terminal

		runErr := cmd.Run()

		// command succeeded — behave as if welp wasn't there
		if runErr == nil {
			os.Exit(0)
		}

		// command failed — now capture output for AI
		// re-run silently to capture combined output for AI context
		cmd2 := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd2.Stdin = nil
		out, _ := cmd2.CombinedOutput()
		errorText = strings.TrimSpace(string(out))

		if errorText == "" {
			fmt.Fprintf(os.Stderr, "Command failed with no output.\n")
			os.Exit(1)
		}
	} else {
		// pipe mode — only triggers if stdin has content
		var err error
		errorText, err = readStdin()
		if err != nil || strings.TrimSpace(errorText) == "" {
			fmt.Fprintln(os.Stderr, "Usage:")
			fmt.Fprintln(os.Stderr, "  welp <command> [args...]")
			fmt.Fprintln(os.Stderr, "  <command> 2>&1 | welp")
			os.Exit(1)
		}
	}

	// only reach here if something went wrong — now show welp UI
	fmt.Print(banner)
	startTime := time.Now()

	config := loadConfig()
	detectedSources := DetectAvailableCredentials()

	if !isConfigured(config) {
		fmt.Println("No AI provider configured. Run: welp setup")
		os.Exit(1)
	}

	if len(detectedSources) > 0 {
		PrintDetectedCredentials(detectedSources)
	}

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
				altProvider, aerr := providers.CreateProvider(source.ProviderName)
				if aerr != nil {
					continue
				}
				if aerr := altProvider.ValidateAPIKeyWithConfig(config); aerr == nil {
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
		fmt.Fprintf(os.Stderr, "⏱️  Failed after %v\n", time.Since(startTime).Round(time.Millisecond))
		os.Exit(1)
	}

	fmt.Printf("\n⏱️  %v\n", time.Since(startTime).Round(time.Millisecond))
	os.Exit(0)
}