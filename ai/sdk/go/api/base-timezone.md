# base/timezone

`import "github.com/joaoprofile/gofi-sdk-go/base/timezone"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const BrazilName = "America/Sao_Paulo"
```

BrazilName is the IANA name for Brazil's primary timezone and the default
applied when no name is configured.

## Funções

### timezone.Apply

```go
func Apply(cfg Config) error
```

Apply sets time.Local from the configured timezone. An empty Name selects
UTC; an invalid Name returns an error and leaves time.Local untouched. The
IANA database is embedded, so names resolve in scratch/distroless images.

## timezone.Config

```go
type Config struct {
	// Name is the IANA timezone name (e.g. "America/Sao_Paulo"). When empty,
	// UTC is used.
	Name string
}
```

Config configures the process-wide local timezone.

