# base/bucket/mem

`import "github.com/joaoprofile/gofi-sdk-go/base/bucket/mem"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package mem implements bucket.Store in memory, for tests and local runs.
Importing it registers the "mem" provider for bucket.Open.

## mem.Manager

```go
type Manager struct {
	// contains filtered or unexported fields
}
```

Manager is an in-memory bucket.Manager; each bucket is a Store. Safe for
concurrent use.

### mem.NewManager

```go
func NewManager() *Manager
```

NewManager returns a Manager without buckets.

### Manager.CreateBucket

```go
func (m *Manager) CreateBucket(_ context.Context, name string, _ bucket.CreateOptions) error
```

CreateBucket ignores opts.Region.

### Manager.DeleteBucket

```go
func (m *Manager) DeleteBucket(ctx context.Context, name string, opts bucket.DeleteOptions) error
```

### Manager.ListBuckets

```go
func (m *Manager) ListBuckets(context.Context) ([]bucket.BucketInfo, error)
```

ListBuckets returns the buckets in name order; Region is empty.

### Manager.Open

```go
func (m *Manager) Open(_ context.Context, name string) (bucket.Store, error)
```

## mem.Store

```go
type Store struct {
	// contains filtered or unexported fields
}
```

Store is an in-memory bucket.Store safe for concurrent use.

### mem.New

```go
func New(name string) *Store
```

New returns an empty Store; name only appears in presigned URLs.

### Store.All

```go
func (s *Store) All(_ context.Context, prefix string) iter.Seq2[bucket.Object, error]
```

All yields objects in key order.

### Store.Delete

```go
func (s *Store) Delete(_ context.Context, key string) error
```

### Store.DeleteMany

```go
func (s *Store) DeleteMany(_ context.Context, keys []string) error
```

### Store.Get

```go
func (s *Store) Get(_ context.Context, key string) (bucket.Object, io.ReadCloser, error)
```

### Store.List

```go
func (s *Store) List(ctx context.Context, prefix string) ([]bucket.Object, error)
```

### Store.PresignGet

```go
func (s *Store) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error)
```

PresignGet returns a mem:// URL; it is not downloadable.

### Store.Put

```go
func (s *Store) Put(_ context.Context, in bucket.PutInput) error
```

### Store.Stat

```go
func (s *Store) Stat(_ context.Context, key string) (bucket.Object, error)
```

