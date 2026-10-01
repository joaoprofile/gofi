# base/environment

`import "github.com/joaoprofile/gofi-sdk-go/base/environment"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const APP_MAX_PARALLEL_WORKERS = 1
```

## Variáveis

```go
var (
	ErrLoadingEnvironmentFile   = errors.New("error loading environment file")
	ErrParsingEnvironment       = errors.New("error parsing environment variables")
	ErrInvalidEnvironment       = errors.New("invalid environment")
	ErrInvalidMessagingProvider = errors.New("invalid messaging provider")
	ErrInvalidCacheType         = errors.New("invalid cache type")
)
```

## Funções

### environment.IsCloudEnvironment

```go
func IsCloudEnvironment() bool
```

### environment.IsEnvironmentDev

```go
func IsEnvironmentDev() bool
```

### environment.IsEnvironmentProd

```go
func IsEnvironmentProd() bool
```

### environment.IsEnvironmentStage

```go
func IsEnvironmentStage() bool
```

### environment.IsEnvironmentTest

```go
func IsEnvironmentTest() bool
```

### environment.IsLocalEnvironment

```go
func IsLocalEnvironment() bool
```

### environment.LoadError

```go
func LoadError() error
```

LoadError returns the error of the Instance load, if any.

### environment.ResetForTesting

```go
func ResetForTesting()
```

ResetForTesting resets the singleton so that Instance() re-initialises on
the next call. Must only be called from tests.

## environment.AuthConfig

```go
type AuthConfig struct {
	JWTSecret       []byte
	Issuer          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}
```

AuthConfig groups the fields a JWT-based authentication flow needs.
Returned by Environment.Auth(). Values come from JWT_SECRET, JWT_ISSUER,
ACCESS_TOKEN_TTL and REFRESH_TOKEN_TTL — defaults applied when missing.

## environment.CacheType

```go
type CacheType string
```

```go
const (
	REDIS_CACHE CacheType = "redis"
	OCI_CACHE   CacheType = "oci"
)
```

Cache Types

## environment.Environment

