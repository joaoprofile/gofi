---
name: persistence-rules
description: Regras de persistência em Go/sqln — repository, escrita, transação, IDs, listagem, conflito UNIQUE, cache
sdk: v0.8.2
keywords: [repository, sqln, statement, transaction, aggregate, uuidv7, pagination, unique, cache, persistence]
---

# Regras — persistência (repository, SQL, IDs, cache)

Índice das regras; o detalhe mora no arquivo apontado. API do SDK em
`.claude/sdk/go/api/sqln*.md`; esqueleto em `.claude/sdk/go/boilerplates/repository.md`.

- **Repository é arquivo único** — interface, constantes SQL e implementação
  juntas (`structure.md`).
- **Construtor não toca o banco.** `New{Contexto}Repository()` só monta a
  struct (campo `stm sqln.Statement` — alias de `statement.Statement`, de
  `sqln.NewStatement()` — e, se houver mutação multi-tabela,
  `tx transaction.Transaction`). Nada de `Prepare`, ping ou `logging.Fatal` no
  construtor — a conexão global só existe depois do `Build`
  (`database-connection.md`).
- **Escrita via `r.stm.Execute(ctx, query, args...)`.** `statement.Statement`
  executa direto (sem prepared statement nomeado — seguro com PgBouncer; o
  driver postgres já faz cache do describe) e **entra sozinho na transação
  carregada pelo `ctx`**. Não guarde `*sql.Stmt` em campo do repositório:
  não ganha round-trip, quebra com PgBouncer em modo transação e escapa da tx
  se executado dentro de `Execute`. `Prepare` só dentro de uma transação, para
  laço de bulk, com `defer stmt.Close()` (`repository-aggregate-pattern.md`).
- **Leitura via `sqln.FindFromCriteria[T]` / `sqln.Find[T]` /
  `sqln.FindWithFilter[T]`** — o manager mapeia pelas tags `db`; nada de
  `rows.Scan` manual. Um resultado: `.UniqueResult()` (`nil, nil` quando não
  há linha). Lista: `.List()`. Página: `.WithPage(p).PagedList()`. Volume
  grande sem slice: `.All()` (streaming). Mapeamento e armadilhas:
  `value-objects.md`.
- **Helpers de persistência são MÉTODOS do receiver.** Toda função do arquivo
  do repo que recebe `ctx` e executa SQL é `func (r *{contexto}Repository)`.
  Exceção: transformações puras sem `ctx`/I/O (`entityArgs(e) []any`) podem ser
  funções privadas de pacote (`repository-aggregate-pattern.md`).
- **Transação vive no repository, não no service.** Mutação que toca N tabelas
  atomicamente = struct `{Aggregate}Aggregate` em `model/` + métodos
  `CreateAggregate`/`UpdateAggregate`/`DeleteAggregate` que envolvem tudo em
  `r.tx.Execute(ctx, fn)`. Isolamento declarado no construtor
  (`sql.LevelReadCommitted` por padrão); acima disso, com `MaxRetries`.
  Service nunca importa `sqln/transaction`. Bulk (`CreateAggregatesBulk`) só
  quando a spec declara consumidor. Detalhe: `repository-aggregate-pattern.md`
  e `transactions.md`.
- **IDs UUID em `tenant` e `user` (e FKs para eles) — UUIDv7 gerado na
  aplicação.** Schema `id UUID PRIMARY KEY` sem `DEFAULT`; em Go
  `uuid.NewV7()` (`github.com/google/uuid` ≥ v1.6.0), nunca `uuid.New()`/
  `uuid.NewString()` (v4). `INSERT` inclui `id`, sem `RETURNING id`; `Save`
  devolve só `error`. Modelar como `string`; path param validado com
  `uuid.Parse` (400 quando malformado); DTO usa `validate:"uuid"` (qualquer
  versão). Demais entidades: `BIGINT IDENTITY`. Detalhe:
  `.claude/expertise/ddd-architecture/id-types.md`.
- **Listagem simples (`.List()`) ou paginada (`.WithPage(...).PagedList()`)** —
  a spec marca qual. Simples devolve `([]T, error)`; paginada
  `(*sqln.Page[T], error)`. Nunca simular simples com página gigante e
  descartar o envelope (paga `COUNT(*)` à toa). Detalhe: `pagination.md`.
- **Conflito de UNIQUE: SQLSTATE no repo, nunca string matching.**
  ```go
  if pgErr, ok := connection.AsPgError(err); ok && pgErr.Code == "23505" &&
      pgErr.Constraint == "uq_{tabela}_{colunas}" {
      return ErrDuplicate{Entidade} // erro sentinela do pacote repository
  }
  ```
  Constraints com nome explícito na migration. Nunca `FindBy<Chave>` antes do
  `Save` só para detectar conflito (TOCTOU + round-trip).
- **Insert simples — sem `RETURNING`, sem `(*Entity, error)`, sem `Scan`**
  quando ninguém consome campos gerados: `r.stm.Execute` e `error`. Handler
  responde `201` sem corpo.
- **Update sem checar `RowsAffected`** (salvo lock otimista declarado) —
  `repository-update-simple.md`.
- **Presença de valor primitivo devolve `(*T, error)`** —
  `repository-primitive-return.md`.
- **Cache só no repository**, via `.WithCache(sqln.NewCache[T](nome, ttl))`
  (uma query) ou `cache.Get`/`Set` (DTO composto); invalidação por
  `NewCache[T](nome, ttl).Del(ctx)`. Nunca `sqln.InstanceRedis()` +
  `json.Marshal` à mão. Detalhe: `cache-layer.md`.
- **Migrations:** `migrations.md`. **Conexão/driver/réplica:**
  `database-connection.md`. **Índices:** `postgres-index-strategy.md`.
