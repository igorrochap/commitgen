# Enums, Time, Globals, and Marshaling

## Enums and Constants

### Start Enums at One

- When using `iota` for enums, usually start at `iota + 1` so that the zero value of the enum type is not a valid, meaningful value.
- Only use zero as a valid enum when a "zero" value is intentionally a meaningful default (for example, a default logging output).

**Example**

```go
// Bad
type Operation int

const (
  Add Operation = iota
  Subtract
  Multiply
)

// Add=0, Subtract=1, Multiply=2
```

```go
// Good
type Operation int

const (
  Add Operation = iota + 1
  Subtract
  Multiply
)

// Add=1, Subtract=2, Multiply=3
```

**When zero is a valid default**

```go
type LogOutput int

const (
  LogToStdout LogOutput = iota
  LogToFile
  LogToRemote
)

// LogToStdout=0, LogToFile=1, LogToRemote=2
```

## Time Handling

### General

- Always use the standard `time` package for time and date operations; do not treat days, hours, weeks, or years as fixed durations because of DST and calendar complexity.

### Instants: `time.Time`

- Use `time.Time` to represent points in time and rely on methods like `Before`, `After`, and `Equal` for comparisons instead of comparing integers.

**Example**

```go
// Bad
func isActive(now, start, stop int) bool {
  return start <= now && now < stop
}
```

```go
// Good
func isActive(now, start, stop time.Time) bool {
  return (start.Before(now) || start.Equal(now)) && now.Before(stop)
}
```

### Durations: `time.Duration`

- Use `time.Duration` for time intervals; accept `time.Duration` parameters rather than bare ints and construct durations at call sites (`10*time.Second`).
- Prefer `Time.AddDate` when you intend to move to the same clock‑time on a different calendar day and `Time.Add` when you want an exact time offset like "24 hours later".

**Example**

```go
// Bad
func poll(delay int) {
  for {
    // ...
    time.Sleep(time.Duration(delay) * time.Millisecond)
  }
}

poll(10) // was it seconds or milliseconds?
```

```go
// Good
func poll(delay time.Duration) {
  for {
    // ...
    time.Sleep(delay)
  }
}

poll(10 * time.Second)
```

**Adding one day vs 24 hours**

```go
newDay := t.AddDate(0 /* years */, 0 /* months */, 1 /* days */)
maybeNewDay := t.Add(24 * time.Hour)
```

## Globals and Initialization

### Avoid Mutable Globals

- Avoid mutating global state, including function pointer globals; instead use dependency injection via struct fields or constructor parameters.
- For testability, inject collaborators (such as a clock function) into types rather than mutating global function variables.

**Example**

```go
// Bad: mutable global function
// sign.go

var _timeNow = time.Now

func sign(msg string) string {
  now := _timeNow()
  return signWithTime(msg, now)
}
```

```go
// Bad: test mutates global
// sign_test.go

func TestSign(t *testing.T) {
  oldTimeNow := _timeNow
  _timeNow = func() time.Time {
    return someFixedTime
  }
  defer func() { _timeNow = oldTimeNow }()

  assert.Equal(t, want, sign(give))
}
```

```go
// Good: dependency injected via struct
// sign.go

type signer struct {
  now func() time.Time
}

func newSigner() *signer {
  return &signer{
    now: time.Now,
  }
}

func (s *signer) Sign(msg string) string {
  now := s.now()
  return signWithTime(msg, now)
}
```

```go
// Good: tests configure dependency
// sign_test.go

func TestSigner(t *testing.T) {
  s := newSigner()
  s.now = func() time.Time {
    return someFixedTime
  }

  assert.Equal(t, want, s.Sign(give))
}
```

### `init()` Usage

- Avoid `init()` whenever possible.
- If `init()` is used, it must be deterministic, not depend on ordering or side effects of other `init()` functions, and avoid I/O and environment state.
- Prefer explicit helpers called from `main()` over hidden `init` logic, especially in libraries.

**Example**

```go
// Bad
type Foo struct {
  // ...
}

var _defaultFoo Foo

func init() {
  _defaultFoo = Foo{
    // ...
  }
}
```

```go
// Good
var _defaultFoo = Foo{
  // ...
}

// or, better, for testability:

var _defaultFoo = defaultFoo()

func defaultFoo() Foo {
  return Foo{
    // ...
  }
}
```

## Atomic Operations

- Prefer `go.uber.org/atomic` over `sync/atomic` for shared state, to avoid accidentally using non‑atomic operations on plain `int32`, `int64`, etc.
- Use types like `atomic.Bool` to read/write atomically via methods rather than manipulating raw integers.

**Example**

```go
// Bad
type foo struct {
  running int32 // atomic
}

func (f *foo) start() {
  if atomic.SwapInt32(&f.running, 1) == 1 {
    // already running…
    return
  }
  // start the Foo
}

func (f *foo) isRunning() bool {
  return f.running == 1 // race!
}
```

```go
// Good
type foo struct {
  running atomic.Bool
}

func (f *foo) start() {
  if f.running.Swap(true) {
    // already running…
    return
  }
  // start the Foo
}

func (f *foo) isRunning() bool {
  return f.running.Load()
}
```

## Marshaling and Field Tags

- Any struct field that is marshaled to JSON, YAML, or similar should have explicit tags naming the field (e.g. ``Price int `json:"price"` ``).
- Rely on tags as the external contract; this allows you to rename struct fields internally without changing serialized formats.
