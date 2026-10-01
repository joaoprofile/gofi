---
name: boilerplate-repository
description: Esqueleto do repository Go com sqln v0.8.2 — statement, criteria, paginação, filtro dinâmico, cache, conflito UNIQUE
sdk: v0.8.2
keywords: [boilerplate, repository, sqln, statement, criteria, FindFromCriteria, BuildQuery, cache]
---

# Boilerplate — Repository

> **`//gofi:context {contexto}`** abre a cláusula `package` (basta em um arquivo
> do pacote) — é o elo entre o símbolo no grafo e `specs/{contexto}/`. Ver
> `.claude/sdk/go/boilerplates/model.md`.

Regras: `.claude/sdk/go/knowledge/persistence-rules.md`. API:
`.claude/sdk/go/api/sqln.md`, `sqln-criteria.md`, `sqln-statement.md`.
Exemplos executáveis: `examples/sqln/search/product/repository.go` e
`examples/sqln/filter-api/product/repository.go`.

```go
//gofi:context {contexto}
package repository

import (
	"context"
	"errors"

	"github.com/joaoprofile/gofi-sdk-go/sqln"
	"github.com/joaoprofile/gofi-sdk-go/sqln/connection"
	"github.com/joaoprofile/gofi-sdk-go/sqln/criteria"

	"<module>/domain/{contexto}/model"
)

// ErrDuplicateEntity is returned when the natural key already exists.
var ErrDuplicateEntity = errors.New("entity already exists")

const (
	entitySelectFields = "e.id, e.name, e.email, e.status, e.created_at"

	entityInsertQuery = `INSERT INTO entities (id, tenant_id, name, email, status) VALUES ($1, $2, $3, $4, $5)`
	entityUpdateQuery = `UPDATE entities SET name = $1, email = $2, status = $3 WHERE id = $4`
	entityDeleteQuery = `DELETE FROM entities WHERE id = $1`

	uqEntityEmail = "uq_entities_tenant_email"
)

type EntityRepository interface {
	Save(ctx context.Context, e model.Entity) error
	Update(ctx context.Context, id string, req model.UpdateEntityRequest) error
	Delete(ctx context.Context, id string) error
	FindByFilter(ctx context.Context, f model.EntityFilter) (*sqln.Page[model.Entity], error)
	FindByID(ctx context.Context, id string) (*model.Entity, error)
}

type entityRepository struct {
	stm sqln.Statement
}

// NewEntityRepository does not touch the database: the global connection
// only exists after gofi's Build.
func NewEntityRepository() EntityRepository {
	return &entityRepository{stm: sqln.NewStatement()}
}

func (r *entityRepository) Save(ctx context.Context, e model.Entity) error {
	_, err := r.stm.Execute(ctx, entityInsertQuery, e.ID, e.TenantID, e.Name, e.Email, e.Status)
	if pgErr, ok := connection.AsPgError(err); ok && pgErr.Code == "23505" && pgErr.Constraint == uqEntityEmail {
		return ErrDuplicateEntity
	}
	return err
}

func (r *entityRepository) Update(ctx context.Context, id string, req model.UpdateEntityRequest) error {
	_, err := r.stm.Execute(ctx, entityUpdateQuery, req.Name, req.Email, req.Status, id)
	return err
}

func (r *entityRepository) Delete(ctx context.Context, id string) error {
	_, err := r.stm.Execute(ctx, entityDeleteQuery, id)
	return err
}

func (r *entityRepository) FindByFilter(ctx context.Context, f model.EntityFilter) (*sqln.Page[model.Entity], error) {
	where := []criteria.Predicate{criteria.Eq("e.tenant_id", f.TenantID)}
	if f.Name != "" {
		where = append(where, criteria.Contains("e.name", "%"+f.Name+"%"))
	}
	if len(f.Statuses) > 0 {
		where = append(where, criteria.In("e.status", f.Statuses))
	}
	q := criteria.From("entities", "e").Select(entitySelectFields).Where(where...)

	page := sqln.NewPageRequest(f.Page, min(f.Limit, maxLimit), []sqln.Sort{
		sqln.NewSort("e.created_at", sqln.DESC),
	})
	return sqln.FindFromCriteria[model.Entity](ctx, q).WithPage(page).PagedList()
}

const maxLimit = 200

func (r *entityRepository) FindByID(ctx context.Context, id string) (*model.Entity, error) {
	return sqln.FindFromCriteria[model.Entity](ctx,
		criteria.From("entities", "e").
			Select(entitySelectFields).
			Where(criteria.Eq("e.id", id)),
	).UniqueResult()
}
```

