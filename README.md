# commitgen

A CLI tool that uses an LLM provider to generate [Conventional Commits](https://www.conventionalcommits.org) messages from your staged changes. It supports local generation with [Ollama](https://ollama.com) by default, plus hosted providers such as OpenAI, Anthropic Claude, and Google Gemini.

## How it works

1. Detects your staged changes
2. Sends the diff to the selected LLM provider with a prompt that enforces the Conventional Commits format
3. Presents the generated message for you to **accept**, **edit**, or **regenerate**

## Requirements

- [Go](https://go.dev) 1.21+
- [Git](https://git-scm.com)
- [Ollama](https://ollama.com) running locally with at least one model pulled, or an API key for a hosted provider

## Quality gates

Run `scripts/quality.sh` to run all gates, or `scripts/quality.sh <gate>` to run one.

- `format`, `style`, and `complexity` require golangci-lint v2.13.1. The `format` gate uses its `gofmt` and `goimports` formatters. CI installs this exact version as a prebuilt binary with the official golangci-lint GitHub Action.
- `vet` runs `go vet ./...` and requires Go 1.25.2+.
- `style` runs `errcheck`, `staticcheck`, `revive`, `gocritic`, `errorlint`, `unused`, and `ineffassign` on changed Go files.
- `complexity` runs `gocyclo` (limit 10) and `gocognit` (limit 15) on changed Go files, including tests. A function at a limit passes; only a function above it fails.
- Both diff gates use the merge base with the selected base ref and report that ref plus the changed Go file count. Set `QUALITY_BASE_REF` to select a ref explicitly. Otherwise, `GITHUB_BASE_REF` tries `origin/<name>` and then `<name>`; local runs default to `origin/main` and then `main`. An explicitly supplied ref that does not exist is an error, with no fallback.
- `coverage` runs `go test -count=1 -coverpkg=./... -coverprofile=.cache/coverage.out ./...` to measure fresh statement coverage across the whole module, including packages without test files, and enforces a 70% floor.
- `tests` runs `go test ./...` and requires Go 1.25.2+.
- `architecture` runs the Go AST checker in `scripts/architecture` and checks imports across all Go source and test files:
  - R1: Packages under `internal/` cannot import `cmd`; internal packages stay independent from CLI wiring.
  - R2: `internal/config`, `internal/prompts`, `internal/loading`, `internal/selection`, and `internal/updatecheck` cannot import other module packages; these foundational packages stay reusable.
  - R3: `internal/generator` can import only `internal/loading`, `internal/prompts`, and `internal/selection`; generation stays within its declared collaborators.
  - R4: Only `cmd` can import `github.com/spf13/cobra`; Cobra-specific wiring stays at the CLI boundary.
  - R5: Only `internal/loading` and `internal/selection` can import `github.com/charmbracelet/*`; terminal UI dependencies stay behind packages that can isolate TTY behavior.
  - R6: The root `main` package can import only `cmd` from this module; the executable entry point stays a small delegate.

## Installation

```bash
go install github.com/igorrochap/commitgen@latest
```

This drops the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`). Make sure that directory is on your `$PATH`.

### Alternative: build from source

```bash
git clone https://github.com/igorrochap/commitgen.git
cd commitgen
./install.sh
```

This builds the binary and installs it to `/usr/local/bin/commitgen`.

## Updating

Once installed, you can upgrade to the latest version at any time with:

```bash
commitgen update
```

When a newer release is available, `commitgen` shows a short notice before running your command.

Under the hood this runs `go install github.com/igorrochap/commitgen@latest`, so the `go` toolchain must be on your `$PATH`.

If you originally installed via `./install.sh` (to `/usr/local/bin`), the fresh binary lands in `$(go env GOPATH)/bin`. `commitgen update` will tell you the exact command to copy it over the system binary.

## Usage

Inside any git repository, run:

```bash
commitgen [flags]
```

Stage the files you want to commit before running `commitgen`.

Show the installed version with:

```bash
commitgen version
commitgen --version
commitgen -v
```

### Flags

| Flag         | Default        | Description                              |
|--------------|----------------|------------------------------------------|
| `-v`, `--version` |              | Show commitgen version                   |
| `--context`  |                | Additional context for generation        |
| `--language` | `en`           | Language for the commit message          |
| `--model`    | `gemma4:31b-cloud`  | Model to use for generation       |
| `--provider` | `ollama`       | LLM provider: `ollama`, `openai`, `anthropic`, or `gemini` |

### Persistent defaults

Save your preferred language, provider, model, or API key once with:

```bash
commitgen config set --language pt-BR
commitgen config set --model llama3.2
commitgen config set --language pt-BR --model llama3.2
commitgen config set --provider openai --model gpt-5.5 --api-key "$OPENAI_API_KEY"
commitgen config set --provider anthropic --model claude-sonnet-4-5 --api-key "$ANTHROPIC_API_KEY"
commitgen config set --provider gemini --model gemini-3.5-flash --api-key "$GEMINI_API_KEY"
```

Show the current defaults with:

```bash
commitgen config show
```

Saved defaults apply to future `commitgen` runs. One-off flags still take precedence:

```bash
commitgen --language en
```

### Supported languages

| Value   | Language            |
|---------|---------------------|
| `en`    | English (default)   |
| `pt-BR` | Brazilian Portuguese|

### Examples

```bash
# Generate a commit in English using the default model
commitgen

# Use a different Ollama model
commitgen --model llama3.2

# Use OpenAI after configuring its API key
commitgen --provider openai --model gpt-5.5

# Use Anthropic Claude after configuring its API key
commitgen --provider anthropic --model claude-sonnet-4-5

# Use Google Gemini after configuring its API key
commitgen --provider gemini --model gemini-3.5-flash

# Generate the commit message in Brazilian Portuguese
commitgen --language pt-BR

# Provide additional context for the generated commit message
commitgen --context "fix ci failure"

# Combine flags
commitgen --model llama3.2 --language pt-BR
```

Generate a Pull Request title and description with additional context:

```bash
commitgen pr --context "closes the issue #15 on github"
```

## Interactive selection

After the message is generated, you are prompted to choose an action:

- **Accept** — commits immediately with the generated message
- **Edit** — opens the message in your `$EDITOR` (falls back to `nano`, then `vim`) so you can tweak it before committing
- **Regenerate** — discards the message and generates a new one
