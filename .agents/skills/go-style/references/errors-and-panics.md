# Errors, Type Assertions, and Panics

## Error Handling

### Choosing Error Forms

- Use `errors.New` for static error messages that do not need parameters.
- Use `fmt.Errorf` for dynamic error messages that only need a formatted string and do not need caller‑side matching.
- Use exported package‑level `var` errors initialized with `errors.New` when callers must match specific error conditions using `errors.Is`.
- Use custom `error` types when callers need to match dynamic errors with structured data using `errors.As`.

**Static error (no matching vs matching)**

```go
// package foo

func Open() error {
  return errors.New("could not open")
}

// package bar

if err := foo.Open(); err != nil {
  // Can't handle the error.
  panic("unknown error")
}
```

```go
// package foo

var ErrCouldNotOpen = errors.New("could not open")

func Open() error {
  return ErrCouldNotOpen
}

// package bar

if err := foo.Open(); err != nil {
  if errors.Is(err, foo.ErrCouldNotOpen) {
    // handle the error
  } else {
    panic("unknown error")
  }
}
```

**Dynamic error (no matching vs matching)**

```go
// package foo

func Open(file string) error {
  return fmt.Errorf("file %q not found", file)
}

// package bar

if err := foo.Open("testfile.txt"); err != nil {
  // Can't handle the error.
  panic("unknown error")
}
```

```go
// package foo

type NotFoundError struct {
  File string
}

func (e *NotFoundError) Error() string {
  return fmt.Sprintf("file %q not found", e.File)
}

func Open(file string) error {
  return &NotFoundError{File: file}
}

// package bar

if err := foo.Open("testfile.txt"); err != nil {
  var notFound *NotFoundError
  if errors.As(err, &notFound) {
    // handle the error
  } else {
    panic("unknown error")
  }
}
```

### Error Wrapping

- Either return the original error unchanged, or wrap it with `fmt.Errorf("context: %w", err)` when adding context.
- Use `%w` when callers should be able to match the underlying error using `errors.Is` / `errors.As`; use `%v` when you intentionally want to hide the original error type.
- Keep context succinct; prefer `"new store: %w"` over `"failed to create new store: %w"` to avoid noisy "failed to … failed to …" chains.

**Example**

```go
// Bad
s, err := store.New()
if err != nil {
  return fmt.Errorf(
    "failed to create new store: %w", err)
}
```

```go
// Good
s, err := store.New()
if err != nil {
  return fmt.Errorf(
    "new store: %w", err)
}
```

### Error Naming

- Name exported error variables with the `Err` prefix (`ErrCouldNotOpen`); name unexported error variables with `err` prefix (`errNotFound`).
- Name custom error types with the `Error` suffix (for example, `NotFoundError`, `resolveError`).

**Example**

```go
var (
  // The following two errors are exported
  // so that users of this package can match them
  // with errors.Is.

  ErrBrokenLink   = errors.New("link is broken")
  ErrCouldNotOpen = errors.New("could not open")

  // This error is not exported because
  // we don't want to make it part of our public API.
  // We may still use it inside the package
  // with errors.Is.

  errNotFound = errors.New("not found")
)

// Custom error types
type NotFoundError struct {
  File string
}

func (e *NotFoundError) Error() string {
  return fmt.Sprintf("file %q not found", e.File)
}

type resolveError struct {
  Path string
}

func (e *resolveError) Error() string {
  return fmt.Sprintf("resolve %q", e.Path)
}
```

### Handle Errors Once

- Each error should generally be handled once; avoid logging an error and then returning it, as upstream callers may also log it.
- Use one of these patterns:
  - Wrap and return the error, letting callers handle it further up the stack.
  - Log and degrade gracefully when the failure is non‑fatal (e.g., metrics emission).
  - Match specific expected errors with `errors.Is`/`errors.As` and handle those cases specially while returning other errors.

**Examples**

```go
// Bad: Log the error and return it
u, err := getUser(id)
if err != nil {
  // BAD: See description
  log.Printf("Could not get user %q: %v", id, err)
  return err
}
```

```go
// Good: Wrap the error and return it
u, err := getUser(id)
if err != nil {
  return fmt.Errorf("get user %q: %w", id, err)
}
```

```go
// Good: Log the error and degrade gracefully
if err := emitMetrics(); err != nil {
  // Failure to write metrics should not
  // break the application.
  log.Printf("Could not emit metrics: %v", err)
}
```

```go
// Good: Match the error and degrade gracefully
tz, err := getUserTimeZone(id)
if err != nil {
  if errors.Is(err, ErrUserNotFound) {
    // User doesn't exist. Use UTC.
    tz = time.UTC
  } else {
    return fmt.Errorf("get user %q: %w", id, err)
  }
}
```

## Type Assertions

### Handle Type Assertion Failures

- Always use the "comma ok" form of type assertions unless you are certain a panic on mismatch is acceptable.

**Example**

```go
// Bad
t := i.(string)
```

```go
// Good
t, ok := i.(string)
if !ok {
  // handle the error gracefully
}
```

## Panics and Program Exit

### Don't Panic

- Do not use panics as a normal error‑handling mechanism in production code; functions should return `error` values and let callers decide how to react.
- Reserve panics for truly unrecoverable conditions or well‑scoped initialization cases.
- In tests, prefer `t.Fatal` / `t.FailNow` instead of panics so that test failures are reported correctly.

**Example: application code**

```go
// Bad
func run(args []string) {
  if len(args) == 0 {
    panic("an argument is required")
  }
  // ...
}

func main() {
  run(os.Args[1:])
}
```

```go
// Good
func run(args []string) error {
  if len(args) == 0 {
    return errors.New("an argument is required")
  }
  // ...
  return nil
}

func main() {
  if err := run(os.Args[1:]); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
}
```

**Example: tests**

```go
// Bad
// func TestFoo(t *testing.T)

f, err := os.CreateTemp("", "test")
if err != nil {
  panic("failed to set up test")
}
```

```go
// Good
// func TestFoo(t *testing.T)

f, err := os.CreateTemp("", "test")
if err != nil {
  t.Fatal("failed to set up test")
}
```

### Exit in `main` Only

- Call `os.Exit` or `log.Fatal*` only in `main`; all non‑`main` functions must return errors instead of exiting the process.
- Structure `main` to call a single `run()` function that returns an error or exit code, and exit in exactly one place.
