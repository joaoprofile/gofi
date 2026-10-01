---
name: observability-otel
description: Observabilidade com OpenTelemetry no gofi — componente observability, o que já vem instrumentado, métricas de negócio via obs/metrics (lazy, nil-guard, cardinalidade fechada, classifier, decorator), spans de job e teste com ManualReader
sdk: v0.8.2
keywords: [observabilidade, OpenTelemetry, OTLP, observability.New, obs/metrics, metrics.NewInt64Counter, histogram, cardinalidade, attrs, classify, decorator, ManualReader, service_name, db_pool, traces, spans]
---

# Observabilidade — OpenTelemetry via `gofi`

API: `.claude/sdk/go/api/gofi-component-observability.md`, `obs.md`,
`obs-metrics.md`, `obs-logging.md`. Exemplo executável completo (handlers,
cliente HTTP, fila, job, dashboard): `examples/obs` —
`.claude/sdk/go/api/examples.md`. Logs: `logging.md`.

## Ligar

```go
With(observability.New(), /* database.New(), httpserver.New(...) ... */)
```

- `observability.New()` lê `OTEL_EXPORTER_OTLP_ENDPOINT` (+ `_HEADERS`,
  `_INSECURE`) e exporta **traces, métricas e logs** por uma conexão OTLP/gRPC.
  Endpoint vazio → pulado com warning; todo instrumento vira no-op e o código
  segue rodando.
- Inicia primeiro (`StageObservability`) e faz flush por último, depois dos
  logs de shutdown.
- Serviço que não importa o componente não linka gRPC nem SDK OTel. Código de
  aplicação usa só `obs/metrics` e a API `go.opentelemetry.io/otel` — nunca
  `obs.Init` à mão num serviço gofi.
- Resource: `service.name` = nome do `gofi.New`, `service.version` =
  `APP_VERSION` (ou `OTEL_RESOURCE_ATTRIBUTES`, senão `unknown`),
  `deployment.environment.name` = `APP_ENVIRONMENT`, atributos de host e
  container, e o que vier em `OTEL_RESOURCE_ATTRIBUTES` (ex.:
  `service.instance.id` pela downward API).

## O que já vem instrumentado — não duplicar

| Origem | Traces | Métricas |
|---|---|---|
| `httpserver` / `netx` server | span por rota (`GET /orders/{id}`); health não é traçado | `http.server.*` com `http.route` |
| `netx.HttpClient` | span cliente + propagação W3C | `http.client.*` |
| `msq` (todo provider) | `send`/`process`, contexto pelos headers | `messaging.*` |
| `database` | — | `db_pool_connections{pool,state}`, `db_pool_wait_count_total`, `db_pool_wait_duration_seconds_total` (`pool` = `main`/`replica`) |
| runtime Go / processo | — | nomes semconv (`go.*`, `process.*`); `GOFI_OTEL_LEGACY_METRIC_NAMES=true` mantém os antigos `gofi_*` |

Pool fora do componente (`connection.NewConnection` extra):
`metrics.ObserveDBStats("<pool>", db)`, uma vez por pool. Os gauges são
observáveis (amostram `sql.DB.Stats()` na coleta) — custo zero no hot path.
Use-os para diagnosticar exaustão de pool / espera por conexão.

Logs estruturados (`logging.*` com `slog.*`) já vão para o collector com
`trace_id`/`span_id` quando emitidos via `logging.FromContext(ctx)`. Este
arquivo cobre **métricas de negócio** e spans próprios.

## Princípios das métricas de negócio

1. **Lazy init via `sync.Once`** — instrumentos sobem na 1ª chamada de
   `RecordX`; zero mudança nos `main.go`. (Criar uma vez no boot e injetar,
   como `examples/obs/telemetry`, também vale — nunca por request.)
2. **Nil-guard** — observabilidade **nunca** derruba o pipeline: falha na
   criação deixa o instrumento `nil` e o helper vira no-op.
3. **Cardinalidade fechada** — todo atributo é enum declarado em `attrs.go`.
   **Zero valor livre** (`err.Error()`, IDs de entidade/tenant, slugs). Valor
   livre vai no log.
4. **Classifier centralizado** — `errs.AppError` → label fechado por **uma**
   função (`ClassifyHTTPError` / `FailureReason`).
5. **Decorator na fronteira** — bridges/ports ganham instrumentação por wrapper
   no factory; implementação fica sem acoplamento (ver
   `bridge-factory-adapter-pattern.md` § Decorators).
