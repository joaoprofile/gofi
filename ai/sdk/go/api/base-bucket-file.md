# base/bucket/file

`import "github.com/joaoprofile/gofi-sdk-go/base/bucket/file"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package file implements bucket.Store on a local directory, for development
and single-node deployments. Keys map to paths below the root and cannot
escape it. Importing it registers the "file" provider for bucket.Open
(Config.Endpoint is the directory) and bucket.OpenManager (Config.Endpoint
is the parent directory, one subdirectory per bucket). ContentType is
derived from the key extension.

Folders are directories: a "path/" marker key creates one, an empty
directory lists as its "path/" marker, and a non-empty one is implied by
the keys below it.

## file.Manager

```go
type Manager struct {
	// contains filtered or unexported fields
}
```

Manager is a bucket.Manager over a directory: each bucket is a
subdirectory whose name passes bucket.ValidateName. Safe for concurrent use.

### file.NewManager

```go
func NewManager(dir string) (*Manager, error)
```

NewManager uses dir (created when missing) as the parent of the buckets.

### Manager.Close

```go
func (m *Manager) Close() error
```

Close releases the directory handles of the opened stores.

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

ListBuckets returns the subdirectories with a valid bucket name, in name
order; CreatedAt is the directory's modification time and Region is empty.

### Manager.Open

```go
func (m *Manager) Open(ctx context.Context, name string) (bucket.Store, error)
```

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

All walks the directory in lexical order. An empty directory yields its
"dir/" folder marker.

### Store.Close

```go
func (s *Store) Close() error
```

Close releases the root directory handle.

### Store.CreateFolder

```go
func (s *Store) CreateFolder(_ context.Context, path string) error
```

CreateFolder creates the directory of path ("a/b" or "a/b/").

### Store.Delete

```go
func (s *Store) Delete(_ context.Context, key string) error
```

Delete removes the file of key. For a folder marker ("dir/") it removes the
directory when empty and is a no-op otherwise, since the folder still
exists through the keys below it.

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

PresignGet returns a file:// URL; it is only readable on this host.

### Store.Put

```go
func (s *Store) Put(ctx context.Context, in bucket.PutInput) error
```

Put writes to a temporary file and renames it, so readers never see a
partial object. A key ending in "/" is a folder marker: it creates the
directory and its body must be empty.

### Store.Stat

```go
func (s *Store) Stat(_ context.Context, key string) (bucket.Object, error)
```

