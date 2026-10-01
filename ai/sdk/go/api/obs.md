# obs

`import "github.com/gofi-labs/gofi-sdk-go/obs"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Funções

### obs.Meter

```go
func Meter() metric.Meter
```

Meter returns the gofi meter.

Deprecated: use metrics.Meter.

### obs.NewFloat64Counter

```go
func NewFloat64Counter(name, description string) (metric.Float64Counter, error)
```

Deprecated: use metrics.NewFloat64Counter.

### obs.NewFloat64Gauge

```go
func NewFloat64Gauge(name, description, unit string) (metric.Float64Gauge, error)
```

Deprecated: use metrics.NewFloat64Gauge.

### obs.NewFloat64Histogram

```go
func NewFloat64Histogram(name, description, unit string) (metric.Float64Histogram, error)
```

Deprecated: use metrics.NewFloat64Histogram.

### obs.NewFloat64UpDownCounter

```go
func NewFloat64UpDownCounter(name, description string) (metric.Float64UpDownCounter, error)
```

Deprecated: use metrics.NewFloat64UpDownCounter.

### obs.NewInt64Counter

```go
func NewInt64Counter(name, description string) (metric.Int64Counter, error)
```

Deprecated: use metrics.NewInt64Counter.

### obs.NewInt64Gauge

```go
func NewInt64Gauge(name, description, unit string) (metric.Int64Gauge, error)
```

Deprecated: use metrics.NewInt64Gauge.

### obs.NewInt64UpDownCounter

```go
func NewInt64UpDownCounter(name, description string) (metric.Int64UpDownCounter, error)
```

Deprecated: use metrics.NewInt64UpDownCounter.

### obs.ObserveDBStats

```go
func ObserveDBStats(pool string, db *sql.DB) error
```

ObserveDBStats registers the sql.DB pool gauges.

Deprecated: use metrics.ObserveDBStats.

## obs.TeleConfig

```go
type TeleConfig struct {
	ServiceName string
	// ServiceVersion falls back to service.version in OTEL_RESOURCE_ATTRIBUTES,
	// then to "unknown".
	ServiceVersion string
	ServiceEnv     string // exported as deployment.environment.name
	CollectorAddr  string

	// TLS encrypts the OTLP gRPC connection; the default is plaintext for
	// in-cluster collectors.
	TLS bool

	// MetricInterval overrides OTEL_METRIC_EXPORT_INTERVAL (SDK default 60s).
	MetricInterval time.Duration

	// LegacyMetricNames keeps the pre-v0.3 gofi_* metric names for existing
	// dashboards instead of the OpenTelemetry semantic-convention names.
	LegacyMetricNames bool
}
```

## obs.Telemetry

```go
type Telemetry struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *metric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider
	// contains filtered or unexported fields
}
```

Telemetry holds the tracer, meter and logger providers and the gRPC
connection they share. Call Shutdown to flush.

### obs.Init

```go
func Init(ctx context.Context, cfg TeleConfig) (tele *Telemetry, err error)
```

Init creates and registers the TracerProvider, MeterProvider, LoggerProvider
and the W3C trace-context/baggage propagator over one gRPC connection to the
collector. The resource merges the SDK defaults, OTEL_RESOURCE_ATTRIBUTES and
host, container and SDK attributes.

Logs are exported by attaching an OTLP handler to the global logger
(logging.Attach); when logging.InitGlobal has not run, logs are not exported
and a warning is printed.

### Telemetry.Shutdown

```go
func (t *Telemetry) Shutdown(ctx context.Context) error
```

Shutdown flushes and closes all providers and the underlying gRPC connection.
All errors are accumulated and returned together via errors.Join.

