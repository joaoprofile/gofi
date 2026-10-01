# base/bucket/s3

`import "github.com/gofi-labs/gofi-sdk-go/base/bucket/s3"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package s3 implements bucket.Store for Amazon S3 and S3-compatible services
(MinIO, Cloudflare R2, ...). Importing it registers the "s3" and "minio"
providers for bucket.Open.

## s3.Config

```go
type Config struct {
	Bucket string
	AWS    cloudaws.Config
	// PathStyle forces path-style addressing; it is implied by a custom
	// AWS.Endpoint, which MinIO and most S3-compatible services require.
	PathStyle bool
}
```

Config configures the store. With an empty AWS config the default
credential chain and region are used.

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

### Store.Get

```go
func (s *Store) Get(ctx context.Context, key string) (bucket.Object, io.ReadCloser, error)
```

### Store.List

```go
func (s *Store) List(ctx context.Context, prefix string) ([]bucket.Object, error)
```

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

