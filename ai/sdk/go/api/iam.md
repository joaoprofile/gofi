# iam

`import "github.com/joaoprofile/gofi-sdk-go/iam"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package iam is an identity and access facade for the gofi SDK.

Provides authentication (local and social), RBAC authorization, session management
with real revocation, and multi-tenancy with zero infrastructure provider lock-in.

## Funções

### iam.New

```go
func New(cfg Config) (*core.IAMService, error)
```

New builds an IAMService with full control over providers.
Validates minimum security contracts before returning and returns an error if any are violated.

### iam.NewDefault

```go
func NewDefault(cfg DefaultConfig) (*core.IAMService, error)
```

NewDefault builds an IAMService with built-in providers from DefaultConfig.
Sessions, login throttling and single-use tenant tickets live in Redis when
RedisAddr is configured (one shared client), in memory otherwise.

## iam.Config

```go
type Config = iamconfig.Config
```

Re-exports so callers do not need to import sub-packages in simple use cases.

## iam.DefaultConfig

```go
type DefaultConfig = iamconfig.DefaultConfig
```

Re-exports so callers do not need to import sub-packages in simple use cases.

## iam.IDPConfig

```go
type IDPConfig = iamconfig.IDPConfig
```

Re-exports so callers do not need to import sub-packages in simple use cases.

## iam.SecurityConfig

```go
type SecurityConfig = iamconfig.SecurityConfig
```

Re-exports so callers do not need to import sub-packages in simple use cases.

