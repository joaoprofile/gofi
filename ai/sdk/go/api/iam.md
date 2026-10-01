# iam

`import "github.com/gofi-labs/gofi-sdk-go/iam"` · SDK v0.8.2

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
Selects Redis if RedisAddr is configured, otherwise uses in-memory with TTL.

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

