# gofi/component/httpserver

`import "github.com/joaoprofile/gofi-sdk-go/gofi/component/httpserver"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package httpserver is the gofi component for the HTTP server (netx/httpx). It is a
gofi.Runner: ListenAndServe serves it and stops it gracefully.

## Funções

### httpserver.ConfigFromEnv

```go
func ConfigFromEnv(env *environment.Environment) (httpx.WSConfig, error)
```

ConfigFromEnv builds the httpx.WSConfig fields set by the environment:

  - HTTP_TLS_CERT_FILE / HTTP_TLS_KEY_FILE → TLS (both or neither)
  - HTTP_TLS_CLIENT_CA_FILE               → TLS.ClientCAFile (mTLS)
  - HTTP_TLS_CLIENT_AUTH                  → TLS.ClientAuth: none | request |
    require | verify_if_given | require_and_verify (default
    require_and_verify with a client CA, none without)
  - HTTP_TRUSTED_PROXIES                  → TrustedProxies (CSV of CIDRs, or "private")
  - HTTP_MAX_CONCURRENT                   → StressControl.DefaultMaxConcurrent
  - HTTP_ALLOWED_ORIGINS (ALLOWED_ORIGINS as fallback) → AllowedOrigins (CSV)

HTTP_RATE_LIMIT_FAIL_CLOSED needs a rate limiter backend, which only code
can provide, so it is applied by New to the WSConfig.RateLimiter given there.
Invalid values are returned as errors, never panics.

## httpserver.Component

```go
type Component struct {
	// contains filtered or unexported fields
}
```

Component wraps a httpx.HttpServer. With New the server is built in Start,
once the environment is known; handlers and middleware attached before
that are replayed on it in order.

### httpserver.FromServer

```go
func FromServer(server httpx.HttpServer) *Component
```

FromServer uses a server built by the caller; gofi serves and stops it, and
the environment does not change it.

### httpserver.New

```go
func New(port string, cfg ...*httpx.WSConfig) *Component
```

New creates the server listening on port (":8080"); cfg is optional and is
copied. In Start its unset fields are filled from the HTTP_* environment
(see ConfigFromEnv): code-provided values always win. The readiness checks
of the other components are served when cfg.Health is set.

### Component.Handlers

```go
func (c *Component) Handlers(handlers ...httpx.RouterHandler) *Component
```

Handlers registers the route handlers.

### Component.InsecureTransports

```go
func (c *Component) InsecureTransports(env *environment.Environment) []gofi.InsecureTransport
```

InsecureTransports reports the server without TLS; in prod and stage it is
only a warning unless HTTP_REQUIRE_TLS=true (see core.InsecureHTTP).
Servers passed to FromServer are the caller's and are not checked.

### Component.Name

```go
func (c *Component) Name() string
```

### Component.Run

```go
func (c *Component) Run() error
```

### Component.Server

```go
func (c *Component) Server() httpx.HttpServer
```

Server returns the underlying httpx server: with New, nil until Build
started the component.

### Component.Stage

```go
func (c *Component) Stage() gofi.Stage
```

### Component.Start

```go
func (c *Component) Start(_ context.Context, rt *gofi.Runtime) error
```

Start builds the server from the code config and the environment, then
registers the readiness checks of the components started before it.

### Component.Stop

```go
func (c *Component) Stop(ctx context.Context) error
```

### Component.Use

```go
func (c *Component) Use(middleware ...httpx.Middleware) *Component
```

Use adds global middleware, run on every route.

### Component.UseAuth

```go
func (c *Component) UseAuth(auth httpx.Middleware) *Component
```

UseAuth sets the middleware run on private routes; routes added before it
do not get it.

