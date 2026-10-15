# base/bucket/buckettest

`import "github.com/joaoprofile/gofi-sdk-go/base/bucket/buckettest"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package buckettest is the contract every bucket.Store must satisfy. Run it
from each provider's tests so behaviour stays identical across backends.

## Funções

### buckettest.Run

```go
func Run(t *testing.T, s bucket.Store)
```

Run exercises s; it must start empty and accept any key.

### buckettest.RunManager

```go
func RunManager(t *testing.T, m bucket.Manager)
```

RunManager exercises m; the account must not hold a bucket named
"gofi-buckettest" nor "gofi-buckettest-missing".

