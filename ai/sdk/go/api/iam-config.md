# iam/config

`import "github.com/joaoprofile/gofi-sdk-go/iam/config"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## config.Config

```go
type Config struct {
	// Domain ports.
	Auth    port.AuthPort    // nil uses the built-in implementation (requires User, Tenant, Token, Session)
	User    port.UserPort    // may be nil if Auth encapsulates UserPort
	Tenant  port.TenantPort  // may be nil if Auth encapsulates TenantPort
	Token   port.TokenPort   // required when Auth is nil (built-in)
	Session port.SessionPort // must never be nil
	RBAC    port.RBACPort    // required

	// Registered social IDPs. Key is the provider name ("google", "github", etc.).
	IDPs map[string]port.IDPAuthPort

	// Security settings.
	Security SecurityConfig

	// Observability callback invoked for every authentication and authorization event.
	// Implementations must never include passwords, raw tokens, or refresh tokens.
	OnEvent func(ctx context.Context, event types.IAMEvent)
}
```

Config is the full IAMService configuration with complete control over providers.
At minimum Token, Session, and RBAC are required.
Auth may be nil to use the built-in implementation (requires User and Tenant).
User and Tenant may be nil if a custom AuthPort encapsulates them.

## config.DefaultConfig

```go
type DefaultConfig struct {
	JWTSecret string // HS256, minimum 32 chars
	// Key rotation: tokens carry JWTKeyID as kid; tokens signed with the
	// previous secret stay valid until they expire.
	JWTKeyID          string
	JWTPreviousKeyID  string
	JWTPreviousSecret string

	// Session provider.
	RedisAddr     string // empty string selects in-memory with TTL
	RedisPassword string
	RedisTLS      bool

	// IDPs (optional).
	IDPs []IDPConfig

	// Security settings (uses safe defaults when nil).
	Security *SecurityConfig

	// Observability callback.
	OnEvent func(ctx context.Context, event types.IAMEvent)

	// Application ports. User and Tenant enable login (Authenticate,
	// SelectTenant, RefreshToken and IDP callbacks); without them the service
	// only validates tokens and revokes sessions. RBAC backs IAMService.RBAC.
	User   port.UserPort
	Tenant port.TenantPort
	RBAC   port.RBACPort
}
```

DefaultConfig is the shortcut for quick setup with built-in providers.
Selects Redis if RedisAddr is configured, otherwise uses in-memory with TTL.

## config.IDPConfig

```go
type IDPConfig struct {
	Provider     string // "google", "github", "microsoft", "oidc"
	ClientID     string
	ClientSecret string
	RedirectURI  string

	// Required for the generic oidc provider.
	IssuerURL string
	Scopes    []string
}
```

IDPConfig configures an external identity provider in DefaultConfig.

## config.SecurityConfig

```go
type SecurityConfig struct {
	AccessTokenTTL  time.Duration // default: 15 min, enforced maximum: 60 min
	RefreshTokenTTL time.Duration // default: 7 days, enforced maximum: 90 days
	Issuer          string        // default: "gofi/iam"
	VerifyIssuer    bool          // optional: reject tokens whose iss differs from Issuer
	Audience        string        // optional: issued as aud and required on validation

	// TenantTicketSecret (≥32 bytes) signs the Authenticate→SelectTenant ticket.
	// NewDefault derives it from JWTSecret when empty.
	TenantTicketSecret []byte
	// RequireTenantTicket rejects SelectTenant calls without a valid ticket.
	RequireTenantTicket bool
	ClockSkew           time.Duration // optional: leeway for token time claims

	// Refresh token cookie settings.
	CookieName     string        // default: "iam_rt"
	CookiePath     string        // default: "/auth/refresh"
	CookieSecure   bool          // default: true
	CookieSameSite http.SameSite // default: SameSiteStrictMode
	CookieDomain   string        // optional

	// Temporary cookie for OAuth2 state and PKCE verifier.
	IDPStateCookieName string        // default: "iam_idp_state"
	IDPStateTTL        time.Duration // default: 10 min
}
```

SecurityConfig defines the security parameters of the IAMService.
Default values are production-safe.

### SecurityConfig.ApplyDefaults

```go
func (s *SecurityConfig) ApplyDefaults()
```

ApplyDefaults fills in default values for SecurityConfig.
Safe to call multiple times as it does not overwrite values already set.

