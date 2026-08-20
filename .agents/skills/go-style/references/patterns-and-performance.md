# Performance, Printf Functions, and Test/Option Patterns

## Performance Guidelines

Apply these only to code on hot paths or where profiling indicates a performance bottleneck.

### `strconv` vs `fmt`

- Use `strconv` for primitive–string conversions instead of `fmt`, as it is faster and allocates less.

### Avoid Repeated String–Byte Conversions

- Do not repeatedly convert the same string to `[]byte` in a loop; do the conversion once and reuse the slice.

### Container Capacity

- When creating maps using `make(map[K]V, n)`, pass an approximate capacity hint when you know how many elements you will add to reduce reallocation.
- When creating slices that will be appended to, use `make([]T, 0, n)` to preallocate capacity when you know (or can estimate) the size.

## Printf‑style Functions

### Format Strings

- When storing format strings used with `Printf`‑style functions, declare them as `const` so that `go vet` can analyze them.

### Naming Conventions

- Prefer standard `Printf`‑family names (`Printf`, `Sprintf`, etc.) so `go vet` recognizes them automatically.
- If you must use custom names, end them with `f` (e.g., `Wrapf`, `Statusf`) and configure `go vet` with `-printfuncs` to enable checking.

## Patterns

### Test Tables

- Use table‑driven tests (slices of structs with `name`, `give`, `want` fields, etc.) to organize related test cases.
- You may omit field names in test table literals when there are three or fewer fields and the meaning is obvious.

### Functional Options

- Prefer functional option patterns for constructing complex objects with many optional configuration parameters.
- Use an unexported `options` struct to hold configuration, and an exported `Option` interface with an `apply(*options)` method implemented by option types.
- Implement helpers like `WithCache(bool)` or `WithLogger(*zap.Logger)` returning types that satisfy `Option` and record into the `options` struct.
- In constructors (e.g., `Open(addr string, opts ...Option)`), initialize default options, then range over `opts` calling `o.apply(&options)`.
