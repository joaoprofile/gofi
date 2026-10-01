---
name: repository-aggregate-pattern
description: Mutação multi-tabela atômica — aggregate no model, transação no repository (sqln/transaction), helpers como métodos, bulk
sdk: v0.8.2
keywords: [repository, aggregate, transaction, tx, isolation, bulk, statement, helpers, receiver]
---

# Repository Aggregate Pattern — transação encapsulada no repo

Mecânica de transação (propagação por `ctx`, isolamento, retry, aninhamento):
`transactions.md`. API: `.claude/sdk/go/api/sqln-transaction.md`,
`sqln-statement.md`.

## Regra

Mutação que toca **N tabelas relacionadas numa transação** (raiz +
dependentes, snapshot de auditoria junto) tem a transação **no repository**:

- **model** declara `{Aggregate}Aggregate` com todas as entidades envolvidas.
- **repository** guarda `tx transaction.Transaction` (isolamento escolhido no
  construtor) e expõe `CreateAggregate`/`UpdateAggregate`/`DeleteAggregate`,
  cada um envolvendo tudo em `r.tx.Execute(ctx, fn)`.
- **service** nunca importa `sqln/transaction`: monta o aggregate e chama o
  método do repo (1 linha).

## Por quê

- Atomicidade fica com quem conhece o schema (ordem dos INSERTs, isolamento).
- Service testável sem banco: o mock do repo devolve `error` — sem `txRunner`.
- Aggregate root + dependentes = unidade de consistência (DDD); a persistência
  mantém a unidade coesa.

## Estrutura

```go
// model/aggregate.go
type {Aggregate}Aggregate struct {
    Root     *{Root}      // raiz
    Children []{Child}    // dependentes
    Snapshot *{LogEntry}  // (opcional) auditoria
}
```

```go
// repository/{contexto}_repository.go
type {Aggregate}Repository interface {
    CreateAggregate(ctx context.Context, a *model.{Aggregate}Aggregate) error
    UpdateAggregate(ctx context.Context, a *model.{Aggregate}Aggregate) error
    DeleteAggregate(ctx context.Context, rootID string) error

    FindRootByID(ctx context.Context, id string) (*model.{Root}, error)
    ListChildrenByRoot(ctx context.Context, rootID string) ([]model.{Child}, error)
}

type {aggregate}Repository struct {
    stm statement.Statement
    tx  transaction.Transaction
}

func New{Aggregate}Repository() {Aggregate}Repository {
    return &{aggregate}Repository{
        stm: sqln.NewStatement(),
        tx:  transaction.New(transaction.Options{Isolation: sql.LevelReadCommitted}),
    }
}

func (r *{aggregate}Repository) CreateAggregate(ctx context.Context, a *model.{Aggregate}Aggregate) error {
    return r.tx.Execute(ctx, func(ctx context.Context) error {
        if err := r.insertRoot(ctx, a.Root); err != nil {
            return err
        }
        for i := range a.Children {
            a.Children[i].RootID = a.Root.ID
            if err := r.insertChild(ctx, &a.Children[i]); err != nil {
                return err
            }
        }
        if a.Snapshot != nil {
            a.Snapshot.RootID = a.Root.ID
            return r.insertSnapshot(ctx, a.Snapshot)
        }
        return nil
    })
}

// Helpers: métodos do receiver; r.stm usa a tx carregada pelo ctx.
func (r *{aggregate}Repository) insertRoot(ctx context.Context, e *model.{Root}) error {
    _, err := r.stm.Execute(ctx, {root}InsertQuery, rootArgs(e)...)
    return err
}
```

- ID gerado na aplicação (UUIDv7) já vem preenchido no aggregate — sem
  `RETURNING`. Raiz com `IDENTITY`:
  `row, err := r.stm.QueryRow(ctx, insertReturningID, args...)` +
  `row.Scan(&a.Root.ID)` dentro do `fn`.
- **Sempre o `ctx` do `fn`** nos helpers — é ele que carrega a transação.

```go
// service — sem transação
if err := s.repo.CreateAggregate(ctx, aggregate); err != nil {
    return nil, Err{Aggregate}Persist.Wrap(err)
}
```

## Quando NÃO usar aggregate method

Operação **single-table** (`Update`, `DeleteByID`, leituras) é método simples
com `r.stm.Execute` / `sqln.Find*`. Se o `Delete` precisa apagar dependentes
antes da raiz, vira `DeleteAggregate` com tx.

## Helpers de persistência são MÉTODOS do receiver

