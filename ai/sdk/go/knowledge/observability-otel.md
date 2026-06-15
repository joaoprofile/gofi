# Observabilidade — OpenTelemetry via `gofi/obs`

Padrão SDK pra instrumentar qualquer contexto Go com métricas + traces + logs
estruturados que fluem pelo OTLP collector configurado em `gofi.New().AddObservability()`.

> **Logs estruturados (slog) já fluem pra Loki automaticamente** via
> `gofi/obs/logging` (otelslog bridge). Não duplicar log shipping — basta usar
> `logging.Info/Warn/Error` com `slog.String/Any` que o resto é grátis. Este
> knowledge cobre só **métricas**.

## Princípios

1. **Lazy init via `sync.Once`** — instrumentos sobem na 1ª chamada de qualquer
   `RecordX`. Zero mudança nos `main.go` dos binários. Idempotente.
2. **Defensive nil-guard** — observabilidade **nunca** derruba o pipeline. Se
   init falhar, instrumento fica `nil`; helpers checam e viram no-op.
3. **Cardinality controlada** — toda label é enum fechado declarado em
   `attrs.go`. **Zero label free-form** (`error.Error()`, IDs de entidade,
   IDs em geral). Labels free-form vão pelo log (`slog`), nunca pela métrica.
4. **Classifier centralizado** — `errs.AppError` é mapeado pra label fechado
   por **uma** função (`ClassifyHTTPError` / `FailureReason`). Sem if-chain
   espalhado pelo código.
5. **Decorator pattern pra interfaces** — bridges, repositórios, etc. ganham
   instrumentação via wrapper na fronteira (factory). Implementações ficam
   zero-acopladas (ver `bridge-factory-adapter-pattern.md` § Decorators).
6. **`ResetForTesting` exposto** — pra trocar `MeterProvider` global entre
   testes (usar `sdkmetric.NewManualReader` pra asserir contador por
   attribute set).

## Onde o pacote mora — `domain/{ctx}/observability/` por padrão, `common/` só por força

O pacote de observabilidade **nasce junto do bounded context** que o origina —
`services/domain/{ctx}/observability/` — porque seus `attrs`/`outcomes`/nomes de
métrica **são vocabulário de domínio** (`op=create_campaign`,
`outcome=no_active_campaign`, `{ctx}_event_published_total`), não infra neutra.
Hoistar isso para `common/` polui a camada compartilhada com semântica que só um
contexto entende.

Sobe para `services/common/observability/{ctx}/` em **exatamente dois** casos:

- **(a) Direção de dependência força.** Algum pacote em `common/` precisa gravar
  na métrica (ex.: o runner genérico de `common/scheduler` registra `RecordX` do
  contexto). Como **`common/` não pode importar `domain/`** (violação de camada /
  risco de ciclo), o pacote de observabilidade é **empurrado para cima**, para
  `common/`. Não é estética — é a única posição legal.
- **(b) Capacidade de plataforma transversal.** A observabilidade instrumenta uma
  capacidade que **muitos contextos** vão produzir (notificação, auditoria,
  outbox genérico), não um único dono. Aí mora em `common/` por natureza, mesmo
  que hoje só um importador exista — é aposta deliberada de "infra compartilhada",
  não obrigação.

**Heurística de decisão:** grep quem importa o pacote. Se **algum importador está
em `common/`** → tem que estar em `common/` (caso a). Se os importadores são só o
próprio `domain/{ctx}`, seus adapters e binários top-level (`pathCmd`, que podem
importar qualquer coisa) → fica em `domain/{ctx}/observability/` (default). O caso
(b) é o único julgamento subjetivo — na dúvida, **fica no domínio**; promover
depois é barato, despromover (com `common/` já dependendo) é breaking.

## Layout canônico

```
{base}/observability/{ctx}/            # {base} = services/domain/{ctx} (default) | services/common (casos a/b)
├── metrics.go              — declaração + init lazy de todos os instrumentos
├── attrs.go                — chaves + enums fechados (cardinality controlada)
├── classify.go             — ClassifyHTTPError + FailureReason
├── classify_test.go        — testes de comportamento (1 por categoria de erro)
├── recorder.go             — helpers RecordX (closure-stop pra duração+outcome)
├── bridge_middleware.go    — Metered{X}Bridge (decorator do port externo)
└── bridge_middleware_test.go — teste real com sdkmetric.NewManualReader
```

`{ctx}` = nome do contexto/domínio que origina as métricas. Permite múltiplos
contextos coexistirem sem colisão de init (cada um tem seu `sync.Once` e seu
conjunto de instrumentos isolado). Quando mora no domínio, o nome do pacote é
`observability` (qualificado pelo path do contexto); quando sobe para `common/`,
ganha alias curto no import (`syncobs`, `notifobs`) pra desambiguar.

