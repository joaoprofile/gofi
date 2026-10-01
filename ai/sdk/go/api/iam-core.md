# iam/core

`import "github.com/gofi-labs/gofi-sdk-go/iam/core"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Variáveis

```go
var (
	// Authentication errors are intentionally generic to prevent user enumeration.
	ErrInvalidCredentials = errors.New("iam: invalid credentials")
	ErrAccountInactive    = errors.New("iam: account inactive")

	// Token errors.
	ErrTokenExpired    = errors.New("iam: token expired")
	ErrTokenInvalid    = errors.New("iam: token invalid")
	ErrSessionRevoked  = errors.New("iam: session revoked")
	ErrSessionNotFound = errors.New("iam: session not found")
	ErrSessionExpired  = errors.New("iam: session expired")

	// Authorization errors.
	ErrAccessDenied        = errors.New("iam: access denied")
	ErrTenantAccessDenied  = errors.New("iam: tenant access denied")
	ErrInvalidTenantTicket = errors.New("iam: invalid or expired tenant ticket")

	// IDP errors.
	ErrInvalidIDPState   = errors.New("iam: invalid idp state — possible CSRF")
	ErrInvalidIDPNonce   = errors.New("iam: invalid idp nonce — possible token replay")
	ErrIDPCallbackFailed = errors.New("iam: idp callback processing failed")

	// Configuration errors detected in New() before initialization.
	ErrSessionPortRequired        = errors.New("iam: SessionPort is required — session cannot be nil")
	ErrAccessTokenTTLExceeded     = errors.New("iam: AccessTokenTTL must be ≤ 60 minutes")
	ErrRefreshTokenTTLExceeded    = errors.New("iam: RefreshTokenTTL must be ≤ 90 days")
	ErrJWTSecretTooShort          = errors.New("iam: JWTSecret must be at least 32 bytes")
	ErrTenantTicketSecretTooShort = errors.New("iam: TenantTicketSecret must be at least 32 bytes")
	ErrProviderNotFound           = errors.New("iam: IDP provider not registered")

	// ErrLoginPortsRequired is returned by the login operations when the
	// service was built without a UserPort or TenantPort.
	ErrLoginPortsRequired = errors.New("iam: UserPort and TenantPort are required for login; set User and Tenant in the config")
)
```

## Funções

### core.NewLocalAuth

```go
func NewLocalAuth(cfg LocalAuthConfig) port.AuthPort
```

NewLocalAuth builds the built-in AuthPort.

### core.NonceForState

```go
func NonceForState(state string) string
```

NonceForState derives the OIDC nonce from the flow state, so callers that only
persist the state (and never pass ExpectedNonce) still get replay protection.

## core.AuthConfig

```go
type AuthConfig struct {
	// contains filtered or unexported fields
}
```

AuthConfig holds token lifetime and issuer identification settings.

### core.AuthConfigFromSecurity

```go
func AuthConfigFromSecurity(sec iamconfig.SecurityConfig) AuthConfig
```

AuthConfigFromSecurity converts SecurityConfig to the internal AuthConfig.

## core.IAMService

```go
type IAMService struct {
	// contains filtered or unexported fields
}
```

IAMService is the central facade of the iam package.
Orchestrates authentication, session management, social IDP login, and RBAC.
Obtained via iam.New() or iam.NewDefault().

### core.NewService

```go
func NewService(cfg ServiceConfig) *IAMService
```

NewService builds an IAMService from validated configuration.
Not intended to be called directly — use iam.New() or iam.NewDefault().

### IAMService.Authenticate

```go
func (s *IAMService) Authenticate(ctx context.Context, input port.AuthInput) (*port.AuthResult, error)
```

Authenticate validates credentials and returns available tenants without issuing tokens.

### IAMService.IDPHandleCallback

```go
func (s *IAMService) IDPHandleCallback(ctx context.Context, provider string, input port.IDPCallbackInput) (*port.IDPCallbackResult, error)
```

IDPHandleCallback processes the IDP callback for the given provider.

### IAMService.IDPInitFlow

```go
func (s *IAMService) IDPInitFlow(ctx context.Context, provider, redirectURI string, extraScopes []string) (*port.IDPAuthURL, error)
```

IDPInitFlow prepares the OAuth2/OIDC flow for the given provider.
Returns the authorization URL and security values (state, code_verifier)
that the caller must store in an HttpOnly cookie.

### IAMService.IDPSelectTenant

```go
func (s *IAMService) IDPSelectTenant(ctx context.Context, provider string, input port.SelectTenantInput) (*types.Session, error)
```

IDPSelectTenant creates a session after authentication via an IDP.

### IAMService.ListSessions

```go
func (s *IAMService) ListSessions(ctx context.Context, userID string) ([]*types.Session, error)
```

ListSessions returns the active sessions for a user for device management.

### IAMService.ListTenants

```go
func (s *IAMService) ListTenants(ctx context.Context, userID string) ([]types.TenantAccess, error)
```

ListTenants lists the available tenants for the user.

### IAMService.Logout

```go
func (s *IAMService) Logout(ctx context.Context, sessionID string) error
```

Logout revokes the session — a real logout independent of token expiry.

### IAMService.LogoutAll

```go
func (s *IAMService) LogoutAll(ctx context.Context, userID string) error
```

LogoutAll revokes all sessions for the user across all devices.

### IAMService.RBAC

```go
func (s *IAMService) RBAC() port.RBACPort
```

RBAC returns the authorization provider for direct use in handlers.

### IAMService.RefreshToken

```go
func (s *IAMService) RefreshToken(ctx context.Context, refreshToken string) (*types.Session, error)
```

RefreshToken renews the access token with mandatory refresh token rotation.

### IAMService.SelectTenant

```go
func (s *IAMService) SelectTenant(ctx context.Context, input port.SelectTenantInput) (*types.Session, error)
```

SelectTenant issues tokens and creates a session for the selected tenant.

### IAMService.Session

```go
func (s *IAMService) Session() port.SessionPort
```

Session returns the session port.

### IAMService.Tenant

```go
func (s *IAMService) Tenant() port.TenantPort
```

Tenant returns the tenant port for use in middleware.

### IAMService.ValidateToken

```go
func (s *IAMService) ValidateToken(ctx context.Context, accessToken string) (*types.Claims, error)
```

ValidateToken validates the access token and verifies the corresponding session.

## core.IDPService

```go
type IDPService struct {
	// contains filtered or unexported fields
}
```

IDPService orchestrates the social login flow (OAuth2/OIDC) for a specific provider.

### core.NewIDPService

```go
func NewIDPService(cfg IDPServiceConfig) *IDPService
```

NewIDPService builds an IDPService for the given provider.

### IDPService.HandleCallback

```go
func (s *IDPService) HandleCallback(ctx context.Context, input port.IDPCallbackInput) (*port.IDPCallbackResult, error)
```

HandleCallback processes the IDP callback, resolves the local user, and returns available tenants.

### IDPService.InitFlow

```go
func (s *IDPService) InitFlow(ctx context.Context, redirectURI string, extraScopes []string) (*port.IDPAuthURL, error)
```

InitFlow prepares the OAuth2/OIDC flow by generating state, PKCE, and returning the authorization URL.
The caller must store IDPAuthURL.State and CodeVerifier in an HttpOnly cookie.

### IDPService.SelectTenant

```go
func (s *IDPService) SelectTenant(ctx context.Context, input port.SelectTenantInput) (*types.Session, error)
```

SelectTenant creates a session after IDP login, reusing the same logic as the local flow.

## core.IDPServiceConfig

```go
type IDPServiceConfig struct {
	Provider port.IDPAuthPort
	User     port.UserPort
	Tenant   port.TenantPort
	Token    port.TokenPort
	Session  port.SessionPort
	Cfg      AuthConfig
	Emit     func(context.Context, types.IAMEvent)
}
```

IDPServiceConfig holds the parameters for building an IDPService.

## core.LocalAuthConfig

```go
type LocalAuthConfig struct {
	User    port.UserPort
	Tenant  port.TenantPort
	Token   port.TokenPort
	Session port.SessionPort
	Cfg     AuthConfig
	Emit    func(context.Context, types.IAMEvent)
}
```

LocalAuthConfig holds the parameters for building the built-in AuthPort.

## core.ServiceConfig

```go
type ServiceConfig struct {
	Auth    port.AuthPort
	Session port.SessionPort
	Tenant  port.TenantPort
	RBAC    port.RBACPort
	IDPs    map[string]*IDPService
	OnEvent func(context.Context, types.IAMEvent)
}
```

ServiceConfig holds the internal parameters for building an IAMService.

