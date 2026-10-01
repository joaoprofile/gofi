# gofi/component/observability

`import "github.com/gofi-labs/gofi-sdk-go/gofi/component/observability"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package observability is the gofi component that exports traces, metrics
and logs over OTLP/gRPC. It is the only component that links the
OpenTelemetry SDK and gRPC; services that do not import it carry neither.

## Funções

### observability.ConfigFromEnv

```go
func ConfigFromEnv(env *environment.Environment) obs.TeleConfig
```

ConfigFromEnv builds obs.TeleConfig from the environment: the service
identity (APP_NAME / APP_VERSION / APP_ENVIRONMENT) and
OTEL_EXPORTER_OTLP_ENDPOINT. Without APP_VERSION, service.version comes from
OTEL_RESOURCE_ATTRIBUTES or is "unknown".
A zero-value CollectorAddr means telemetry is not configured.

## observability.Component

```go
type Component struct {
	// contains filtered or unexported fields
}
```

Component starts OpenTelemetry for the service.

### observability.FromTelemetry

```go
func FromTelemetry(t *obs.Telemetry) *Component
```

FromTelemetry uses telemetry initialized by the caller, who owns and shuts
it down.

### observability.New

```go
func New() *Component
```

New configures telemetry from OTEL_* (see ConfigFromEnv). When
OTEL_EXPORTER_OTLP_ENDPOINT is empty, Start skips it with a warning.

### Component.Name

```go
func (c *Component) Name() string
```

### Component.Stage

```go
func (c *Component) Stage() gofi.Stage
```

### Component.Start

```go
func (c *Component) Start(ctx context.Context, rt *gofi.Runtime) error
```

### Component.Telemetry

```go
func (c *Component) Telemetry() *obs.Telemetry
```

Telemetry returns the running telemetry; nil when it was skipped.

