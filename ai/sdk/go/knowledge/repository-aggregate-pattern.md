# Repository Aggregate Pattern — transação encapsulada no repo

## Regra

Quando uma operação de mutação precisa tocar **N tabelas relacionadas em
uma única transação** (entidade-raiz + dependentes em cascata, snapshot
de auditoria emitido junto da mutação, etc.), a transação **vive no
repository**, não no service.

- O **model** declara uma struct `{Aggregate}Aggregate` que agrega todas
  as entidades envolvidas como ponteiros.
- O **repository** injeta `tx sqln.Transaction` no constructor (geralmente
  `sqln.NewTransaction(sql.LevelXxx)`) e expõe métodos
  `CreateAggregate(ctx, *Aggregate) error`, `UpsertAggregate(ctx, *Aggregate) error`,
  `UpdateAggregate(ctx, *Aggregate) error`, `DeleteAggregate(ctx, ...) error`
  que abrem a transação via `r.tx.Execute(ctx, fn)` e fazem todas as
  operações dentro.
- O **service** **nunca** chama `sqln.NewTransaction` direto. Só constrói
  o aggregate e chama `repo.CreateAggregate(ctx, aggregate)` (1 linha).

## Por quê

- **Atomicidade fica grudada na pessoa que conhece o schema** — o
  repositório sabe a ordem dos INSERTs, nomes de tabelas, isolation level
  apropriado. Service não precisa saber nada disso.
- **Service vira testável sem DB real** — o mock do repo retorna `error`
  e pronto; não precisa simular tx via `txRunner` injetável ou
  `sqln.NewTransaction` mockado.
- **Padrão único no projeto** — todo lugar que precisa de tx atômica
  segue o mesmo formato. Reviewer reconhece imediatamente.
- **Boundaries DDD**: o aggregate root e seus dependentes formam uma
  unidade de consistência transacional — é responsabilidade da camada
  de persistência mantê-la coesa.

## Estrutura

### Model — declara o aggregate

```go
// model/aggregate.go
type {Aggregate}Aggregate struct {
    Root       *{RootEntity}     // entidade raiz
    Children   []{ChildEntity}   // dependentes em coleção
    Snapshot   *{LogEntity}      // (opcional) evento de auditoria
}
```

Ponteiros (`*T`) na raiz e no snapshot permitem que o repo grave campos
gerados (IDs autogerados pelo banco via `RETURNING`) e mute a referência
do caller. Slice para coleções.

### Repository — guarda `tx`, prepara stmts, expõe `*Aggregate` methods

```go
type {Aggregate}Repository interface {
    CreateAggregate(ctx context.Context, a *model.{Aggregate}Aggregate) error
    UpdateAggregate(ctx context.Context, a *model.{Aggregate}Aggregate) error
    DeleteAggregate(ctx context.Context, rootID string) error

    // ops simples que não precisam de tx — métodos individuais OK
    FindRootByID(ctx context.Context, id string) (*model.{RootEntity}, error)
    ListChildrenByRoot(ctx context.Context, rootID string) ([]model.{ChildEntity}, error)

    Close() error
}

type {aggregate}Repository struct {
    tx        sqln.Transaction
    stmUpdate *sql.Stmt        // prepared stmts para ops simples fora de tx
    stmDelete *sql.Stmt
}

func New{Aggregate}Repository(ctx context.Context) {Aggregate}Repository {
    stmUpdate, err := sqln.NewStatement().Prepare(ctx, updateRootQuery)
    if err != nil {
        logging.Fatal("...", slog.Any("error", err))
    }
    // ...
    return &{aggregate}Repository{
        tx:        sqln.NewTransaction(sql.LevelSerializable),
        stmUpdate: stmUpdate,
        // ...
    }
}
```

### CreateAggregate — abre tx, executa INSERT+children+snapshot