6. **`ResetForTesting` exposto** — troca de `MeterProvider` entre testes.

Instrumentos vêm de `github.com/joaoprofile/gofi-sdk-go/obs/metrics`
(`metrics.NewInt64Counter`, `metrics.NewFloat64Histogram`,
`metrics.NewInt64UpDownCounter`, `metrics.NewInt64Gauge`… ou
`metrics.Meter()` para opções extras). Os wrappers `obs.New*` / `obs.Meter`
estão **deprecated**.

## Onde o pacote mora — `domain/{contexto}/observability/` por padrão, `common/` só por força

O pacote nasce junto do bounded context que o origina — atributos, outcomes e
nomes de métrica **são vocabulário de domínio**. Sobe para
`common/observability/{contexto}/` em **exatamente dois** casos:

- **(a) Direção de dependência.** Algum pacote em `common/` grava na métrica;
  como `common/` não importa `domain/`, o pacote é empurrado para cima.
- **(b) Capacidade de plataforma transversal** (notificação, auditoria,
  outbox) que muitos contextos produzem.

Heurística: grep quem importa. Importador em `common/` → `common/` (a). Só o
próprio contexto, seus adapters e `pathCmd` → fica no domínio. Na dúvida,
domínio; promover depois é barato, despromover é breaking.

O pacote do domínio se chama `observability`, como o componente
`gofi/component/observability`: onde os dois se encontram (ou quando o pacote
sobe para `common/`), importe com alias curto (`{contexto}obs`).

## Layout canônico

```
{base}/observability/            # {base} = domain/{contexto} (default) | common/observability/{contexto}
├── metrics.go                   — declaração + init lazy dos instrumentos
├── attrs.go                     — chaves + enums fechados
├── classify.go                  — ClassifyHTTPError + FailureReason
├── classify_test.go             — 1 teste por categoria de erro
├── recorder.go                  — helpers RecordX (closure-stop duração+outcome)
├── bridge_middleware.go         — Metered{X}Bridge (decorator do port)
└── bridge_middleware_test.go    — teste com sdkmetric.NewManualReader
```

## `metrics.go` — declaração lazy

```go
// Package observability holds the OTel instruments of the {contexto} context.
// Instruments are created lazily on the first RecordX call; the global
// MeterProvider is installed by the observability component during Build.
package observability

import (
    "log/slog"
    "sync"

    "github.com/joaoprofile/gofi-sdk-go/obs/logging"
    "github.com/joaoprofile/gofi-sdk-go/obs/metrics"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    XxxRequests        metric.Int64Counter
    XxxRequestDuration metric.Float64Histogram
    XxxErrors          metric.Int64Counter
)

// durationBuckets fits seconds; the SDK default buckets (0..10000) fit
// milliseconds and would put every run in the first bucket.
var durationBuckets = []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// ensureInit creates the instruments once. Failures are logged, never panic:
// the instrument stays nil and the recorders skip it.
func ensureInit() {
    once.Do(func() {
        var err error
        report := func(name string) {
            if err != nil {
                logging.Error("{contexto} observability: instrument init failed",
                    slog.String("instrument", name), slog.Any("error", err))
            }
        }
        XxxRequests, err = metrics.NewInt64Counter("{contexto}.xxx.requests",
            "Requests sent, by method and status class")
        report("xxx.requests")
        XxxRequestDuration, err = metrics.Meter().Float64Histogram("{contexto}.xxx.request.duration",
            metric.WithDescription("Latency of xxx requests"),
            metric.WithUnit("s"),
            metric.WithExplicitBucketBoundaries(durationBuckets...))
        report("xxx.request.duration")
        XxxErrors, err = metrics.NewInt64Counter("{contexto}.xxx.errors", "Failed xxx requests, by reason")
        report("xxx.errors")
    })
}

// ResetForTesting forces a re-init on the next RecordX. Tests only.
func ResetForTesting() {
    once = sync.Once{}
    XxxRequests, XxxRequestDuration, XxxErrors = nil, nil, nil
}
```

## `attrs.go` — enums fechados