## `metrics.go` — declaração lazy

```go
// Package {ctx}obs centraliza os instrumentos OTel do contexto {ctx}. Init
// lazy via sync.Once — instrumentos sobem na 1ª chamada de qualquer RecordX.
// gofi.New().AddObservability() já configurou o MeterProvider global antes.
//
// Convenção de naming: `{ctx}_<area>_<unit_or_total>` (snake_case), alinhado
// ao que o OTel collector exporta pra Prometheus/Grafana.
package {ctx}obs

import (
    "log/slog"
    "sync"

    "github.com/joaoprofile/gofi/obs"
    "github.com/joaoprofile/gofi/obs/logging"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    // 1 var por instrumento — global do package.
    XxxRequestsTotal     metric.Int64Counter
    XxxRequestDuration   metric.Float64Histogram
    XxxErrorsTotal       metric.Int64Counter
    // ...
)

// ensureInit cria todos os instrumentos uma vez. Idempotente. Chamado por
// todos os RecordX antes de tocar nos instrumentos.
//
// Falhas de criação são logadas mas NÃO panicam — instrumento fica nil, e os
// recorders fazem nil-guard. Observabilidade nunca derruba o pipeline.
func ensureInit() {
    once.Do(func() {
        var err error
        create := func(label string, fn func() error) {
            if err = fn(); err != nil {
                logging.Error("{ctx}obs: instrument init failed",
                    slog.String("instrument", label), slog.Any("error", err))
            }
        }

        create("xxx_requests_total", func() error {
            XxxRequestsTotal, err = obs.NewInt64Counter(
                "{ctx}_xxx_requests_total",
                "requests sent — labeled by {dim}/method/status_class")
            return err
        })
        create("xxx_request_duration_seconds", func() error {
            XxxRequestDuration, err = obs.NewFloat64Histogram(
                "{ctx}_xxx_request_duration_seconds",
                "latency of xxx requests", "s")
            return err
        })
        // ...
    })
}

// ResetForTesting força re-init dos instrumentos no próximo RecordX. Usado
// pelos testes que trocam o MeterProvider global pra coletar via ManualReader.
// NÃO deve ser chamado em produção.
func ResetForTesting() {
    once = sync.Once{}
    XxxRequestsTotal = nil
    XxxRequestDuration = nil
    XxxErrorsTotal = nil
    // ...
}
```

## `attrs.go` — enums fechados

```go
package {ctx}obs

import "go.opentelemetry.io/otel/attribute"

// Chaves padronizadas — usadas em todas as métricas do contexto.
const (
    AttrXxx         = "xxx"
    AttrMethod      = "method"
    AttrStatusClass = "status_class"
    AttrReason      = "reason"
    AttrOutcome     = "outcome"
    // ...
)

// Outcomes — enum fechado. Cardinality controlada.
const (
    OutcomeSuccess = "success"
    OutcomeFailed  = "failed"
    OutcomeSkipped = "skipped"
)

// Status classes — buckets HTTP/categoria.
const (
    StatusClass2xx     = "2xx"
    StatusClass4xx     = "4xx"
    StatusClass5xx     = "5xx"
    StatusClassTimeout = "timeout"
    StatusClassNetwork = "network"
    StatusClassUnknown = "unknown"
)

// Reasons — enum fechado (NÃO usar error.Error()).
const (
    ReasonTimeout       = "timeout"
    ReasonNetwork       = "network"
    ReasonHTTP5xx       = "http_5xx"
    ReasonRateLimit     = "http_429_rate_limit"
    ReasonAuth          = "auth_failed"
    ReasonParseFailed   = "parse_failed"
    ReasonNotSupported  = "not_supported"
    // ...
)

// Helpers pra montar attribute.KeyValue (açúcar — caller pode usar
// attribute.String() direto também).
func MethodAttr(name string) attribute.KeyValue       { return attribute.String(AttrMethod, name) }
func StatusClassAttr(s string) attribute.KeyValue     { return attribute.String(AttrStatusClass, s) }
func ReasonAttr(r string) attribute.KeyValue          { return attribute.String(AttrReason, r) }
func OutcomeAttr(o string) attribute.KeyValue         { return attribute.String(AttrOutcome, o) }
```

**Vetado:** labels com identificadores variáveis (entity IDs, tenant IDs,
slugs livres do usuário, `error.Error()`), ou qualquer string que cresça
sem upper bound. Tudo
isso vai pelo log (`slog.String("sku", x)`), nunca pela métrica.