```go
type Environment struct {
	AppName               string `env:"APP_NAME"`
	AppVersion            string `env:"APP_VERSION"`
	AppEnvironment        string `env:"APP_ENVIRONMENT"`
	AppTenant             int    `env:"APP_TENANT"`
	AppMaxParallelWorkers int    `env:"APP_MAX_PARALLEL_WORKERS"`

	// Timezone is the IANA name applied to time.Local at startup. Empty defaults
	// to Brazil (America/Sao_Paulo).
	Timezone string `env:"TIMEZONE"`

	// TLSInsecureSkipVerify disables certificate checks on http.DefaultTransport.
	// Legacy escape hatch for self-signed endpoints; never enable in production.
	TLSInsecureSkipVerify bool `env:"TLS_INSECURE_SKIP_VERIFY"`

	ServiceDebug     bool   `env:"SERVICE_DEBUG"`
	ServiceDebugAddr string `env:"SERVICE_DEBUG_ADDR"`
	ServiceDebugUser string `env:"SERVICE_DEBUG_USER"`
	ServiceDebugPass string `env:"SERVICE_DEBUG_PASS"`
	ServicePort      int    `env:"PORT"`

	LogLevel  string `env:"LOG_LEVEL"`
	LogOutput string `env:"LOG_OUTPUT"`

	DatabaseDriver       string        `env:"DATABASE_DRIVER"`
	DatabaseHost         string        `env:"DATABASE_HOST"`
	DatabasePort         int           `env:"DATABASE_PORT"`
	DatabaseUser         string        `env:"DATABASE_USER"`
	DatabasePassword     string        `env:"DATABASE_PASSWORD"`
	DatabaseName         string        `env:"DATABASE_NAME"`
	DatabaseSSLMode      string        `env:"DATABASE_SSL_MODE"`
	DatabaseMigration    bool          `env:"DATABASE_MIGRATION"`
	DatabaseMaxOpenConns int           `env:"DATABASE_MAX_OPEN_CONNS"`
	DatabaseMaxIdleConns int           `env:"DATABASE_MAX_IDLE_CONNS"`
	DatabaseMaxLifetime  time.Duration `env:"DATABASE_MAX_LIFETIME"`
	DatabaseMaxIdleTime  time.Duration `env:"DATABASE_MAX_IDLE_TIME"`
	// Read replica; user, password and database are shared with the primary.
	DatabaseReadHost string `env:"DATABASE_READ_HOST"`
	DatabaseReadPort int    `env:"DATABASE_READ_PORT"`

	CacheType     string `env:"CACHE_TYPE"`
	CacheURI      string `env:"CACHE_URI"`
	CachePassword string `env:"CACHE_PASSWORD"`
	CacheUseTLS   bool   `env:"CACHE_USE_TLS"`

	MessagingProvider        string `env:"MESSAGING_PROVIDER"`
	MessagingUser            string `env:"MESSAGING_USER"`
	MessagingPassword        string `env:"MESSAGING_PASSWORD"`
	MessagingHost            string `env:"MESSAGING_HOST"`
	MessagingPort            int    `env:"MESSAGING_PORT"`
	MessagingUseTLS          bool   `env:"MESSAGING_USE_TLS"`
	MessagingSASLMechanism   string `env:"MESSAGING_SASL_MECHANISM"`
	MessagingPollingInterval int    `env:"MESSAGING_POLLING_INTERVAL"`
	// MessagingEncoding selects the wire format where the broker supports
	// more than one: envelope (default) or cloudevents.
	MessagingEncoding string `env:"MESSAGING_ENCODING"`
	// MessagingRedisMode selects pubsub (default) or streams for the Redis broker.
	MessagingRedisMode string `env:"MESSAGING_REDIS_MODE"`

	// OCI Queue identity; MESSAGING_OCI_AUTH_MODE selects api_key (default),
	// instance_principal, resource_principal or workload_identity.
	MessagingOCIAuthMode    string `env:"MESSAGING_OCI_AUTH_MODE"`
	MessagingOCITenancyId   string `env:"MESSAGING_OCI_TENANCY_ID"`
	MessagingOCIUserId      string `env:"MESSAGING_OCI_USER_ID"`
	MessagingOCIRegion      string `env:"MESSAGING_OCI_REGION"`
	MessagingOCIFingerPrint string `env:"MESSAGING_OCI_FINGERPRINT"`
	MessagingOCIPrivateKey  string `env:"MESSAGING_OCI_PRIVATE_KEY"`

	// OCIPrivateKey is the shared OCI API key fallback (OCI_PRIVATE_KEY).
	OCIPrivateKey string `env:"OCI_PRIVATE_KEY"`

	// Object-storage bucket configuration. gofi's config.Bucket maps these into
	// the typed bucket.Config.
	BucketProvider string `env:"BUCKET_PROVIDER"`
	BucketName     string `env:"BUCKET_NAME"`
	BucketRegion   string `env:"BUCKET_REGION"`
	BucketEndpoint string `env:"BUCKET_ENDPOINT"`

	// OCI Object Storage credentials. BUCKET_OCI_AUTH_MODE selects the
	// credential source (empty/"api_key", "instance_principal",
	// "resource_principal", "workload_identity"); the key fields below are
	// consumed only by the api_key mode.
	BucketOCIAuthMode    string `env:"BUCKET_OCI_AUTH_MODE"`
	BucketOCINamespace   string `env:"BUCKET_OCI_NAMESPACE"`
	BucketOCITenancyID   string `env:"BUCKET_OCI_TENANCY_ID"`
	BucketOCIUserID      string `env:"BUCKET_OCI_USER_ID"`
	BucketOCIFingerPrint string `env:"BUCKET_OCI_FINGERPRINT"`
	BucketOCIPrivateKey  string `env:"BUCKET_OCI_PRIVATE_KEY"`
	BucketOCIPassphrase  string `env:"BUCKET_OCI_PASSPHRASE"`

	// MinIO / S3-compatible credentials.
	BucketS3AccessKey string `env:"BUCKET_S3_ACCESS_KEY"`
	BucketS3SecretKey string `env:"BUCKET_S3_SECRET_KEY"`
	BucketS3UseSSL    bool   `env:"BUCKET_S3_USE_SSL"`

	OtelExporterOTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OtelExporterOTLPHeaders  string `env:"OTEL_EXPORTER_OTLP_HEADERS"`
	// OtelExporterOTLPInsecure follows the OTel convention: "false" enables TLS.
	OtelExporterOTLPInsecure string `env:"OTEL_EXPORTER_OTLP_INSECURE"`
	// OtelLegacyMetricNames keeps the pre-v0.3 gofi_* metric names.
	OtelLegacyMetricNames bool `env:"GOFI_OTEL_LEGACY_METRIC_NAMES"`

	// Auth / IAM — universal to any service that has sessions.
	JWTSecret string `env:"JWT_SECRET"`
	JWTIssuer string `env:"JWT_ISSUER"`
	// JWT key rotation (see iam DefaultConfig).
	JWTKeyID          string        `env:"JWT_KEY_ID"`
	JWTPreviousKeyID  string        `env:"JWT_PREVIOUS_KEY_ID"`
	JWTPreviousSecret string        `env:"JWT_PREVIOUS_SECRET"`
	AccessTokenTTL    time.Duration `env:"ACCESS_TOKEN_TTL"`
	RefreshTokenTTL   time.Duration `env:"REFRESH_TOKEN_TTL"`

	// OAuth — provedores externos. Prefixo OAUTH_<PROVIDER>_*.
	OAuthGoogleClientID     string `env:"OAUTH_GOOGLE_CLIENT_ID"`
	OAuthGoogleClientSecret string `env:"OAUTH_GOOGLE_CLIENT_SECRET"`
	OAuthGoogleRedirectURI  string `env:"OAUTH_GOOGLE_REDIRECT_URI"`

	// HTTP / CORS — origens permitidas como CSV ("a,b,c").
	AllowedOrigins string `env:"ALLOWED_ORIGINS"`

	// Mail / SMTP — envio transacional e em massa por qualquer provedor SMTP.
	MailHost       string        `env:"MAIL_HOST"`
	MailPort       int           `env:"MAIL_PORT"`
	MailUsername   string        `env:"MAIL_USERNAME"`
	MailPassword   string        `env:"MAIL_PASSWORD"`
	MailFromName   string        `env:"MAIL_FROM_NAME"`
	MailFromEmail  string        `env:"MAIL_FROM_EMAIL"`
	MailEncryption string        `env:"MAIL_ENCRYPTION"` // none | starttls | tls
	MailAuth       string        `env:"MAIL_AUTH"`       // plain | login | cram-md5 | none
	MailTimeout    time.Duration `env:"MAIL_TIMEOUT"`
	MailMaxRetries int           `env:"MAIL_MAX_RETRIES"`
	MailPoolSize   int           `env:"MAIL_POOL_SIZE"`
	MailHELODomain string        `env:"MAIL_HELO_DOMAIN"`
}
```

