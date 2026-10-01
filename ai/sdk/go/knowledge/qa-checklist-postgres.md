---
name: qa-checklist-postgres
description: Itens de auditoria Go + PostgreSQL do gofi-qa no SDK v0.8.2 — repository aggregate, escrita e transação no sqln, driver postgres, índices e valores monetários
sdk: v0.8.2
keywords: [qa, auditoria, postgres, sqln, transacao, statement, aggregate, indices, pgbouncer, dinheiro]
---

# Escopo de auditoria — Go + PostgreSQL (SDK v0.8.2)

Itens que se somam a `qa-checklist.md` quando o contexto toca banco
PostgreSQL. Bootstrap, `Config` e ambiente estão em `qa-checklist.md` §2–§3;
filtro dinâmico em §6. Como verificar: `gofi show <símbolo>` (quem chama),
`gofi find --in code "<termo>"`, e só então `gofi find --text`/`--regex
... --in code`, declarado. Prova de ausência segue
`.claude/skills/gofi-qa/reference/graph-audit.md` §"Validar o modo do grafo antes de escrever \"não há violação\"":
em escopo `fast`, confirme com `--deep` ou declare a limitação.

### Escrita e statements (ver `persistence-rules.md` e `database-connection.md`)
- [ ] **Mutação por `Statement.Execute(ctx, sql, args...)`** (campo
  `statement.Statement` do repo, de `sqln.NewStatement()`) — roda na
  transação do `ctx` quando há uma (`connection.QuerierFrom`) e não cria
  prepared statement nomeado no servidor (o driver postgres usa
  `cache_describe`), então funciona atrás de PgBouncer em modo transação.
  É o padrão; SQL estático executado assim **não** é defeito.
- [ ] **Construtor do repository não toca o banco** — sem `Prepare`, sem ping:
  a conexão global só existe depois do `Build`, e o construtor pode rodar
  antes. `Prepare` no construtor: **MAJOR** (falha com
  `ErrDatabaseNotInitialized` quando montado antes do `Build`)
- [ ] **Sem `*sql.Stmt` em campo do repository** — não ganha round-trip,
  quebra com PgBouncer em modo transação e escapa da transação.
  `gofi find --regex "\*sql\.Stmt" --in code`: campo no struct é **MAJOR**;
  `r.stmXxx.ExecContext` chamado dentro de `transaction.Execute` (outra
  conexão do pool, fora da atomicidade) é **BLOCKER**. `Prepare` só dentro da
  transação, em laço de bulk, com `defer stmt.Close()`
- [ ] Repository não expõe nem chama `Close` de conexão — quem fecha o pool é
  o componente no `Shutdown`. Presente: **MAJOR**
- [ ] **Helpers de persistência são métodos do receiver** —
  `gofi show {pathContext}repository -n 40` lista os símbolos (o `-n` padrão
  é 12): método aparece como `repository.{Contexto}Repository.Metodo`, função
  solta como `repository.helper`. Função solta que recebe `ctx` e executa SQL:
  **MAJOR**. Função pura sem `ctx`/I-O (ex.: `configArgs(e) []any`) pode ser de
  pacote
- [ ] Violação de constraint mapeada por código SQLSTATE
  (`connection.AsPgError(err)` → `pgErr.Code`, ex.: `23505` unique) para
  `errs.RegisterConflict`. Comparação por texto da mensagem de erro:
  **MINOR**; unique violation virando 500: **MAJOR**

### Repository aggregate e transação (ver `repository-aggregate-pattern.md` e `transactions.md`)
- [ ] Mutação multi-tabela atômica tem struct `{Aggregate}Aggregate` em
  `model/` e métodos `CreateAggregate`/`UpdateAggregate`/`DeleteAggregate`
  no repository, todos dentro de `r.tx.Execute(ctx, fn)`. Service salvando N
  entidades relacionadas em sequência, sem transação: **MAJOR**
- [ ] A transação vive **no repository**: campo `tx sqln.Transaction`
  criado no construtor com `sqln.NewTransaction(...)` ou
  `transaction.New(transaction.Options{...})`. `gofi show sqln.NewTransaction`
  e `gofi show transaction.New` — chamador em `service/`: **MAJOR**
- [ ] Service não injeta `txRunner` (`func(ctx, fn) error`) nem usa `noopTx`
  em teste; o teste do service mocka `CreateAggregate` devolvendo `error`.
  Presente: **MAJOR** (a transação vazou para o service)
- [ ] **Isolamento padrão `sql.LevelReadCommitted`.** `LevelSerializable`/
  `LevelRepeatableRead` só com RN/ADR que justifique invariante entre linhas
  **e** com `transaction.Options{MaxRetries: n}` (reexecuta em
  `connection.IsRetryable`: serialização e deadlock). Serializable sem RN:
  **MAJOR**; Serializable sem `MaxRetries` (o `40001` sobe como erro ao
  cliente): **MAJOR**
