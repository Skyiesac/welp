# welp

```
               .__          
__  _  __ ____ |  | ______  
\ \/ \/ // __ \|  | \____ \ 
 \     /\  ___/|  |_|  |_> >
  \/\_/  \___  >____/   __/ 
             \/     |__|    
```

`welp` is a CLI tool written in Go that automatically explains terminal errors using AI. It hooks into your shell so that whenever a command fails, the error is analyzed on the spot, no extra typing required.

---

## How it works

After setup, `welp` can be wired into your shell through your `.zshrc` or `.bashrc`. From that point on, failed commands can be sent to `welp`, which forwards the error to your configured AI provider and streams the explanation back to your terminal.

You run your commands exactly as you always would. `welp` only speaks up when something breaks.

For one-off use or piping manually, `welp` also works as a direct command — see [Manual usage](#manual-usage) below.

---

## Requirements

- Go 1.21+
- At least one of the following:
  - An API key for Anthropic, OpenAI, or Gemini
  - GitHub Copilot (via `gh auth login` or `GH_TOKEN`)
  - Ollama running locally (`localhost:11434`)
  - LocalAI running locally (`localhost:8080`)
  - Claude CLI

---

## Installation

**Clone and build:**

```bash
git clone https://github.com/Skyiesac/welp.git
cd welp
go build -o welp
```

**Install to `~/.local/bin`:**

```bash
mkdir -p ~/.local/bin
cp welp ~/.local/bin/welp
```

Make sure `~/.local/bin` is in your `PATH`. If it isn't, add this to your shell config:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then reload your shell:

```bash
source ~/.zshrc   # or ~/.bashrc
```

---

## Setup

After installing, run the interactive setup to configure your AI provider:

```bash
welp setup
```

This scans for available providers in the following order and prompts you to confirm each:

1. GitHub Copilot (via `gh auth` or `GH_TOKEN`)
2. Claude Desktop
3. LocalAI (localhost:8080)
4. Ollama (localhost:11434)
5. API keys for Anthropic, OpenAI, or Gemini

For Ollama and GitHub Copilot, `welp setup` will fetch available models and let you pick one using arrow keys.

For API key providers, you can also export keys as environment variables instead of going through setup:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
export GEMINI_API_KEY="AIza..."
```

`welp` detects these at runtime automatically.

Add the shell integration for your shell:

**Zsh** (`~/.zshrc`):

```zsh
__welp_accept_line() { [[ -n "$BUFFER" && "$BUFFER" != welp* ]] && BUFFER="welp $BUFFER"; zle .accept-line }
zle -N accept-line __welp_accept_line
```

**Bash** (`~/.bashrc`):

```bash
__welp_err_trap() { local status=$?; [[ $status -eq 130 || $status -eq 143 ]] && return $status; echo "$BASH_COMMAND" | timeout --foreground 60 welp 2>/dev/null || true; }
trap __welp_err_trap ERR
```

Then reload your shell:

```bash
source ~/.zshrc   # or ~/.bashrc
```

---

## Shell integration

Use the hook for your shell from [Setup](#setup). Once it is active, failed commands will trigger `welp` automatically.

If you ever want to remove the integration, delete the hook from your shell config file.

---

## Manual usage

You can also invoke `welp` directly when you want to analyze a specific error or pipe output manually.

**Pipe a command's output:**

```bash
pip install nonexistent-package 2>&1 | welp
go build ./... 2>&1 | welp
```

**Add extra context:**

```bash
go build ./... 2>&1 | welp --context "just added a new dependency, getting a linker error"
```

The `--context` flag passes additional information to the AI alongside the error, which can improve the quality of the explanation.

**Force a specific provider:**

```bash
go build ./... 2>&1 | welp --provider gemini
```

---

## Supported providers

| Provider | How to configure |
|---|---|
| Anthropic (Claude) | `ANTHROPIC_API_KEY` env var or `welp setup` |
| OpenAI | `OPENAI_API_KEY` env var or `welp setup` |
| Google Gemini | `GEMINI_API_KEY` env var or `welp setup` |
| GitHub Copilot | `gh auth login` or `GH_TOKEN` env var |
| Ollama | Run Ollama locally on port 11434 |
| LocalAI | Run LocalAI locally on port 8080 |

If multiple providers are configured, `welp` uses this priority order: `ANTHROPIC_API_KEY` → `OPENAI_API_KEY` → `GEMINI_API_KEY` → config default → auto-detected credentials. If the primary provider fails validation, `welp` automatically tries the next available one.

---

## Flags

| Flag | Description |
|---|---|
| `--context <string>` | Additional context to pass to the AI alongside the error output |
| `--provider <name>` | Force a specific provider: `anthropic`, `openai`, `gemini`, `copilot`, `ollama`, `localai` |

---

## Configuration

Config is saved to `~/.config/welp/config.json`. You generally don't need to edit it directly — `welp setup` handles everything. To switch providers or reconfigure, run `welp setup` again.

---

## Contributing

Pull requests are welcome. To add a new provider, implement the provider interface in the `providers/` directory. The existing implementations (Anthropic, OpenAI, Gemini, Copilot, Ollama, LocalAI) are good references for the expected shape.

Give it a ⭐ if you like it :)
---
