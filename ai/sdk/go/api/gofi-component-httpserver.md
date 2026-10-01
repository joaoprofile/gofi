# gofi/component/httpserver

`import "github.com/joaoprofile/gofi-sdk-go/gofi/component/httpserver"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package httpserver is the gofi component for the HTTP server (netx). It is a
gofi.Runner: ListenAndServe serves it and stops it gracefully.

## httpserver.Component

```go
type Component struct {
	// contains filtered or unexported fields
}
```

Component wraps a netx.HttpServer. The server lives in memory until
ListenAndServe, so handlers and middleware can be attached while chaining.

### httpserver.FromServer

```go
func FromServer(server netx.HttpServer) *Component
```

FromServer uses a server built by the caller; gofi serves and stops it.

### httpserver.New

```go
func New(port string, cfg ...*netx.WSConfig) *Component
```

New creates the server listening on port (":8080"); cfg is optional.
The readiness checks of the other components are served when cfg.Health is set.

### Component.Handlers

```go
func (c *Component) Handlers(handlers ...netx.RouterHandler) *Component
```

Handlers registers the route handlers.

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
func (c *Component) Server() netx.HttpServer
```

Server returns the underlying netx server.

### Component.Stage

```go
func (c *Component) Stage() gofi.Stage
```

### Component.Start

```go
func (c *Component) Start(_ context.Context, rt *gofi.Runtime) error
```

Start registers the readiness checks of the components started before it.

### Component.Stop

```go
func (c *Component) Stop(ctx context.Context) error
```

### Component.Use

```go
func (c *Component) Use(middleware ...netx.Middleware) *Component
```

Use adds global middleware, run on every route.

### Component.UseAuth

```go
func (c *Component) UseAuth(auth netx.Middleware) *Component
```

UseAuth sets the middleware run on private routes.

