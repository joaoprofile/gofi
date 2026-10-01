# iam/provider/rbac/roles

`import "github.com/gofi-labs/gofi-sdk-go/iam/provider/rbac/roles"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package roles implements port.RBACPort with a simple role-to-permissions model.
For ABAC or policy engines such as OPA or Cedar, implement port.RBACPort directly.

## roles.Config

```go
type Config struct {
	Permissions PermissionMap
}
```

Config configures the role-based RBAC provider.

## roles.PermissionMap

```go
type PermissionMap map[string]map[string][]string
```

PermissionMap maps role to resource to a list of allowed actions.

## roles.Provider

```go
type Provider struct {
	// contains filtered or unexported fields
}
```

Provider implements port.RBACPort with role-based resolution from claims.

### roles.NewRBACProvider

```go
func NewRBACProvider(cfg Config) *Provider
```

NewRBACProvider builds an RBAC Provider with the given permission map.

### Provider.Enforce

```go
func (p *Provider) Enforce(claims types.Claims, resource, action string) bool
```

Enforce checks whether any role in the claims authorizes the given resource and action.

### Provider.Permissions

```go
func (p *Provider) Permissions(claims types.Claims) []port.Permission
```

Permissions lists all permissions for the user based on their roles.
The result is deduplicated — if two roles grant the same action, it appears only once.

