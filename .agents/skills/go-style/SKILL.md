---
name: go-style
description: Apply Uber's Go Style Guide when generating, editing, or reviewing Go code.
---

# Go Style

You are a Go coding assistant that strictly follows the Uber Go Style Guide when generating, modifying, or reviewing Go code. Produce idiomatic, safe, maintainable Go that adheres to these conventions unless the user explicitly overrides them.

## General Expectations

- Treat this skill as the authoritative style and correctness guide for Go code you produce.
- Prefer idiomatic Go (Effective Go, Go Common Mistakes, Go Code Review Comments) when consistent with these rules.
- Assume code must build clean under `go vet`, `goimports`, `revive`, `errcheck`, and `staticcheck`.
- When in doubt, choose the simpler, clearer, more consistent option.

## How to Apply

1. Identify which topic file(s) below cover the code you're touching, and read them before writing — don't rely on memory for anything beyond General Expectations.
2. Write code as if `goimports`, `go vet`, `revive`, `staticcheck`, and `errcheck` will run against it and must pass.
3. Within each topic file, prefer the "Good" pattern over the "Bad" one; don't introduce a pattern a topic file marks bad.
4. Deviate from a rule only if the user explicitly asks for a different style or constraint — state the deviation, don't apply it silently.

## Topics

Each file below covers a distinct area with runnable Good/Bad examples:

- [`references/interfaces-and-concurrency.md`](references/interfaces-and-concurrency.md) — interface compliance, pointer vs. value receivers, mutexes, goroutines, channels, defer, copying slices/maps at boundaries.
- [`references/errors-and-panics.md`](references/errors-and-panics.md) — choosing/wrapping/naming errors, handling errors once, type assertions, panics, `os.Exit`.
- [`references/data-and-state.md`](references/data-and-state.md) — enums with `iota`, `time.Time`/`time.Duration`, mutable globals, `init()`, atomic operations, struct field tags.
- [`references/style-and-structure.md`](references/style-and-structure.md) — line length, imports, naming, nesting, variable scope, struct/map initialization, embedding, predeclared-identifier shadowing.
- [`references/patterns-and-performance.md`](references/patterns-and-performance.md) — table-driven tests, functional options, printf-style functions, hot-path performance rules.

## Linting

Code should pass `goimports`, `errcheck`, `revive`, `go vet`, and `staticcheck` — run via `golangci-lint` where the project configures it.
