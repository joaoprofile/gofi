---
name: database-connection
description: Conexão SQL do sqln — componente database, DATABASE_*, registro de driver por import, réplica de leitura, IAM (rdsauth)
sdk: v0.8.2
keywords: [sqln, connection, database, DATABASE_DRIVER, driver, postgres, replica, rdsauth, pool]
---

# Conexão com o banco — `sqln/connection` + componente `database`

API: `.claude/sdk/go/api/gofi-component-database.md`, `sqln-connection.md`,
`sqln-driver*.md`, `sqln-rdsauth.md`. Exemplos executáveis:
`examples/sqln/search` (job) e `examples/sqln/filter-api` (HTTP) — ver
`.claude/sdk/go/api/examples.md`.

## Regra — o serviço abre o banco pelo componente, nunca à mão

```go
import (
    "github.com/joaoprofile/gofi-sdk-go/gofi"
    "github.com/joaoprofile/gofi-sdk-go/gofi/component/database"
    _ "github.com/joaoprofile/gofi-sdk-go/sqln/driver/postgres" // DATABASE_DRIVER=postgres
)

svc, err := gofi.New("{servico}").
    With(database.New() /*, httpserver.New(...) */).
    Build()
```

- `database.New()` lê `DATABASE_*` no `Build`, abre o pool, publica a
  **conexão global** (`connection.SetGlobal`), registra health check e métricas
  do pool e fecha tudo no `Shutdown`. Repositórios usam a global — nenhum
  `*sql.DB` circula entre camadas.
- **O driver é registrado por import em branco no `main`.** O componente não
  linka driver nenhum: sem o import, `Build` falha com
  `database driver not registered`. Um import por banco usado:
  `sqln/driver/postgres` (pgx/v5), `sqln/driver/mysql`,
  `sqln/driver/sqlserver`, `sqln/driver/oracle`.
- `DATABASE_DRIVER` vazio = `postgres`. Valores: `postgres`, `mysql`,
  `sqlserver`, `oracle` (`connection.DriverName`).

## Variáveis (`base/environment`)

| Variável | Uso |
|---|---|
| `DATABASE_DRIVER` | driver registrado (default `postgres`) |
| `DATABASE_HOST` / `DATABASE_PORT` / `DATABASE_NAME` / `DATABASE_USER` / `DATABASE_PASSWORD` | DSN montado pelo driver (`Driver.DSN`) |
| `DATABASE_SSL_MODE` | postgres: vazio vira `disable` |
| `DATABASE_MAX_OPEN_CONNS` / `DATABASE_MAX_IDLE_CONNS` / `DATABASE_MAX_LIFETIME` / `DATABASE_MAX_IDLE_TIME` | pool (`connection.PoolConfig`) |
| `DATABASE_READ_HOST` / `DATABASE_READ_PORT` | réplica de leitura (opcional) |
| `DATABASE_MIGRATION` | `true` aplica `.migrations` no boot — ver `migrations.md` |

Segredo nunca em texto: `DATABASE_PASSWORD=secret://<provider>/<nome>[#chave]`
ou `DATABASE_PASSWORD_FILE=/caminho`. Padrão geral em `env-vars-standard.md`.

## Ciclo de vida — o repositório não é dono de nada

- A conexão global só existe **depois** de `Build`. Construtor de repositório
  **não toca o banco** (sem `Prepare`, sem ping): `New{Contexto}Repository()`
  só monta a struct, então pode ser chamado antes do `Build` ao montar os
  handlers.
- Quem fecha o pool é o componente (`Shutdown`). Repositório não expõe nem
  chama `Close` de conexão.
- Job sem HTTP: `Build` → trabalho → `svc.Shutdown(ctx)` (fecha banco e faz
  flush dos logs) — ver `examples/sqln/search/main.go`.

## Réplica de leitura

Com `DATABASE_READ_HOST`, `sqln.Find*` (`List`, `UniqueResult`, `PagedList`,
`All`) **fora de transação** lê da réplica; `statement`, transações e qualquer
leitura dentro de `transaction.Execute` usam o primário.

- **Lag de réplica é real:** ler logo depois de escrever (read-your-writes)
  fora de transação pode não ver a escrita. Quando o fluxo exige ler o que
  acabou de gravar, faça a leitura dentro da mesma transação (ver
  `transactions.md`).

## Vários bancos

`connection.NewConnection(cfg)` abre uma conexão extra (não global). Use-a
explicitamente: `sqln.Find[T](...).WithConnection(conn)`,
`statement.NewWithConnection(conn)`,
`transaction.New(transaction.Options{Connection: conn})`. Feche-a no
encerramento (`rt.OnClose` num componente próprio, ou `defer conn.Close()`).

## IAM no RDS/Aurora — `sqln/rdsauth` (módulo separado)

Token IAM de 15 min, assinado a cada conexão física nova; exige TLS
(`DATABASE_SSL_MODE=require` ou mais estrito); só o driver postgres aceita
(`connection.ErrPasswordFuncUnsupported` nos demais). O componente
`database.New()` não expõe o callback, então a conexão é aberta à mão e
entregue ao gofi com `database.FromDB`:

```go
env, err := environment.Load()
// ...
cfg, err := database.ConfigFromEnv(env)
// ...
awsCfg, err := cloudaws.Load(ctx, cloudaws.Config{})
// ...
cfg.Password = rdsauth.Password(awsCfg, endpoint /* host:porta */, env.DatabaseUser)
conn, err := connection.NewConnection(cfg)
// ...
connection.SetGlobal(conn)
defer conn.Close() // FromDB: o chamador é dono do pool

svc, err := gofi.New("{servico}").With(database.FromDB(conn.DB()) /*, ... */).Build()
```

`FromDB` dá health check e métricas, mas **não** publica a global — por isso o
`connection.SetGlobal`. Migrations nesse caminho: `connection.WithMigrations`
(ver `migrations.md`).

## Erros do driver

- PostgreSQL: `connection.AsPgError(err)` extrai o `*pgconn.PgError`
  (`Code`, `Constraint`) — base da tradução de UNIQUE para conflito
  (`persistence-rules.md`). Nunca `strings.Contains(err.Error(), ...)`.
- `connection.IsRetryable(err)` reconhece conflito de serialização/deadlock
  (ver `transactions.md`).

## Anti-padrões

- `sql.Open(...)` / `*sql.DB` próprio no serviço para o banco principal.
- Esquecer o import em branco do driver (falha só no boot).
- `connection.MustDB()` / `connection.DB()` em repositório para montar SQL à
  mão — use `sqln.Find*`, `statement` e `transaction`.
- Construtor de repositório que acessa o banco (quebra quando montado antes do
  `Build`).