- [ ] Com `MaxRetries > 0`, `fn` só mexe no banco (ver `qa-checklist.md` §5).
  Publicação em fila/HTTP dentro de `fn`: **MAJOR**
- [ ] Leitura que decide a escrita (saldo, estoque, unicidade de negócio) roda
  **dentro** de `r.tx.Execute` — dentro da transação o `sqln.Find*` usa o
  primário; fora, a réplica. Fora com `DATABASE_READ_HOST`: **MAJOR**
- [ ] Spec declara carga em lote (importação, sincronização): existe
  `CreateAggregatesBulk(ctx, []*Aggregate) error` com **uma** transação.
  Ausente: **MAJOR**; bulk sem consumidor declarado na spec: **MINOR** (YAGNI)

### Driver e conexão PostgreSQL (ver `database-connection.md`)
- [ ] `main` importa `_ "github.com/joaoprofile/gofi-sdk-go/sqln/driver/postgres"`
  junto com `database.New()`. Ausente: **MAJOR** (o `Build` falha)
- [ ] Produção com `DATABASE_SSL_MODE` explícito (`require` ou mais forte) —
  vazio vira `disable` no driver. Vazio no manifesto de produção: **MAJOR**
- [ ] `DATABASE_PASSWORD` por `secret://` ou `DATABASE_PASSWORD_FILE`; em AWS
  pode ser token IAM (`sqln/rdsauth`). Senha literal no manifesto: **BLOCKER**
- [ ] Migrations (ver `migrations.md`) em `.migrations/` de `pathService`, aplicadas com
  `DATABASE_MIGRATION=true`. Migration de produção que recria índice sem
  `CONCURRENTLY`: **MAJOR**

### Índices e perfil de acesso (ver `postgres-index-strategy.md`)
- [ ] Cada tabela tem **perfil de acesso declarado** na spec (`cold` /
  `hot UPDATE` / `hot DELETE+INSERT` / `append-only`). Ausente: **MAJOR** (a
  migration não tem como decidir índice)
- [ ] Tabela multi-tenant: todo índice tem **leading column = tenant**
  (composite `(tenant, x)` ou partial). Single-column em coluna não-tenant:
  **MAJOR**
- [ ] Filtro de texto por `sqln.Contains`/`criteria.Contains` (vira `ILIKE` no
  postgres), `LIKE '%x%'` ou regex: índice **GIN `gin_trgm_ops`**
  (`pg_trgm`). Btree single-column nessa coluna: **MAJOR** (o planner não usa)
- [ ] Hot UPDATE: poucos índices em colunas voláteis (`status`, contadores) —
  cada um quebra HOT update. `fillfactor=70-80` com colunas indexadas
  estáveis: ausente é **SUGGESTION**; em tabela cold é **MINOR**
- [ ] Hot UPDATE / hot DELETE+INSERT: autovacuum tunado
  (`autovacuum_vacuum_scale_factor=0.01` e afins). Ausente: **MINOR**
- [ ] Worker transversal (purge, archive, replicação seletiva) declarado no
  projeto: toda tabela com a coluna do worker tem índice nela. Ausente:
  **MAJOR**
- [ ] Append-only de volume indefinido: tabela particionada
  (`PARTITION BY RANGE`), índices no parent. Ausente: **MINOR**
- [ ] `BOOLEAN` indexado sem partial: **MINOR**
- [ ] Coluna ordenável do `FilterMapping` (`Sortable: true`) em tabela grande
  tem índice compatível com a ordenação + filtro de tenant. Ausente:
  **MINOR**

### Valores monetários, moeda e país
O pacote monetário do projeto está declarado em `.claude/memory/project.md`
(convenções). Os itens valem para qualquer pacote que cumpra esse papel.
- [ ] Valor monetário em tipo decimal/inteiro de menor unidade — nunca
  `float64` persistido ou somado; coluna `numeric(p,s)` ou `bigint`. `float64`
  em dinheiro: **MAJOR**
- [ ] Parse de valor por texto usa o parser do pacote monetário, com a moeda
  conhecida ou resolvida. `strconv.ParseFloat` cru em string de dinheiro:
  **MAJOR** (quebra com separador decimal de outra localidade)
- [ ] Arredondamento pelas casas decimais **da moeda**, vindas do catálogo.
  `math.Round` com casas fixas: **MAJOR** (moeda sem centavos ganha centavo
  fantasma)
- [ ] País → moeda, símbolo, casas e separador vêm do catálogo único; mapa
  local redeclarado no contexto: **MAJOR**
- [ ] Código novo importa o pacote monetário canônico, não alias legado de
  compatibilidade. Alias em código novo: **MINOR**
