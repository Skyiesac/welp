package main

import (
	"bufio"
	"io"
	"os"

	"runtime"
	"strings"
	"sync"

	"welp/providers"
)

// collectContextInParallel gathers OS, shell, cwd, and history concurrently
func collectContextInParallel() providers.SystemContext {
	var wg sync.WaitGroup
	sysCtx := providers.SystemContext{}

	wg.Add(4)

	go func() {
		defer wg.Done()
		sysCtx.OS = runtime.GOOS
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
		sysCtx.HistoryCmds = getLastCommands()
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

func getLastCommands() []string {
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
