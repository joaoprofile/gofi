---
name: loader-pattern
description: Loader — snapshot de várias tabelas para motor de decisão, com sqln.Find, errgroup ou transação read-only
sdk: v0.8.2
keywords: [loader, snapshot, decider, errgroup, sqln, Find, read-only-transaction, replica]
---

# Loader pattern — snapshot consistente para motores de decisão

Pattern Go para carregar **snapshot consistente** de múltiplas tabelas em
uma chamada, usado por motores de decisão (decider em event-driven
decider/executor) que precisam operar sobre estado local estável.

## Quando usar

Aplicar **quando todas** as condições abaixo se aplicam:

1. **Motor de decisão** (decider) sobre estado local — não chama sistema
   externo durante a decisão. Ver
   `.claude/expertise/event-driven/executor-pattern.md`.
2. **Snapshot lê 3+ tabelas relacionadas** que devem ser **consistentes
   entre si** durante a avaliação (mudanças concomitantes devem afetar só
   o próximo ciclo).
3. **Avaliação tem 1 entidade-alvo dominante** (`productID`, `userID`,
   `orderID`) — o snapshot é "tudo que preciso saber sobre essa entidade
   agora".
4. **Volume justifica otimização** — o decider roda em loop (scheduler tick
   ou consumer Kafka reativo) sobre N entidades; N round-trips × M tabelas
   custa caro.

Cenários típicos: motor de pricing (decidir preço por anúncio), motor de
promoção (decidir adesão por anúncio), motor de risco (decidir aprovação
por transação), motor de ranking (decidir posição por item).

**Não usar quando:**

- Domínio CRUD trivial (handler → service → repository é suficiente).
- Decisão lê **1 tabela** — `repository.FindByID` direto basta.
- Decisão envolve I/O externo (bridge HTTP) — não é decider; é
  application com workflow.
- Cada chamada precisa de **conjunto diferente** de tabelas (sem padrão
  estável de snapshot). Loader fixa o conjunto; queries dinâmicas vão pro
  repository.

---

## Layout canônico

```
{pathService}/domain/{ctx}/loader/
  contract.go       — interface Loader + struct {Ctx}EvaluationContext
  loader.go         — Load + List* (sqln.Find*)
  queries.go        — SQL bruto (constantes string)
  snapshot.go       — DTOs do loader ({Entidade}Snapshot, ...)
  errors.go         — erros próprios do loader
  loader_test.go    — testes
```

**Pacote `loader/`** é irmão de `service/`, `repository/`, `engine/`,
`application/` no domínio. Não é sub-pacote de repository.

---

## Interface canônica

```go
// loader/contract.go
package loader

import (
    "context"

    "github.com/joaoprofile/gofi-sdk-go/base/errs"
)

type Loader interface {
    // Load carrega o snapshot completo de 1 entidade.
    // Devolve Err{Ctx}NotEligible quando o gate falha.
    Load(ctx context.Context, entityID int64) (*{Ctx}EvaluationContext, errs.AppError)

    // ListBy{Dimension} lista entidades elegíveis por dimensão de iteração
    // (tenant, canal, ...). Um método por dimensão.
    ListBy{Dimension}(ctx context.Context, dim {DimType}) ([]int64, errs.AppError)
}
```

`{Ctx}EvaluationContext` agrega:

```go
type {Ctx}EvaluationContext struct {
    EntityID int64  `db:"entity_id"`
    TenantID string `db:"tenant_id"`

    // single-row (query monolítica com JOINs 1:1) — VO aninhado com tag db
    {Tabela1} {Tabela1}Snapshot `db:"t1"`
    {Tabela2} {Tabela2}Snapshot `db:"t2"` // LEFT JOIN: folhas *T (NULL); ponteiro para struct não é expandido

    // many (sub-queries) — sem tag db
    {ListaN} []{ListaN}Item
}
```

---

## Implementação canônica — `sqln.Find` + errgroup