```go
func (r *{aggregate}Repository) CreateAggregate(ctx context.Context, a *model.{Aggregate}Aggregate) error {
    return r.tx.Execute(ctx, func(ctx context.Context) error {
        // INSERT raiz com RETURNING id (se id é IDENTITY)
        if err := sqln.NewStatement().QueryRow(ctx, insertRootQuery,
            a.Root.Field1, a.Root.Field2, ...,
        ).Scan(&a.Root.ID); err != nil {
            return err
        }

        // INSERT dependentes — usa o id recém-gerado
        for i := range a.Children {
            a.Children[i].RootID = a.Root.ID
            if _, err := sqln.NewStatement().Execute(ctx, insertChildQuery,
                a.Children[i].Field1, a.Children[i].RootID,
            ); err != nil {
                return err
            }
        }

        // (opcional) INSERT snapshot de auditoria
        if a.Snapshot != nil {
            a.Snapshot.RootID = a.Root.ID
            if _, err := sqln.NewStatement().Execute(ctx, insertSnapshotQuery,
                a.Snapshot.Field1, a.Snapshot.RootID,
            ); err != nil {
                return err
            }
        }

        return nil
    })
}
```

### Service — sem transação, só constrói e delega

```go
func (s *xxxService) Create(ctx context.Context, req CreateRequest, ...) (*Response, errs.AppError) {
    // validação + lookup tenancy + hidratação de campos

    aggregate := &model.{Aggregate}Aggregate{
        Root:     &model.{RootEntity}{...},
        Children: []model.{ChildEntity}{ {...}, {...} },
        Snapshot: &model.{LogEntity}{...},  // opcional
    }

    if err := s.repo.CreateAggregate(ctx, aggregate); err != nil {
        if isUniqueViolation(err) {
            return nil, ErrXxxConflict.Wrap(err)
        }
        return nil, ErrXxxPersist.Wrap(err)
    }

    resp := model.NewResponse(*aggregate.Root, aggregate.Children)
    return &resp, errs.AppError{}
}
```

### Test do service — mock retorna error, sem `txRunner`

```go
type mockRepo struct {
    createAggregateFn func(ctx context.Context, a *model.{Aggregate}Aggregate) error
    // ... outros métodos
}

func (m *mockRepo) CreateAggregate(ctx context.Context, a *model.{Aggregate}Aggregate) error {
    if m.createAggregateFn != nil {
        return m.createAggregateFn(ctx, a)
    }
    return nil
}

func TestCreate_Success(t *testing.T) {
    var captured *model.{Aggregate}Aggregate
    repo := &mockRepo{
        createAggregateFn: func(_ context.Context, a *model.{Aggregate}Aggregate) error {
            captured = a
            a.Root.ID = "generated-id"  // simula RETURNING
            return nil
        },
    }
    svc := NewXxxService(repo)
    // ...
}
```

## Quando NÃO usar aggregate method

Operações **single-table** continuam métodos individuais simples, com ou
sem prepared stmt:

- `Update(ctx, *Entity) error` — só toca a raiz; usa `stmUpdate.ExecContext`
- `DeleteByID(ctx, id) error` — só toca a raiz
- `FindByID(ctx, id)`, `List(ctx, filter)` — reads

Se o `Delete` precisa cascatear em N tabelas (apagar children antes do
root), aí vira `DeleteAggregate(ctx, rootID) error` com tx.

## Helpers de persistência são MÉTODOS do receiver, nunca funções de pacote

**Regra inviolável.** Toda operação que toca o banco no contexto pertence
ao **struct do repositório** como método `(r *{contexto}Repository) ...`.
Helpers como `insertConfig`, `updateConfig`, `deleteGroupsByConfigID`,
`insertLog` **nunca** são funções de pacote — mesmo quando aparecem como
"sub-passos" de um aggregate method.

### Motivos

- **Coesão e encapsulamento**: o repositório é a fronteira do acesso a
  banco. Funções soltas no mesmo pacote borram essa fronteira — qualquer
  outro arquivo do pacote pode chamar `insertConfig(ctx, e)` direto,
  burlando o ciclo de vida da instância (prepared stmts, tx, cache).
- **Acesso aos campos do struct**: prepared stmts (`r.stmInsertConfig`),
  `r.tx`, conexão de cache, métricas — tudo vive no receiver. Função de
  pacote força `sqln.NewStatement().Execute(...)` inline, sem reuso dos
  stmts preparados no constructor.
