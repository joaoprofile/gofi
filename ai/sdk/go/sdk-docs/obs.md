# gofi/obs — Observabilidade

## Variáveis de Ambiente

| Variável | Descrição |
|----------|-----------|
| `APP_NAME` | Nome do serviço (aparece nos logs e traces) |
| `APP_ENVIRONMENT` | `development`, `staging`, `production` |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` (default: `info`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Endereço do OTEL collector — omitir para apenas stdout |
| `SERVICE_DEBUG` | `true` para ativar pprof/debug endpoints |
| `SERVICE_DEBUG_ADDR` | Endereço do servidor de debug (ex: `:6060`) |
| `SERVICE_DEBUG_USER` | Usuário para autenticação básica no debug |
| `SERVICE_DEBUG_PASS` | Senha para autenticação básica no debug |

## gofi/obs/logging — Logging Estruturado

### Inicialização

```go
import (
    "github.com/joaoprofile/gofi/obs/logging"
    obsconfig "github.com/joaoprofile/gofi/obs/config"
)

logging.InitGlobal(ctx, obsconfig.Config{
    ServiceName:  "my-service",
    Environment:  "production",
    EnableDebug:  false,
    CollectorAddr: "otel-collector:4317", // OTLP via gRPC — omitir para log local apenas
})
```

Quando `CollectorAddr` é definido, logs são exportados via OTLP para o coletor. Sem ele, somente stdout.

### Shortcuts de log (uso direto)

```go
logging.Info("mensagem", slog.String("key", "value"))
logging.Error("erro ao processar", slog.Any("error", err))
logging.Debug("dados de debug", slog.Int("count", n))
logging.Warn("atenção", slog.String("reason", "rate limit approaching"))
logging.Fatal("falha crítica na inicialização", slog.Any("error", err)) // chama os.Exit(1)
```

Nunca use `fmt.Println`, `log.Println` ou `log.Fatal` em código de produção — sempre `logging.*`.

### Campos estruturados

```go
// Tipos comuns de slog
slog.String("key", "value")
slog.Int("count", 42)
slog.Any("error", err)
slog.Bool("cached", true)
slog.Duration("elapsed", time.Since(start))
```

### Uso típico no repository

```go
func NewPersonRepository(ctx context.Context) PersonRepository {
    stmt, err := sqln.NewStatement().Prepare(ctx, query)
    if err != nil {
        logging.Fatal("error on NewPersonRepository", slog.Any("error", err))
    }
    return &personRepository{stmt: stmt}
}
```

### TeeHandler (múltiplos destinos)

```go
// Interno — usado pelo InitGlobal quando CollectorAddr é definido
// Não é necessário configurar manualmente na maioria dos casos
```

## gofi/obs — Tracing e Métricas

O módulo `obs` também oferece wrappers para OpenTelemetry tracing e métricas.  
Consulte `obs/tracing` e `obs/metrics` para uso avançado.  
Para a maioria dos contextos CRUD, apenas o logging é necessário.
