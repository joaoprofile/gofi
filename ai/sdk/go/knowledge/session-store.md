---
name: session-store
description: base/session e o componente session — store chave/valor com TTL e lock distribuído sobre Redis; não é a sessão de login do iam
sdk: v0.8.2
keywords: [base/session, session.Instance, CreateOrGet, Force, WithLock, TryLock, DistributedLocker, NewKey, lock distribuído, idempotência, componente session, CACHE_TYPE]
---

# Session store — `base/session` + `gofi/component/session`

Referência: `.claude/sdk/go/api/base-session.md`, `gofi-component-session.md`.

## Não confundir

| Precisa de | Use |
|---|---|
| Sessão de **login** (token, refresh, logout, revogação) | `iam` — `SessionPort` (`iam.md`) |
| Estado efêmero por chave com TTL, "processar uma vez só", lock entre réplicas | `base/session` (este arquivo) |

`base/session` não sabe nada de usuário nem token. Guardar tokens do iam aqui
só como **vault do modo BFF** (`http-auth-middleware.md`), e com os tokens em
campos próprios.

## Montagem

```go
svc, err := gofi.New("<service>").
    With(cache.New(), session.New(&basesession.Config{TTL: 30 * time.Minute, Prefix: "<ctx>"})).
    Build()
store := basesession.Instance() // *basesession.Session, nil antes do Build
```

- Exige `CACHE_TYPE=redis` (reusa o client do componente `cache`, ou abre o
  compartilhado). Outro `CACHE_TYPE` falha no `Build`.
- `session.New()` sem config = `DefaultSessionConfig()`: **TTL 10 min**,
  prefixo `session`. Defina o TTL pelo caso de uso — o default raramente serve.
- Singleton: o primeiro `basesession.New` vence; chamadas seguintes são no-op.
- Injete no consumidor pela interface `basesession.DistributedLocker` quando
  só precisa de lock — testável com fake.

## Operações

| Método | Semântica |
|---|---|
| `CreateOrGet(ctx, key, data)` | devolve a entrada viva ou cria — **sob lock**; idempotência de "primeira vez" |
| `Force(ctx, key, data)` | sobrescreve (sob lock), novo `ExpiresAt = agora + TTL` |
| `Get(ctx, key)` | sem lock; **`nil, nil` quando não existe** — cheque o ponteiro |
| `Delete(ctx, key)` | remove |
| `WithLock(ctx, key, fn)` | `(false, nil)` se já travado — **não espera**; `(true, err)` com erro de `fn`; lock liberado sempre |
| `TryLock`/`Unlock`/`IsLocked` | lock manual (TTL fixo 10 s) |

```go
key := basesession.NewKey("<ctx>:job", tenantID, entityID) // "<ctx>:job:<16 hex>"
ran, err := store.WithLock(ctx, key, func() error { return s.process(ctx, entityID) })
if err != nil {
    return ErrEntityProcess.Wrap(err)
}
if !ran {
    return ErrEntityInProgress.New() // outra réplica está processando
}
```

## Armadilhas

- **Chave é usada como veio.** O `Prefix` da config só entra na chave de lock
  interna; monte chaves com `basesession.NewKey(prefix, parts...)` (hash
  estável, sem dado sensível exposto).
- **Lock com TTL de 10 s.** `fn` que pode passar disso perde a exclusão
  mútua; divida o trabalho ou use outro mecanismo (fila, `SELECT … FOR UPDATE`).
- **`Data` volta de JSON.** `SessionData` é `map[string]any`: número volta
  `float64`, struct volta `map[string]any`. Guarde tipos simples ou serialize
  você mesmo (string JSON) e converta na leitura.
- `WithLock` não é fila: quem perde **não** executa. Para "esperar e depois
  ler o resultado", perdedor relê com `Get` após curto intervalo.