```go
// loader/loader.go
package loader

import (
    "context"
    "errors"

    "github.com/joaoprofile/gofi-sdk-go/base/errs"
    "github.com/joaoprofile/gofi-sdk-go/sqln"
    "golang.org/x/sync/errgroup"
)

type {ctx}Loader struct{}

func NewLoader() Loader { return &{ctx}Loader{} } // não toca o banco

func (l *{ctx}Loader) Load(ctx context.Context, entityID int64) (*{Ctx}EvaluationContext, errs.AppError) {
    if entityID <= 0 {
        return nil, ErrLoaderEntityIDInvalid.New()
    }

    // 1) snapshot principal — tags db dos *Snapshot casam com os aliases do SELECT
    ec, err := sqln.Find[{Ctx}EvaluationContext](ctx, snapshotQuery, entityID).UniqueResult()
    if err != nil {
        return nil, ErrLoaderSnapshotFailed.Wrap(err)
    }
    if ec == nil {
        return nil, Err{Ctx}NotEligible.New() // gate no WHERE não passou
    }

    // 2) sub-queries independentes em paralelo
    g, gctx := errgroup.WithContext(ctx)
    g.Go(func() (err error) {
        ec.ManyA, err = sqln.Find[ManyAItem](gctx, manyAQuery, entityID).List()
        return err
    })
    g.Go(func() (err error) {
        ec.ManyB, err = sqln.Find[ManyBItem](gctx, manyBQuery, ec.TenantID).List()
        return err
    })
    if err := g.Wait(); err != nil {
        return nil, ErrLoaderSnapshotFailed.Wrap(err)
    }
    return ec, errs.AppError{}
}
```

- Sem prepared statement e sem `Close()`: `sqln.Find` executa direto e o
  driver faz cache do describe por conexão — o laço sobre N entidades não
  paga prepare repetido.
- Campos many do `EvaluationContext` (preenchidos depois) ficam **sem** tag
  `db` (`value-objects.md`).

### Consistência do snapshot

- **Queries paralelas não compartilham snapshot**: cada uma lê o commit mais
  recente no seu momento. Aceitável quando o decider tolera mudança
  concorrente só no próximo ciclo (caso comum).
- **Réplica:** fora de transação `sqln.Find` lê da réplica
  (`DATABASE_READ_HOST`) — decisão tomada logo após uma escrita pode ver dado
  anterior.
- Quando a decisão exige **um snapshot único** (ou ler o primário), carregue
  tudo **sequencialmente** dentro de uma transação read-only:
  ```go
  snap := transaction.New(transaction.Options{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
  err := snap.Execute(ctx, func(ctx context.Context) error { /* mesmas sqln.Find, em sequência */ })
  ```
  Uma `*sql.Tx` não aceita queries concorrentes — não misture com errgroup.
  Declare na spec qual dos dois o motor exige.

### Como decidir: snapshot monolítico (JOINs) vs sub-queries paralelas?

| Sinal | Vai pro JOIN do snapshot | Vai pra sub-query paralela |
|---|---|---|
| **Cardinalidade** | 1:1 com a entidade | 1:N |
| **Tamanho do row resultado** | Pequeno (< 200 colunas no SELECT) | Qualquer |
| **JOIN explode linhas?** | Não (cada JOIN reduz a 1 linha — INNER/LEFT por PK) | Sim |
| **Dependência** | Independente do resultado do snapshot | Pode depender (ex.: usar `tenantID` do snapshot) |
| **Pode ser nulo** | LEFT JOIN + ponteiro/`sql.Null*` no snapshot | Sub-query devolve `[]` vazio |

**Regra prática:** se a tabela tem **uma linha por entidade** (1:1, 0..1),
vai pro JOIN. Se tem **N linhas por entidade** (campanhas elegíveis,
histórico, exceções, members), vai pra sub-query paralela. Esquema gera
queries enxutas e paraleliza bem.

### Carregamento condicional

Quando uma sub-query só faz sentido em **alguns casos** (ex.: membros de um
grupo só quando a entidade é de um tipo agrupado), o `if` vem antes do `g.Go`:

