# base/bucket/oci

`import "github.com/joaoprofile/gofi-sdk-go/base/bucket/oci"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package oci implements bucket.Store on top of OCI Object Storage.

Build a Store with New, or blank-import this package and call bucket.Open
to select the backend from configuration.

## oci.Config

```go
type Config struct {
	Bucket string
	// Namespace is the Object Storage namespace. When empty it is resolved
	// lazily with a GetNamespace call and cached.
	Namespace string
	// Endpoint overrides the regional Object Storage endpoint.
	Endpoint string
	// Credentials selects the principal and region (see base/cloud/oci).
	Credentials cloudoci.Config
}
```

Config holds the settings required to reach a single OCI Object Storage
bucket.

## oci.Store

```go
type Store struct {
	// contains filtered or unexported fields
}
```

Store is the OCI Object Storage implementation of bucket.Store. It targets a
single bucket and resolves the Object Storage namespace lazily on first use.

### oci.New

```go
func New(cfg Config) (*Store, error)
```

New builds an OCI-backed Store from cfg. It validates credentials and
constructs the SDK client but performs no network I/O; the namespace and
bucket are contacted only when a Store method is called.

### Store.All

```go
func (s *Store) All(ctx context.Context, prefix string) iter.Seq2[bucket.Object, error]
```

All streams the listing page by page.

### Store.Delete

```go
func (s *Store) Delete(ctx context.Context, key string) error
```

Delete removes an object. It is idempotent: a missing key is treated as a
successful delete, matching the abstraction's contract.

### Store.Get

```go
func (s *Store) Get(ctx context.Context, key string) (bucket.Object, io.ReadCloser, error)
```

Get downloads an object. The caller must close the returned reader.

### Store.List

```go
func (s *Store) List(ctx context.Context, prefix string) ([]bucket.Object, error)
```

List returns every object whose key starts with prefix.

### Store.PresignGet

```go
func (s *Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
```

PresignGet issues a read-only Pre-Authenticated Request (PAR) scoped to a
single object and returns the absolute download URL valid for ttl.

### Store.Put

```go
func (s *Store) Put(ctx context.Context, in bucket.PutInput) error
```

