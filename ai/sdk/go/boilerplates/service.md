---
name: service-boilerplate
description: Esqueleto do service Go — errors.go com errs.Register*, interface + implementação devolvendo errs.AppError, variante com split CRUD + Auth/IAM
sdk: v0.8.2
---

# Boilerplate — Service

> **`//gofi:context {contexto}`** abre a cláusula `package` (basta em um arquivo
> do pacote) — é o elo entre o símbolo no grafo e `specs/{contexto}/`. Ver
> `.claude/sdk/go/boilerplates/model.md`.

Regras de erro: `.claude/sdk/go/knowledge/error-handling.md`. Validação:
`.claude/sdk/go/knowledge/validation.md`.

## errors.go

```go
//gofi:context {contexto}
package service

import "github.com/joaoprofile/gofi-sdk-go/base/errs"

var (
	ErrEntityNotFound   = errs.RegisterNotFound("ENTITY_NOT_FOUND", "entity not found [%s]")
	ErrEntityConflict   = errs.RegisterConflict("ENTITY_CONFLICT", "entity already exists")
	ErrEntityValidation = errs.RegisterValidation("ENTITY_VALIDATION", "invalid entity data")
	ErrEntityCreate     = errs.RegisterOperation("ENTITY_CREATE_FAILED", "error creating entity")
	ErrEntityDelete     = errs.RegisterOperation("ENTITY_DELETE_FAILED", "error deleting entity")
	ErrEntityUpdate     = errs.RegisterOperation("ENTITY_UPDATE_FAILED", "error updating entity")
	ErrEntityQuery      = errs.RegisterOperation("ENTITY_QUERY_FAILED", "error querying entities")
)
```

Outros kinds quando o caso existir: `RegisterForbidden` (403, regra de
negócio que nega o ator), `RegisterUnauthorized` (401, credencial),
`RegisterExternalError` (502, falha de parceiro/integração).

## entity_service.go

```go
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/joaoprofile/gofi-sdk-go/base/errs"
	"github.com/joaoprofile/gofi-sdk-go/sqln"

	"<module>/common/enums"
	"<module>/domain/{contexto}/model"
	"<module>/domain/{contexto}/repository"
)

type EntityService interface {
	Create(ctx context.Context, tenantID string, req model.CreateEntityRequest) errs.AppError
	Update(ctx context.Context, id string, req model.UpdateEntityRequest) errs.AppError
	Delete(ctx context.Context, id string) errs.AppError
	GetByFilter(ctx context.Context, filter model.EntityFilter) (*sqln.Page[model.Entity], errs.AppError)
	GetByID(ctx context.Context, id string) (*model.Entity, errs.AppError)
}

type entityService struct {
	repo repository.EntityRepository
}

func NewEntityService(repo repository.EntityRepository) EntityService {
	return &entityService{repo: repo}
}

func (s *entityService) Create(ctx context.Context, tenantID string, req model.CreateEntityRequest) errs.AppError {
	if err := req.Validate(); err != nil {
		return ErrEntityValidation.WithDetails(err)
	}
	err := s.repo.Save(ctx, model.Entity{
		ID:       uuid.NewString(),
		TenantID: tenantID, // from the claims, never from the body
		Name:     req.Name,
		Email:    req.Email,
		Status:   enums.EntityStatusActive,
	})
	if errors.Is(err, repository.ErrDuplicateEntity) {
		return ErrEntityConflict.New()
	}
	if err != nil {
		return ErrEntityCreate.Wrap(err)
	}
	return errs.AppError{}
}

func (s *entityService) Update(ctx context.Context, id string, req model.UpdateEntityRequest) errs.AppError {
	if err := req.Validate(); err != nil {
		return ErrEntityValidation.WithDetails(err)
	}
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrEntityUpdate.Wrap(err)
	}
	if existing == nil {
		return ErrEntityNotFound.New(id)
	}
	if err := s.repo.Update(ctx, id, req); err != nil {
		return ErrEntityUpdate.Wrap(err)
	}
	return errs.AppError{}
}

func (s *entityService) Delete(ctx context.Context, id string) errs.AppError {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrEntityDelete.Wrap(err)
	}
	if existing == nil {
		return ErrEntityNotFound.New(id)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return ErrEntityDelete.Wrap(err)
	}
	return errs.AppError{}
}

func (s *entityService) GetByFilter(ctx context.Context, filter model.EntityFilter) (*sqln.Page[model.Entity], errs.AppError) {
	if err := filter.Validate(); err != nil {
		return nil, ErrEntityValidation.WithDetails(err)
	}
	page, err := s.repo.FindByFilter(ctx, filter)
	if err != nil {
		return nil, ErrEntityQuery.Wrap(err)
	}
	return page, errs.AppError{}
}

func (s *entityService) GetByID(ctx context.Context, id string) (*model.Entity, errs.AppError) {
	entity, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrEntityQuery.Wrap(err)
	}
	if entity == nil {
		return nil, ErrEntityNotFound.New(id)
	}
	return entity, errs.AppError{}
}
```

`enums.EntityStatusActive` é a constante do enum de status (mesmo pacote de `enums.EntityStatusMap` do `model.md`);
`EntityFilter.Validate()` só existe se o filtro tiver tag `validate`
(ex.: teto de `limit`) — senão, remova a chamada.

## Filtro Dinâmico — GetByDynamicQuery

Quando o contexto usa filtro dinâmico (regras completas:
`.claude/sdk/go/knowledge/dynamic-filter.md`):

