# gofi/config

`import "github.com/gofi-labs/gofi-sdk-go/gofi/config"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package config is gofi's composition layer between the environment loader
(base/environment) and each library's typed Config: the libraries (mail,
bucket, iam, ...) stay decoupled from environment, while this package maps
env vars into their explicit Config structs.

The mappings of the resources started by gofi live with their components
(component/database.ConfigFromEnv, component/messaging.ConfigFromEnv, ...),
and the process-wide settings in config/core, so that importing gofi links
only what the service uses.

Each adapter takes an *environment.Environment explicitly so it can be tested
without the process-wide singleton. Applications that don't use gofi's
environment loader can ignore this package and build each library's Config
themselves.

## Funções

### config.ApplyTLS

```go
func ApplyTLS(env *environment.Environment)
```

ApplyTLS applies TLS_INSECURE_SKIP_VERIFY to http.DefaultTransport.

### config.Bucket

```go
func Bucket(env *environment.Environment) bucket.Config
```

Bucket builds a bucket.Config from the BUCKET_* variables of the environment.
Open it with bucket.Open after importing the provider package
(base/bucket/s3 or base/bucket/oci).

### config.Debug

```go
func Debug(env *environment.Environment) debug.Config
```

Debug builds a debug.Config from the SERVICE_DEBUG_* variables.

### config.IAM

```go
func IAM(env *environment.Environment) iamconfig.DefaultConfig
```

IAM builds iam's DefaultConfig from the environment, bridging gofi's env
schema to the IAM library without the IAM module ever importing
base/environment. Mapping:

  - JWT_SECRET                     → JWTSecret
  - JWT_KEY_ID / JWT_PREVIOUS_KEY_ID / JWT_PREVIOUS_SECRET → key rotation
  - CACHE_* (when CACHE_TYPE=redis) → Redis session store
  - OAUTH_GOOGLE_*                 → Google IDP
  - ACCESS_TOKEN_TTL / REFRESH_TOKEN_TTL / JWT_ISSUER → SecurityConfig

Remaining defaults are applied by iam (SecurityConfig.ApplyDefaults).

### config.InitLogging

```go
func InitLogging(env *environment.Environment, serviceName string) error
```

InitLogging initialises the global logger from the environment.

### config.Logging

```go
func Logging(env *environment.Environment, serviceName string) logging.Config
```

Logging builds a logging.Config from the environment for the given service.

### config.Mail

```go
func Mail(env *environment.Environment) (mail.Config, error)
```

Mail builds a mail.Config from the MAIL_* variables of the given environment.
Returns mail.ErrNotConfigured when MAIL_HOST or MAIL_FROM_EMAIL are absent.
Remaining defaults (port, encryption, auth, timeout) are applied by mail.New.

### config.NewMailer

```go
func NewMailer(env *environment.Environment) (mail.Mailer, error)
```

NewMailer builds a ready mail.Mailer from the environment. Returns
mail.ErrNotConfigured when the minimum MAIL_* settings are absent.

### config.SetTimezone

```go
func SetTimezone(env *environment.Environment) error
```

SetTimezone applies the process-wide local timezone (time.Local) from the environment.

### config.StartDebug

```go
func StartDebug(env *environment.Environment)
```

StartDebug starts the pprof debug server in the background when SERVICE_DEBUG
is enabled; otherwise it is a no-op.

### config.Timezone

```go
func Timezone(env *environment.Environment) timezone.Config
```

Timezone builds a timezone.Config from the TIMEZONE variable.