## `classify.go` — mapeamento `errs.AppError → label fechado`

```go
package {ctx}obs

import (
    "context"
    "errors"
    "net"
    "strings"

    "github.com/joaoprofile/gofi/base/errs"
)

// ClassifyHTTPError mapeia um errs.AppError pra status_class fechado.
// Heurística: errs.AppError.Kind diz a categoria de domínio; quando é
// External (HTTP), inspeciona o err embutido pra distinguir timeout/network.
//
//   - sem erro → "2xx"
//   - código de domínio mapeado (ex.: NOT_SUPPORTED) → label dedicado
//   - código com substring PARSE/EMPTY → "parse"
//   - External + ctx.DeadlineExceeded → "timeout"
//   - External + net.DNSError/net.OpError → "network"
//   - External + net.Error.Timeout() → "timeout"
//   - External genérico → "5xx" (bucket genérico de "externo falhou")
//   - Operation/Validation/NotFound domain → "unknown" (não polui buckets HTTP)
func ClassifyHTTPError(appErr errs.AppError) string {
    if !appErr.Exists() {
        return StatusClass2xx
    }
    // ... (códigos específicos do contexto primeiro)
    if strings.Contains(appErr.Code, "PARSE") || strings.Contains(appErr.Code, "EMPTY_PAYLOAD") {
        return "parse"
    }
    if appErr.IsExternalError() {
        if classified := classifyRawError(appErr.Err); classified != "" {
            return classified
        }
        return StatusClass5xx
    }
    return StatusClassUnknown
}

// FailureReason refina o reason. Cardinality controlada — só strings do enum.
func FailureReason(appErr errs.AppError) string { /* ... */ }

func classifyRawError(err error) string {
    if err == nil { return "" }
    if errors.Is(err, context.DeadlineExceeded) { return StatusClassTimeout }
    var netErr net.Error
    if errors.As(err, &netErr) && netErr.Timeout() { return StatusClassTimeout }
    var dnsErr *net.DNSError
    if errors.As(err, &dnsErr) { return StatusClassNetwork }
    var opErr *net.OpError
    if errors.As(err, &opErr) { return StatusClassNetwork }
    return ""
}
```

## `recorder.go` — helpers RecordX

Padrão **closure-stop** pra registrar duração + outcome no fim do escopo:

```go
package {ctx}obs

import (
    "context"
    "time"

    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

// Defensive helpers — guardam contra instrumento nil (init falhou).
func recordCounter(ctx context.Context, c metric.Int64Counter, delta int64, attrs ...attribute.KeyValue) {
    if c == nil { return }
    c.Add(ctx, delta, metric.WithAttributes(attrs...))
}

func recordHistogram(ctx context.Context, h metric.Float64Histogram, val float64, attrs ...attribute.KeyValue) {
    if h == nil { return }
    h.Record(ctx, val, metric.WithAttributes(attrs...))
}

// RecordPipeline retorna closure-stop. Padrão de uso:
//
//   stop := {ctx}obs.RecordPipeline(ctx, ...)
//   defer func() { stop(outcome) }()  // outcome decidido no return
//
// Ou inline quando o outcome é trivial:
//
//   defer {ctx}obs.RecordPipeline(ctx, ...)({ctx}obs.OutcomeSuccess)
func RecordPipeline(ctx context.Context, dim, eventType, source string) func(outcome string) {
    ensureInit()
    start := time.Now()
    base := []attribute.KeyValue{
        attribute.String("dim", dim),
        attribute.String("type", eventType),
        attribute.String("source", source),
    }
    return func(outcome string) {
        recordHistogram(ctx, XxxDuration, time.Since(start).Seconds(), base...)
        recordCounter(ctx, XxxProcessedTotal, 1, append(base, OutcomeAttr(outcome))...)
    }
}

func RecordSkip(ctx context.Context, dim, eventType, reason string) {
    ensureInit()
    recordCounter(ctx, XxxSkippedTotal, 1,
        attribute.String("dim", dim),
        attribute.String("type", eventType),
        ReasonAttr(reason),
    )
}
```

## `bridge_middleware.go` — decorator de instrumentação

