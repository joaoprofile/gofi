---
name: transactions
description: Transações com sqln/transaction — propagação por ctx, isolamento, retry de conflito, aninhamento, onde a tx vive
sdk: v0.8.2
keywords: [sqln, transaction, tx, isolation, retry, serializable, QuerierFrom, TxFrom, aggregate]
---

# Transações — `sqln/transaction`

API: `.claude/sdk/go/api/sqln-transaction.md` e `sqln-connection.md`
(`TxFrom`, `TxFor`, `QuerierFrom`, `IsRetryable`). Onde a transação mora
(repository, nunca service): `repository-aggregate-pattern.md`.

## Como funciona

```go
tx := transaction.New(transaction.Options{Isolation: sql.LevelReadCommitted})
err := tx.Execute(ctx, func(ctx context.Context) error {
    // tudo que usar ESTE ctx participa da transação
    return nil
})
```

- `Execute` abre `BEGIN`, põe o `*sql.Tx` no `ctx` e passa **esse ctx** ao
  `fn`. Erro (ou panic) → rollback; `nil` → commit.
- **A transação viaja pelo `ctx`**: `statement.Statement` (`Execute`,
  `QueryRow`, `Prepare`) e `sqln.Find*` detectam a tx no ctx e a usam
  sozinhos. Sempre use o `ctx` recebido pelo `fn`, nunca o de fora.
- `sqln.NewTransaction(level)` = `transaction.New(Options{Isolation: level})`.
  Sem argumento o isolamento é `sql.LevelDefault` (o do banco) — declare o
  nível explicitamente.
- `Transaction` é stateless por chamada: um valor em campo de struct serve a N
  goroutines (cada `Execute` pega sua própria conexão). Sem mutex.

## Options

| Campo | Uso |
|---|---|
| `Isolation` | `sql.LevelReadCommitted` por padrão de projeto; suba só com invariante cross-row que o schema não cobre |
| `ReadOnly` | leitura consistente de várias queries (snapshot) sem risco de escrita |
| `Connection` | banco não-global (`nil` = global) |
| `MaxRetries` | refaz o `fn` inteiro em conflito de serialização/deadlock (`connection.IsRetryable`), com backoff |

## Regras

- **`MaxRetries` só com `fn` repetível**: nada de efeito fora do banco dentro
  do `fn` (publicar evento, chamar HTTP, mutar estado do caller que não se
  refaz). Com `LevelSerializable` ou `LevelRepeatableRead`, **configure
  `MaxRetries`** (ex.: 3) — conflito `40001` sob concorrência é esperado,
  não bug.
- **Aninhamento junta, não cria savepoint:** `Execute` chamado com um ctx que
  já carrega tx do mesmo banco roda o `fn` na tx externa (e não refaz retry —
  só a externa refaz). Um método de repositório transacional pode ser
  composto dentro de outra transação sem mudar.
- **Leitura dentro da tx vai ao primário:** `sqln.Find*` com ctx transacional
  ignora a réplica — use isso para read-your-writes.
- **SQL cru dentro da tx:** `connection.QuerierFrom(ctx, db)` devolve a tx do
  ctx (ou o `db`); prefira `statement.NewStatement()`, que já faz isso.
- **`*sql.Stmt` preparado fora da tx não participa dela.** Dentro do `fn`,
  prepare com o ctx da tx (`stm.Prepare(ctx, q)` → preparado na tx) e feche
  com `defer`. Statement preparado no construtor + `ExecContext` dentro da tx
  executa em outra conexão do pool — a escrita escapa da transação.
- Transação curta: nada de I/O externo (HTTP, fila) com a tx aberta.

## Anti-padrões

- `transaction` / `sqln.NewTransaction` no **service** — a transação é detalhe
  de persistência (`repository-aggregate-pattern.md`).
- `ctx.Value(connection.SqlTxContextKey).(*sql.Tx)` com type assertion crua —
  use `connection.TxFrom(ctx)` / `connection.QuerierFrom(ctx, db)`.
- `LevelSerializable` como "default seguro" sem `MaxRetries`.
- Retry manual em volta de `Execute` — use `MaxRetries`.