Environment holds all configuration loaded from environment variables.

### environment.Instance

```go
func Instance() *Environment
```

Instance returns the singleton Environment, loading it on first call. Load
errors do not panic: the partially loaded Environment is returned and the
error is available through LoadError (gofi's Build returns it).

### environment.Load

```go
func Load() (*Environment, error)
```

Load reads the environment into a new Environment and returns every problem
joined. The .env file is only read outside production (see shouldLoadDotEnv),
each variable X can be supplied as a file path in X_FILE (mounted secrets),
and a value of the form secret://<provider>/<name>[#key] is fetched from
that secret manager (see package secrets).

### Environment.Auth

```go
func (env *Environment) Auth() AuthConfig
```

Auth returns an AuthConfig populated from the environment, applying SDK
defaults for TTLs and falling back to AppName for the issuer when not set.
Does NOT validate the secret — use RequireAuth() for fail-fast.

### Environment.GetCacheType

```go
func (env *Environment) GetCacheType() CacheType
```

GetCacheType returns the strongly-typed cache type.

### Environment.GetEnvironmentType

```go
func (env *Environment) GetEnvironmentType() EnvironmentType
```

GetEnvironmentType returns the strongly-typed environment value.

### Environment.GetLogLevel

```go
func (env *Environment) GetLogLevel() common.LogLevel
```

GetLogLevel returns the strongly-typed log level.

### Environment.GetMessagingProvider

```go
func (env *Environment) GetMessagingProvider() MessagingProvider
```

GetMessagingProvider returns the strongly-typed messaging provider.

### Environment.HTTP

```go
func (env *Environment) HTTP() HTTPConfig
```

HTTP returns an HTTPConfig populated from the environment.

### Environment.IsAuthConfigured

```go
func (env *Environment) IsAuthConfigured() bool
```

IsAuthConfigured reports whether JWT_SECRET is set. Lets the caller decide
the policy (warn vs. fatal) instead of forcing it inside the SDK.

### Environment.IsCacheConfigured

```go
func (env *Environment) IsCacheConfigured() bool
```

IsCacheConfigured reports whether a cache type is set.

### Environment.IsDatabaseConfigured

```go
func (env *Environment) IsDatabaseConfigured() bool
```

IsDatabaseConfigured reports whether at minimum a driver and name/host are
present.

### Environment.IsMessagingConfigured

```go
func (env *Environment) IsMessagingConfigured() bool
```

IsMessagingConfigured reports whether a messaging provider is set.

### Environment.OAuth

```go
func (env *Environment) OAuth() OAuthConfig
```

OAuth returns an OAuthConfig populated from the environment.

### Environment.Observability

```go
func (env *Environment) Observability() ObservabilityConfig
```

Observability returns an ObservabilityConfig populated from the environment.

### Environment.RequireAuth

```go
func (env *Environment) RequireAuth() error
```

RequireAuth returns an error wrapping ErrInvalidEnvironment when JWT_SECRET
is missing. main.go (or LoadConfig) is the right place to fatalize.

### Environment.RequireGoogleOAuth

```go
func (env *Environment) RequireGoogleOAuth() error
```

RequireGoogleOAuth returns an error wrapping ErrInvalidEnvironment when any
of the Google OAuth fields are missing.

## environment.EnvironmentType

```go
type EnvironmentType string
```

```go
const (
	ENV_DEV   EnvironmentType = "dev"
	ENV_STAGE EnvironmentType = "stage"
	ENV_TEST  EnvironmentType = "test"
	ENV_PROD  EnvironmentType = "prod"
)
```

Environment types

## environment.GoogleOAuthConfig

```go
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}
```

GoogleOAuthConfig holds Google IDP credentials for OAuth flows.

### GoogleOAuthConfig.IsConfigured

```go
func (g GoogleOAuthConfig) IsConfigured() bool
```

IsGoogleConfigured reports whether the Google OAuth flow is fully set up.

## environment.HTTPConfig

```go
type HTTPConfig struct {
	Port           int
	AllowedOrigins []string
}
```

HTTPConfig groups HTTP server configuration. AllowedOrigins is the parsed
CSV from ALLOWED_ORIGINS (each origin already trimmed; empty entries dropped).

## environment.MessagingProvider

```go
type MessagingProvider string
```

```go
const (
	MESSAGING_RABBITMQ MessagingProvider = "rabbitmq"
	MESSAGING_KAFKA    MessagingProvider = "kafka"
	MESSAGING_SQS      MessagingProvider = "sqs"
	MESSAGING_OCI      MessagingProvider = "oci"
	MESSAGING_REDIS    MessagingProvider = "redis"
	MESSAGING_NATS     MessagingProvider = "nats"
)
```

Messaging Providers

## environment.OAuthConfig

```go
type OAuthConfig struct {
	Google GoogleOAuthConfig
}
```

OAuthConfig groups OAuth provider configuration. Each provider lives in its
own field — extend with Microsoft, Apple, OIDC etc. as the SDK supports them.

## environment.ObservabilityConfig

```go
type ObservabilityConfig struct {
	OTLPEndpoint string
	OTLPHeaders  string
}
```

ObservabilityConfig groups all OpenTelemetry configuration.

