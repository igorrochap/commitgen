# commit-generator

A CLI tool that uses a local LLM via [Ollama](https://ollama.com) to generate [Conventional Commits](https://www.conventionalcommits.org) messages from your staged changes.

## How it works

1. Detects your staged changes (or stages all changes if nothing is staged yet)
2. Sends the diff to an Ollama model with a prompt that enforces the Conventional Commits format
3. Presents the generated message for you to **accept**, **edit**, or **regenerate**

## Requirements

- [Go](https://go.dev) 1.21+
- [Git](https://git-scm.com)
- [Ollama](https://ollama.com) running locally with at least one model pulled

## Installation

```bash
git clone https://github.com/igorrochap/commit-generator.git
cd commit-generator
./install.sh
```

This builds the binary and installs it to `/usr/local/bin/commitgen`, making it available anywhere on your machine.

### Alternative: install with `go install`

If you have Go installed, you can skip cloning the repo entirely:

```bash
go install github.com/igorrochap/commit-generator@latest
```

This drops the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`). Make sure that directory is on your `$PATH`.

## Updating

Once installed, you can upgrade to the latest version at any time with:

```bash
commitgen update
```

Under the hood this runs `go install github.com/igorrochap/commit-generator@latest`, so the `go` toolchain must be on your `$PATH`.

If you originally installed via `./install.sh` (to `/usr/local/bin`), the fresh binary lands in `$(go env GOPATH)/bin`. `commitgen update` will tell you the exact command to copy it over the system binary.

## Usage

Inside any git repository, run:

```bash
commitgen [flags]
```

If there are no staged changes, all current changes are staged automatically before generating the commit.

### Flags

| Flag         | Default        | Description                              |
|--------------|----------------|------------------------------------------|
| `--language` | `en`           | Language for the commit message          |
| `--model`    | `glm-5:cloud`  | Ollama model to use for generation       |

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

# Generate the commit message in Brazilian Portuguese
commitgen --language pt-BR

# Combine both flags
commitgen --model llama3.2 --language pt-BR
```

## Interactive selection

After the message is generated, you are prompted to choose an action:

- **Accept** — commits immediately with the generated message
- **Edit** — opens the message in your `$EDITOR` (falls back to `nano`, then `vim`) so you can tweak it before committing
- **Regenerate** — discards the message and generates a new one
