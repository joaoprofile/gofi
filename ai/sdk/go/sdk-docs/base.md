# gofi/base — Primitivos de Domínio

## gofi/base/errs — Erros Estruturados

### AppError

```go
import "github.com/joaoprofile/gofi/base/errs"

type AppError struct {
    Kind    string
    Code    string
    Message string
    Details any
    Err     error
}

// Verificar se houve erro
if appErr.Exists() { ... }

// Verificar kind
appErr.IsValidation() bool
appErr.IsNotFound()   bool
appErr.IsConflict()   bool
appErr.IsOperation()  bool
```

### Registro de Erros (vars de pacote)

```go
// em service/errors.go
var (
    ErrPersonNotFound   = errs.RegisterNotFound("PERSON_NOT_FOUND", "person not found")
    ErrPersonConflict   = errs.RegisterConflict("PERSON_CONFLICT", "person already exists")
    ErrPersonValidation = errs.RegisterValidation("PERSON_VALIDATION", "invalid person data")
    ErrPersonCreate     = errs.RegisterOperation("PERSON_CREATE_FAILED", "error creating person")
    ErrPersonUpdate     = errs.RegisterOperation("PERSON_UPDATE_FAILED", "error updating person")
    ErrPersonDelete     = errs.RegisterOperation("PERSON_DELETE_FAILED", "error deleting person")
    ErrPersonQuery      = errs.RegisterOperation("PERSON_QUERY_FAILED", "error querying persons")
)
```

### Instanciar e Enriquecer

```go
// Erro simples
return ErrPersonNotFound.New()

// Wrapping de erro original
return ErrPersonCreate.Wrap(err)

// Com detalhes (ex: erros de validação)
return ErrPersonValidation.WithDetails(validationErr)

// AppError vazio = sem erro
return errs.AppError{}
```

### Convenção de Nomes

- `ErrContextNotFound` — entidade não encontrada
- `ErrContextConflict` — violação de unicidade
- `ErrContextValidation` — falha de validação de DTO
- `ErrContextCreate/Update/Delete` — falha de operação de escrita
- `ErrContextQuery` — falha de operação de leitura

---

## gofi/base/validator — Validação de Structs

### Setup (singleton de pacote)

```go
import "github.com/joaoprofile/gofi/base/validator"

var v = validator.New()
```

Instanciar uma vez por pacote, não por request.

### Uso nos DTOs

```go
type CreatePersonRequest struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
    CPF   string `json:"cpf"   validate:"required"`
    Age   int    `json:"age"   validate:"required,min=1"`
}

func (r CreatePersonRequest) Validate() error {
    return v.ValidateStruct(r)
}
```

### Tags disponíveis

| Tag | Descrição |
|-----|-----------|
| `required` | campo obrigatório (não-zero) |
| `email` | formato de e-mail válido |
| `min=N` | valor mínimo (int) ou comprimento mínimo (string) |
| `max=N` | valor máximo (int) ou comprimento máximo (string) |
| `len=N` | comprimento exato |
| `oneof=a b c` | enum de valores permitidos |
| `uuid` | UUID v4 válido |
| `url` | URL válida |

### ValidationError

```go
// O erro retornado por ValidateStruct é *validator.ValidationError
// Contém Errors []FieldError com Field e Message por campo
// Compatível com errs.AppError.Details para propagação ao cliente
```

### Uso no Service

```go
func (s *personService) Create(ctx context.Context, req model.CreatePersonRequest) errs.AppError {
    if err := req.Validate(); err != nil {
        return ErrPersonValidation.WithDetails(err)
    }
    // ...
}
```
