package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"io"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Skyiesac/welp/providers"
)

const banner = `
               .__          
__  _  __ ____ |  | ______  
\ \/ \/ // __ \|  | \____ \ 
 \     /\  ___/|  |_|  |_> >
  \/\_/  \___  >____/   __/ 
             \/     |__|    
`

var currentCommandMu sync.Mutex
var currentCommandProcess *os.Process
var interruptCount atomic.Int32
var sigChan = make(chan os.Signal, 8)
var interrupted = make(chan struct{})   // signals main that we were interrupted

func formatOutputWithBanner(timeStr, providerStr string) string {
	timeInfo := fmt.Sprintf("Time:  %s | Using: %s", timeStr, providerStr)
	const terminalWidth = 80
	const welpText = "WELP"
	spacing := terminalWidth - len(timeInfo) - len(welpText)
	if spacing < 5 {
		spacing = 5
	}

	return timeInfo + strings.Repeat("   ", spacing/3) + welpText
}

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
	installStrictKillHandler()
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
	var currentCommand string

	if len(cmdArgs) > 0 {
		currentCommand = strings.Join(cmdArgs, " ")
		var buf bytes.Buffer
		cmd := newTrackedCommand(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdin = nil
		cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &buf)

		runErr := runTrackedCommand(cmd)
		exitIfInterrupted(runErr)

		if runErr == nil {
			os.Exit(0)
		}

		errorText = strings.TrimSpace(buf.String())
		if errorText == "" {
			fmt.Fprintf(os.Stderr, "Command failed with no output.\n")
			os.Exit(1)
		}
	} else {
		currentCommand = "pipe input"
		var err error
		errorText, err = readStdin()
		if err != nil || strings.TrimSpace(errorText) == "" {
			fmt.Fprintln(os.Stderr, "Usage:")
			fmt.Fprintln(os.Stderr, "  welp <command> [args...]")
			fmt.Fprintln(os.Stderr, "  <command> 2>&1 | welp")
			os.Exit(1)
		}
	}
	startTime := time.Now()

	config := loadConfig()
	detectedSources := DetectAvailableCredentials()

	if !isConfigured(config) {
		fmt.Println("No AI provider configured. Run: welp setup")
		os.Exit(1)
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

	// populate current command and output for context, then persist the rolling log
	if currentCommand != "" {
		sysCtx.CurrentCommand = currentCommand
		if shouldStoreOutput(currentCommand, errorText) {
			sysCtx.CurrentOutput = errorText
		} else {
			sysCtx.CurrentOutput = ""
		}

		sysCtx.RecentCommands = appendRecentCommand(sysCtx.RecentCommands, providers.RecentCommand{
			Command: sysCtx.CurrentCommand,
			Output:  sysCtx.CurrentOutput,
		})
		saveRecentCommands(sysCtx.RecentCommands)
	}

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
	streamDone := make(chan error, 1)
	go func() { streamDone <- provider.StreamResponseWithConfig(errorText, *contextFlag, sysCtx, config) }()
	select {
	case err := <-streamDone:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error calling %s API: %v\n", provider.GetName(), err)
			os.Exit(1)
		}
	case <-interrupted:
		fmt.Println("\ninterrupted.")
		os.Exit(130)
	}

	fmt.Print("\n" + formatOutputWithBanner(time.Since(startTime).Round(time.Millisecond).String(), provider.GetName()))
	os.Exit(0)
}

func installStrictKillHandler() {

	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	 go func() {
        for sig := range sigChan {
            count := interruptCount.Add(1)
            if count == 1 {
                killCurrentCommand()
                close(interrupted) // notify main
                if sig == syscall.SIGTERM {
                    os.Exit(143)
                }
                // don't os.Exit here — let main handle it cleanly
            } else {
                forceKillSelf() // second Ctrl+C: hard kill
            }
        }
	}()
}

func forceKillSelf() {
	if runtime.GOOS == "windows" {
		os.Exit(130)
	}
	_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
	os.Exit(130)
}

func newTrackedCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

func runTrackedCommand(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	trackCommandProcess(cmd.Process)
	err := cmd.Wait()
	clearCommandProcess(cmd.Process)
	return err
}

func combinedOutputTrackedCommand(cmd *exec.Cmd) ([]byte, error) {
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := runTrackedCommand(cmd)
	return output.Bytes(), err
}

func trackCommandProcess(process *os.Process) {
	currentCommandMu.Lock()
	currentCommandProcess = process
	currentCommandMu.Unlock()
}

func clearCommandProcess(process *os.Process) {
	currentCommandMu.Lock()
	if currentCommandProcess == process {
		currentCommandProcess = nil
	}
	currentCommandMu.Unlock()
}

func killCurrentCommand() {
	currentCommandMu.Lock()
	process := currentCommandProcess
	currentCommandMu.Unlock()
	if process == nil {
		return
	}

	_ = syscall.Kill(-process.Pid, syscall.SIGKILL)
	_ = process.Kill()
}

func exitIfInterrupted(err error) {
	if !wasInterrupted(err) {
		return
	}
	killCurrentCommand()
	os.Exit(130)
}

func wasInterrupted(err error) bool {
	if err == nil {
		return false
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}
	return status.Signaled() && (status.Signal() == syscall.SIGINT || status.Signal() == syscall.SIGTERM)
}
