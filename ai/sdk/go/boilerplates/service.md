# Boilerplate — Service

## errors.go

```go
package service

import "github.com/joaoprofile/gofi/base/errs"

var (
	ErrPersonNotFound   = errs.RegisterNotFound("PERSON_NOT_FOUND", "person not found [%d]")
	ErrPersonConflict   = errs.RegisterConflict("PERSON_CONFLICT", "person already exists")
	ErrPersonValidation = errs.RegisterValidation("PERSON_VALIDATION", "invalid person data")
	ErrPersonCreate     = errs.RegisterOperation("PERSON_CREATE_FAILED", "error creating person")
	ErrPersonDelete     = errs.RegisterOperation("PERSON_DELETE_FAILED", "error deleting person")
	ErrPersonUpdate     = errs.RegisterOperation("PERSON_UPDATE_FAILED", "error updating person")
	ErrPersonQuery      = errs.RegisterOperation("PERSON_QUERY_FAILED", "error querying persons")
)
```

## person_service.go

```go
package service

import (
	"context"
	"errors"

	"github.com/joaoprofile/examples/api/src/person/model"
	"github.com/joaoprofile/examples/api/src/person/repository"
	"github.com/joaoprofile/gofi/base/errs"
)

type PersonService interface {
	Create(ctx context.Context, req model.CreatePersonRequest) errs.AppError
	Update(ctx context.Context, id string, req model.UpdatePersonRequest) errs.AppError
	Delete(ctx context.Context, id string) errs.AppError
	GetByFilter(ctx context.Context, filter model.PersonFilter) (model.PersonResponse, errs.AppError)
	GetByID(ctx context.Context, id string) (*model.Person, errs.AppError)
}

type personService struct {
	repo repository.PersonRepository
}

func NewPersonService(repo repository.PersonRepository) PersonService {
	return &personService{repo: repo}
}

func (s *personService) Create(ctx context.Context, req model.CreatePersonRequest) errs.AppError {
	if err := req.Validate(); err != nil {
		return ErrPersonValidation.WithDetails(err)
	}
	err := s.repo.Save(ctx, model.Person{
		Name:  req.Name,
		Email: req.Email,
		CPF:   req.CPF,
		Age:   req.Age,
	})
	if err != nil {
		return ErrPersonCreate.Wrap(err)
	}
	return errs.AppError{}
}

func (s *personService) Update(ctx context.Context, id string, req model.UpdatePersonRequest) errs.AppError {
	if err := req.Validate(); err != nil {
		return ErrPersonValidation.WithDetails(err)
	}
	err := s.repo.Update(ctx, id, req)
	if err == nil {
		return errs.AppError{}
	}
	if errors.Is(err, repository.ErrNoRowsAffected) {
		return ErrPersonNotFound.New(id)
	}
	return ErrPersonUpdate.Wrap(err)
}

func (s *personService) Delete(ctx context.Context, id string) errs.AppError {
	if err := s.repo.Delete(ctx, id); err != nil {
		return ErrPersonDelete.Wrap(err)
	}
	return errs.AppError{}
}

func (s *personService) GetByFilter(ctx context.Context, filter model.PersonFilter) (model.PersonResponse, errs.AppError) {
	result, err := s.repo.FindByFilter(ctx, filter)
	if err != nil {
		return nil, ErrPersonQuery.Wrap(err)
	}
	return result, errs.AppError{}
}

func (s *personService) GetByID(ctx context.Context, id string) (*model.Person, errs.AppError) {
	person, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrPersonQuery.Wrap(err)
	}
	if person == nil {
		return nil, ErrPersonNotFound.New(id)
	}
	return person, errs.AppError{}
}
```

## Filtro Dinâmico — GetByDynamicQuery

Adicionado à interface e implementação quando o contexto usa filtro dinâmico:

```go
// Na interface
GetByDynamicQuery(ctx context.Context, filters *sqln.Filters) (model.PersonQueryResponse, errs.AppError)

// Na implementação
func (s *personService) GetByDynamicQuery(ctx context.Context, filters *sqln.Filters) (model.PersonQueryResponse, errs.AppError) {
    result, err := s.repo.FindByDynamicQuery(ctx, filters)
    if err != nil {
        return nil, ErrPersonQuery.Wrap(err)
    }
    return result, errs.AppError{}
}
```

**Regras do service para filtro dinâmico:**
- Service **não valida** os filtros — validação via `queryMapping.Validate(filters)` já foi feita no handler
- Service **não transforma** `*sqln.Filters` — passa diretamente para o repository
- Usa o mesmo erro `ErrXxxQuery` do `GetByFilter` — não criar novo erro só para query dinâmica
- Import necessário: `"github.com/joaoprofile/gofi/sqln"`

## Padrões de Retorno

| Situação | Retorno |
|----------|---------|
| Sucesso (void) | `return errs.AppError{}` |
| Sucesso (com dado) | `return result, errs.AppError{}` |
| Validação falhou | `return ErrXxxValidation.WithDetails(err)` |
| Not found | `return ErrXxxNotFound.New(id)` |
| Not found (no rows) | detectar `errors.Is(err, repository.ErrNoRowsAffected)` |
| Erro de repo | `return ErrXxxCreate.Wrap(err)` |

## Regras

- Validar DTO **antes** de qualquer I/O
- Interface separada da implementação — testabilidade
- `errors.go` com todos os erros do contexto — uma var por caso de erro
- Nunca retornar `error` puro — sempre `errs.AppError`
