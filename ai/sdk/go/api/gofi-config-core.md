# gofi/config/core

`import "github.com/gofi-labs/gofi-sdk-go/gofi/config/core"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package core holds the process-wide settings gofi's Build applies before any
component starts: timezone, logging and TLS verification. It depends only on
base/environment, base/timezone and obs/logging, so importing it links no
resource; package config re-exports these functions.

## Funções

### core.ApplyTLS

```go
func ApplyTLS(env *environment.Environment)
```

ApplyTLS applies TLS_INSECURE_SKIP_VERIFY to http.DefaultTransport; it is a no-op by default.

### core.InitLogging

```go
func InitLogging(env *environment.Environment, serviceName string) error
```

InitLogging initialises the global logger from the environment. This is the
env-driven entry point services should use; logging.NewLogger is a
default-only convenience for tests.

### core.Logging

```go
func Logging(env *environment.Environment, serviceName string) logging.Config
```

Logging builds a logging.Config from the environment for the given service.
OTLP log export is not configured here: obs.Init attaches it.

### core.SetTimezone

```go
func SetTimezone(env *environment.Environment) error
```

SetTimezone applies the process-wide local timezone (time.Local) from the
environment. This is the env-driven entry point gofi's builder uses.

### core.Timezone

```go
func Timezone(env *environment.Environment) timezone.Config
```

Timezone builds a timezone.Config from the TIMEZONE variable of the given
environment. An empty value means UTC.

