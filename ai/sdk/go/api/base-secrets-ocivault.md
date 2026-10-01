# base/secrets/ocivault

`import "github.com/gofi-labs/gofi-sdk-go/base/secrets/ocivault"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package ocivault resolves secret://ocivault/<secret-ocid>[#key] and
secret://ocivault/<vault-ocid>/<secret-name>[#key] from OCI Vault.

OCI has no implicit credential chain, so enable it explicitly before the
environment loads, typically in main:

	ocivault.Register(ocivault.Config{Credentials: cloudoci.Config{AuthMode: cloudoci.AuthWorkloadIdentity}})

## Constantes

```go
const Provider = "ocivault"
```

Provider is the reference provider name.

## Funções

### ocivault.Register

```go
func Register(cfg Config)
```

Register enables the provider for secret references.

## ocivault.Config

```go
type Config struct {
	Credentials cloudoci.Config
	// Endpoint overrides the service host (tests, dedicated endpoints).
	Endpoint string
}
```

Config selects the identity; Region defaults to the one in each OCID.

## ocivault.Store

```go
type Store struct {
	// contains filtered or unexported fields
}
```

Store reads secret bundles; it keeps one client per region.

### ocivault.New

```go
func New(cfg Config) (*Store, error)
```

New validates the identity; clients are created on first use.

### Store.Get

```go
func (s *Store) Get(ctx context.Context, name string) (string, error)
```

Get returns the current version of a secret by OCID or <vault-ocid>/<name>.

