# gofi/component/iam

`import "github.com/joaoprofile/gofi-sdk-go/gofi/component/iam"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package iam is the gofi component for the identity service (iam): JWT
tokens, sessions with real revocation and RBAC, configured from the
environment. The application supplies its users and tenants as ports.

## iam.Component

```go
type Component struct {
	// contains filtered or unexported fields
}
```

Component builds the iam service during Build.

### iam.FromService

```go
func FromService(svc *core.IAMService) *Component
```

FromService uses a service built by the caller with iam.New.

### iam.New

```go
func New(cfg ...Config) *Component
```

New builds the service with iam.NewDefault from the environment (see
config.IAM): JWT_SECRET (required, 32+ bytes), JWT_ISSUER, ACCESS_TOKEN_TTL,
REFRESH_TOKEN_TTL, JWT key rotation, OAUTH_GOOGLE_* and, with
CACHE_TYPE=redis, Redis sessions (in memory otherwise).

### Component.Name

```go
func (c *Component) Name() string
```

### Component.Service

```go
func (c *Component) Service() *core.IAMService
```

Service returns the iam service; nil before Build.

### Component.Stage

```go
func (c *Component) Stage() gofi.Stage
```

### Component.Start

```go
func (c *Component) Start(_ context.Context, rt *gofi.Runtime) error
```

## iam.Config

```go
type Config struct {
	User    port.UserPort
	Tenant  port.TenantPort
	RBAC    port.RBACPort
	OnEvent func(ctx context.Context, event types.IAMEvent)

	// Configure adjusts the configuration read from the environment before
	// the service is built, e.g. security settings or extra IDPs.
	Configure func(*iamconfig.DefaultConfig)
}
```

Config holds what the environment cannot provide. Every field is optional:
without User and Tenant the service only validates tokens and revokes
sessions (a resource server); with them it also logs users in.

