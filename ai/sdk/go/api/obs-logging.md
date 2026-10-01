# obs/logging

`import "github.com/gofi-labs/gofi-sdk-go/obs/logging"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	EnvDevelopment = "dev"
	EnvProduction  = "prod"
)
```

Deployment environment values that influence console formatting. Development
uses a human-readable text handler; any other value uses a JSON handler.

```go
const (
	LOG_START_ERROR string = "\nCONFIGURATION ERROR: The global logger has not been initialized.\nMake sure to call InitGlobal() at the start of your service.\n"
)
```

## Variáveis

```go
var ErrNotInitialized = errors.New("logging: the global logger is not initialized; call InitGlobal first")
```

ErrNotInitialized is returned by Attach when InitGlobal has not run yet.

## Funções

### logging.Attach

```go
func Attach(h slog.Handler, shutdown func(context.Context) error) error
```

Attach tees the global logger to h, keeping the console output, and makes
the result slog's default. h receives only records at or above the level
set in Config, like the console, so an exporter does not ship the Debug
records the console drops. shutdown, when not nil, runs in Shutdown so h can
flush. Loggers derived (With, FromContext) before Attach keep writing only to
the previous handlers. It returns ErrNotInitialized before InitGlobal.

### logging.Debug

```go
func Debug(msg string, args ...any)
```

### logging.Error

```go
func Error(msg string, args ...any)
```

### logging.Fatal

```go
func Fatal(msg string, args ...any)
```

### logging.FromContext

```go
func FromContext(ctx context.Context) *slog.Logger
```

### logging.Info

```go
func Info(msg string, args ...any)
```

### logging.InitGlobal

```go
func InitGlobal(ctx context.Context, cfg Config) error
```

### logging.NewLogger

```go
func NewLogger(serviceName string) error
```

NewLogger initialises the global logger with sane defaults (Info level, JSON
console). It reads no environment; gofi's config.InitLogging builds
an env-driven Config and is what services should use in production.

### logging.PrintStruct

```go
func PrintStruct(v any)
```

PrintStruct prints the %+v representation of a struct to stdout.

### logging.PrintStructToJson

```go
func PrintStructToJson(v any)
```

PrintStructToJson prints the indented JSON representation of a struct to stdout.

### logging.ResetForTesting

```go
func ResetForTesting()
```

ResetForTesting resets the singleton so that InitGlobal re-initialises on the
next call. Must only be called from tests.

### logging.Shutdown

```go
func Shutdown(ctx context.Context) error
```

### logging.SlogLevel

```go
func SlogLevel(l common.LogLevel) slog.Level
```

SlogLevel maps a common.LogLevel to its slog.Level. Exported so gofi's config
package can build a logging.Config from the environment.

### logging.Warn

```go
func Warn(msg string, args ...any)
```

## logging.Config

```go
type Config struct {
	ServiceName string
	Environment string     // deployment environment; EnvDevelopment selects text output
	EnableDebug bool       // legado: equivale a Level=Debug
	Level       slog.Level // nível do handler (zero = Info); EnableDebug tem precedência
}
```

## logging.Logger

```go
type Logger struct {
	*slog.Logger
	// contains filtered or unexported fields
}
```

Logger is the global logger: console output plus the handlers attached with
Attach (the OTLP bridge installed by obs.Init, for instance).

### logging.Instance

```go
func Instance() *Logger
```

Instance returns the global logger. Before InitGlobal it falls back to
slog.Default() (warning once), so SDK packages never crash an app that did
not initialise gofi logging.

### logging.New

```go
func New(_ context.Context, cfg Config) (*Logger, error)
```

New builds a console logger (text in development, JSON otherwise) and makes
it slog's default. It exports nothing: exporters are attached with Attach, so
this package does not link any OpenTelemetry SDK or gRPC code.

### Logger.ErrorWithStack

```go
func (l *Logger) ErrorWithStack(ctx context.Context, msg string, attrs ...any)
```

ErrorWithStack logs an error with the current goroutine stack trace.

### Logger.FromContext

```go
func (l *Logger) FromContext(ctx context.Context) *slog.Logger
```

FromContext returns a logger enriched with trace_id and span_id from ctx.

### Logger.Shutdown

```go
func (l *Logger) Shutdown(ctx context.Context) error
```

Shutdown flushes the attached handlers, in reverse order of Attach.

## logging.TeeHandler

```go
type TeeHandler struct {
	// contains filtered or unexported fields
}
```

TeeHandler fans out log records to multiple slog.Handler implementations.

### TeeHandler.Enabled

```go
func (t *TeeHandler) Enabled(ctx context.Context, level slog.Level) bool
```

### TeeHandler.Handle

```go
func (t *TeeHandler) Handle(ctx context.Context, record slog.Record) error
```

### TeeHandler.WithAttrs

```go
func (t *TeeHandler) WithAttrs(attrs []slog.Attr) slog.Handler
```

### TeeHandler.WithGroup

```go
func (t *TeeHandler) WithGroup(name string) slog.Handler
```

