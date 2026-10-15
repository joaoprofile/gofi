# gofi/component/grpcserver

`import "github.com/joaoprofile/gofi-sdk-go/gofi/component/grpcserver"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package grpcserver is the gofi component for the gRPC server (netx/grpcx).
It is a gofi.Runner: ListenAndServe serves it next to the other runners and
stops it gracefully.

	grpc := grpcserver.New(":9090").Register(func(r grpc.ServiceRegistrar) {
	    ordersv1.RegisterOrdersServer(r, ordersImpl)
	})
	svc, err := gofi.New("orders").With(database.New(), grpc).Build()

## Funções

### grpcserver.ConfigFromEnv

```go
func ConfigFromEnv(env *environment.Environment) (*netx.TLSConfig, error)
```

ConfigFromEnv builds the server TLS from the environment, nil when no
GRPC_TLS_* variable is set:

  - GRPC_TLS_CERT_FILE / GRPC_TLS_KEY_FILE → certificate (both or neither,
    reloaded on change)
  - GRPC_TLS_CLIENT_CA_FILE                → client CAs (mTLS)
  - GRPC_TLS_CLIENT_AUTH                   → none | request | require |
    verify_if_given | require_and_verify (default require_and_verify with
    a client CA, none without)

Invalid values are returned as errors, never panics.

## grpcserver.Component

```go
type Component struct {
	// contains filtered or unexported fields
}
```

Component wraps a grpcx.Server. With New the server is built in Start,
once the environment is known; services registered before that are
replayed on it in order.

### grpcserver.FromServer

```go
func FromServer(server grpcx.Server) *Component
```

FromServer uses a server built by the caller; gofi serves and stops it, and
the environment does not change it.

### grpcserver.New

```go
func New(addr string, cfg ...*grpcx.ServerConfig) *Component
```

New creates the server listening on addr (":9090"); cfg is optional and is
copied. In Start its unset TLS is filled from GRPC_TLS_* (see
ConfigFromEnv): code-provided values always win.

### Component.InsecureTransports

```go
func (c *Component) InsecureTransports(env *environment.Environment) []gofi.InsecureTransport
```

InsecureTransports reports the server without TLS; in prod and stage it is
only a warning unless GRPC_REQUIRE_TLS=true (see core.InsecureGRPC).
Servers passed to FromServer are the caller's and are not checked.

### Component.Name

```go
func (c *Component) Name() string
```

### Component.Register

```go
func (c *Component) Register(register func(r grpc.ServiceRegistrar)) *Component
```

Register registers services, typically with the generated
Register*Server functions.

### Component.Run

```go
func (c *Component) Run() error
```

### Component.Server

```go
func (c *Component) Server() grpcx.Server
```

Server returns the underlying grpcx server: with New, nil until Build
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
registers the readiness checks of the components started before it on the
grpc.health.v1 service.

### Component.Stop

```go
func (c *Component) Stop(ctx context.Context) error
```