Toda função do arquivo do repo que recebe `ctx` e executa SQL é
`func (r *{contexto}Repository) ...` — inclusive sub-passos privados
(`insertRoot`, `deleteChildrenByRoot`, `insertLog`).

- **Fronteira:** o repository é o único acesso a banco; função solta no
  pacote pode ser chamada de qualquer arquivo, fora do ciclo da instância.
- **Estado do receiver:** `r.stm`, `r.tx`, cache e conexão explícita vivem
  no struct.
- **Descoberta:** `func (r *{contexto}Repository)` lista tudo que o repo faz.

**Exceção:** transformação **pura** (sem `ctx`, sem I/O) pode ser função
privada de pacote:

```go
func rootArgs(e *model.{Root}) []any {
    return []any{e.ID, e.Name, e.Status}
}
```

Regra simples: *recebe `ctx` e toca banco → método do receiver.*

## Isolamento — default `ReadCommitted`

- `sql.LevelReadCommitted` — default: aggregate que só toca as próprias
  linhas; UNIQUE/CHECK no schema protegem as invariantes.
- `sql.LevelRepeatableRead` — lê e modifica várias linhas dependendo do mesmo
  snapshot.
- `sql.LevelSerializable` — invariante cross-row que o schema não cobre.
  Conflito `40001` sob concorrência é esperado: declare
  `transaction.Options{Isolation: sql.LevelSerializable, MaxRetries: 3}` e
  mantenha o `fn` repetível (sem efeito fora do banco).

`r.tx` é stateless por chamada: seguro para N goroutines, sem mutex.

## Bulk — `CreateAggregatesBulk(ctx, []*Aggregate) error`

Só quando a spec declara consumidor em massa (importação, sincronização).
Uma transação para o lote + statement preparado **na tx** e reutilizado:

```go
func (r *{aggregate}Repository) CreateAggregatesBulk(ctx context.Context, items []*model.{Aggregate}Aggregate) error {
    return r.tx.Execute(ctx, func(ctx context.Context) error {
        rootStmt, err := r.stm.Prepare(ctx, {root}InsertQuery) // ctx da tx → preparado na tx
        if err != nil {
            return err
        }
        defer rootStmt.Close()
        childStmt, err := r.stm.Prepare(ctx, {child}InsertQuery)
        if err != nil {
            return err
        }
        defer childStmt.Close()

        for _, a := range items {
            if _, err := rootStmt.ExecContext(ctx, rootArgs(a.Root)...); err != nil {
                return err
            }
            for i := range a.Children {
                if _, err := childStmt.ExecContext(ctx, childArgs(&a.Children[i])...); err != nil {
                    return err
                }
            }
        }
        return nil
    })
}
```

- **Chunk no caller** (ex.: 100 por chamada): uma linha ruim aborta só o
  chunk. Tamanho do chunk vai na spec do consumidor.
- **All-or-nothing por chunk.** Salvamento parcial linha a linha é lógica
  acima do repo, chamando `CreateAggregate` um a um.
- **Idempotência** por UNIQUE + tradução do `23505`.
- Multi-VALUES (`INSERT ... VALUES (...), (...)`) só se o profile mostrar que
  o prepare reutilizado não basta.

## Anti-padrões

- Transação no service (`sqln.NewTransaction().Execute(...)` orquestrando N
  repos) ou `txRunner` injetável no service.
- `*sql.Stmt` preparado no construtor e executado dentro da tx — roda em outra
  conexão e **escapa da transação** (e o construtor nem tem conexão ainda).
- `ctx.Value(connection.SqlTxContextKey).(*sql.Tx)` — o `statement` já usa a
  tx do `ctx`; para SQL cru, `connection.QuerierFrom(ctx, db)`.
- Aggregate parcial (`Children: nil` esperando o repo "adivinhar") — divida em
  métodos distintos.
- Um repository por tabela-filha + tx no service — espelhe o aggregate, não o
  schema.

## Checklist

- [ ] `{Aggregate}Aggregate` em `model/`
- [ ] `tx transaction.Transaction` no struct, isolamento explícito no construtor
- [ ] Serializable/RepeatableRead ⇒ `MaxRetries` e `fn` repetível
- [ ] Aggregate methods envolvem tudo em `r.tx.Execute(ctx, fn)` e usam o `ctx` do `fn`
- [ ] Service não importa `sqln/transaction`; teste do service mocka `CreateAggregate`
- [ ] Helpers com `ctx` são métodos do receiver; só funções puras no pacote
- [ ] Nenhum `*sql.Stmt` em campo; `Prepare` só no bulk, dentro da tx, com `defer Close`
- [ ] Bulk só com consumidor declarado na spec