```go
package observability

import "go.opentelemetry.io/otel/attribute"

const (
    AttrMethod      = "method"
    AttrStatusClass = "status_class"
    AttrReason      = "reason"
    AttrOutcome     = "outcome"
)

const (
    OutcomeSuccess = "success"
    OutcomeFailed  = "failed"
    OutcomeSkipped = "skipped"
)

const (
    StatusClass2xx     = "2xx"
    StatusClass4xx     = "4xx"
    StatusClass5xx     = "5xx"
    StatusClassTimeout = "timeout"
    StatusClassNetwork = "network"
    StatusClassUnknown = "unknown"
)

const (
    ReasonTimeout   = "timeout"
    ReasonNetwork   = "network"
    ReasonHTTP5xx   = "http_5xx"
    ReasonRateLimit = "http_429_rate_limit"
    ReasonAuth      = "auth_failed"
    ReasonParse     = "parse_failed"
)

func MethodAttr(v string) attribute.KeyValue      { return attribute.String(AttrMethod, v) }
func StatusClassAttr(v string) attribute.KeyValue { return attribute.String(AttrStatusClass, v) }
func ReasonAttr(v string) attribute.KeyValue      { return attribute.String(AttrReason, v) }
func OutcomeAttr(v string) attribute.KeyValue     { return attribute.String(AttrOutcome, v) }
```

**Vetado:** atributo com identificador variável ou string sem teto. **Nunca**
nomear atributo `job` ou `instance` — o Prometheus reserva esses labels e o
collector descarta a série inteira no conflito.

## `classify.go` — `errs.AppError` → label fechado

```go
package observability

import (
    "context"
    "errors"
    "net"

    "github.com/joaoprofile/gofi-sdk-go/base/errs"
)

// ClassifyHTTPError maps an AppError to a closed status_class.
func ClassifyHTTPError(appErr errs.AppError) string {
    switch {
    case !appErr.Exists():
        return StatusClass2xx
    case appErr.IsUnauthorized(), appErr.IsForbidden(), appErr.IsValidation(), appErr.IsNotFound():
        return StatusClass4xx
    case appErr.IsExternalError():
        if c := classifyRawError(appErr.Err); c != "" {
            return c
        }
        return StatusClass5xx
    default:
        return StatusClassUnknown
    }
}

func classifyRawError(err error) string {
    if err == nil {
        return ""
    }
    if errors.Is(err, context.DeadlineExceeded) {
        return StatusClassTimeout
    }
    var netErr net.Error
    if errors.As(err, &netErr) && netErr.Timeout() {
        return StatusClassTimeout
    }
    var opErr *net.OpError
    var dnsErr *net.DNSError
    if errors.As(err, &opErr) || errors.As(err, &dnsErr) {
        return StatusClassNetwork
    }
    return ""
}
```

`FailureReason(appErr) string` segue o mesmo formato, devolvendo só
constantes `Reason*`. Códigos específicos do contexto (`appErr.Code`) são
tratados antes dos genéricos.

## `recorder.go` — closure-stop

```go
package observability

import (
    "context"
    "time"

    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

func add(ctx context.Context, c metric.Int64Counter, attrs ...attribute.KeyValue) {
    if c != nil {
        c.Add(ctx, 1, metric.WithAttributes(attrs...))
    }
}

func record(ctx context.Context, h metric.Float64Histogram, v float64, attrs ...attribute.KeyValue) {
    if h != nil {
        h.Record(ctx, v, metric.WithAttributes(attrs...))
    }
}

// RecordRequest returns the stop function; call it with the outcome:
//
//	stop := observability.RecordRequest(ctx, "fetch")
//	defer func() { stop(outcome) }()
func RecordRequest(ctx context.Context, method string) func(outcome string) {
    ensureInit()
    start := time.Now()
    return func(outcome string) {
        record(ctx, XxxRequestDuration, time.Since(start).Seconds(), MethodAttr(method))
        add(ctx, XxxRequests, MethodAttr(method), OutcomeAttr(outcome))
    }
}
```

## `bridge_middleware.go` — decorator

```go
type MeteredBridge struct {
    inner bridge.{Contexto}Bridge
    dim   string // closed set: the factory key
}

func WrapBridge(inner bridge.{Contexto}Bridge, dim string) bridge.{Contexto}Bridge {
    if inner == nil {
        return nil
    }
    return &MeteredBridge{inner: inner, dim: dim}
}

func (m *MeteredBridge) FetchSomething(ctx context.Context /*, args */) (Result, errs.AppError) {
    start := time.Now()
    res, appErr := m.inner.FetchSomething(ctx /*, args */)
    m.observe(ctx, "fetch_something", start, appErr)
    return res, appErr
}

func (m *MeteredBridge) observe(ctx context.Context, method string, start time.Time, appErr errs.AppError) {
    ensureInit()
    base := []attribute.KeyValue{attribute.String("dim", m.dim), MethodAttr(method)}
    record(ctx, XxxRequestDuration, time.Since(start).Seconds(), base...)
    add(ctx, XxxRequests, append(base, StatusClassAttr(ClassifyHTTPError(appErr)))...)
    if appErr.Exists() {
        add(ctx, XxxErrors, append(base, ReasonAttr(FailureReason(appErr)))...)
    }
}
```

