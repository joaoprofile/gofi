# gofi

`import "github.com/joaoprofile/gofi-sdk-go/gofi"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## gofi.Builder

```go
type Builder interface {
	// With adds components. The order does not matter: Build starts them by
	// Stage and closes them in reverse.
	With(components ...Component) Builder

	// Build loads the environment, sets up logging and starts every component.
	// It returns every declaration error joined; if a component fails to
	// start, the ones already started are closed and the Service is nil.
	Build() (Service, error)
}
```

Builder declares the components of a GOFI service. Call Build to start them
and obtain a Service.

The root package only knows the Component contract: each resource (database,
cache, session, messaging, observability, HTTP server) lives in its own
package under component/, so a binary links only the resources it imports.

	db := database.New()
	svc, err := gofi.New("catalog-api").
		With(db, httpserver.New(":8080", httpserver.Handlers(h))).
		Build()

### gofi.New

```go
func New(serviceName string) Builder
```

New returns a Builder for the named service. It has no side effects: the
environment, logging and every component are set up by Build.

## gofi.Component

```go
type Component interface {
	Name() string
	Stage() Stage
	Start(ctx context.Context, rt *Runtime) error
}
```

Component is a resource started by Build. Start must register in rt what it
opens (rt.OnClose) so Shutdown and a failed Build can release it.

## gofi.HealthCheck

```go
type HealthCheck struct {
	Name  string
	Check func(ctx context.Context) error
}
```

HealthCheck is a readiness check registered by a component; the HTTP server
component serves them when its Health config is set.

## gofi.Runner

```go
type Runner interface {
	Component
	// Run blocks until the runner stops; it returns nil on a clean stop.
	Run() error
	// Stop makes Run return; it is safe to call more than once.
	Stop(ctx context.Context) error
}
```

Runner is a Component that serves until stopped, such as the HTTP server.
ListenAndServe runs every Runner and stops them all when one returns.

## gofi.Runtime

```go
type Runtime struct {
	// contains filtered or unexported fields
}
```

Runtime is what Build hands to each Component.Start: the environment, the
registry of what must be closed, the readiness checks and the resources that
components share (one Redis client for cache and session, for instance).

### gofi.NewRuntime

```go
func NewRuntime(env *environment.Environment) *Runtime
```

NewRuntime returns a Runtime for env. Build creates its own; it is exported
so components can be started in tests without a Builder.

### Runtime.AddHealthCheck

```go
func (r *Runtime) AddHealthCheck(name string, check func(ctx context.Context) error)
```

AddHealthCheck registers a readiness check.

### Runtime.Close

```go
func (r *Runtime) Close(ctx context.Context) error
```

Close runs the registered closers in reverse order and forgets them.

### Runtime.Env

```go
func (r *Runtime) Env() *environment.Environment
```

Env returns the environment loaded by Build.

### Runtime.HealthChecks

```go
func (r *Runtime) HealthChecks() []HealthCheck
```

HealthChecks returns the checks registered so far.

### Runtime.OnClose

```go
func (r *Runtime) OnClose(fn func(ctx context.Context) error)
```

OnClose registers fn to run on Shutdown, or when a later component fails to
start. Closers run in reverse order of registration.

### Runtime.Shared

```go
func (r *Runtime) Shared(key string, open func() (any, error)) (any, error)
```

Shared returns the value stored under key, calling open to create it the
first time. Components use it to share one connection; open is responsible
for registering its closer. A failed open is not stored.

## gofi.Service

```go
type Service interface {
	Environment() *environment.Environment

	// ListenAndServe runs the Runner components (the HTTP server) and blocks
	// until SIGINT/SIGTERM, Shutdown or a Runner failure; without Runners it
	// just waits for the signal. It then stops the Runners and closes what the
	// components opened, in reverse order. It returns nil on a clean stop.
	ListenAndServe() error

	// Shutdown stops a running ListenAndServe and waits for it to finish; when
	// the service is not running it closes the resources directly. Resources
	// injected with From* constructors are owned by the caller and not closed.
	Shutdown(ctx context.Context) error
}
```

Service is the runtime contract of a built GOFI service. The resources are
reached through the components themselves (db.DB(), cache.Client(), ...).

## gofi.Stage

```go
type Stage int
```

Stage orders Build: components start in ascending Stage (declaration order
breaks ties) and are closed in reverse.

```go
const (
	StageObservability Stage = 100
	StageDatabase      Stage = 200
	StageCache         Stage = 300
	StageSession       Stage = 400
	StageMessaging     Stage = 500
	StageIAM           Stage = 600
	// StageServer starts last, so it sees the health checks registered by
	// every other component.
	StageServer Stage = 900
)
```