```go
// errors.go (acréscimo)
ErrEntityInvalidFilter = errs.RegisterValidation("ENTITY_INVALID_FILTER", "invalid filter")

// Na interface
GetByDynamicQuery(ctx context.Context, tenantID string, f *sqln.Filters) (*sqln.Page[model.Entity], errs.AppError)

// Na implementação
func (s *entityService) GetByDynamicQuery(ctx context.Context, tenantID string, f *sqln.Filters) (*sqln.Page[model.Entity], errs.AppError) {
	page, err := s.repo.FindByDynamicQuery(ctx, tenantID, f)
	if errors.Is(err, sqln.ErrInvalidFilter) {
		return nil, ErrEntityInvalidFilter.Wrap(err) // errs.RegisterValidation → 400
	}
	if err != nil {
		return nil, ErrEntityQuery.Wrap(err)
	}
	return page, errs.AppError{}
}
```

**Regras do service para filtro dinâmico:**
- Service **não valida** o filtro — a allowlist (`sqln.FilterMapping`) é aplicada no repository por `sqln.BuildQuery`/`NewPageRequestFilter`; o service só traduz `sqln.ErrInvalidFilter` para 400
- Service **não transforma** `*sqln.Filters` — passa direto ao repository
- Tenant é **argumento explícito** (vem das claims no handler), nunca `f.Tenant`
- Imports: `"errors"`, `"github.com/joaoprofile/gofi-sdk-go/sqln"`

## Variante — Service com split CRUD + Auth/IAM

Quando o contexto também autentica (login, refresh, logout, `/me`): **uma**
interface, **um** struct/constructor, dois arquivos. `{contexto}_service.go`
fica como acima; `auth_service.go` acrescenta os métodos de auth no mesmo
`*entityService`, que passa a receber o `*core.IAMService`. `errors.go`
continua único. Detalhe do iam: `.claude/sdk/go/knowledge/iam.md`.

```go
// errors.go (acréscimo)
var (
	ErrAuthInvalidCredentials = errs.RegisterUnauthorized("AUTH_INVALID_CREDENTIALS", "invalid email or password")
	ErrAuthSession            = errs.RegisterUnauthorized("AUTH_SESSION_INVALID", "session is not valid")
	ErrAuthTenantDenied       = errs.RegisterForbidden("AUTH_TENANT_DENIED", "no access to this tenant")
	ErrAuthLogout             = errs.RegisterOperation("AUTH_LOGOUT_FAILED", "error ending session")
)
```

```go
// auth_service.go
package service

import (
	"context"
	"errors"

	"<module>/domain/{contexto}/model"
	"github.com/joaoprofile/gofi-sdk-go/base/errs"
	"github.com/joaoprofile/gofi-sdk-go/iam/core"
	"github.com/joaoprofile/gofi-sdk-go/iam/port"
	"github.com/joaoprofile/gofi-sdk-go/iam/types"
)

// On the interface: Login, Refresh, Logout (LogoutAll follows Logout with s.iam.LogoutAll).
// model.LoginRequest (DTO with Validate), model.ClientMeta (IP, UserAgent) and model.Module are context types.

func (s *entityService) Login(ctx context.Context, in model.LoginRequest, meta model.ClientMeta) (*types.Session, errs.AppError) {
	res, err := s.iam.Authenticate(ctx, port.AuthInput{Email: in.Email, Password: in.Password})
	if err != nil {
		return nil, ErrAuthInvalidCredentials.Wrap(err) // one message for every credential failure
	}
	if len(res.Tenants) == 0 {
		return nil, ErrAuthTenantDenied.New()
	}
	session, err := s.iam.SelectTenant(ctx, port.SelectTenantInput{
		UserID:    res.UserID,
		TenantID:  res.Tenants[0].Tenant.ID, // multi-tenant: tenant chosen by the user, checked by the ticket
		Module:    model.Module,
		Ticket:    res.Ticket,
		IPAddress: meta.IP,
		UserAgent: meta.UserAgent,
	})
	if errors.Is(err, core.ErrTenantAccessDenied) {
		return nil, ErrAuthTenantDenied.New()
	}
	if err != nil {
		return nil, ErrAuthSession.Wrap(err)
	}
	return session, errs.AppError{}
}

func (s *entityService) Refresh(ctx context.Context, refreshToken string) (*types.Session, errs.AppError) {
	session, err := s.iam.RefreshToken(ctx, refreshToken) // rotation: the old pair stops working
	if err != nil {
		return nil, ErrAuthSession.Wrap(err)
	}
	return session, errs.AppError{}
}

func (s *entityService) Logout(ctx context.Context, sessionID string) errs.AppError {
	if err := s.iam.Logout(ctx, sessionID); err != nil {
		return ErrAuthLogout.Wrap(err)
	}
	return errs.AppError{}
}
```

- Nada de OAuth, state, nonce, hash de refresh ou parse de JWT escrito à mão
  no service — o iam faz (`iam-social-login.md` para login social).
- Teste do `auth_service.go`: `iam.New` com `memory.NewTestProvider()` e
  `UserPort`/`TenantPort` fake — sem mockar o `IAMService`.

## Padrões de Retorno

| Situação | Retorno |
|----------|---------|
| Sucesso (void) | `return errs.AppError{}` |
| Sucesso (com dado) | `return result, errs.AppError{}` |
| Validação falhou | `return ErrXxxValidation.WithDetails(err)` |
| Not found | `FindByID` devolveu `nil` → `return ErrXxxNotFound.New(id)` |
| Erro de repo | `return ErrXxxCreate.Wrap(err)` |
| Erro do iam | `ErrAuthXxx.Wrap(err)` (Unauthorized/Forbidden) — cliente vê só a mensagem registrada |

## Regras

- Validar DTO **antes** de qualquer I/O
- Interface separada da implementação — testabilidade
- `errors.go` com todos os erros do contexto — uma var por caso de erro
- Nunca retornar `error` puro — sempre `errs.AppError`
- Not-found decidido no service (`FindByID` → `nil`), nunca por `RowsAffected`
