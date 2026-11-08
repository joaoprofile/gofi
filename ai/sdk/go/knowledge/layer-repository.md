---
name: layer-repository
description: Resumo da camada repository em Go — arquivo único, escrita por statement, leitura por sqln.Find*, conflito UNIQUE, transação, cache e erro puro
sdk: v0.2.18
keywords: [camada, repository, sqln, statement, criteria, FindFromCriteria, UniqueResult, PagedList, IsUniqueViolation, transaction, WithCache]
---

# Camada Repository — resumo

Leia este primeiro; o documento completo só na situação da última seção.

## O que faz e o que não faz

**Faz:** executa SQL via `sqln` e devolve entidade ou `error` puro. Normaliza a
query (allowlist de sort, teto de limit), traduz conflito de constraint em erro
sentinela, é dono da transação e do cache.

**Não faz:** regra de negócio; decidir not-found; devolver `errs.AppError`;
guardar recurso de banco ou expor `Close()`; adapter ou factory (vão em
`adapter/`).

## Padrão

```go
//gofi:context {contexto}
package repository

var ErrDuplicateEntity = errors.New("entity already exists")

const (
	entitySelectFields = "e.id, e.name, e.status"
	entityInsertQuery  = `INSERT INTO entity (id, tenant_id, name, status) VALUES ($1, $2, $3, $4)`
	uqEntityName       = "uq_entity_tenant_name"
)

type EntityRepository interface {
	Save(ctx context.Context, e model.Entity) error
	Update(ctx context.Context, id string, req model.UpdateEntityRequest) error
	FindByFilter(ctx context.Context, f model.EntityFilter) (*sqln.Page[model.Entity], error)
	FindByID(ctx context.Context, id string) (*model.Entity, error)
}

type entityRepository struct {
	stm sqln.Statement
}

func NewEntityRepository() EntityRepository { // sem ctx e sem tocar o banco
	return &entityRepository{stm: sqln.NewStatement()}
}

func (r *entityRepository) Save(ctx context.Context, e model.Entity) error {
	_, err := r.stm.Execute(ctx, entityInsertQuery, e.ID, e.TenantID, e.Name, e.Status)
	if connection.IsUniqueViolation(err, uqEntityName) {
		return ErrDuplicateEntity
	}
	return err
}

func (r *entityRepository) FindByID(ctx context.Context, id string) (*model.Entity, error) {
	return sqln.FindFromCriteria[model.Entity](ctx,
		criteria.From("entity", "e").
			Select(entitySelectFields).
			Where(criteria.Eq("e.id", id)),
	).UniqueResult()
}
```

Listagem paginada:

```go
q := criteria.From("entity", "e").Select(entitySelectFields).Where(where...)
page := sqln.NewPageRequest(f.Page, min(f.Limit, maxLimit), []sqln.Sort{
	sqln.NewSort("e.created_at", sqln.DESC),
})
return sqln.FindFromCriteria[model.Entity](ctx, q).WithPage(page).PagedList()
```

Predicados: `criteria.Eq`, `criteria.Contains`, `criteria.In`.

## Regras que mais causam erro

- **Arquivo único** `{contexto}_repository.go`: interface, constantes SQL e
  implementação. Nunca dois arquivos em `repository/`.
- **Nunca `*sql.DB`** em repository: nada de `sql.Open`, `db.DB()`,
  `connection.DB()`. O construtor não faz `Prepare`, ping nem `logging.Fatal`:
  a conexão global só existe depois do `Build`.
- **Escrita:** só `r.stm.Execute(ctx, query, args...)`, que entra sozinho na tx
  do `ctx`. `*sql.Stmt` em campo não: quebra com PgBouncer e escapa da tx.
  `Prepare` só dentro de transação, em laço de bulk, com `defer stmt.Close()`.
- **Leitura:** só `sqln.Find[T]`, `FindFromCriteria[T]`, `FindWithFilter[T]` +
  `.List()`, `.UniqueResult()`, `.PagedList()`, `.All()`. Sem `rows.Scan`.
  Nunca `ExecuteListQuery(db)` e afins.
- **`UniqueResult()`:** `nil, nil` = sem linha, e quem responde 404 é o service.
- **Primitivo** (`Exists*`, flag): `(*T, error)`, sem converter para `(T, error)`.
- **SQL em constantes** com `$1, $2, …`; nunca valor concatenado. Tabela em
  **singular** `snake_case` (`naming.md`).
- **Helper com `ctx` que executa SQL é método do receiver.** Função de pacote
  só para transformação pura (`entityArgs(e) []any`).
- **`Update` não checa `RowsAffected`**, salvo lock otimista declarado.
- **Insert simples:** sem `RETURNING` e sem `(*Entity, error)`; devolve `error`.
- **Conflito:** `connection.IsUniqueViolation(err, "uq_...")`, constraint com
  nome explícito na migration. Nunca string matching, nem `FindBy<Chave>` antes
  do `Save` (TOCTOU).
- **Transação vive aqui:** mutação multi-tabela = campo
  `tx transaction.Transaction` + `r.tx.Execute(ctx, fn)`. O service nunca
  importa `sqln/transaction`.
- **Simples ou paginada:** a spec marca. Não simule lista com página gigante
  (paga `COUNT(*)` à toa).
- **Cache só aqui:** `.WithCache(sqln.NewCache[T](nome, ttl))`, invalidação por
  `NewCache[T](nome, ttl).Del(ctx)`. Nunca `sqln.InstanceRedis()` à mão.
- **Filtro dinâmico:** base termina dentro do `WHERE`, com tenant em `$1`;
  `sqln.BuildQuery` + `sqln.NewPageRequestFilter` + `sqln.FindWithFilter`.
- **Testes** usam mock handcraft do repository; nunca mock de banco.

## Quando abrir o detalhe

| Documento | Abra quando |
|---|---|
| `.claude/sdk/go/boilerplates/repository.md` | for criar o arquivo do zero |
| `.claude/sdk/go/knowledge/persistence-rules.md` | a regra aqui não bastar (índice completo) |
| `.claude/sdk/go/knowledge/repository-aggregate-pattern.md` | houver mutação multi-tabela ou bulk |
| `.claude/sdk/go/knowledge/transactions.md` | precisar de isolamento, `MaxRetries` ou `*sql.Stmt` na tx |
| `.claude/sdk/go/knowledge/read-endpoints.md` | for GET agregado para o front, com cache e sort |
| `.claude/sdk/go/knowledge/dynamic-filter.md` | houver `FindByDynamicQuery` |
| `.claude/sdk/go/knowledge/cache-layer.md` | a spec pedir cache |
| `.claude/sdk/go/knowledge/pagination.md` | houver dúvida entre `.List()` e `.PagedList()` |
| `.claude/sdk/go/knowledge/value-objects.md` | o mapeamento falhar no scan |
| `.claude/sdk/go/knowledge/repository-primitive-return.md` | for método `Exists*` |
| `.claude/sdk/go/knowledge/migrations.md` | for criar ou alterar schema |
| `.claude/sdk/go/knowledge/postgres-index-strategy.md` | for decidir índice |
| `.claude/sdk/go/api/sqln-criteria.md` | precisar de predicado fora dos acima |
