# base/common

`import "github.com/joaoprofile/gofi-sdk-go/base/common"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	AUTHORIZATION_HEADER = "Authorization"
	USER_ID_HEADER       = "X-User-Id"
	COMPANY_ID_HEADER    = "X-Company-Id"

	ACCEPT                  = "Accept"
	CONTENT_TYPE            = "content-type"
	APPLICATION_JSON        = "application/json"
	APPLICATION_URL_ENCODED = "application/x-www-form-urlencoded"
)
```

## Funções

### common.CamelToSnake

```go
func CamelToSnake(s string) string
```

### common.ParseStructAnnotation

```go
func ParseStructAnnotation(cfg any, annotation string) error
```

### common.ParseStructAnnotationFunc

```go
func ParseStructAnnotationFunc(cfg any, annotation string, lookup func(string) string) error
```

ParseStructAnnotationFunc fills cfg from lookup(tag) and returns every
conversion error joined, so all invalid variables are reported at once.

### common.ParseStructColumns

```go
func ParseStructColumns(s any) (string, error)
```

### common.ParseStructName

```go
func ParseStructName(s any) (string, error)
```

### common.Ptr

```go
func Ptr(s string) *string
```

### common.ToFixed

```go
func ToFixed(num float64, precision int) float64
```

## common.LogLevel

```go
type LogLevel string
```

```go
const (
	LogLevelError LogLevel = "error"
	LogLevelWarn  LogLevel = "warn"
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
)
```

## common.String

```go
type String sql.NullString
```

### String.MarshalJSON

```go
func (t String) MarshalJSON() ([]byte, error)
```

### String.Scan

```go
func (t *String) Scan(value any) error
```

### String.UnmarshalJSON

```go
func (t *String) UnmarshalJSON(data []byte) error
```

### String.Value

```go
func (s String) Value() (driver.Value, error)
```

