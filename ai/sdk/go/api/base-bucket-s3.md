# base/bucket/s3

`import "github.com/joaoprofile/gofi-sdk-go/base/bucket/s3"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package s3 implements bucket.Store for Amazon S3 and S3-compatible services
(MinIO, Cloudflare R2, ...). Importing it registers the "s3" and "minio"
providers for bucket.Open and bucket.OpenManager.

## s3.Config

```go
type Config struct {
	Bucket string
	AWS    cloudaws.Config
	// PathStyle forces path-style addressing; it is implied by a custom
	// AWS.Endpoint, which MinIO and most S3-compatible services require.
	PathStyle bool
	// PresignMaxTTL lowers the longest presigned URL validity; see
	// bucket.PresignLimit.
	PresignMaxTTL time.Duration
}
```

Config configures the store. With an empty AWS config the default
credential chain and region are used.

## s3.Manager

```go
type Manager struct {
	// contains filtered or unexported fields
}
```

Manager administers the S3 buckets of an account. Safe for concurrent use.

### s3.NewManager

```go
func NewManager(ctx context.Context, cfg Config) (*Manager, error)
```

NewManager builds a Manager; credentials resolve through base/cloud/aws.
cfg.Bucket is ignored.

### Manager.CreateBucket

```go
func (m *Manager) CreateBucket(ctx context.Context, name string, opts bucket.CreateOptions) error
```

CreateBucket creates name in opts.Region, else the configured region, else
us-east-1. A bucket the account already owns is ErrBucketExists in every
region, although us-east-1 answers a repeated CreateBucket with success.

### Manager.DeleteBucket

```go
func (m *Manager) DeleteBucket(ctx context.Context, name string, opts bucket.DeleteOptions) error
```

DeleteBucket deletes name from its own region. With Force it first deletes
every object; a versioned bucket still holds the old versions afterwards,
so it stays bucket.ErrBucketNotEmpty.

### Manager.ListBuckets

```go
func (m *Manager) ListBuckets(ctx context.Context) ([]bucket.BucketInfo, error)
```

### Manager.Open

```go
func (m *Manager) Open(ctx context.Context, name string) (bucket.Store, error)
```

Open returns the Store of name, in the bucket's own region unless a custom
endpoint is configured. The Store is cached per name.

## s3.Store

```go
type Store struct {
	// contains filtered or unexported fields
}
```

Store is an S3-backed bucket.Store.

### s3.New

```go
func New(ctx context.Context, cfg Config) (*Store, error)
```

New builds a Store; credentials resolve through base/cloud/aws.

### Store.All

```go
func (s *Store) All(ctx context.Context, prefix string) iter.Seq2[bucket.Object, error]
```

All streams the listing page by page.

### Store.Delete

```go
func (s *Store) Delete(ctx context.Context, key string) error
```

### Store.DeleteMany

```go
func (s *Store) DeleteMany(ctx context.Context, keys []string) error
```

DeleteMany deletes keys with DeleteObjects, up to 1000 per request. Every
key the response reports as failed becomes an error naming it.

### Store.Get

```go
func (s *Store) Get(ctx context.Context, key string) (bucket.Object, io.ReadCloser, error)
```

### Store.List

```go
func (s *Store) List(ctx context.Context, prefix string) ([]bucket.Object, error)
```

### Store.ListDir

```go
func (s *Store) ListDir(ctx context.Context, prefix string) (bucket.Dir, error)
```

ListDir lists one level server-side with the "/" delimiter.

### Store.PresignGet

```go
func (s *Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
```

### Store.Put

```go
func (s *Store) Put(ctx context.Context, in bucket.PutInput) error
```

Put streams the body; unknown sizes and large objects use multipart upload
without buffering the whole object in memory.

### Store.Stat

```go
func (s *Store) Stat(ctx context.Context, key string) (bucket.Object, error)
```

Stat reads the metadata with HeadObject. A HEAD response has no body, so
a missing bucket is reported as bucket.ErrNotFound too.

