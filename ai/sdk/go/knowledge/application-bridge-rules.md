---
name: application-bridge-rules
description: Regras de application vs domain service e bridge/factory/adapter em Go — layout, dependências, NotSupported, adapter HTTP sobre netx, httptest, cache da factory
sdk: v0.8.2
keywords: [application, domain-service, bridge, factory, adapter, integration, netx, httptest, NotSupported, decorator, rate-limit]
---

# Regras — application, bridge, factory e adapter

Índice; detalhe no arquivo apontado. Cliente HTTP do SDK:
`.claude/sdk/go/api/netx.md` (`netx.NewClient`, `netx.NewRequest`,
`netx.FromError`).

- **Application service vs domain service — separação por camada.**
  Domain service (`service/`) possui regra de domínio: persistência em batch,
  hidratação de tenancy, validação cross-aggregate, lookup, delete-by-policy.
  Application service (`application/`) possui o **use case**: resolve
  bridge/factory, chama port externo, delega persistência ao service, gerencia
  transação/idempotência.
  - **Direção:** `handler → application → service → repository`
    (+ `application → bridge/factory`). Service **nunca** importa application
    nem bridge/factory; application **nunca** chama repository.
  - **Gatilho para `application/`** (QUALQUER um): (a) bridge/factory de
    integração externa, (b) coordena ≥2 domínios, (c) transação multi-passo /
    outbox / saga, (d) mesmo use case em ≥2 transportes. Senão, **só
    `service/`**.
  - **Granularidade:** uma struct por workflow (`{Aggregate}Application` com
    `Execute(ctx, ...)`), deps no constructor. Ops simples (lookup,
    delete-by-policy, CRUD trivial) **ficam no service**, chamadas direto.
  - **Naming:** arquivos `_application.go`, **nunca** `_use_case.go`; interface
    `{Workflow}Application`, struct `{workflow}Application`, constructor
    `New{Workflow}Application(...)`.
  - **Erros por camada:** `application/errors.go` (bridge/fetch/input);
    `service/errors.go` (persistência/lookup/regra).
  - **Testes:** service mocka repository; application mocka service (nunca o
    repository por baixo).
  Detalhe: `.claude/expertise/ddd-architecture/application-vs-domain-service.md`.
- **Bridge / Factory / Adapter — dimensão polimórfica externa.** N
  implementações intercambiáveis da mesma dimensão externa (gateways, provedores
  federados, parceiros):
  - `services/domain/{ctx}/bridge/` — interface pura (contrato)
  - `services/domain/{ctx}/factory/` — registry tipada `{key → BridgeBuilder}`
  - `services/domain/{ctx}/application/` — use cases
  - `services/domain/{ctx}/service/` — domain service
  - `services/adapter/{tech}/{ctx}/` — implementação (package com o nome do
    domínio + alias no import)
  Invioláveis: domain **não importa** adapter; adapter importa só
  `domain/{ctx}/{bridge,model}`; registro na factory **só no composition root**
  (`wire.go`), **nunca via `init()`**; service hidrata tenancy antes de
  persistir (adapter não conhece schema interno). Bridge cresce de read-only
  para read+write numa interface única (adapter ainda sem suporte devolve erro
  estável). Implementação única → `services/domain/{ctx}/adapter/` direto, sem
  bridge/factory. Detalhe: `bridge-factory-adapter-pattern.md`.
  - **Exceção — decider/executor exige DUAS bridges:** `bridge/decision_bridge.go`
    (PURA: sem `ctx`, sem `error` de I/O; usada pelo decider sobre snapshot) e
    `bridge/execution_bridge.go` (com `ctx` + `error`; usada pelo executor).
    Cada adapter implementa as duas em arquivos separados;
    `decision_bridge.go` que importa `net/http`/`netx` ou faz chamada externa é
    violação. Detalhe: `.claude/expertise/event-driven/executor-pattern.md`.
- **Application dispatched por evento — split físico por type.**
  `Execute(ctx, event) → switch event.Type → applyX/Y/Z` vira, no mesmo package:
  `{ctx}_application.go` (interface + struct + constructor + dispatch) +
  `apply_{type}.go` por tipo. Anti-padrão: 1 arquivo com switch + N handlers
  inline.
- **Bridge `NotSupported` per-adapter é first-class.** Adapter que não suporta
  um método devolve `Err{Ctx}BridgeNotSupported` (em
  `services/domain/{ctx}/bridge/errors.go`). Application chama plano; se cair
  em `NotSupported`, o consumer dá `msq.Ack` + log (não `Nack`: retry não muda
  a resposta). **Não** ramificar por dimensão na application.
- **`netx.Request.Execute()` devolve `(nil, nil)` sem `Content-Type: application/json`.**
  Resposta não-JSON (ou `204`) tem o corpo descartado, sem erro → nil deref no
  caller. Adapter checa `resp != nil`; em `httptest.NewServer`, sempre
  `w.Header().Set("Content-Type", "application/json")` antes do `Write`.
  Sistema externo que devolve HTML de erro cai no mesmo caso.
- **`netx` só retenta método idempotente.** `POST`/`PATCH` não repetem em
  rede/5xx, a menos que o request leve `Idempotency-Key`; `429` é retentado
  (e, esgotado, volta com status `408`) salvo `DisableRetryOn429`. O
  classificador do adapter trata `408` como rate limit. Detalhe:
  `bridge-factory-adapter-pattern.md` § Regras invioláveis do adapter HTTP.
- **Bridge é puro fetch+map — zero DB, zero enriquecimento pesado.** Adapter é
  a borda: HTTP/SDK + parse → snapshot. Não chama repository, não consulta
  cache, não faz N+1 em APIs auxiliares; enriquecimento mora na application
  (batch read prévio) ou num enricher fora do hot path. >2 chamadas externas
  por entidade no adapter = extrair etapa.
- **Adapter em `httptest` — baseURL com trailing slash.** `netx.NewRequest`
  monta `client.BaseURL + path`. Se o `baseURL` real termina em `/` e os paths
  começam sem `/`, o teste passa `srv.URL + "/"`; se o `baseURL` não termina em
  `/` e os paths começam com `/`, passa `srv.URL`.
- **Factory de bridge cacheia 1 instância por key.** Sem cache, cada `Get()`
  cria `netx.HttpClient` novo (cada um com rate limiter local) → N consumers × M
  workers clients independentes, mas o externo limita por token → estoura o
  limite real. `sync.Map` + `LoadOrStore`, decorators (observabilidade — pacote
  do projeto, não do SDK) aplicados **antes** de cachear. Detalhe:
  `bridge-factory-adapter-pattern.md` § `factory/{ctx}_factory.go`.