```go
package {ctx}obs

import (
    "context"
    "time"

    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"

    "github.com/joaoprofile/gofi/base/errs"
    "<module>/services/domain/{ctx}/bridge"
)

// MeteredBridge envelopa qualquer {Ctx}Bridge instrumentando latência,
// status_class e reason por chamada. Dimensão é fixada no constructor —
// vem do factory key (chave da dimensão polimórfica do contexto).
type MeteredBridge struct {
    inner bridge.{Ctx}Bridge
    dim   string
}

func WrapBridge(inner bridge.{Ctx}Bridge, dim string) bridge.{Ctx}Bridge {
    if inner == nil { return nil }
    return &MeteredBridge{inner: inner, dim: dim}
}

func (m *MeteredBridge) FetchSomething(ctx context.Context, /* args */) (Result, errs.AppError) {
    start := time.Now()
    res, appErr := m.inner.FetchSomething(ctx, /* args */)
    m.record(ctx, "fetch_something", start, appErr)
    return res, appErr
}

func (m *MeteredBridge) record(ctx context.Context, method string, start time.Time, appErr errs.AppError) {
    ensureInit()
    base := []attribute.KeyValue{
        attribute.String("dim", m.dim),
        MethodAttr(method),
    }
    statusClass := ClassifyHTTPError(appErr)

    if XxxRequestDuration != nil {
        XxxRequestDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(base...))
    }
    if XxxRequestsTotal != nil {
        XxxRequestsTotal.Add(ctx, 1, metric.WithAttributes(append(base, StatusClassAttr(statusClass))...))
    }
    if appErr.Exists() && XxxErrorsTotal != nil {
        XxxErrorsTotal.Add(ctx, 1, metric.WithAttributes(append(base, ReasonAttr(FailureReason(appErr)))...))
    }
}
```

Wire no factory ([bridge-factory-adapter-pattern.md](bridge-factory-adapter-pattern.md)
§ Decorators) — `Get()` envolve com `WrapBridge` antes de cachear.

## `bridge_middleware_test.go` — teste real com ManualReader

```go
package {ctx}obs_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/sdk/metric/metricdata"

    {ctx}obs "<module>/{base}/observability/{ctx}"   // {base}: services/domain/{ctx} (default) | services/common
)

// setupManualReader instala um MeterProvider em memória, força
// ResetForTesting pra ensureInit registrar instrumentos no provider novo, e
// devolve o reader pra coletar data points.
func setupManualReader(t *testing.T) *sdkmetric.ManualReader {
    t.Helper()
    reader := sdkmetric.NewManualReader()
    mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
    otel.SetMeterProvider(mp)
    {ctx}obs.ResetForTesting()
    t.Cleanup({ctx}obs.ResetForTesting)
    return reader
}

func TestMeteredBridge_RecordsSuccess(t *testing.T) {
    reader := setupManualReader(t)

    wrapped := {ctx}obs.WrapBridge(&stubBridge{}, "adapter-a")
    _, _ = wrapped.FetchSomething(context.Background())

    rm := collect(t, reader)
    requireCounter(t, rm, "{ctx}_xxx_requests_total",
        map[string]string{"dim": "adapter-a", "method": "fetch_something", "status_class": "2xx"}, 1)
}

// Helper requireCounter — itera ScopeMetrics → Metrics → DataPoints, encontra
// o data point com attribute set matching e assere o valor.
func requireCounter(t *testing.T, rm *metricdata.ResourceMetrics, name string, want map[string]string, expected int64) {
    t.Helper()
    for _, sm := range rm.ScopeMetrics {
        for _, m := range sm.Metrics {
            if m.Name != name { continue }
            sum, ok := m.Data.(metricdata.Sum[int64])
            require.True(t, ok)
            for _, dp := range sum.DataPoints {
                if attrsMatch(dp.Attributes, want) {
                    assert.Equal(t, expected, dp.Value)
                    return
                }
            }
        }
    }
    t.Fatalf("metric %s with attrs %v not found", name, want)
}

func attrsMatch(set attribute.Set, want map[string]string) bool {
    got := make(map[string]string)
    for _, kv := range set.ToSlice() {
        got[string(kv.Key)] = kv.Value.AsString()
    }
    for k, v := range want {
        if got[k] != v { return false }
    }
    return true
}
```

## Naming convention

| Métrica | Padrão | Exemplo |
|---|---|---|
| Counter | `{ctx}_<area>_<thing>_total` | `{ctx}_pipeline_processed_total` |
| Histogram (duração) | `{ctx}_<area>_<thing>_seconds` | `{ctx}_bridge_request_duration_seconds` |
| Histogram (count) | `{ctx}_<area>_<thing>` | `{ctx}_batch_size` |
| Gauge | `{ctx}_<area>_<thing>` | `{ctx}_queue_depth` |

**Não usar:** UpperCamelCase, kebab-case, `.` no nome (`.` quebra Prometheus
exporter; OTel collector converte mas vira ruído).

## Cardinality budget

Estimativa antes de adicionar label novo:
```
unique_series ≈ ∏(cardinality de cada label)
```

