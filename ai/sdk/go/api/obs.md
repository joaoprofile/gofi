# obs

`import "github.com/joaoprofile/gofi-sdk-go/obs"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Variáveis

```go
var ErrInvalidTLS = errors.New("obs: invalid OTLP TLS configuration")
```

ErrInvalidTLS reports an unusable OTLP TLS setting; Init never falls back
to plaintext.

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
	// in-cluster collectors. The TLS fields below need TLS=true: setting any
	// of them without it makes Init fail instead of sending plaintext.
	TLS bool

	// TLSConfig is used as is when set; the file fields and ServerName are
	// then ignored.
	TLSConfig *tls.Config
	// CAFile is a PEM bundle trusted for the collector certificate. As the
	// OTel spec defines for OTEL_EXPORTER_OTLP_CERTIFICATE, it replaces the
	// system roots; empty verifies against the system roots.
	CAFile string
	// CertFile and KeyFile are the PEM client certificate and key (mTLS);
	// set both or neither.
	CertFile string
	KeyFile  string
	// ServerName overrides the name verified in the collector certificate
	// (and sent as SNI); empty uses the dialed host.
	ServerName string

	// MetricInterval overrides OTEL_METRIC_EXPORT_INTERVAL (SDK default 60s).
	MetricInterval time.Duration

	// MetricCardinalityLimit caps the attribute sets kept per instrument in
	// each collection; extra sets fold into one otel.metric.overflow series,
	// so unbounded labels cannot exhaust memory. 0 keeps the SDK default
	// (2000, or OTEL_GO_X_CARDINALITY_LIMIT); negative removes the cap.
	MetricCardinalityLimit int

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

