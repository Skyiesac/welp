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

After installation, `welp` adds a shell hook to your `.zshrc` or `.bashrc`. From that point on, every time a command exits with a non-zero status, the hook captures the error output and pipes it to `welp`, which forwards it to your configured AI provider and streams the explanation back to your terminal.

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

**Install to `~/.local/bin` and add shell integration:**

```bash
./install.sh
```

This does three things:

1. Builds the binary and copies it to `~/.local/bin/welp`
2. Detects your shell (bash or zsh) and appends the error hook to your shell config
3. Prints next steps

> Do not run `install.sh` with `sudo`. The binary is installed to your user's local bin, not a system path.

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

---

## Shell integration

The `install.sh` script adds the following hook to your `.zshrc` or `.bashrc`:

```bash
# welp shell integration
__welp_handler() {
  local exit_code=$?
  if [ $exit_code -ne 0 ]; then
    local cmd="${BASH_COMMAND}"
    if [[ ! "$cmd" =~ ^welp|^echo|^read ]]; then
      echo "Command failed: $cmd"
      echo "Analyzing error..."
      sleep 0.5
    fi
  fi
  return $exit_code
}

trap '__welp_handler' ERR
```

Once this is active, any command that fails will trigger `welp` automatically. You do not need to prepend `welp` to anything or change how you work.

If you ever want to remove the integration, delete the block between `# welp shell integration` and the `trap` line from your shell config file.

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

---

## License

MIT