- **Refator e descoberta**: `grep "func (r \*pricingRepository)"` lista
  tudo que o repo faz. Funções soltas escapam dessa visão e viram dead
  code latente quando o método público que as chamava some.
- **Mock + interface**: quando o helper é método privado do struct, o
  contrato externo (interface) permanece enxuto. Função de pacote sugere
  que ela tem vida própria — convidando outros pacotes a importarem.

### Forma correta

```go
type pricingRepository struct {
    tx               sqln.Transaction
    stmInsertConfig  *sql.Stmt
    stmUpdateConfig  *sql.Stmt
    stmDeleteConfig  *sql.Stmt
    stmInsertGroup   *sql.Stmt
    stmDeleteGroups  *sql.Stmt
    stmInsertLog     *sql.Stmt
}

func NewPricingRepository(ctx context.Context) PricingRepository {
    stmInsertConfig, err := sqln.NewStatement().Prepare(ctx, configInsertSQL)
    if err != nil {
        logging.Fatal("NewPricingRepository: stmInsertConfig", slog.Any("error", err))
    }
    stmUpdateConfig, err := sqln.NewStatement().Prepare(ctx, configUpdateSQL)
    if err != nil {
        logging.Fatal("NewPricingRepository: stmUpdateConfig", slog.Any("error", err))
    }
    // ... demais stmts
    return &pricingRepository{
        tx:               sqln.NewTransaction(sql.LevelReadCommitted),
        stmInsertConfig:  stmInsertConfig,
        stmUpdateConfig:  stmUpdateConfig,
        // ...
    }
}

// Helpers privados — métodos do receiver, usam stmts preparados.
// Dentro de tx, rebind via tx.Stmt(stmt) para amarrar à conexão da tx.
func (r *pricingRepository) insertConfig(ctx context.Context, e *model.Config) error {
    txObj := ctx.Value(connection.SqlTxContextKey).(*sql.Tx)
    _, err := txObj.Stmt(r.stmInsertConfig).ExecContext(ctx, configArgs(e)...)
    return err
}

func (r *pricingRepository) updateConfig(ctx context.Context, e *model.Config) error {
    txObj := ctx.Value(connection.SqlTxContextKey).(*sql.Tx)
    _, err := txObj.Stmt(r.stmUpdateConfig).ExecContext(ctx, configUpdateArgs(e)...)
    return err
}

func (r *pricingRepository) CreateAggregate(ctx context.Context, a *model.ConfigAggregate) error {
    return r.tx.Execute(ctx, func(ctx context.Context) error {
        if err := r.insertConfig(ctx, a.Config); err != nil {
            return err
        }
        for i := range a.Groups {
            if err := r.insertGroup(ctx, &a.Groups[i]); err != nil {
                return err
            }
        }
        if a.Log != nil {
            return r.insertLog(ctx, a.Log)
        }
        return nil
    })
}

func (r *pricingRepository) Close() error {
    if err := r.stmInsertConfig.Close(); err != nil { return err }
    if err := r.stmUpdateConfig.Close(); err != nil { return err }
    // ... fecha todos
    return nil
}
```

### Anti-padrão — função de pacote sem receiver

```go
// ❌ ERRADO — função solta no pacote, sem receiver,
//    constrói NewStatement() a cada chamada (sem reuso de prepare).
func insertConfig(ctx context.Context, e *model.Config) error {
    return sqln.NewStatement().Execute(ctx, configInsertSQL, configArgs(e)...)
}

func updateConfig(ctx context.Context, e *model.Config) error {
    return sqln.NewStatement().Execute(ctx, configUpdateSQL, configUpdateArgs(e)...)
}

func (r *pricingRepository) CreateAggregate(ctx context.Context, a *model.ConfigAggregate) error {
    return r.tx.Execute(ctx, func(ctx context.Context) error {
        if err := insertConfig(ctx, a.Config); err != nil { return err }
        // ...
    })
}
```