```go
if ec.{Tabela1}.GroupType == groupTypeRelation {
    g.Go(func() (err error) {
        ec.GroupMembers, err = sqln.Find[GroupMember](gctx, groupMembersQuery, ec.ConfigID).List()
        return err
    })
}
```

Evita query inútil quando o branch do domínio nem usa o resultado.

---

## Resolução de cascata no SQL

Quando a entidade tem **configuração cascateada** (ex.: override por entidade
→ config por tenant → default), o Loader resolve via `COALESCE` no SQL em
vez de chamar `XxxService.GetEffective` (que faria 2 queries):

```sql
SELECT
    COALESCE(pc.weight_a, tc.weight_a, 4) AS weight_a,
    COALESCE(pc.weight_b, tc.weight_b, 3) AS weight_b,
    -- ...
    CASE
        WHEN pc.entity_id IS NOT NULL THEN 'entity'
        WHEN tc.tenant_id IS NOT NULL THEN 'tenant'
        ELSE 'default'
    END AS config_source
FROM entity e
LEFT JOIN entity_config pc ON pc.entity_id = e.id
LEFT JOIN tenant_config tc ON tc.tenant_id = e.tenant_id
WHERE e.id = $1;
```

**Trade-off:** lógica de cascata replicada em 2 lugares (SQL no loader +
Go no `XxxService.GetEffective`). `gofi-qa` audita aderência quando a
cascata muda. Aceito porque ambos lêem das **mesmas tabelas** — divergência
semântica é estruturalmente improvável.

`XxxService.GetEffective` continua sendo o único método de domain para a
cascata **fora** do motor (HTTP handler `/effective`, UI). O Loader replica
via SQL apenas dentro do pipeline.

**Cláusula de proteção:** se a regra de cascata virar não-trivial (branches
condicionais por data, RBAC), centralizar via stored procedure ou abandonar
a otimização SQL e voltar a chamar o service.

---

## Erros próprios do Loader

```go
// loader/errors.go
package loader

import "github.com/joaoprofile/gofi-sdk-go/base/errs"

var (
    ErrLoaderEntityIDInvalid = errs.RegisterValidation("LOADER_ENTITY_ID_INVALID", "invalid entity id")
    Err{Ctx}NotEligible      = errs.RegisterNotFound("{CTX}_NOT_ELIGIBLE", "entity not eligible") // gate falhou
    ErrLoaderSnapshotFailed  = errs.RegisterOperation("LOADER_SNAPSHOT_FAILED", "failed to load snapshot")
    ErrLoaderList{Dim}Failed = errs.RegisterOperation("LOADER_LIST_{DIM}_FAILED", "failed to list entities")
)
```

Erros do loader são **separados** dos erros do service/repository — o
caller (application/processor) discrimina por tipo:
- `Err{Ctx}NotEligible` → skip silencioso (entidade não passa no gate).
- `ErrLoader*Failed` → log de erro + métrica; processor continua o lote.

---

## Como o caller usa

### Application orchestrador

```go
type evaluateApplication struct {
    loader  loader.Loader
    decider service.DecisionService
    log     service.LogService
}

func (a *evaluateApplication) Execute(ctx context.Context, entityID int64, emit EmitFunc) errs.AppError {
    ec, err := a.loader.Load(ctx, entityID)
    if err.Exists() {
        if err.Code == "{CTX}_NOT_ELIGIBLE" {
            return errs.AppError{}  // skip silencioso
        }
        return err
    }

    // Pipeline puro sobre *ec — sem mais I/O até decidir
    decision := a.decider.Decide(ec)
    if decision.ShouldEmit() && emit != nil {
        _ = emit(decision.Envelope, decision.Payload)
    }
    return a.log.Record(ctx, decision.AsLogEntry())
}
```

### Processor scheduler-driven

