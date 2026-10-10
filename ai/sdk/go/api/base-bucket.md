# base/bucket

`import "github.com/joaoprofile/gofi-sdk-go/base/bucket"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package bucket defines a provider-agnostic object-storage abstraction.

A Store models a single bucket and exposes the minimal set of operations
shared by every object-storage backend: upload, download, list and delete.
Concrete backends (OCI Object Storage, AWS S3, GCS, …) live in their own
sub-packages and register themselves through Register so that callers can
obtain a Store by provider name via Open, without importing the backend
directly.

## Constantes

```go
const MaxPresignTTL = 7 * 24 * time.Hour
```

MaxPresignTTL is the longest validity PresignGet grants on any backend: the
S3 SigV4 limit. OCI PARs cannot be revoked by the URL holder, so the same
cap bounds how long a leaked URL stays usable.

## Variáveis

```go
var (
	// ErrNotFound is returned when an object (or its bucket) does not exist.
	ErrNotFound = errors.New("bucket: object not found")

	// ErrInvalidConfig is returned by a factory when the supplied Config is
	// missing required fields.
	ErrInvalidConfig = errors.New("bucket: invalid configuration")

	// ErrInvalidTTL is returned by PresignGet when ttl is not positive or
	// exceeds the store's limit.
	ErrInvalidTTL = errors.New("bucket: invalid presign ttl")

	// ErrBucketNotFound is returned when the bucket itself does not exist. It
	// wraps ErrNotFound, so errors.Is(err, ErrNotFound) still holds.
	ErrBucketNotFound = fmt.Errorf("%w: bucket does not exist", ErrNotFound)

	// ErrBucketExists is returned by CreateBucket when the name is taken.
	ErrBucketExists = errors.New("bucket: bucket already exists")

	// ErrBucketNotEmpty is returned by DeleteBucket without Force when the
	// bucket still holds objects.
	ErrBucketNotEmpty = errors.New("bucket: bucket is not empty")

	// ErrInvalidBucketName is returned when a name breaks the provider's rules.
	ErrInvalidBucketName = errors.New("bucket: invalid bucket name")

	// ErrAccessDenied is returned when the credentials lack permission.
	ErrAccessDenied = errors.New("bucket: access denied")

	// ErrNotSupported is returned when a provider does not implement a feature.
	ErrNotSupported = errors.New("bucket: not supported by provider")
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

### bucket.CheckPresignTTL

```go
func CheckPresignTTL(ttl, limit time.Duration) error
```

CheckPresignTTL returns ErrInvalidTTL unless 0 < ttl <= PresignLimit(limit).

### bucket.CreateFolder

```go
func CreateFolder(ctx context.Context, s Store, path string) error
```

CreateFolder makes path ("a/b" or "a/b/") show up in ListDir while empty:
FolderMaker when available, otherwise Put of an empty "path/" marker.
FolderMaker receives the normalized "a/b/" form. An empty path returns
ErrInvalidConfig.

### bucket.DeletePrefix

```go
func DeletePrefix(ctx context.Context, s Store, prefix string) (int, error)
```

DeletePrefix deletes every object under prefix ("" = the whole bucket)
and returns how many were deleted. It uses BatchDeleter when available,
Delete one by one otherwise, and stops at the first error.

Keys are deleted deepest first, together with the implied folder markers
of the subtree (deleting a missing key is a no-op), so stores that model
folders natively (e.g. a filesystem) end up without the subtree too. The
count only includes the objects the listing returned.

### bucket.PresignLimit

```go
func PresignLimit(limit time.Duration) time.Duration
```

PresignLimit returns the effective cap for limit: MaxPresignTTL when limit
is not positive or larger than it.

### bucket.Register

```go
func Register(p Provider, o Opener)
```

Register makes a provider available to Open. Provider packages call it from
init, so importing one (e.g. _ ".../base/bucket/oci") is enough to enable it.

### bucket.RegisterManager

```go
func RegisterManager(p Provider, o ManagerOpener)
```

RegisterManager makes a provider's Manager available to OpenManager.
Providers call it from the same init that calls Register.

### bucket.ValidateName

```go
func ValidateName(name string) error
```

ValidateName checks name against the S3 bucket naming rules, the strictest
common subset: 3 to 63 characters of [a-z0-9.-], starting and ending with a
letter or digit, no "..", and not shaped like an IPv4 address. It returns
ErrInvalidBucketName otherwise. Local providers reuse it so names stay
portable.

## bucket.BatchDeleter

```go
type BatchDeleter interface {
	DeleteMany(ctx context.Context, keys []string) error
}
```

BatchDeleter is implemented by stores that delete many keys per call.
Missing keys are not an error, as with Delete.

## bucket.BucketInfo

```go
type BucketInfo struct {
	Name      string
	Region    string    // empty when the provider does not report it
	CreatedAt time.Time // zero when the provider does not report it
}
```

BucketInfo describes a bucket of the account.

## bucket.Config

```go
type Config struct {
	Provider Provider
	Name     string
	Region   string
	Endpoint string
	// PresignMaxTTL lowers the longest validity PresignGet accepts; zero
	// keeps MaxPresignTTL, and larger values are capped at it.
	PresignMaxTTL time.Duration
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

## bucket.CreateOptions

```go
type CreateOptions struct {
	Region string // empty uses Config.Region
}
```

CreateOptions tunes CreateBucket.

## bucket.DeleteOptions

```go
type DeleteOptions struct {
	Force bool // delete every object first (DeletePrefix with "")
}
```

DeleteOptions tunes DeleteBucket.

## bucket.Dir

```go
type Dir struct {
	Prefix  string
	Folders []string // full prefixes ("docs/sub/"), sorted, never nil
	Objects []Object // objects of this level only, never nil
}
```

Dir is one folder level.

### bucket.ListDir

```go
func ListDir(ctx context.Context, s Store, prefix string) (Dir, error)
```

ListDir uses DirLister when s implements it; otherwise it walks All(prefix)
and groups keys by the next "/". The folder's own marker (key == prefix)
is omitted. prefix is normally "" or ends with "/".

## bucket.DirLister

```go
type DirLister interface {
	ListDir(ctx context.Context, prefix string) (Dir, error)
}
```

DirLister is implemented by stores that list one level server-side.

## bucket.FolderMaker

```go
type FolderMaker interface {
	CreateFolder(ctx context.Context, path string) error
}
```

FolderMaker is implemented by stores where an empty "path/" object is not
the native way to create a folder (e.g. a filesystem).

## bucket.Manager

```go
type Manager interface {
	ListBuckets(ctx context.Context) ([]BucketInfo, error)
	CreateBucket(ctx context.Context, name string, opts CreateOptions) error
	DeleteBucket(ctx context.Context, name string, opts DeleteOptions) error
	// Open returns the Store of an existing bucket, configured for it
	// (e.g. its own region). It returns ErrBucketNotFound when missing.
	Open(ctx context.Context, name string) (Store, error)
}
```

Manager administers the buckets of an account. Safe for concurrent use.

Every implementation follows the same rules: ListBuckets never returns a nil
slice; CreateBucket validates the name (ErrInvalidBucketName) and reports
ErrBucketExists for a taken name; DeleteBucket returns ErrBucketNotEmpty
without Force and ErrBucketNotFound for a missing bucket; Open caches the
Store per name and DeleteBucket evicts it.

### bucket.OpenManager

```go
func OpenManager(ctx context.Context, cfg Config) (Manager, error)
```

OpenManager builds the Manager for cfg.Provider; cfg.Name is ignored. It
returns ErrNotSupported when the provider is imported but has no Manager.

## bucket.ManagerOpener

```go
type ManagerOpener func(ctx context.Context, cfg Config) (Manager, error)
```

ManagerOpener builds a Manager for a provider from a generic Config.

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
	PrivateKey  string `redact:"true"`
	Passphrase  string `redact:"true"`
}
```

OCICredentials holds auth fields exclusive to the OCI backend. AuthMode
selects the credential source; the API-key fields are consumed only when
AuthMode is OCIAuthAPIKey (or empty).

### OCICredentials.Format

```go
func (c OCICredentials) Format(f fmt.State, verb rune)
```

### OCICredentials.GoString

```go
func (c OCICredentials) GoString() string
```

### OCICredentials.LogValue

```go
func (c OCICredentials) LogValue() slog.Value
```

### OCICredentials.MarshalJSON

```go
func (c OCICredentials) MarshalJSON() ([]byte, error)
```

### OCICredentials.String

```go
func (c OCICredentials) String() string
```

String, GoString, Format, LogValue and MarshalJSON mask the secret fields.

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

### bucket.Stat

```go
func Stat(ctx context.Context, s Store, key string) (Object, error)
```

Stat uses Statter when available; otherwise Get and closes the body. It
returns ErrNotFound when the key does not exist.

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
	SecretKey string `redact:"true"`
	UseSSL    bool   // for custom endpoints without a scheme
}
```

S3Credentials holds auth fields exclusive to the S3 backend. Empty keys use
the AWS default credential chain (IRSA, EKS Pod Identity, instance profile).

### S3Credentials.Format

```go
func (c S3Credentials) Format(f fmt.State, verb rune)
```

### S3Credentials.GoString

```go
func (c S3Credentials) GoString() string
```

### S3Credentials.LogValue

```go
func (c S3Credentials) LogValue() slog.Value
```

### S3Credentials.MarshalJSON

```go
func (c S3Credentials) MarshalJSON() ([]byte, error)
```

### S3Credentials.String

```go
func (c S3Credentials) String() string
```

String, GoString, Format, LogValue and MarshalJSON mask the secret fields.

## bucket.Statter

```go
type Statter interface {
	Stat(ctx context.Context, key string) (Object, error)
}
```

Statter is implemented by stores that read metadata without the body.

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
	// bytes through the application. ttl bounds the URL's validity and must be
	// in (0, limit], where limit is Config.PresignMaxTTL capped at
	// MaxPresignTTL; otherwise it returns ErrInvalidTTL. OCI issues a
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

