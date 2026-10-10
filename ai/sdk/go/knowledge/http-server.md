---
name: http-server
description: Servidor HTTP netx/httpx pelo componente httpserver — RouterHandler, rotas públicas/privadas, ordem Use → UseAuth → Handlers, middlewares já embutidos, WSConfig e limites por rota
sdk: v0.2.4
keywords: [httpx, netx, httpserver, RouterHandler, PublicRoutes, PrivateRoutes, UseAuth, Use, Handlers, WSConfig, CrossOriginProtection, Health, Timeouts, Cors, MaxBodyBytes, middleware]
---

# Servidor HTTP — `netx/httpx` + `gofi/component/httpserver`

Referência: `.claude/sdk/go/api/netx-httpx.md`, `gofi-component-httpserver.md`.
Ciclo de vida (`Build`/`ListenAndServe`): `gofi-orchestrator.md`. Programa
completo: `examples/netx/api`. Esqueleto de handler:
`.claude/sdk/go/boilerplates/handler.md`.

## Rotas — o handler declara as suas

```go
func (h *EntityHandler) Handlers() []*httpx.Route { // httpx.RouterHandler
    public := httpx.PublicRoutes("/v1/entities",
        httpx.GET("/").To(h.list),
        httpx.GET("/{id}").To(h.getByID),
    )
    private := httpx.PrivateRoutes("/v1/entities",
        httpx.POST("/").To(h.create),
        httpx.PUT("/{id}").To(h.update).Timeouts(30*time.Second, 30*time.Second),
        httpx.DELETE("/{id}").To(h.delete),
    )
    return append(public, private...)
}
```

- `PrivateRoutes` = embrulhadas pelo middleware do `UseAuth`. `PublicRoutes` = não.
- Verbos: `httpx.GET/POST/PUT/PATCH/DELETE(path).To(fn)`; path param `{id}` lido
  com `httpx.GetPathParam("id", r)`.
- Por rota só existem `.Cors(&httpx.CorsConfig{...})` e `.Timeouts(read, write)`
  (estende deadline de conexão; `RequestTimeout` do servidor continua teto do
  contexto). **Não há middleware por rota.**
- Não existem `ProtectedRoutes`, `NewRouter` nem um `Config` no `httpx` — rotas
  saem só de `PublicRoutes`/`PrivateRoutes`.

## Ordem de registro

```go
server := httpserver.New(":8080", &httpx.WSConfig{ /* … */ })
svc, err := gofi.New("<service>").With(/* componentes */, server).Build()
// …
server.Use(globalMW).          // 1. global
    UseAuth(authMW).           // 2. auth das rotas privadas
    Handlers(h1, h2, h3)       // 3. rotas — uma chamada, todos os handlers
```

- **`UseAuth` antes de `Handlers`.** A rota captura o middleware de auth no
  momento em que é registrada; privada registrada antes do `UseAuth` fica
  **aberta**, sem erro nenhum.
- **`Use` antes de `Handlers`** (o roteador é chi: middleware depois de rota
  derruba o processo).
- **Todos os handlers numa chamada de `Handlers`.** As rotas são agrupadas
  por prefixo dentro da chamada; o mesmo prefixo em duas chamadas colide.
- Handler que depende de componente iniciado (`identity.Service()`) é
  registrado **depois** do `Build` — o servidor só sobe no `ListenAndServe`.

## O que já vem ligado (não adicione de novo)

`httpx.NewServer` já aplica, nesta ordem: IP do cliente
(`ClientIPMiddleware`, proxies confiáveis = `WSConfig.TrustedProxies`, nil =
redes privadas), CORS global (`AllowedOrigins`), `CrossOriginProtection` se
ligado, rate limit se `RateLimiter` configurado, request ID, recover de
panic, `LoggingMiddleware` (só 5xx e 401/403/429), controle de concorrência
(`StressControl`; default 50), `RequestTimeout`, bloqueio de TRACE/CONNECT,
`SecurityHeaders`, limite de body (`MaxBodyBytes`, default 10 MB). Cada request
também ganha span OTel e métricas com a rota.

`Use` é para middleware **do projeto** (versão de API, contexto de tenant…).

## `WSConfig` — o que decidir

| Campo | Quando |
|---|---|
| `AllowedOrigins` | front em outra origem; sem curinga em produção |
| `CrossOriginProtection: true` | **sempre** que houver cookie de sessão (CSRF) |
| `Health: &httpx.HealthConfig{}` | serviço em orquestrador — `/livez`, `/readyz` antes de qualquer middleware; readiness dos componentes entra sozinha |
| `MaxBodyBytes` + `ReadTimeout` + `WriteTimeout` | upload grande: os três juntos (ou `Timeouts` na rota) |
| `RequestTimeout` | handler legitimamente mais lento que 30 s |
| `RateLimiter: &httpx.RedisRateLimiterConfig{Backend: httpx.NewRedisBackend(client), …}` | endpoint exposto (login, público) |
| `TrustedProxies` | LB fora das redes privadas; `[]string{}` = não confia em ninguém |
| `H2C` | mesh/LB falando HTTP/2 sem TLS |

## Anti-padrões

- ❌ Rota que exige login em `PublicRoutes`; checagem de token dentro do handler.
- ❌ `Handlers(...)` antes de `UseAuth(...)`/`Use(...)`.
- ❌ Reaplicar `LoggingMiddleware`, `SecurityHeaders`, `LimitBody` ou CORS via `Use`.
- ❌ `ExposeErrorCause = true` fora de dev.
- ❌ `httpx.NewServer` + `ListenAndServe` à mão num serviço que usa `gofi.New` — use o componente.