Pra contador com 5 labels com cardinality {3, 5, 6, 3, 10}: **2700 séries**.
Adicionar 1 label novo de cardinality 50 vira **135 000 séries** — instável.

**Regra do polegar:** total do contexto < 1000 séries por counter/histogram.
Acima disso, repensar quais labels são realmente actionable. Labels descartáveis
(entity IDs em alta cardinalidade) **NÃO** vão pra métrica —
vão pelo log estruturado.

## Anti-padrões vetados

- ❌ Labels com strings variáveis (`sku`, `account_id`, `error.Error()`, IDs)
- ❌ Init de instrumentos no `main.go` (espalha responsabilidade — usar lazy)
- ❌ Métrica por adapter individual (`{adapter-a}_xxx_total`) — viola "dimensão é label"
- ❌ Instrumentar dentro do adapter — usar decorator no factory
- ❌ Skip do classifier — `error.Error()` direto como label cria infinitas séries
- ❌ Métrica sem assertable threshold — se "ninguém alarma neste número", não emite
- ❌ Helpers sem nil-guard — observabilidade não pode derrubar pipeline

## Onde a observabilidade é exportada

`gofi.New().AddObservability()` configura:
- **MeterProvider** OTLP gRPC → collector (Grafana Mimir/Prometheus)
- **TracerProvider** OTLP gRPC → collector (Grafana Tempo/Jaeger)
- **LoggerProvider** OTLP gRPC → collector (Grafana Loki via otelslog bridge)

Endereço do collector vem do `environment.Instance().Observability().CollectorAddr`.
Sem essa env, OTel emite pra nowhere (graceful degradation — código continua
rodando).

## Convenção de label em dashboards Grafana — `service_name`, **nunca** `job`

Toda métrica/trace/log carrega o resource attribute `service.name` (= nome
passado em `gofi.New("<svc>")` → `env.AppName`, setado em `obs.Init`). O **OTel
collector** (`collector-config.yaml`, processor `transform/metrics`) promove
esse resource attr para o **datapoint label `service_name`** — e idem
`environment`. Portanto:

- **Particionar/filtrar por serviço usa `service_name`.** `sum by (service_name)`,
  `<metric>{service_name=~"$service"}`. A variável de template é
  `label_values(<metric>, service_name)`.
- **`job` NÃO identifica o serviço.** Todos os binários exportam pelo **mesmo**
  collector → `job` é o scrape job do Prometheus (constante, ex.: `otel-collector`),
  igual para todos. Um filtro `job=~"$service"` ou `label_values(..., job)`
  **parece funcionar mas está quebrado**: a variável lista um valor só e o filtro
  não discrimina serviço. Armadilha recorrente — já tinha pego os dashboards de
  Infra e Pricing.
- **O label já está em TODA métrica** — runtime do SDK (`gofi_*`, `db_pool_*`) e
  de domínio (`synchronization_*`, `notification_*`, `pricing_*`). Adicionar um
  filtro por serviço a qualquer dashboard é **de graça** (o dado sempre esteve lá);
  não exige instrumentação nova.
- **Tabelas (`instant`/`table`):** manter `job` e `instance` no
  `transformations[].organize.excludeByName` (`"job": true`) — o collector ainda
  anexa essas labels; esconder a coluna é o comportamento correto.
- **Sem breakdown por pod/réplica hoje:** o resource só tem `service.name`,
  `service.version`, `environment` — **não** `service.instance.id`. Réplicas do
  mesmo serviço colapsam na mesma série. Habilitar é trabalho de infra: setar
  `service.instance.id` (downward API → `OTEL_RESOURCE_ATTRIBUTES`) + promover no
  transform do collector.

## DB pool stats — automáticas via `Build()`

`gofi.New().AddDatabase().AddObservability().Build()` registra **observable
gauges** do pool de conexões da conexão gerenciada, sem código por serviço:
- `db_pool_connections{pool,state=open|in_use|idle}`
- `db_pool_wait_count_total{pool}`
- `db_pool_wait_duration_seconds_total{pool}`

São **observable** (callback amostra `sql.DB.Stats()` no momento da coleta) →
zero custo no hot path. Registro acontece no `Build()` (depois de DB + obs).
Helper público: `obs.ObserveDBStats(pool, db)` — para pools fora do managed
(`WithDatabase` custom). No-op se DB nil ou telemetria não inicializada.

Use esses gauges pra diagnosticar **exaustão de pool / espera por conexão**
(p.ex. hang/lentidão de query que satura conexões) — sinal de causa-raiz que
não aparece em métrica de latência por operação.