```go
type {ctx}Processor struct {
    loader      loader.Loader
    application application.EvaluateApplication
    dim         {DimType}
}

func (p *{ctx}Processor) Process(ctx context.Context, emit scheduler.EmitFunc) error {
    ids, err := p.loader.ListBy{Dimension}(ctx, p.dim)
    if err.Exists() {
        return &err  // erro fatal — derruba o tick
    }
    for _, id := range ids {
        if e := p.application.Execute(ctx, id, emit); e.Exists() {
            logging.Warn("{ctx} processor: evaluate failed",
                slog.Int64("id", id), slog.String("code", e.Code))
            // continua o lote (best-effort por entidade)
        }
    }
    return nil
}
```

---

## Testes

### Test do Loader (`loader_test.go`)

Integration test contra Postgres real (preferido) ou `sqlmock`:
- Setup: insere fixtures nas tabelas.
- Asserts: `Load(id)` devolve `*EvaluationContext` com campos esperados;
  `Load(id_inexistente)` devolve `Err{Ctx}NotEligible`; `Load(gate_falhou)`
  devolve `Err{Ctx}NotEligible`.

### Test da Application (mockando Loader)

```go
type mockLoader struct {
    loadFn          func(context.Context, int64) (*loader.EvaluationContext, errs.AppError)
    listByDimFn     func(context.Context, int32) ([]int64, errs.AppError)
    loadCalls       int
}

func (m *mockLoader) Load(ctx context.Context, id int64) (*loader.EvaluationContext, errs.AppError) {
    m.loadCalls++
    return m.loadFn(ctx, id)
}
func (m *mockLoader) ListBy{Dim}(...) {...}
```

Application test não toca em SQL — só verifica o pipeline sobre o
`*EvaluationContext` retornado pelo mock.

---

## Anti-padrões

- **Chamar `service.GetEffective` ou `repository.FindByX` dentro do Loader.**
  Loader é dono do SQL; service vive em cima. Cruzar a fronteira gera
  importação circular (`loader` → `service` → `loader`) e perde a
  garantia de snapshot consistente (cada chamada do service abre nova
  query).
- **`rows.Scan` manual / `*sql.Stmt` em campo.** Use `sqln.Find[T]` com tags
  `db`; statement preparado em campo quebra com PgBouncer e não ganha
  round-trip (`persistence-rules.md`).
- **Sub-queries sequenciais.** Se 4 sub-queries são independentes, sequencial
  custa `t1 + t2 + t3 + t4`; paralelo custa `max(t1,t2,t3,t4)`. `errgroup`
  é gratuito.
- **errgroup dentro de transação.** `*sql.Tx` não aceita queries
  concorrentes; snapshot transacional é sequencial.
- **Snapshot incompleto que força query extra no service.** Se o service
  precisa de algo que não está no `EvaluationContext`, adicione ao snapshot
  (mais um campo no SELECT) — não chame outro repo no meio do pipeline.
- **Loader devolvendo entidades de domínio (`model.Product`, `model.Config`).**
  Devolva `Snapshot`s próprios (`ProductSnapshot`, `ConfigSnapshot`) — eles
  são DTOs do Loader, isolam o consumidor do schema do banco. Entidades
  ricas seguem em `model/` e são construídas pelos repositórios "normais".
- **`Loader.Load` recebendo `ctx`, `entityID` E parâmetros opcionais.**
  Loader é fixo — o snapshot é sempre o mesmo formato. Branching condicional
  (ex.: "carrega membros do grupo só se for RELATION") fica **dentro** do
  Loader (`if ec.GroupType == 'RELATION' { ... }`), nunca exposto na
  assinatura.

---

## Referência cruzada

- `.claude/expertise/event-driven/executor-pattern.md` — quando
  aplicar split decider/executor (Loader é peça do decider).
- `.claude/sdk/go/knowledge/repository-aggregate-pattern.md` — diferença
  entre Loader (read-only, snapshot) e Aggregate Repository (write,
  transação multi-tabela).
- `.claude/sdk/go/knowledge/postgres-index-strategy.md` — índices nas
  tabelas que o snapshot lê precisam suportar o JOIN do `snapshotQuery`.
- `.claude/sdk/go/knowledge/transactions.md` — transação read-only para
  snapshot único.
