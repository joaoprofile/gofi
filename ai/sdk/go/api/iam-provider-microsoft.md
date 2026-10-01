# iam/provider/microsoft

`import "github.com/joaoprofile/gofi-sdk-go/iam/provider/microsoft"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package microsoft implements IDPAuthPort for Microsoft Identity Platform (Entra ID).
Supports single-tenant, multi-tenant, and personal Microsoft accounts.

## Variáveis

```go
var (
	// ErrIssuerMismatch is returned when iss is not the Entra issuer of the token's tid.
	ErrIssuerMismatch = errors.New("iam/microsoft: issuer does not match token tenant")
	// ErrMissingObjectID is returned when the id_token lacks tid or oid.
	ErrMissingObjectID = errors.New("iam/microsoft: id_token without tid/oid (profile scope required)")
	// ErrTenantNotAllowed is returned when the token tenant is outside the configured scope.
	ErrTenantNotAllowed = errors.New("iam/microsoft: tenant not allowed")
)
```

## microsoft.Config

```go
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string

	// TenantID defines the authentication scope.
	// A specific Azure AD tenant ID restricts login to that tenant only.
	// "organizations" accepts any Azure AD tenant.
	// "consumers" accepts only personal Microsoft accounts.
	// "common" accepts both and is the default.
	TenantID string

	// AllowedTenants optionally restricts logins to these tenant IDs (tid claim).
	AllowedTenants []string

	// HTTPClient allows injecting a custom client useful for tests.
	HTTPClient *http.Client
}
```

Config configures the Microsoft Entra ID provider.

## microsoft.Provider

```go
type Provider struct {
	// contains filtered or unexported fields
}
```

Provider implements port.IDPAuthPort for Microsoft Entra.

### microsoft.New

```go
func New(cfg Config) *Provider
```

New creates a Microsoft OIDC provider.

### Provider.AuthorizationURL

```go
func (p *Provider) AuthorizationURL(ctx context.Context, input port.IDPAuthInput) (*port.IDPAuthURL, error)
```

### Provider.HandleCallback

```go
func (p *Provider) HandleCallback(ctx context.Context, input port.IDPCallbackInput) (*port.IDPCallbackResult, error)
```

HandleCallback identifies the user by tenant and object ID. Entra's sub is
pairwise per application, so it changes with the app registration; tid+oid
is stable across apps and matches Microsoft Graph.

### Provider.ProviderName

```go
func (p *Provider) ProviderName() string
```