Problemas:
- Helper é função de pacote — não tem acesso a `r.stmInsertConfig`, então
  prepara o stmt a cada chamada (`PrepareContext`+`ExecContext`+`Close`).
- Em bulk, isso vira N×prepare desnecessário (a otimização do bulk method
  resolve dentro dele, mas o aggregate single-row também paga o custo).
- Outros arquivos do pacote podem chamar `insertConfig(...)` direto,
  burlando o ciclo do repo.

### Exceção — `configArgs`, `groupArgs`, `logArgs` (montadores de slice)

Funções **puras** que só transformam um valor em `[]any` (args para
`ExecContext`) **podem** ficar como funções de pacote: não tocam banco,
não precisam de `r`, são internas ao mapeamento entidade↔SQL e ajudam a
não duplicar listas longas de campos. Continuam privadas (lowercase) e
não exportadas do pacote.

```go
// ✅ OK — função pura, sem I/O, sem estado, só monta args
func configArgs(e *model.Config) []any {
    return []any{e.ID, e.ProductID, /* ... */}
}
```

Regra simples: **se a função recebe `ctx context.Context` e toca banco,
ela é método do receiver**. Se a função é puramente transformação de
dados, pode ser função de pacote.

### Checklist específico

- [ ] Nenhuma função no arquivo do repo com assinatura
      `func xxx(ctx context.Context, ...) error` que execute SQL — todas
      são métodos `(r *{contexto}Repository)`
- [ ] Todo SQL de mutation tem stmt preparado em campo do struct
      (`stmInsertX`, `stmUpdateX`, `stmDeleteX`), preparado **uma única
      vez** no `New{Contexto}Repository(ctx)`
- [ ] Dentro de `r.tx.Execute(...)`, helpers fazem rebind via
      `ctx.Value(connection.SqlTxContextKey).(*sql.Tx).Stmt(r.stmXxx)`
      antes de `ExecContext` (necessário pra amarrar o stmt à conexão da
      tx; chamar `r.stmXxx.ExecContext` direto pega outra conexão do
      pool e não participa da transação)
- [ ] `Close()` fecha todos os `*sql.Stmt` em sequência

## Anti-padrões

### 1. Transação no service via `sqln.NewTransaction` direto

```go
// ❌ ERRADO — transação no service
func (s *xxxService) Create(ctx context.Context, req Request) errs.AppError {
    return sqln.NewTransaction().Execute(ctx, func(tx context.Context) error {
        if err := s.repoA.Save(tx, ...); err != nil { return err }
        if err := s.repoB.SaveMany(tx, ...); err != nil { return err }
        return s.repoC.Save(tx, ...)
    })
}
```

Problemas:
- Service conhece detalhe de persistência (isolation level, ordem).
- Test do service exige mockar `sqln.NewTransaction` ou injetar `txRunner`
  — boilerplate desnecessário.
- Cada caller pode escolher isolation level diferente para a mesma
  operação lógica (inconsistência).

### 2. `txRunner` injetável no service

```go
// ❌ ERRADO — overkill
type txRunner func(ctx context.Context, fn func(context.Context) error) error

type xxxService struct {
    repo  XxxRepository
    runTx txRunner  // injetável no test com noopTx
}
```

Solução verdadeira é mover a tx pro repo. O `txRunner` injetável só serve
para esconder o problema (acoplamento service ↔ tx) atrás de uma camada
extra de indireção.

### 3. Aggregate method que esconde múltiplos chamadores

Se o repo tem `CreateAggregate(ctx, *Aggregate)` mas no service o caller
quer só criar o root **sem** children/snapshot, dividir em métodos
distintos — não passar `Aggregate{Root: ..., Children: nil, Snapshot: nil}`
e esperar o repo "saber" o que fazer. Aggregate sempre carrega o
conjunto **completo** que pertence àquela operação lógica.

### 4. Repository de cada child sem aggregate root claro

Quando 3 entidades sempre mudam juntas, ter 3 repositórios separados com
SaveMany em cada **e** orquestrar tx no service é o pior dos mundos. Um
único repository `{Aggregate}Repository` com aggregate methods + métodos
simples para reads/updates pontuais cobre tudo. Espelhar o DDD aggregate
boundary, não o schema das tabelas.

