# iam/provider/google

`import "github.com/joaoprofile/gofi-sdk-go/iam/provider/google"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package google implements IDPAuthPort for Google OAuth2/OIDC.
Uses PKCE (S256), automatic discovery, and id_token validation via JWKS.

## Variáveis

```go
var ErrHostedDomainMismatch = errors.New("iam/google: hosted domain mismatch")
```

ErrHostedDomainMismatch is returned when the id_token hd claim differs from Config.HostedDomain.

## google.Config

```go
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string

	// HostedDomain restricts login to users from a specific Google Workspace domain.
	// Leave empty to accept any Google account.
	HostedDomain string

	// HTTPClient allows injecting a custom client useful for tests.
	HTTPClient *http.Client
}
```

Config configures the Google provider.

## google.Provider

```go
type Provider struct {
	// contains filtered or unexported fields
}
```

Provider implements port.IDPAuthPort for Google.

### google.New

```go
func New(cfg Config) *Provider
```

New creates a Google OIDC provider.

### Provider.AuthorizationURL

```go
func (p *Provider) AuthorizationURL(ctx context.Context, input port.IDPAuthInput) (*port.IDPAuthURL, error)
```

### Provider.HandleCallback

```go
func (p *Provider) HandleCallback(ctx context.Context, input port.IDPCallbackInput) (*port.IDPCallbackResult, error)
```

### Provider.ProviderName

```go
func (p *Provider) ProviderName() string
```

