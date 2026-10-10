# netx

`import "github.com/joaoprofile/gofi-sdk-go/netx"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package netx holds what the HTTP and gRPC transports share: the hardened
server TLS (TLSConfig, ServerTLSConfig) and the request ID carried in the
context (RequestIDKey, GetRequestID).

The transports live in subpackages:

  - netx/httpx: HTTP server (routes, middleware, CORS, rate limiting,
    health) and HTTP client (retries, signing, response parsing).
  - netx/grpcx: gRPC server and client, a separate module so services that
    only speak HTTP never link gRPC.
  - netx/awssign: AWS Signature V4 for the httpx client.

## Constantes

```go
const RequestIDKey contextKey = "request_id"
```

RequestIDKey is the context key under which the HTTP and gRPC servers store
the request ID.

## Funções

### netx.GetRequestID

```go
func GetRequestID(ctx context.Context) string
```

GetRequestID returns the request ID the HTTP or gRPC server stored in ctx.

### netx.NewRequestID

```go
func NewRequestID() string
```

NewRequestID returns 128 random bits (crypto/rand, base32).

### netx.ServerTLSConfig

```go
func ServerTLSConfig(c *TLSConfig) (*tls.Config, error)
```

ServerTLSConfig builds the hardened server tls.Config described by c, shared
by httpx and grpcx: TLS 1.2+, forward-secret AEAD suites, certificate
reload on file change and optional mTLS. ALPN offers h2 and http/1.1;
grpcx narrows it to h2. A nil c returns nil.

### netx.ValidRequestID

```go
func ValidRequestID(id string) bool
```

ValidRequestID accepts 1-64 characters of [A-Za-z0-9._-], so a client ID
can neither forge log fields nor bloat them.

## netx.TLSConfig

```go
type TLSConfig struct {
	// Config is a ready TLS configuration; it wins over the file fields.
	Config *tls.Config

	// CertFile and KeyFile are the PEM server certificate chain and key.
	CertFile string
	KeyFile  string

	// ClientCAFile is a PEM bundle of CAs trusted for client certificates
	// (mTLS). It is read once at startup.
	ClientCAFile string

	// ClientAuth defaults to tls.RequireAndVerifyClientCert when ClientCAFile
	// is set, tls.NoClientCert otherwise. With mTLS required, probes must
	// present a client certificate too; kubelet httpGet probes cannot, so use
	// tcpSocket/exec probes or a separate plain listener for them.
	ClientAuth tls.ClientAuthType
}
```

TLSConfig enables TLS on the httpx and grpcx servers. Config, when set, is
used as is (cloned, with MinVersion raised to TLS 1.2); otherwise CertFile
and KeyFile are loaded and reloaded on change, so certificate rotations
(cert-manager, mounted secrets) apply without a restart.

