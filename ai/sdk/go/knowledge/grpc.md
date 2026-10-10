---
name: grpc
description: gRPC entre serviços com netx/grpcx e o componente grpcserver — registro de serviços, TLS, token de serviço, AppError atravessando a fronteira, cliente com timeout e retry opt-in
sdk: v0.2.4
keywords: [grpc, grpcx, grpcserver, comunicação entre serviços, api interna, AppError, FromError, health, token de serviço, keepalive, round_robin]
---

# gRPC entre serviços — `netx/grpcx` + `gofi/component/grpcserver`

API: `.claude/sdk/go/api/netx-grpcx.md` e `gofi-component-grpcserver.md`. Exemplo executável com
servidor HTTP + gRPC e cliente: `examples/grpc`. Erros: `error-handling.md`.

`netx/grpcx` é um **módulo próprio**: serviço que só fala HTTP não linka gRPC. Use-o para
chamada síncrona entre serviços do mesmo sistema (API interna, rede privada). Contrato em
`.proto`; o código gerado (`Register{X}Server`, `New{X}Client`) é usado direto.

## Servidor — sempre pelo componente

```go
import (
    "google.golang.org/grpc"

    "github.com/joaoprofile/gofi-sdk-go/gofi"
    "github.com/joaoprofile/gofi-sdk-go/gofi/component/grpcserver"
    "github.com/joaoprofile/gofi-sdk-go/netx/grpcx"
)

internal := grpcserver.New(":9090", &grpcx.ServerConfig{
    Auth: &grpcx.AuthConfig{Validate: validateServiceToken}, // opcional
}).Register(func(r grpc.ServiceRegistrar) {
    pkgv1.RegisterEntityAdminServer(r, entityAdminHandler)
})

svc, err := gofi.New("<service>").With(database.New(), httpserver.New(":8080"), internal).Build()
```

- É `gofi.Runner`: sobe depois dos recursos, roda com o HTTP e para junto no shutdown —
  reporta `NOT_SERVING` no health, drena (`DrainDelay`, padrão 5 s) e faz `GracefulStop` dentro de
  `ShutdownTimeout` (padrão 25 s).
- Já embutido: recuperação de pânico, `x-request-id` (o handler lê com `netx.GetRequestID(ctx)`;
  ecoado no **trailer**), log só de erro (`Internal`/`Unknown` → Error; `Unauthenticated`/
  `PermissionDenied`/`ResourceExhausted` → Warn), OpenTelemetry (`otelgrpc`, health fora do
  trace), keepalive casado com o cliente, `grpc.health.v1` alimentado pelas readiness checks dos
  componentes.
- Reflection desligado por padrão (`Reflection: true` só se a porta for privada).
- TLS: `GRPC_TLS_CERT_FILE` / `GRPC_TLS_KEY_FILE` (+ `GRPC_TLS_CLIENT_CA_FILE` e
  `GRPC_TLS_CLIENT_AUTH` para mTLS), com recarga do certificado. Sem TLS, prod/stage **avisa**;
  `GRPC_REQUIRE_TLS=true` recusa o `Build`; `GOFI_ALLOW_INSECURE_TRANSPORT=grpc` rebaixa a aviso.

### Autenticação serviço a serviço

`grpcx.AuthConfig{Validate: fn, PublicMethods: [...]}` — `fn(ctx, token)` valida o bearer de
`authorization` e devolve o contexto do handler (com a identidade do chamador). Qualquer erro vira
`Unauthenticated` com mensagem genérica; o erro do validador **nunca** vai ao chamador. Health é
sempre público; reflection só se listado em `PublicMethods`. Para usuários finais com sessão, use os
interceptors de `iam/middleware` em `UnaryInterceptors`.

## Erros — `AppError` dos dois lados

O handler devolve `errs.AppError` como qualquer camada (`layers.md`). O servidor converte:

| `AppError.Kind` | Código gRPC |
|---|---|
| Validation | `InvalidArgument` |
| NotFound | `NotFound` |
| Conflict | `AlreadyExists` |
| Unauthorized | `Unauthenticated` |
| Forbidden | `PermissionDenied` |
| ExternalError | `Unavailable` |
| Operation / Unknown | `Internal` (mensagem genérica; causa só no log) |

O código do erro viaja num `ErrorInfo` (domínio `gofi.errs`). No cliente,
`appErr, ok := grpcx.FromError(err)` devolve o **mesmo `AppError` registrado** — decida com
`errors.Is(appErr, ErrX)`, nunca comparando mensagem. Status explícito (`status.Error(codes.X, …)`)
devolvido pelo handler passa intacto.

## Cliente

```go
conn, err := grpcx.NewClient("dns:///entity-svc.ns.svc.cluster.local:9090", grpcx.ClientConfig{
    Insecure: true,                                   // só rede privada/mesh; senão TLS: &grpcx.ClientTLSConfig{CAFile: ...}
    Token:    func(ctx context.Context) (string, error) { return mintServiceToken(ctx) },
})
client := pkgv1.NewEntityAdminClient(conn)
```

- Padrões: TLS com as raízes do sistema (texto puro só com `Insecure: true`); timeout de 10 s quando
  o `ctx` não tem deadline (`Timeout`); `round_robin` (Service headless entrega todos os pods);
  keepalive 30 s; repassa o `x-request-id` do `ctx`.
- `Retry` é **opt-in** (`&grpcx.RetryPolicy{}` = `UNAVAILABLE`, 3 tentativas) e só para método
  idempotente.
- A conexão é recurso: crie no composition root e feche no shutdown pelo ciclo de vida do `gofi`
  (componente/closer) — nunca `defer conn.Close()` no `main` (`gofi-orchestrator.md`).
- Em adapter, o cliente gerado fica atrás de um port do domínio (`application-bridge-rules.md`);
  o service não importa `grpc`.

## Anti-padrões

- ❌ `grpc.NewServer` / `grpc.NewClient` crus — perdem TLS endurecido, request ID, telemetria,
  health, parada graciosa e o mapeamento de erros.
- ❌ `grpcx.Server.ListenAndServe()` chamado no `main` — o componente é quem serve.
- ❌ Setar `x-request-id` como header no handler — já vai no trailer; header impede retry do cliente.
- ❌ Devolver `error` cru do handler esperando que o cliente entenda — devolva `AppError`.
- ❌ `Retry` em método que cria ou altera estado sem chave de idempotência.
- ❌ Token de serviço longo ou reutilizado entre serviços — curto, com `aud` do serviço de destino.
