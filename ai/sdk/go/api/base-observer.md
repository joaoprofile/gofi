# base/observer

`import "github.com/gofi-labs/gofi-sdk-go/base/observer"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Variáveis

```go
var (
	WAIT_GROUP_TIMEOUT_SECONDS = 90
)
```

## Funções

### observer.Attach

```go
func Attach(o Observer)
```

### observer.GetWaitGroup

```go
func GetWaitGroup() *sync.WaitGroup
```

### observer.Instance

```go
func Instance() *instanceObserver
```

### observer.Shutdown

```go
func Shutdown()
```

Shutdown closes every attached observer in reverse (LIFO) order, once.
The observer no longer listens to OS signals: gofi's Service.Run (or the
application, via signal.NotifyContext) decides when to call it.

### observer.WaitRunningTimeout

```go
func WaitRunningTimeout() bool
```

## observer.Closer

```go
type Closer interface {
	Close(ctx context.Context) error
}
```

## observer.Observer

```go
type Observer interface {
	Close()
}
```

## observer.Registry

```go
type Registry struct {
	// contains filtered or unexported fields
}
```

### observer.New

```go
func New() *Registry
```

### Registry.CloseAll

```go
func (r *Registry) CloseAll(ctx context.Context) error
```

CloseAll closes registered resources in reverse (LIFO) order, continues past
failures and returns every error joined.

### Registry.CloseWithTimeout

```go
func (r *Registry) CloseWithTimeout(timeout time.Duration) error
```

### Registry.Register

```go
func (r *Registry) Register(c ...any)
```