## Concorrência — `r.tx` é safe pra goroutines

O `sqln.Transaction` retornado por `sqln.NewTransaction(level)` é **stateless
por chamada** — só guarda o `isolationLevel` como configuração read-only.
Cada `Execute(ctx, fn)` chama `db.BeginTx` no pool e passa o `*sql.Tx` via
`context.WithValue(...)`, **nunca via campo da struct**.

Consequência: o `r.tx sqln.Transaction` no campo do repo pode ser
compartilhado entre N goroutines com segurança. 100 chamadas paralelas a
`r.repo.CreateAggregate(...)` → 100 transações independentes do pool, sem
shared mutable state.

**Não precisa** sincronização extra (mutex, sync.Pool, etc.) no repo.

## Isolation level — default `ReadCommitted`

Escolha consciente, declarada no constructor:

```go
tx: sqln.NewTransaction(sql.LevelReadCommitted)
```

- **`sql.LevelReadCommitted`** — default. Suficiente para aggregates que
  só tocam linhas próprias (PK/FK do próprio agregado), sem leitura
  cross-row complexa. Sob concorrência alta, **não gera serialization
  failures** — cada transação simplesmente vê snapshots consistentes.
- **`sql.LevelRepeatableRead`** — necessário quando o aggregate **lê e
  modifica** múltiplas linhas e a consistência depende de ver o mesmo
  snapshot durante toda a transação (ex.: ranking que lê posições antes
  de atualizar).
- **`sql.LevelSerializable`** — só quando há **invariante cross-row** que
  o banco precisa proteger contra anomalias serializáveis (ex.: limite
  global por tenant calculado a partir de N linhas). Pagar o custo:
  Postgres pode abortar transações com `SQLSTATE 40001
  serialization_failure` em concorrência alta, e o caller precisa
  retentar. **Não usar Serializable como "default seguro"** — vira
  flakey em bulk.

Regra geral: comece com `ReadCommitted` + UNIQUE constraint para
prevenir duplicates (a UNIQUE faz o trabalho de proteção em vez do
isolation level). Suba o nível só se houver invariante específica que o
schema não cobre.

## Cenário bulk — `CreateAggregatesBulk(ctx, []*Aggregate) error`

Quando a spec prevê **caller que cria N aggregates em massa** (importação
de planilha, sincronização batch, replicação), o custo de
`sqln.NewStatement().Execute(...)` por INSERT vira pesado:

- Cada `Execute()` faz `PrepareContext` + `ExecContext` + `Close` — sem cache
- Cada `CreateAggregate(...)` abre uma transação própria (BEGIN/COMMIT)

Para `N = 1000` aggregates com 3 INSERTs cada = 1000 × (2 + 3×3) ≈ **11k
round-trips**.

Adicionar método dedicado **na mesma interface do repo**:

```go
type {Aggregate}Repository interface {
    CreateAggregate(ctx context.Context, a *model.{Aggregate}Aggregate) error       // 1 item
    CreateAggregatesBulk(ctx context.Context, items []*model.{Aggregate}Aggregate) error  // N items

    // ... métodos existentes
}
```

Implementação combina **uma transação só** + **prepare reutilizado por
SQL**:

```go
func (r *{aggregate}Repository) CreateAggregatesBulk(ctx context.Context, items []*model.{Aggregate}Aggregate) error {
    return r.tx.Execute(ctx, func(ctx context.Context) error {
        tx := ctx.Value(connection.SqlTxContextKey).(*sql.Tx)

        // Prepare 1x por SQL distinto, reutilizar para N rows
        rootStmt, err := tx.PrepareContext(ctx, insertRootSQL)
        if err != nil { return err }
        defer rootStmt.Close()

        childStmt, err := tx.PrepareContext(ctx, insertChildSQL)
        if err != nil { return err }
        defer childStmt.Close()

        for _, a := range items {
            if _, err := rootStmt.ExecContext(ctx, /* args do root */); err != nil {
                return err
            }
            for i := range a.Children {
                if _, err := childStmt.ExecContext(ctx, /* args do child */); err != nil {
                    return err
                }
            }
        }
        return nil
    })
}
```

