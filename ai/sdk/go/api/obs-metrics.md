# obs/metrics

`import "github.com/gofi-labs/gofi-sdk-go/obs/metrics"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package metrics is the OpenTelemetry metric API used by gofi and by
applications: it records through the global MeterProvider and links no
exporter or gRPC code. obs.Init installs the exporting provider; until then
(or without it) every instrument is a no-op.

## Funções

### metrics.Meter

```go
func Meter() metric.Meter
```

Meter returns the gofi meter from the global MeterProvider.

### metrics.NewFloat64Counter

```go
func NewFloat64Counter(name, description string) (metric.Float64Counter, error)
```

### metrics.NewFloat64Gauge

```go
func NewFloat64Gauge(name, description, unit string) (metric.Float64Gauge, error)
```

### metrics.NewFloat64Histogram

```go
func NewFloat64Histogram(name, description, unit string) (metric.Float64Histogram, error)
```

### metrics.NewFloat64UpDownCounter

```go
func NewFloat64UpDownCounter(name, description string) (metric.Float64UpDownCounter, error)
```

### metrics.NewInt64Counter

```go
func NewInt64Counter(name, description string) (metric.Int64Counter, error)
```

### metrics.NewInt64Gauge

```go
func NewInt64Gauge(name, description, unit string) (metric.Int64Gauge, error)
```

### metrics.NewInt64UpDownCounter

```go
func NewInt64UpDownCounter(name, description string) (metric.Int64UpDownCounter, error)
```

### metrics.ObserveDBStats

```go
func ObserveDBStats(pool string, db *sql.DB) error
```

ObserveDBStats registers OpenTelemetry observable gauges that sample
sql.DB pool stats at collection time — no hot-path cost. Reports:
  - db_pool_connections{state=open|in_use|idle}
  - db_pool_wait_count_total
  - db_pool_wait_duration_seconds_total

pool labels the pool (e.g. "main"). No-op when db is nil; when telemetry is
not initialized, Meter() returns a noop meter and the gauges are inert.
Call once per pool (gofi's database component does it for its connections).

