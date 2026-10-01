# base/bucket

`import "github.com/joaoprofile/gofi-sdk-go/base/bucket"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package bucket defines a provider-agnostic object-storage abstraction.

A Store models a single bucket and exposes the minimal set of operations
shared by every object-storage backend: upload, download, list and delete.
Concrete backends (OCI Object Storage, AWS S3, GCS, …) live in their own
sub-packages and register themselves through Register so that callers can
obtain a Store by provider name via Open, without importing the backend
directly.

## Variáveis

```go
var (
	// ErrNotFound is returned when an object (or its bucket) does not exist.
	ErrNotFound = errors.New("bucket: object not found")

	// ErrInvalidConfig is returned by a factory when the supplied Config is
	// missing required fields.
	ErrInvalidConfig = errors.New("bucket: invalid configuration")
)
```

Sentinel errors returned by every Store implementation so that callers can
branch on the failure mode independently of the underlying provider.

## Funções

### bucket.All

```go
func All(ctx context.Context, s Store, prefix string) iter.Seq2[Object, error]
```

All yields the objects whose key starts with prefix without loading the
whole listing in memory when the store is a Walker. A listing error is
yielded once and ends the sequence.

### bucket.Register

```go
func Register(p Provider, o Opener)
```

Register makes a provider available to Open. Provider packages call it from
init, so importing one (e.g. _ ".../base/bucket/oci") is enough to enable it.

## bucket.Config

```go
type Config struct {
	Provider Provider
	Name     string
	Region   string
	Endpoint string
	// OCICredentials holds OCI Object Storage auth fields.
	OCICredentials OCICredentials
	// S3Credentials holds MinIO / S3-compatible auth fields.
	S3Credentials S3Credentials
}
```

Config describes which object-storage backend to open and its credentials.
Build it explicitly and pass it to a factory; gofi's config package can
populate it from BUCKET_* environment variables. Provider-specific
credentials are nested so the struct stays extensible as new backends appear.

### bucket.ParseURL

```go
func ParseURL(raw string) (Config, error)
```

ParseURL builds a Config from a bucket URL; credentials still come from
the provider's default chain or Config fields. Examples:

	s3://my-bucket?region=us-east-1
	s3://my-bucket?endpoint=minio:9000&ssl=false
	oci://my-bucket?region=sa-saopaulo-1&namespace=ns&auth=workload_identity
	file:///var/data/uploads
	mem://test

### Config.IsConfigured

```go
func (c Config) IsConfigured() bool
```

IsConfigured reports whether a backend is explicitly selected, i.e. Provider
is set and is not ProviderNone.

## bucket.OCIAuthMode

```go
type OCIAuthMode string
```

OCIAuthMode selects how the OCI backend obtains its credentials. The OCI SDK
never auto-detects instance identity: the caller must name the principal it
wants, so this choice is always explicit.

```go
const (
	// OCIAuthAPIKey signs requests with a user API key whose fields
	// (TenancyID, UserID, FingerPrint, PrivateKey) are injected via env.
	OCIAuthAPIKey OCIAuthMode = "api_key"
	// OCIAuthInstancePrincipal derives credentials from the compute
	// instance's identity via the OCI metadata service. Works only inside
	// an OCI instance; requires no key material.
	OCIAuthInstancePrincipal OCIAuthMode = "instance_principal"
	// OCIAuthResourcePrincipal derives credentials from resource-principal
	// environment variables (Functions and similar resources).
	OCIAuthResourcePrincipal OCIAuthMode = "resource_principal"
	// OCIAuthWorkloadIdentity derives credentials from an OKE pod's workload
	// identity (service-account based).
	OCIAuthWorkloadIdentity OCIAuthMode = "workload_identity"
)
```

Supported OCI authentication modes. Empty is treated as OCIAuthAPIKey.

## bucket.OCICredentials

```go
type OCICredentials struct {
	AuthMode    OCIAuthMode
	Namespace   string
	TenancyID   string
	UserID      string
	FingerPrint string
	PrivateKey  string
	Passphrase  string
}
```

OCICredentials holds auth fields exclusive to the OCI backend. AuthMode
selects the credential source; the API-key fields are consumed only when
AuthMode is OCIAuthAPIKey (or empty).

## bucket.Object

```go
type Object struct {
	// Key is the object name, unique within the bucket.
	Key string
	// Size is the object size in bytes. It may be zero when the backend does
	// not report it in a listing.
	Size int64
	// ContentType is the MIME type, when known.
	ContentType string
	// LastModified is the object's creation/modification time, when known.
	LastModified time.Time
}
```

Object is the provider-agnostic metadata of a stored object.

### bucket.Collect

```go
func Collect(seq iter.Seq2[Object, error]) ([]Object, error)
```

Collect drains a listing sequence; stores use it to implement List on top
of All. The result is never nil.

## bucket.Opener

```go
type Opener func(ctx context.Context, cfg Config) (Store, error)
```

Opener builds a Store for a provider from a generic Config.

## bucket.Provider

```go
type Provider string
```

Provider selects the object-storage backend a Config targets.

```go
const (
	ProviderOCI Provider = "oci"
	// ProviderS3 targets Amazon S3 or any S3-compatible service (MinIO, R2, ...).
	ProviderS3 Provider = "s3"
	// ProviderMinIO is an alias of ProviderS3 kept for existing BUCKET_PROVIDER values.
	ProviderMinIO Provider = "minio"
	// ProviderFile stores objects under a local directory (Config.Endpoint).
	ProviderFile Provider = "file"
	// ProviderMem keeps objects in memory; for tests.
	ProviderMem  Provider = "mem"
	ProviderNone Provider = "none"
)
```

Supported object-storage providers.

## bucket.PutInput

```go
type PutInput struct {
	// Key is the destination object name. Required.
	Key string
	// Body streams the object contents. Required.
	Body io.Reader
	// Size is the content length in bytes. Set it to a negative value when the
	// size is unknown; backends that require it will buffer or stream
	// accordingly.
	Size int64
	// ContentType is the optional MIME type to store alongside the object.
	ContentType string
}
```

PutInput describes an upload.

## bucket.S3Credentials

```go
type S3Credentials struct {
	AccessKey string
	SecretKey string
	UseSSL    bool // for custom endpoints without a scheme
}
```

S3Credentials holds auth fields exclusive to the S3 backend. Empty keys use
the AWS default credential chain (IRSA, EKS Pod Identity, instance profile).

## bucket.Store

```go
type Store interface {
	// Put uploads an object, overwriting any existing object with the same key.
	Put(ctx context.Context, in PutInput) error

	// Get downloads an object. The caller owns the returned ReadCloser and must
	// close it. It returns ErrNotFound when the key does not exist.
	Get(ctx context.Context, key string) (Object, io.ReadCloser, error)

	// List returns the objects whose key starts with prefix. An empty prefix
	// lists the whole bucket. The returned slice is never nil.
	List(ctx context.Context, prefix string) ([]Object, error)

	// Delete removes an object. It is idempotent: deleting a key that does not
	// exist is not an error.
	Delete(ctx context.Context, key string) error

	// PresignGet returns a time-limited, read-only URL for a single object that
	// a client can download directly from the backend, without proxying the
	// bytes through the application. ttl bounds the URL's validity. OCI issues a
	// Pre-Authenticated Request; S3/MinIO a presigned GET.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}
```

Store is the behaviour every object-storage backend must implement. All
methods are safe for concurrent use.

### bucket.Open

```go
func Open(ctx context.Context, cfg Config) (Store, error)
```

Open builds the Store for cfg.Provider. It fails when the provider is not
set or its package was not imported.

### bucket.OpenURL

```go
func OpenURL(ctx context.Context, raw string) (Store, error)
```

OpenURL parses raw and opens the Store; the provider package must be imported.

## bucket.Walker

```go
type Walker interface {
	All(ctx context.Context, prefix string) iter.Seq2[Object, error]
}
```

Walker is implemented by stores that stream listings page by page.