- Sem `*sql.Stmt` em campo e sem `Close()` — `repository-close.md`.
- `UniqueResult()`: `nil, nil` = não encontrado (o service responde 404);
  `nil, err` = erro de banco.
- Mutação multi-tabela: campo `tx transaction.Transaction` +
  `r.tx.Execute(ctx, fn)` — `repository-aggregate-pattern.md`.
- Presença de primitivo (`Exists*`) → `(*bool, error)` —
  `repository-primitive-return.md`.

## Filtro dinâmico — `FindByDynamicQuery`

Só quando o contexto tem filtro dinâmico (`dynamic-filter.md`):

```go
const entityDynamicQueryBase = `SELECT ` + entitySelectFields + `
FROM entities e
WHERE e.tenant_id = $1`

func (r *entityRepository) FindByDynamicQuery(ctx context.Context, tenantID string, f *sqln.Filters) (*sqln.Page[model.Entity], error) {
	if f.Params == nil {
		f.Params = &sqln.FilterParams{}
	}
	if f.Params.SortField == "" {
		f.Params.SortField, f.Params.SortDirection = "created", string(sqln.DESC)
	}
	q, err := sqln.BuildQuery(entityDynamicQueryBase, []any{tenantID}, f, model.EntityFilterMapping, nil)
	if err != nil {
		return nil, err // wraps sqln.ErrInvalidFilter
	}
	page, err := sqln.NewPageRequestFilter(f, model.EntityFilterMapping)
	if err != nil {
		return nil, err
	}
	return sqln.FindWithFilter[model.Entity](ctx, q).WithPage(page).PagedList()
}
```

- Base termina dentro do `WHERE`; o SDK anexa `AND ( <filtros> )` com
  placeholders depois de `$1`. Tenant nunca é filtro nem `fmt.Sprintf`.
- Erro com `sqln.ErrInvalidFilter` vira validação (400) no service.

## Cache — `WithCache` + invalidação

Só quando a spec pede cache (`cache-layer.md`):

```go
const entityListCacheTTL = 5 * time.Minute

func entityListCache(tenantID string) *sqln.Cache[model.Entity] {
	return sqln.NewCache[model.Entity]("entity:list:"+tenantID, entityListCacheTTL)
}

// in FindByFilter:
return sqln.FindFromCriteria[model.Entity](ctx, q).
	WithCache(entityListCache(f.TenantID)).
	WithPage(page).
	PagedList()

// in the interface: InvalidateListCache(ctx context.Context, tenantID string) error
func (r *entityRepository) InvalidateListCache(ctx context.Context, tenantID string) error {
	return entityListCache(tenantID).Del(ctx)
}
```

O service chama `InvalidateListCache` depois de `Save`/`Update`/`Delete`;
nunca acessa Redis.

## Padrões obrigatórios

- Arquivo único: interface + constantes SQL + implementação.
- SQL em constantes de pacote; placeholders `$1, $2, …` — nunca concatenação
  de valor.
- Escrita: `r.stm.Execute(ctx, query, args...)`. Leitura: `sqln.Find*` com
  tags `db` (nunca `rows.Scan` à mão).
- `Update` não checa `RowsAffected` (`repository-update-simple.md`).
- Conflito UNIQUE traduzido por `connection.AsPgError` + nome da constraint.
- Helpers com `ctx` que tocam banco são métodos do receiver; só funções puras
  (`entityArgs(e) []any`) ficam no pacote.
- Construtor sem `ctx` e sem acesso a banco.
