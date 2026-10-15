# base/debug

`import "github.com/joaoprofile/gofi-sdk-go/base/debug"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const DefaultAddr = "localhost:6060"
```

DefaultAddr is used when Config.Addr is empty; loopback only.

## Funções

### debug.Start

```go
func Start(cfg Config)
```

Start launches the pprof server in a background goroutine using cfg. The
caller decides whether debugging is enabled; gofi's config.StartDebug reads
SERVICE_DEBUG and the SERVICE_DEBUG_* settings from the environment.

## debug.Config

```go
type Config struct {
	// Addr is the TCP address to listen on. Defaults to DefaultAddr.
	Addr string

	// User and Pass, when both non-empty, protect every route with HTTP basic
	// auth. Without them a non-loopback Addr is rebound to 127.0.0.1.
	User string
	Pass string `redact:"true"`
}
```

Config holds the configuration for the pprof debug server.

### Config.Format

```go
func (c Config) Format(f fmt.State, verb rune)
```

### Config.GoString

```go
func (c Config) GoString() string
```

### Config.LogValue

```go
func (c Config) LogValue() slog.Value
```

### Config.MarshalJSON

```go
func (c Config) MarshalJSON() ([]byte, error)
```

### Config.String

```go
func (c Config) String() string
```

String, GoString, Format, LogValue and MarshalJSON mask the secret fields.

## debug.Server

```go
type Server struct {
	// contains filtered or unexported fields
}
```

Server is a pprof debug HTTP server with optional basic-auth protection.

### debug.New

```go
func New(cfg Config) *Server
```

New returns a new debug Server with the given configuration.

### Server.Format

```go
func (s *Server) Format(f fmt.State, _ rune)
```

Format keeps fmt from printing the unexported cfg (and its password) raw.

### Server.Handler

```go
func (s *Server) Handler() http.Handler
```

Handler serves the pprof and expvar routes, behind basic auth when credentials are set.

### Server.ListenAndServe

```go
func (s *Server) ListenAndServe() error
```

ListenAndServe starts the debug HTTP server and blocks until it returns.

