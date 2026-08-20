# Interfaces, Methods, and Concurrency

## Interfaces and Methods

### Pointers to Interfaces

- Do not use pointers to interfaces; pass interfaces as values even if the underlying concrete type is a pointer.
- An interface value internally holds a type descriptor pointer and a data pointer, which may already be a pointer to the underlying value.

### Verify Interface Compliance

- When a type is expected to implement an interface (especially exported types), add a compile‑time assertion like `var _ io.Reader = (*MyType)(nil)` or `var _ http.Handler = MyType{}`.
- Use the zero value of the asserted type on the right side: `nil` for pointer/slice/map types, empty struct for struct types.

**Example (Bad vs Good)**

```go
// Bad
type Handler struct {
  // ...
}

func (h *Handler) ServeHTTP(
  w http.ResponseWriter,
  r *http.Request,
) {
  // ...
}
```

```go
// Good
type Handler struct {
  // ...
}

var _ http.Handler = (*Handler)(nil)

func (h *Handler) ServeHTTP(
  w http.ResponseWriter,
  r *http.Request,
) {
  // ...
}
```

```go
// Another example using a value type
type LogHandler struct {
  h   http.Handler
  log *zap.Logger
}

var _ http.Handler = LogHandler{}

func (h LogHandler) ServeHTTP(
  w http.ResponseWriter,
  r *http.Request,
) {
  // ...
}
```

### Receivers and Interfaces

- Methods with value receivers can be called on both values and pointers; methods with pointer receivers require pointers or addressable values.
- Values stored in maps are not addressable, so you cannot call pointer‑receiver methods on them unless the map stores pointers.
- An interface can be satisfied by a pointer receiver even if the method has a value receiver, and you can assign both value and pointer forms accordingly; the inverse (value satisfying pointer receiver) does not hold.

**Example**

```go
type S struct {
  data string
}

func (s S) Read() string {
  return s.data
}

func (s *S) Write(str string) {
  s.data = str
}

// We cannot get pointers to values stored in maps, because they are not
// addressable values.
sVals := map[int]S{1: {"A"}}

// We can call Read on values stored in the map because Read
// has a value receiver, which does not require the value to
// be addressable.
sVals.Read()[1]

// We cannot call Write on values stored in the map because Write
// has a pointer receiver, and it's not possible to get a pointer
// to a value stored in a map.
//   sVals.Write("test")[1]

sPtrs := map[int]*S{1: {"A"}}

// You can call both Read and Write if the map stores pointers,
// because pointers are intrinsically addressable.
sPtrs.Read()[1]
sPtrs.Write("test")[1]
```

## Concurrency and Synchronization

### Mutexes

- Use `sync.Mutex` and `sync.RWMutex` by value; their zero values are valid and should almost never be pointers.
- Do not embed mutexes in structs (even unexported); instead add a named field like `mu sync.Mutex` to avoid exposing locking methods as part of the public API.

**Example: zero‑value mutex and non‑embedded field**

```go
// Prefer value mutex over pointer
var mu sync.Mutex
mu.Lock()
mu.Unlock()
```

```go
// Bad: embedded mutex
type SMap struct {
  sync.Mutex

  data map[string]string
}

func NewSMap() *SMap {
  return &SMap{
    data: make(map[string]string),
  }
}

func (m *SMap) Get(k string) string {
  m.Lock()
  defer m.Unlock()

  return m.data[k]
}
```

```go
// Good: named field mutex
type SMap struct {
  mu sync.Mutex

  data map[string]string
}

func NewSMap() *SMap {
  return &SMap{
    data: make(map[string]string),
  }
}

func (m *SMap) Get(k string) string {
  m.mu.Lock()
  defer m.mu.Unlock()

  return m.data[k]
}
```

### Copy Slices and Maps at Boundaries

- When receiving slices or maps that you intend to store, make a copy to avoid callers mutating your internal state.
- When returning slices or maps that expose internal state, return a deep copy to prevent callers from racing with or corrupting your internal data.

**Example: receiving slices**

```go
// Bad
func (d *Driver) SetTrips(trips []Trip) {
  d.trips = trips
}

trips := ...
d1.SetTrips(trips)

// Did you mean to modify d1.trips?
trips = ...
```

```go
// Good
func (d *Driver) SetTrips(trips []Trip) {
  d.trips = make([]Trip, len(trips))
  copy(d.trips, trips)
}

trips := ...
d1.SetTrips(trips)

// We can now modify trips without affecting d1.trips.
trips = ...
```

**Example: returning maps**

```go
type Stats struct {
  mu       sync.Mutex
  counters map[string]int
}

// Bad: exposes internal map
func (s *Stats) Snapshot() map[string]int {
  s.mu.Lock()
  defer s.mu.Unlock()

  return s.counters
}
```

```go
type Stats struct {
  mu       sync.Mutex
  counters map[string]int
}

// Good: returns a copy
func (s *Stats) Snapshot() map[string]int {
  s.mu.Lock()
  defer s.mu.Unlock()

  result := make(map[string]int, len(s.counters))
  for k, v := range s.counters {
    result[k] = v
  }
  return result
}

// Snapshot is now a copy.
snapshot := stats.Snapshot()
```

### Defer for Cleanup

- Use `defer` to unlock mutexes, close files, stop tickers, and perform other cleanup at the point of acquisition for clarity and safety.
- Avoid premature micro‑optimizations around `defer`; only avoid it if you can prove the function is so performance‑critical that nanosecond‑level overhead matters.

**Example**

```go
// Bad: easy to miss unlocks due to multiple returns
p.Lock()
if p.count < 10 {
  p.Unlock()
  return p.count
}

p.count++
newCount := p.count
p.Unlock()

return newCount
```

```go
// Good: single defer, easier to reason about
p.Lock()
defer p.Unlock()

if p.count < 10 {
  return p.count
}

p.count++
return p.count
```

### Channels

- Prefer unbuffered channels or channels of size one; any other buffer size must be justified and carefully analyzed for backpressure behavior.
- Before choosing a larger buffer, reason about when it can fill, how writers behave in that scenario, and what failure mode you get under load.

**Example**

```go
// Bad: arbitrary buffer size
// Ought to be enough for anybody!
c := make(chan int, 64)
```

```go
// Good: size of one
c := make(chan int, 1)

// Good: unbuffered channel (size zero)
c := make(chan int)
```

### Goroutines

- Do not "fire‑and‑forget" goroutines; every goroutine must either have a clear termination condition or an explicit signal to stop.
- Provide a way to wait for goroutines to complete, typically using `sync.WaitGroup` or a dedicated `done` channel.
- Never start goroutines from `init()`.
