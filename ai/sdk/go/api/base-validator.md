# base/validator

`import "github.com/gofi-labs/gofi-sdk-go/base/validator"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Funções

### validator.IsStruct

```go
func IsStruct(s any) error
```

### validator.IsStructP

```go
func IsStructP(s any) error
```

## validator.FieldError

```go
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
```

## validator.ValidationError

```go
type ValidationError struct {
	Errors []FieldError `json:"errors"`
}
```

### ValidationError.Error

```go
func (v ValidationError) Error() string
```

## validator.Validator

```go
type Validator struct {
	// contains filtered or unexported fields
}
```

### validator.New

```go
func New() *Validator
```

### Validator.ValidateStruct

```go
func (v *Validator) ValidateStruct(s any) error
```

