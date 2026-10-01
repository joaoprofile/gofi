# base/errs

`import "github.com/joaoprofile/gofi-sdk-go/base/errs"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Funções

### errs.JoinErrors

```go
func JoinErrors(errs []error) error
```

JoinErrors joins the non-nil errors as "a | b" and supports errors.Is/As on each.
It returns a non-nil empty error when there are none (legacy contract); prefer errors.Join.

## errs.AppError

```go
type AppError struct {
	Kind    ErrorKind `json:"kind,omitempty"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
	Err     error     `json:"-"`
}
```

### errs.GetErrorByCode

```go
func GetErrorByCode(code string) *AppError
```

### errs.New

```go
func New(code, message string, err error, args ...any) AppError
```

---- Package level constructors ----

New returns a copy of the reistred AppError, optionally formatting the message.

### errs.Register

```go
func Register(code, message string) AppError
```

### errs.RegisterConflict

```go
func RegisterConflict(code, message string) AppError
```

### errs.RegisterExternalError

```go
func RegisterExternalError(code, message string) AppError
```

### errs.RegisterForbidden

```go
func RegisterForbidden(code, message string) AppError
```

### errs.RegisterNotFound

```go
func RegisterNotFound(code, message string) AppError
```

### errs.RegisterOperation

```go
func RegisterOperation(code, message string) AppError
```

### errs.RegisterUnauthorized

```go
func RegisterUnauthorized(code, message string) AppError
```

### errs.RegisterValidation

```go
func RegisterValidation(code, message string) AppError
```

### AppError.Error

```go
func (e AppError) Error() string
```

Error has a value receiver so AppError values satisfy error directly.

### AppError.ErrorString

```go
func (e *AppError) ErrorString() string
```

### AppError.Exists

```go
func (e AppError) Exists() bool
```

### AppError.GetCode

```go
func (e *AppError) GetCode() string
```

### AppError.GetSafeError

```go
func (e AppError) GetSafeError(defaultMessage string) error
```

### AppError.Is

```go
func (e AppError) Is(target error) bool
```

Is matches another AppError (value or pointer) with the same non-empty Code,
so errors.Is(err, ErrRegistered) works for errors built with New or Wrap.

### AppError.IsConflict

```go
func (e AppError) IsConflict() bool
```

### AppError.IsExternalError

```go
func (e AppError) IsExternalError() bool
```

### AppError.IsForbidden

```go
func (e AppError) IsForbidden() bool
```

### AppError.IsNotFound

```go
func (e AppError) IsNotFound() bool
```

### AppError.IsOperation

```go
func (e AppError) IsOperation() bool
```

### AppError.IsUnauthorized

```go
func (e AppError) IsUnauthorized() bool
```

### AppError.IsValidation

```go
func (e AppError) IsValidation() bool
```

IsValidation reports whether this is a VALIDATION eror

### AppError.New

```go
func (e AppError) New(args ...any) AppError
```

### AppError.ToJSON

```go
func (e *AppError) ToJSON() string
```

### AppError.Unwrap

```go
func (e AppError) Unwrap() error
```

Unwrap exposes the wrapped cause to errors.Is and errors.As.

### AppError.WithDetails

```go
func (e AppError) WithDetails(details any) AppError
```

### AppError.Wrap

```go
func (e AppError) Wrap(err error, args ...any) AppError
```

productErrIdREquired.New()
productErrIdREquired.Wrap(error)

## errs.ErrorKind

```go
type ErrorKind string
```

ErrorKind classifies the category of an AppErro with a readable string identifier.
This value appears verbatim i logs and JSON payloads, aiding observalitiry

```go
const (
	KindUnknown       ErrorKind = "UNKNOWN"
	KindValidation    ErrorKind = "VALIDATION"
	KindNotFound      ErrorKind = "NOT_FOUND"
	KindConflict      ErrorKind = "CONFLICT"
	KindOperation     ErrorKind = "OPERATION"
	KindExternalError ErrorKind = "EXTERNAL_ERROR"
	KindUnauthorized  ErrorKind = "UNAUTHORIZED"
	KindForbidden     ErrorKind = "FORBIDDEN"
)
```

