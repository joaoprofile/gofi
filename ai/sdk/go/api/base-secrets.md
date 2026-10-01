# base/secrets

`import "github.com/gofi-labs/gofi-sdk-go/base/secrets"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package secrets resolves secret references of the form

	secret://<provider>/<name>[#<json-key>]

e.g. secret://awssm/prod/db#password. gofi's environment loader resolves
any variable holding a reference at startup, so credentials stay in the
secret manager instead of the deployment manifest. Providers register
themselves: "env" and "file" are built in; awssm and ocivault live in
their own modules.

## Constantes

```go
const Prefix = "secret://"
```

Prefix starts every secret reference.

## Variáveis

```go
var (
	// ErrInvalidRef is returned for malformed references or unknown providers.
	ErrInvalidRef = errors.New("secrets: invalid reference")
	// ErrNotFound is returned when the secret or its JSON key does not exist.
	ErrNotFound = errors.New("secrets: not found")
)
```

## Funções

### secrets.IsRef

```go
func IsRef(s string) bool
```

IsRef reports whether s is a secret reference.

### secrets.Register

```go
func Register(provider string, o Opener)
```

Register makes a provider available to references.

### secrets.Resolve

```go
func Resolve(ctx context.Context, s string) (string, error)
```

Resolve resolves a single reference with a fresh Resolver.

## secrets.Opener

```go
type Opener func(ctx context.Context) (Store, error)
```

Opener creates a provider's Store on first use.

## secrets.Ref

```go
type Ref struct {
	Provider string
	Name     string
	Key      string // optional field of a JSON secret
}
```

Ref is a parsed secret reference.

### secrets.ParseRef

```go
func ParseRef(s string) (Ref, error)
```

ParseRef parses secret://<provider>/<name>[#<key>].

## secrets.Resolver

```go
type Resolver struct {
	// contains filtered or unexported fields
}
```

Resolver resolves references, opening each provider and fetching each
secret once. It is safe for concurrent use.

### secrets.NewResolver

```go
func NewResolver() *Resolver
```

NewResolver returns an empty Resolver.

### Resolver.Resolve

```go
func (r *Resolver) Resolve(ctx context.Context, s string) (string, error)
```

Resolve returns s unchanged unless it is a reference.

## secrets.Store

```go
type Store interface {
	Get(ctx context.Context, name string) (string, error)
}
```

Store fetches the raw value of a secret by provider-specific name.

## secrets.StoreFunc

```go
type StoreFunc func(ctx context.Context, name string) (string, error)
```

StoreFunc adapts a function to Store.

### StoreFunc.Get

```go
func (f StoreFunc) Get(ctx context.Context, name string) (string, error)
```

