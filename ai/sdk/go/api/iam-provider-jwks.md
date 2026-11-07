# iam/provider/jwks

`import "github.com/joaoprofile/gofi-sdk-go/iam/provider/jwks"` · SDK gofi/v0.2.18 (c5413b9)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package jwks verifies JWTs signed by a third party that publishes its public
keys as a JWKS: tokens a customer's system issues to its own users, for
example. Only asymmetric algorithms are accepted; keys are cached per URL.

The HTTP client fetches a URL configured by someone else: inject an
outbound client that blocks private addresses (netx/httpx.NewSafeClient).

## Constantes

```go
const DefaultCacheTTL = time.Hour
```

DefaultCacheTTL is how long a fetched key set is served before a refetch.

```go
const MaxBody = 1 << 20
```

MaxBody caps a JWKS response.

```go
const MinRSABits = 2048
```

MinRSABits rejects JWKS keys below the NIST minimum for signatures.

## Variáveis

```go
var DefaultAlgorithms = []string{"RS256", "ES256"}
```

DefaultAlgorithms are accepted when Config.Algorithms is empty.

```go
var ErrInsecureEndpoint = errors.New("iam: endpoint must use https")
```

ErrInsecureEndpoint is returned for an endpoint that is not https (plain
http is accepted only on loopback, for tests).

```go
var ErrInvalidConfig = errors.New("iam/jwks: invalid config")
```

ErrInvalidConfig is returned by NewVerifier for an unusable Config.

```go
var ErrInvalidToken = errors.New("iam/jwks: invalid token")
```

ErrInvalidToken wraps every verification failure; the wrapped text names
the reason (signature, issuer, audience, expiry, lifetime) and is safe to
show to whoever configured the issuer, not to the token holder.

```go
var ErrKeyNotFound = errors.New("iam/jwks: key not found")
```

ErrKeyNotFound is returned when no key in the set matches the kid.

## Funções

### jwks.RequireHTTPS

```go
func RequireHTTPS(raw string) error
```

RequireHTTPS accepts https, and http only for loopback hosts.

## jwks.Cache

```go
type Cache struct {
	// contains filtered or unexported fields
}
```

Cache holds the keys of one JWKS URL. Concurrent refetches share one
request, and a failed refetch keeps the previous keys in use.

### jwks.NewCache

```go
func NewCache(ttl time.Duration) *Cache
```

NewCache returns a cache that refetches after ttl; ttl <= 0 uses DefaultCacheTTL.

### Cache.GetKey

```go
func (c *Cache) GetKey(ctx context.Context, client *http.Client, jwksURL, kid string) (any, error)
```

GetKey returns the key for kid (an empty kid matches the first key),
fetching the set when it is stale or the kid is unknown.

### Cache.Refresh

```go
func (c *Cache) Refresh(ctx context.Context, client *http.Client, jwksURL string) (int, error)
```

Refresh fetches the set now and reports how many usable keys it has.

## jwks.Config

```go
type Config struct {
	// JWKSURL is the https URL of the issuer's key set.
	JWKSURL string

	// Issuer must equal the token's iss claim.
	Issuer string

	// Audience must be one of the token's aud values.
	Audience string

	// Algorithms accepted in the token header; asymmetric only.
	// Default: DefaultAlgorithms.
	Algorithms []string

	// ClockSkew is the leeway for exp, iat and nbf. Default: 1 minute;
	// capped at core.MaxClockSkew.
	ClockSkew time.Duration

	// MaxLifetime, when > 0, requires iat and rejects tokens whose exp - iat
	// exceeds it, so an issuer cannot mint long-lived tokens.
	MaxLifetime time.Duration

	// CacheTTL is how long the key set is served before a refetch.
	// Default: DefaultCacheTTL.
	CacheTTL time.Duration

	// HTTPClient fetches the key set. Default: a client with a 10s timeout.
	HTTPClient *http.Client
}
```

Config configures a Verifier.

## jwks.Verifier

```go
type Verifier struct {
	// contains filtered or unexported fields
}
```

Verifier checks tokens of one issuer against its JWKS.

### jwks.NewVerifier

```go
func NewVerifier(cfg Config) (*Verifier, error)
```

NewVerifier validates cfg and returns a Verifier. No request is made.

### Verifier.Check

```go
func (v *Verifier) Check(ctx context.Context) (int, error)
```

Check fetches the key set now; use it to validate an issuer when it is
configured. It returns the number of usable keys.

### Verifier.Verify

```go
func (v *Verifier) Verify(ctx context.Context, raw string) (map[string]any, error)
```

Verify checks signature, algorithm, issuer, audience, exp (required), iat,
nbf and MaxLifetime, and returns the token's claims.

