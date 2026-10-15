# gofi/config/core

`import "github.com/joaoprofile/gofi-sdk-go/gofi/config/core"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package core holds the process-wide settings gofi's Build applies before any
component starts: timezone, logging and TLS verification. It depends only on
base/environment, base/timezone and obs/logging, so importing it links no
resource; package config re-exports these functions.

## Constantes

```go
const (
	ResourceDatabase  = "database"
	ResourceCache     = "cache"
	ResourceMessaging = "messaging"
	ResourceOTLP      = "otlp"
	ResourceBucket    = "bucket"
	// ResourceTLS is TLS_INSECURE_SKIP_VERIFY on http.DefaultTransport.
	ResourceTLS = "tls"
	// ResourceHTTP is the HTTP server served without TLS.
	ResourceHTTP = "http"
	// ResourceGRPC is the gRPC server served without TLS.
	ResourceGRPC = "grpc"
	// AllowAll in GOFI_ALLOW_INSECURE_TRANSPORT allows every resource.
	AllowAll = "all"
)
```

Resource names accepted by GOFI_ALLOW_INSECURE_TRANSPORT.

## Variáveis

```go
var ErrInsecureTransport = errors.New("insecure transport refused in production")
```

ErrInsecureTransport is wrapped by every refusal of CheckTransport.

## Funções

### core.ApplyTLS

```go
func ApplyTLS(env *environment.Environment)
```

ApplyTLS applies TLS_INSECURE_SKIP_VERIFY to http.DefaultTransport; it is a no-op by default.

### core.CheckTransport

```go
func CheckTransport(env *environment.Environment, found []InsecureTransport) error
```

CheckTransport refuses the insecure transports found in prod and stage,
unless GOFI_ALLOW_INSECURE_TRANSPORT names their resource (or "all"), in
which case each one is logged as a warning. Other environments are not checked.

### core.InitLogging

```go
func InitLogging(env *environment.Environment, serviceName string) error
```

InitLogging initialises the global logger from the environment. This is the
env-driven entry point services should use; logging.NewLogger is a
default-only convenience for tests.

### core.IsLoopback

```go
func IsLoopback(addr string) bool
```

IsLoopback reports whether addr (host, host:port or URL) is empty, localhost
or a loopback address: plaintext to it never leaves the machine.

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

## core.InsecureTransport

```go
type InsecureTransport struct {
	Resource string // name accepted by GOFI_ALLOW_INSECURE_TRANSPORT
	Setting  string // environment variable that fixes it
	Detail   string // what is insecure
}
```

InsecureTransport is a connection that would run in plaintext or without
verifying the server certificate.

### core.InsecureCache

```go
func InsecureCache(env *environment.Environment) []InsecureTransport
```

InsecureCache reports the CACHE_* Redis connection in plaintext to a
non-loopback host. Components that open it (cache, session, iam) return it.

### core.InsecureGRPC

```go
func InsecureGRPC(env *environment.Environment, tlsEnabled bool) []InsecureTransport
```

InsecureGRPC reports a gRPC server without TLS, with the same policy as
InsecureHTTP: a warning in prod and stage, a refusal with GRPC_REQUIRE_TLS.
Internal gRPC is often plaintext inside a private network or behind a mesh
that encrypts the hop.

### core.InsecureHTTP

```go
func InsecureHTTP(env *environment.Environment, tlsEnabled bool) []InsecureTransport
```

InsecureHTTP reports an HTTP server without TLS. Most deployments terminate
TLS at an ingress, load balancer or mesh sidecar and reach the pod in
plaintext inside the cluster, so refusing it would break them: in prod and
stage it is only logged as a warning, unless HTTP_REQUIRE_TLS=true, which
reports it for CheckTransport to refuse.

### core.InsecureTransports

```go
func InsecureTransports(env *environment.Environment) []InsecureTransport
```

InsecureTransports reports the process-wide settings that disable TLS
verification or send data in plaintext: TLS_INSECURE_SKIP_VERIFY and an
S3-compatible bucket endpoint over http. Components report their own.

