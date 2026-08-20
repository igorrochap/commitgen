# Style, Formatting, Structs, and Naming

## Style and Formatting

### Line Length

- Aim for a soft limit of 99 characters per line; exceeding this is allowed, but try to keep lines readable without horizontal scrolling.

### Consistency

- Above all, be consistent; avoid mixing multiple conflicting styles within the same package or codebase.
- When updating code, apply these guidelines package‑by‑package or larger to avoid partially styled code.

### Group Similar Declarations

- Group related imports, constants, variables, and type declarations using `import (...)`, `const (...)`, `var (...)`, and `type (...)` blocks.
- Group only related declarations; unrelated constants or values should be in separate groups.

### Import Groups

- Organize imports into two groups: standard library first, followed by all other imports, separated by a blank line; this matches `goimports` behavior.

### Package Names

- Use short, all‑lowercase package names without underscores or plurals.
- Avoid generic names like `common`, `util`, `shared`, or `lib`.
- Choose names that rarely need aliasing at call sites.

### Function Names

- Use MixedCaps for function names (e.g., `GetUserProfile`), following Go conventions.
- Test functions may use underscores to indicate sub‑cases, such as `TestParser_InvalidInput`.

### Import Aliases

- Use import aliases only when the package name does not match the last path element or to resolve name conflicts.
- Avoid unnecessary aliases when the default name is clear and unambiguous.

### Function Grouping and Ordering

- Define types first, then constructors (`newT`/`NewT`), then methods on that type, and finally package‑level helper functions.
- Group functions by receiver so that all methods of a type appear together; place shared utility functions toward the end of the file.

### Reduce Nesting and Unnecessary `else`

- Prefer early returns and `continue` to reduce nested `if` blocks and improve readability.
- Replace `if/else` assignments with a single initialization followed by a conditional override when appropriate.

### Local Variable Declarations

- Use short declaration `:=` when assigning explicit values in local scope.
- Use `var` where the zero value is clearer or preferred, such as declaring an empty slice before appending.

### `nil` Slices

- Treat `nil` as a valid zero‑length slice; return `nil` instead of `[]T{}` when returning empty slices unless serialization format requires otherwise.
- Check slice emptiness using `len(s) == 0` rather than `s == nil`.
- Rely on zero values (`var s []T`) instead of `make([]T)` or `[]T{}` when you intend to build up a slice via `append`.

### Reduce Scope of Variables

- Limit the scope of variables as much as possible (for example, using short `if err := ...; err != nil {}` forms) so long as it does not increase nesting complexity.
- Keep variables that are needed after an `if` block outside the `if` initializer.

### Avoid Naked Parameters

- Avoid passing bare booleans or positional arguments where the meaning is unclear; add C‑style comments (`/* isLocal */`) or, better, use custom types to clarify meaning.

### Raw String Literals

- Prefer raw string literals (backticks) when they improve readability, especially for strings with quotes or escape sequences.

## Structs and Maps

### Initializing Structs

- When constructing non‑test structs, specify field names in composite literals.
- Omit fields that have zero values unless they provide important context; rely on default zero values to reduce noise.
- Use `var s T` when you want the pure zero‑value struct; prefer this over `s := T{}`.
- Use `&T{...}` rather than `new(T)` for struct pointers to keep initialization consistent with value literals.

### Initializing Maps

- Prefer `make(map[K]V)` for empty or programmatically populated maps, and add capacity hints when possible.
- Use map literals when initializing a fixed set of key–value pairs.
- Recognize that `var m map[K]V` is `nil` and will panic on writes, whereas `m := make(map[K]V)` is ready for use.

### Embedding in Structs

- Place embedded types at the top of struct field lists, followed by a blank line, then regular fields.
- Only embed when it provides clear semantic benefit (e.g., augmenting behavior) without leaking implementation details or surprising users.
- Avoid embeddings that change zero‑value usefulness, expose internals, or constrain future changes to the type. Mutexes are the sharpest case of this — see the Mutexes rule in [`interfaces-and-concurrency.md`](interfaces-and-concurrency.md).

### Avoid Embedding Types in Public Structs

- Avoid embedding concrete or interface types in exported structs because it leaks implementation details and constrains evolution.
- Prefer explicit fields and delegate methods (e.g., `list *AbstractList` with forwarding `Add` and `Remove` methods) rather than embedding.

**Example**

```go
type AbstractList struct {}

// Add adds an entity to the list.
func (l *AbstractList) Add(e Entity) {
  // ...
}

// Remove removes an entity from the list.
func (l *AbstractList) Remove(e Entity) {
  // ...
}
```

```go
// Bad
// ConcreteList is a list of entities.
type ConcreteList struct {
  *AbstractList
}
```

```go
// Good
// ConcreteList is a list of entities.
type ConcreteList struct {
  list *AbstractList
}

// Add adds an entity to the list.
func (l *ConcreteList) Add(e Entity) {
  l.list.Add(e)
}

// Remove removes an entity from the list.
func (l *ConcreteList) Remove(e Entity) {
  l.list.Remove(e)
}
```

## Predeclared Identifiers and Names

- Do not reuse or shadow predeclared identifiers like `error`, `string`, etc. as variable or field names; this causes confusion and potential bugs.
- Use alternative names (e.g., `err`, `str`, `errorMessage`) so that references like `error` continue to refer to the built‑in type.

**Example**

```go
// Bad
var error string
// `error` shadows the builtin

func handleErrorMessage(error string) {
  // `error` shadows the builtin
}
```

```go
// Bad: fields that clash with predeclared identifiers
type Foo struct {
  // While these fields technically don't
  // constitute shadowing, grepping for
  // `error` or `string` strings is now
  // ambiguous.
  error  error
  string string
}

func (f Foo) Error() error {
  // `error` and `f.error` are
  // visually similar
  return f.error
}

func (f Foo) String() string {
  // `string` and `f.string` are
  // visually similar
  return f.string
}
```

```go
// Good
type Foo struct {
  // `error` and `string` strings are
  // now unambiguous.
  err error
  str string
}

func (f Foo) Error() error {
  return f.err
}

func (f Foo) String() string {
  return f.str
}
```
