# iam/types

`import "github.com/joaoprofile/gofi-sdk-go/iam/types"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## types.Claims

```go
type Claims struct {
	UserID    string   `json:"sub"`
	TenantID  string   `json:"tid"`
	Module    string   `json:"mod"`
	Roles     []string `json:"roles"`
	SessionID string   `json:"sid"` // links the token to the session for revocation

	// Provider that originated the authentication such as "local", "google", or "github".
	AuthProvider string `json:"apv"`

	Issuer    string    `json:"iss"`
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`

	// Extra carries project-specific custom claims that the SDK schema does not model.
	// Round-tripped through the JWT payload under the "ext" key — preserved across
	// Issue → Parse so the application can recover its domain claims after the SDK
	// validates the token. Values must be JSON-serializable.
	Extra map[string]any `json:"ext,omitempty"`
}
```

Claims represents the payload of a validated access token.
It is the central object carried via context between middleware and handlers.

## types.EventType

```go
type EventType string
```

EventType identifies the type of security or authentication event emitted by the SDK.

```go
const (
	EventLogin              EventType = "auth.login"
	EventLoginFailed        EventType = "auth.login_failed"
	EventIDPLogin           EventType = "auth.idp_login"
	EventIDPLoginFailed     EventType = "auth.idp_login_failed"
	EventNewUser            EventType = "auth.new_user" // first login via IDP
	EventTenantSelected     EventType = "auth.tenant_selected"
	EventLogout             EventType = "auth.logout"
	EventLogoutAll          EventType = "auth.logout_all"
	EventTokenRefreshed     EventType = "auth.token_refreshed"
	EventTokenRefreshFailed EventType = "auth.token_refresh_failed"
	EventAccessDenied       EventType = "authz.access_denied"
	EventTenantAccessDenied EventType = "authz.tenant_access_denied"
	EventTokenValidated     EventType = "token.validated"
	EventTokenInvalid       EventType = "token.invalid"
	EventSessionRevoked     EventType = "session.revoked"
	EventSuspiciousActivity EventType = "security.suspicious" // refresh token reuse or hash mismatch
)
```

## types.ExternalIdentity

```go
type ExternalIdentity struct {
	Provider   string // "google", "github", "microsoft"
	ExternalID string // the user's ID at the external provider
	Email      string // email returned by the provider
	LinkedAt   time.Time
}
```

ExternalIdentity represents an identity link with an external IDP.

## types.IAMEvent

```go
type IAMEvent struct {
	Type      EventType
	UserID    string
	TenantID  string
	Module    string
	SessionID string
	Provider  string // "local", "google", "github", etc.

	IPAddress string
	UserAgent string
	DeviceID  string

	Timestamp time.Time
	Error     error          // nil on success events
	Extra     map[string]any // additional event-specific metadata
}
```

IAMEvent is emitted by the SDK for each relevant security action.
The developer decides where to persist it via Config.OnEvent.
Never includes passwords, raw tokens, or refresh tokens.

## types.IDPUser

```go
type IDPUser struct {
	ExternalID    string
	Provider      string
	Email         string
	EmailVerified bool
	Name          string
	PictureURL    string
	RawClaims     map[string]any // original claims from the IDP for reference
}
```

IDPUser is the normalized profile returned by any external IDP.
Different providers return different formats and the adapter for each IDP
is responsible for normalizing the data into this type.

## types.Session

```go
type Session struct {
	ID     string
	UserID string

	// Context selected after authentication.
	TenantID string
	Module   string

	// Access token issued for this session.
	AccessToken string

	// RefreshToken is the raw high-entropy token.
	// Populated only at issuance (SelectTenant and RefreshToken).
	// Never persisted — the SessionPort stores only RefreshTokenHash.
	RefreshToken string `json:"-"`

	// RefreshTokenHash is the SHA-256 of the raw token and is the value that gets persisted.
	RefreshTokenHash string

	// RefreshTokenLastFour is the suffix used for debugging and auditing without exposing the token.
	RefreshTokenLastFour string

	AuthProvider string // "local", "google", "github", etc.

	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastUsedAt time.Time

	Revoked   bool
	RevokedAt *time.Time
	RevokedBy string // "user", "admin", "system", "token_rotation"

	// Audit context metadata.
	IPAddress string
	UserAgent string
	DeviceID  string

	// Extra carries project-specific session attributes that the SDK schema does not
	// model (e.g. external provider tokens, domain role labels). Persisted by the
	// SessionPort as part of the session blob — round-tripped on Save/Get.
	Extra map[string]string `json:"extra,omitempty"`
}
```

Session represents an authenticated user session.
The raw RefreshToken is only populated at issuance time and must never be persisted.
What is persisted is the RefreshTokenHash, which is the SHA-256 of the raw token.

## types.Tenant

```go
type Tenant struct {
	ID      string
	Name    string
	Modules []string
	Active  bool
}
```

Tenant represents an organization or isolated context in the multi-tenant system.

## types.TenantAccess

```go
type TenantAccess struct {
	Tenant  Tenant
	Modules []string
	Roles   []string
}
```

TenantAccess represents a user's access to a tenant along with their modules and roles.

## types.User

```go
type User struct {
	ID            string
	Email         string
	PasswordHash  string // bcrypt or argon2 hash — never plaintext
	Active        bool
	EmailVerified bool

	// External identities linked for social login via IDP.
	ExternalIdentities []ExternalIdentity
}
```

User represents a system user.
PasswordHash must never be exposed outside the infrastructure layer.

