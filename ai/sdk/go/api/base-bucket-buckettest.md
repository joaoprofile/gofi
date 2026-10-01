# base/bucket/buckettest

`import "github.com/gofi-labs/gofi-sdk-go/base/bucket/buckettest"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package buckettest is the contract every bucket.Store must satisfy. Run it
from each provider's tests so behaviour stays identical across backends.

## Funções

### buckettest.Run

```go
func Run(t *testing.T, s bucket.Store)
```

Run exercises s; it must start empty and accept any key.

