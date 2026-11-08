---
name: layer-service
description: Resumo da camada service em Go — errors.go com errs.Register*, retorno errs.AppError, validação antes de I/O, not-found por FindByID, split CRUD/Auth e quando criar application
sdk: v0.2.18
keywords: [camada, service, errs, AppError, Register, Wrap, WithDetails, Validate, not-found, split, auth, application, mock]
---

# Camada Service — resumo

Leia este primeiro; o documento completo só na situação da última seção.

## O que faz e o que não faz

**Faz:** valida DTO, aplica regra de negócio, opera via interface de
repository, converte `error` em `errs.AppError`, decide not-found, atribui
tenancy recebida do handler.

**Não faz:** conhecer `http.ResponseWriter`/`http.Request`; SQL; `*sql.DB`;
importar `sqln/transaction`; acessar Redis; retornar `error` puro; ler
ambiente; encerrar o processo.

## Padrão

```go
//gofi:context {contexto}
package service // errors.go — um arquivo por contexto, uma var por caso

var (
	ErrEntityNotFound   = errs.RegisterNotFound("ENTITY_NOT_FOUND", "entity not found [%s]")
	ErrEntityConflict   = errs.RegisterConflict("ENTITY_CONFLICT", "entity already exists")
	ErrEntityValidation = errs.RegisterValidation("ENTITY_VALIDATION", "invalid entity data")
	ErrEntityUpdate     = errs.RegisterOperation("ENTITY_UPDATE_FAILED", "error updating entity")
)
```

| Registro | HTTP |
|---|---|
| `RegisterValidation` | 400 |
| `RegisterUnauthorized` | 401 |
| `RegisterForbidden` | 403 |
| `RegisterNotFound` | 404 |
| `RegisterConflict` | 409 |
| `RegisterExternalError` | 502 |
| `RegisterOperation` / `Register` | 500 |

```go
type EntityService interface {
	Create(ctx context.Context, tenantID string, req model.CreateEntityRequest) errs.AppError
	GetByID(ctx context.Context, id string) (*model.Entity, errs.AppError)
}

type entityService struct {
	repo repository.EntityRepository // interface, nunca a struct concreta
}

func NewEntityService(repo repository.EntityRepository) EntityService {
	return &entityService{repo: repo}
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
```

| Situação | Retorno |
|---|---|
| Sucesso | `errs.AppError{}`; com dado, `result, errs.AppError{}` |
| Validação | `ErrX.WithDetails(err)` |
| Not found | `FindByID` devolveu `nil` → `ErrX.New(id)` |
| Sentinela do repo | `errors.Is(err, repository.ErrDuplicateEntity)` → `ErrEntityConflict.New()` |
| Erro de I/O | `ErrX.Wrap(err)` |

## Regras que mais causam erro

- **Nunca `error` puro**; `AppError` vazio é o sucesso. `errors.New` exportado
  como erro de negócio não: registre com `errs.Register*`.
- **`Validate()` antes de qualquer I/O**, sempre aqui. `WithDetails` leva os
  campos ao cliente; `Wrap` guarda a causa e não a expõe. Não use `Wrap` em
  validação.
- **Not-found é decisão do service**, por `FindByID` antes de
  `Update`/`Delete`. Nunca por `RowsAffected` (única exceção: lock otimista →
  `RegisterConflict`).
- **Tenant é argumento explícito** vindo das claims; nunca do body, query ou
  `f.Tenant`.
- **Filtro:** `filter.Validate()` só se o filtro tem tag `validate`. O filtro
  dinâmico passa direto ao repository; o service só traduz
  `errors.Is(err, sqln.ErrInvalidFilter)` para um `RegisterValidation`.
- **IDs de `tenant`/`user` (e FKs):** `uuid.NewV7()`, nunca `uuid.New()` nem
  `uuid.NewString()`. Demais entidades: `BIGINT IDENTITY`.
- **Autorização target-aware** (dono, hierarquia) mora aqui: função pura no
  model + `errs.RegisterForbidden`. O gate estático de permissão é do handler.
- **Cache:** chama `repo.InvalidateListCache` depois de escrever; nunca toca
  Redis.
- **Logging:** só `logging.*`; `logging.Info` apenas no início/fim de fluxo de
  negócio, com `logging.FromContext(ctx)`. Nunca Info em loop.
- **Só o `main` encerra:** sem `panic`/`log.Fatal`/`os.Exit`; devolva erro.
- **Split CRUD/Auth:** uma interface, um struct, dois arquivos
  (`{contexto}_service.go` + `auth_service.go`), `errors.go` único. Sem OAuth,
  hash de refresh ou parse de JWT à mão: o iam faz.
- **Erro do iam:** `ErrAuthXxx.Wrap(err)`, com mensagem única para toda falha
  de credencial.

## Testes

Mock de repository **handcraft** com fn fields (`saveFn func(...)`),
implementando todos os métodos da interface. Compare `appErr.Code` ou
`errors.Is`; nunca `Message`. Nunca mock de banco.

## Quando criar `application/`

Só com bridge/factory, coordenação de ≥2 domínios, transação multi-passo/saga
ou mesmo use case em ≥2 transportes. Senão, só `service/`. Direção:
`handler → application → service → repository`; o service nunca importa
application nem bridge/factory.

## Quando abrir o detalhe

| Documento | Abra quando |
|---|---|
| `.claude/sdk/go/boilerplates/service.md` | for criar o service do zero ou fazer split Auth |
| `.claude/sdk/go/knowledge/error-handling.md` | houver dúvida de erro, corpo da resposta ou causa |
| `.claude/sdk/go/knowledge/validation.md` | precisar de tag cross-field ou config de motor de decisão |
| `.claude/sdk/go/knowledge/application-bridge-rules.md` | surgir gatilho de `application/` |
| `.claude/sdk/go/knowledge/iam.md` | o contexto autenticar |
| `.claude/sdk/go/knowledge/iam-social-login.md` | houver login social |
| `.claude/sdk/go/knowledge/rbac.md` | houver gate target-aware |
| `.claude/sdk/go/knowledge/dynamic-filter.md` | houver `GetByDynamicQuery` |
| `.claude/sdk/go/knowledge/logging.md` | for logar fluxo de negócio |
