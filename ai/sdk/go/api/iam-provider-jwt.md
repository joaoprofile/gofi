# iam/provider/jwt

`import "github.com/joaoprofile/gofi-sdk-go/iam/provider/jwt"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## jwt.Algorithm

```go
type Algorithm string
```

Algorithm defines the supported JWT signing algorithm.

```go
const (
	HS256 Algorithm = "HS256"
	RS256 Algorithm = "RS256"
	ES256 Algorithm = "ES256"
)
```

## jwt.Config

```go
type Config struct {
	Algorithm Algorithm

	// HS256: symmetric key (minimum 32 bytes).
	Secret []byte

	// RS256/ES256: asymmetric key pair.
	PrivateKey any // *rsa.PrivateKey or *ecdsa.PrivateKey
	PublicKey  any // *rsa.PublicKey  or *ecdsa.PublicKey

	// KeyID is written as the kid header, identifying the signing key.
	KeyID string
	// VerificationKeys are previous keys, by kid, still accepted while tokens
	// they signed expire (key rotation). Same algorithm as the current key.
	VerificationKeys map[string]any

	AccessTokenTTL time.Duration // default: 15 min
	Issuer         string        // issued as iss
	VerifyIssuer   bool          // require iss == Issuer; opt-in for services sharing a secret
	Audience       string        // issued as aud and required on validation when set
	Leeway         time.Duration // clock skew tolerance for exp/iat/nbf
}
```

Config configures the JWT provider.

## jwt.Provider

```go
type Provider struct {
	// contains filtered or unexported fields
}
```

Provider implements port.TokenPort using JWT (HS256/RS256/ES256).

### jwt.NewProvider

```go
func NewProvider(cfg Config) (*Provider, error)
```

NewProvider builds a validated JWT Provider.

### Provider.IssueAccessToken

```go
func (p *Provider) IssueAccessToken(claims types.Claims) (string, error)
```

IssueAccessToken issues a signed JWT access token with the provided claims.

### Provider.IssueRefreshToken

```go
func (p *Provider) IssueRefreshToken(claims types.Claims) (string, error)
```

IssueRefreshToken generates a high-entropy opaque token in the format {sessionID}.{random}.
The core stores only the SHA-256 hash of the token.

### Provider.ParseToken

```go
func (p *Provider) ParseToken(token string) (*types.Claims, error)
```

ParseToken validates signature and expiry. Does not validate the session — that is done by AuthPort.

