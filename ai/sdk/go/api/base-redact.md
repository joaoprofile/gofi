# base/redact

`import "github.com/joaoprofile/gofi-sdk-go/base/redact"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package redact renders configuration structs without their secrets, for fmt,
log/slog and encoding/json alike. A field tagged `redact:"true"` prints as
Mask when it holds a non-zero value and as "" otherwise; `redact:"url"`
masks only the password of a URL. A struct opts in by delegating to this
package from its own methods (value receivers, so pointers work too):

	func (c Config) String() string                { return redact.Sprint(c) }
	func (c Config) GoString() string              { return redact.GoSprint(c) }
	func (c Config) Format(f fmt.State, verb rune) { redact.Format(f, verb, c) }
	func (c Config) LogValue() slog.Value          { return redact.LogValue(c) }
	func (c Config) MarshalJSON() ([]byte, error)  { return redact.JSON(c) }

Unexported fields are never rendered.

## Constantes

```go
const Mask = "[REDACTED]"
```

Mask replaces a secret value.

## Funções

### redact.Format

```go
func Format(f fmt.State, verb rune, v any)
```

Format implements fmt.Formatter for v, so every verb (%v, %+v, %#v, %s, %d,
...) goes through the masked rendering.

### redact.GoSprint

```go
func GoSprint(v any) string
```

GoSprint renders v like %#v: pkg.Type{Name:value, ...}.

### redact.JSON

```go
func JSON(v any) ([]byte, error)
```

JSON encodes v as an object of its exported fields, honouring json tag
names, "-", omitempty and omitzero (by zero value).

### redact.LogValue

```go
func LogValue(v any) slog.Value
```

LogValue renders v as a slog group, one attribute per exported field.

### redact.Sprint

```go
func Sprint(v any) string
```

Sprint renders v like %+v: {Name:value ...}.

### redact.String

```go
func String(s string) string
```

String returns Mask for a non-empty s and "" otherwise.

### redact.URL

```go
func URL(s string) string
```

URL masks the password of a URL, keeping the rest readable. A value that
does not parse is masked entirely, since it cannot be inspected.