Para `N = 1000`: agora ≈ 2 (BEGIN/COMMIT) + 3 (Prepare) + 3000 (Exec) =
**~3005 round-trips**. Redução de mais de 70%.

**Cuidados invioláveis do bulk:**

- **Granularidade**: 1 row falhando aborta toda a transação. Para
  `N >> 100`, o caller deve **dividir em chunks** (ex.: 100 em 100), cada
  chunk em sua própria chamada a `CreateAggregatesBulk` — assim 1 row
  ruim no chunk 7 não destrói o trabalho dos chunks 1–6. A decisão de
  tamanho do chunk vai na spec do bulk consumer.
- **Erros parciais**: `CreateAggregatesBulk` é **all-or-nothing por
  chunk**. Se a UX exige "salvar o que deu certo e marcar o que falhou
  linha-a-linha", essa lógica vive **acima** do repo (no service ou
  application), iterando `CreateAggregate(...)` 1-a-1. Repo bulk **não**
  faz salvamento parcial.
- **Idempotência**: import de planilha geralmente reprocessa quando
  retry — UNIQUE constraint no schema + tratamento de SQLSTATE 23505 no
  caller dá idempotência sem precisar de UPSERT.
- **Multi-VALUES** (`INSERT ... VALUES (...), (...), (...)`) é otimização
  adicional possível, mas exige SQL dinâmico (montar a string com N
  placeholders). Reserve para quando o profile mostrar que o
  Prepare-Reuse-pattern acima ainda não é suficiente. Em
  `Postgres + lib/pq`, o `COPY FROM` é a opção mais rápida ainda, mas
  com complexidade maior.

**Quando não criar bulk method**: se a spec não declara consumer de bulk
para o agregado, **não criar** — é YAGNI. Adicionar quando o spec do
consumer entrar.

## Referência viva no projeto

Buscar implementações canônicas via grep:
- `services/domain/{ctx}/repository/{ctx}_repository.go` — repository com
  prepared statements + tx em campo do struct + aggregate methods.
- Critério de match: presença de `sqln.NewTransaction(...)` no constructor +
  `r.tx.Execute(ctx, func(ctx))` nos aggregate methods + helpers como
  métodos do receiver.

## Checklist de implementação

- [ ] Existe `{Aggregate}Aggregate` em `model/`?
- [ ] Repo tem `tx sqln.Transaction` no struct + injetado no constructor?
- [ ] Isolation level no constructor é **`sql.LevelReadCommitted`** (default
      seguro) — só subir se houver invariante cross-row que o schema não cobre?
- [ ] Aggregate methods envolvem todas as ops em `r.tx.Execute(ctx, fn)`?
- [ ] Service **não importa** `sqln.NewTransaction`?
- [ ] Test do service mocka `CreateAggregate` direto, sem `noopTx`/`txRunner`?
- [ ] **Helpers de persistência são métodos do receiver** (`func (r *xxxRepository) insertY(...)`),
      **nunca** funções de pacote. Exceção: funções **puras** sem `ctx` e
      sem I/O (ex.: `configArgs(e) []any`) podem ficar como funções de pacote.
- [ ] **Todos os SQLs de mutation têm `*sql.Stmt` preparado no constructor**
      (`stmInsertX`, `stmUpdateX`, `stmDeleteX`) — nada de
      `sqln.NewStatement().Execute(...)` inline em cada chamada (prepara + executa + descarta a cada vez).
- [ ] Dentro de `r.tx.Execute(...)`, helpers fazem rebind via
      `ctx.Value(connection.SqlTxContextKey).(*sql.Tx).Stmt(r.stmXxx)`
      antes de `ExecContext`.
- [ ] `Close()` fecha todos os `*sql.Stmt` em sequência.
- [ ] Se a spec declara consumer de bulk: existe `CreateAggregatesBulk`
      com 1 tx + prepare reutilizado por SQL? Se a spec não declara: não
      criar (YAGNI).
