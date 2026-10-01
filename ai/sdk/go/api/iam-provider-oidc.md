# iam/provider/oidc

`import "github.com/joaoprofile/gofi-sdk-go/iam/provider/oidc"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package oidc implements a generic IDPAuthPort for any OIDC-compliant provider.
Supports PKCE (RFC 7636 S256), id_token validation via JWKS, and automatic discovery.

## oidc.Config

```go
type Config struct {
	// IssuerURL is the base URL of the provider.
	// The discovery document is fetched from IssuerURL + "/.well-known/openid-configuration".
	IssuerURL string

	ClientID     string
	ClientSecret string
	RedirectURI  string

	// Scopes beyond the minimum set (openid email profile).
	ExtraScopes []string

	// AuthParams are extra authorization URL parameters; standard ones cannot be overridden.
	AuthParams map[string]string

	// ValidateClaims runs after signature, issuer and audience checks of the id_token.
	ValidateClaims func(claims map[string]any) error

	// ValidateIssuer replaces the exact IssuerURL match, for multi-tenant issuers.
	ValidateIssuer func(claims map[string]any) error

	// JWKSCacheTTL defines how long JWKS keys are cached. Default: 1 hour.
	JWKSCacheTTL time.Duration

	// HTTPClient allows injecting a custom client useful for tests.
	HTTPClient *http.Client
}
```

Config configures the generic OIDC provider.

## oidc.Provider

```go
type Provider struct {
	// contains filtered or unexported fields
}
```

Provider implements port.IDPAuthPort for any OIDC-compliant provider.

### oidc.New

```go
func New(name string, cfg Config) *Provider
```

New creates a generic OIDC provider with the given identifier name.

### Provider.AuthorizationURL

```go
func (p *Provider) AuthorizationURL(ctx context.Context, input port.IDPAuthInput) (*port.IDPAuthURL, error)
```

AuthorizationURL generates the OIDC authorization URL with PKCE and state.

### Provider.HandleCallback

```go
func (p *Provider) HandleCallback(ctx context.Context, input port.IDPCallbackInput) (*port.IDPCallbackResult, error)
```

HandleCallback processes the OIDC callback: validates state, exchanges the code, and validates the id_token.

### Provider.ProviderName

```go
func (p *Provider) ProviderName() string
```