O factory envolve com `WrapBridge` antes de cachear.

## Teste com `ManualReader`

```go
func setupManualReader(t *testing.T) *sdkmetric.ManualReader {
    t.Helper()
    reader := sdkmetric.NewManualReader()
    otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
    observability.ResetForTesting() // instruments bind to the new provider
    t.Cleanup(observability.ResetForTesting)
    return reader
}

func TestMeteredBridge_RecordsSuccess(t *testing.T) {
    reader := setupManualReader(t)
    _, _ = observability.WrapBridge(&stubBridge{}, "adapter-a").FetchSomething(context.Background())

    var rm metricdata.ResourceMetrics
    require.NoError(t, reader.Collect(context.Background(), &rm))
    requireCounter(t, &rm, "{contexto}.xxx.requests",
        map[string]string{"dim": "adapter-a", "method": "fetch_something", "status_class": "2xx"}, 1)
}
```

`requireCounter` percorre `ScopeMetrics → Metrics → metricdata.Sum[int64]` e
compara o `attribute.Set` do data point. Funciona porque `metrics.Meter()` lê
o provider global a cada chamada.

## Spans próprios — job e passo de fluxo

- **Execução de job/cron = trace próprio:**
  `tracer.Start(ctx, "job <nome>", trace.WithNewRoot())` — nunca filho de quem
  iniciou o scheduler. Duração e outcome por execução em histograma.
- **Passo relevante de fluxo** = span filho (`tracer.Start(ctx, "<passo>")`).
- **Falha marca o span:** `span.RecordError(err)` **e**
  `span.SetStatus(codes.Error, err.Error())` — só `RecordError` não muda o
  status.
- Tracer: `otel.Tracer("<module>/<pacote>")`. Padrão completo em
  `examples/obs/job/archive.go` e `examples/obs/telemetry/telemetry.go`.

## Nomes

| Tipo | Nome (OTel) | Unidade | No Prometheus |
|---|---|---|---|
| Counter | `{contexto}.<area>.<coisa>` | — | `{contexto}_<area>_<coisa>_total` |
| Histogram de duração | `{contexto}.<area>.<coisa>.duration` | `s` + buckets explícitos | `…_duration_seconds_bucket` |
| UpDownCounter / Gauge | `{contexto}.<area>.<coisa>` | conforme | `{contexto}_<area>_<coisa>` |

Nome com pontos, unidade em `WithUnit` — o exporter converte e acrescenta
sufixos. Métrica já em produção com nome `snake_case` **não** é renomeada
(quebra dashboard); a regra vale para métrica nova.

## Cardinalidade

`séries ≈ ∏(cardinalidade de cada atributo)`. Cinco atributos {3, 5, 6, 3, 10}
= 2700 séries; um sexto com 50 valores = 135 000. **Teto: < 1000 séries por
instrumento no contexto.** Acima disso, cortar atributo.

## Dashboards — filtrar por `service_name`, nunca `job`

O resource `service.name` vira o label `service_name` quando o collector o
promove (config do collector, infra). Todo binário exporta pelo mesmo
collector, então `job` é o scrape job (constante) e **não** discrimina
serviço: `label_values(<métrica>, service_name)` e
`{service_name=~"$service"}`. Réplicas só se separam com
`service.instance.id` em `OTEL_RESOURCE_ATTRIBUTES` + promoção no collector.

## Anti-padrões vetados

- ❌ Atributo com valor livre (`sku`, `account_id`, `err.Error()`, IDs) ou
  chamado `job`/`instance`.
- ❌ Instrumento criado por request / dentro de loop.
- ❌ Métrica por adapter (`{adapter-a}_xxx`) — dimensão é atributo.
- ❌ Instrumentar dentro do adapter — decorator no factory.
- ❌ Histograma em segundos sem `WithExplicitBucketBoundaries`.
- ❌ Helper sem nil-guard.
- ❌ Duplicar span/métrica que `netx`/`msq`/`database` já emitem.
- ❌ `obs.Init` à mão num serviço gofi; `obs.New*`/`obs.Meter` (deprecated).
- ❌ Métrica sem threshold acionável — se ninguém alarma, não emite.
