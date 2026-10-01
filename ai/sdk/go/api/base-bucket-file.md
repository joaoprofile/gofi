# base/bucket/file

`import "github.com/gofi-labs/gofi-sdk-go/base/bucket/file"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package file implements bucket.Store on a local directory, for development
and single-node deployments. Keys map to paths below the root and cannot
escape it. Importing it registers the "file" provider for bucket.Open
(Config.Endpoint is the directory). ContentType is derived from the key
extension.

## file.Store

```go
type Store struct {
	// contains filtered or unexported fields
}
```

Store is a bucket.Store rooted at a directory.

### file.New

```go
func New(dir string) (*Store, error)
```

New opens (creating when missing) the root directory.

### Store.All

```go
func (s *Store) All(_ context.Context, prefix string) iter.Seq2[bucket.Object, error]
```

All walks the directory in lexical order.

### Store.Close

```go
func (s *Store) Close() error
```

Close releases the root directory handle.

### Store.Delete

```go
func (s *Store) Delete(_ context.Context, key string) error
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
func (s *Store) PresignGet(_ context.Context, key string, _ time.Duration) (string, error)
```

PresignGet returns a file:// URL; it is only readable on this host.

### Store.Put

```go
func (s *Store) Put(_ context.Context, in bucket.PutInput) error
```

Put writes to a temporary file and renames it, so readers never see a
partial object.

