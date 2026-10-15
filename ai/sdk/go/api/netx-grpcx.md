# netx/grpcx

`import "github.com/joaoprofile/gofi-sdk-go/netx/grpcx"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package grpcx serves and calls gRPC services with the same defaults as the
httpx HTTP server: hardened TLS with certificate reload and optional mTLS,
request IDs, panic recovery, error-only logging, OpenTelemetry, graceful
shutdown with readiness draining and the standard gRPC health service.

It is a separate module so services that only speak HTTP never link gRPC.

Server:

	srv, err := grpcx.NewServer(grpcx.ServerConfig{Addr: ":9090"})
	ordersv1.RegisterOrdersServer(srv, ordersImpl) // generated code takes a grpc.ServiceRegistrar
	err = srv.ListenAndServe()                     // until SIGINT/SIGTERM or Shutdown

Handlers return errs.AppError (or any error); the server turns it into a
gRPC status whose code follows the AppError kind and whose details carry the
AppError code, so a grpcx client gets the same AppError back (FromError).

Client:

	conn, err := grpcx.NewClient("dns:///orders.internal:9090", grpcx.ClientConfig{
	    TLS:   &grpcx.ClientTLSConfig{CAFile: "/etc/tls/ca.pem"},
	    Token: tokenSource, // optional bearer token per call
	})
	defer conn.Close()
	client := ordersv1.NewOrdersClient(conn)

gofi services declare the server with the gofi/component/grpcserver component.

## Constantes

```go
const (
	// DefaultCallTimeout is the deadline of calls whose context has none.
	DefaultCallTimeout = 10 * time.Second
	// DefaultClientKeepaliveTime is above the server's DefaultKeepaliveMinTime.
	DefaultClientKeepaliveTime    = 30 * time.Second
	DefaultClientKeepaliveTimeout = 10 * time.Second
	// DefaultLoadBalancing spreads calls over every address the resolver
	// returns ("dns:///svc.ns.svc.cluster.local:9090" with a headless
	// Service reaches every pod).
	DefaultLoadBalancing = "round_robin"
)
```

Client defaults applied when the matching ClientConfig field is <= 0.

```go
const (
	DefaultMaxMsgSize        = 4 << 20 // 4 MiB, gRPC's own receive default
	DefaultConnectionTimeout = 10 * time.Second
	// DefaultShutdownTimeout stays below Kubernetes' default 30s grace period.
	DefaultShutdownTimeout = 25 * time.Second
	// DefaultDrainDelay lets endpoints observe NOT_SERVING before the
	// listener closes.
	DefaultDrainDelay = 5 * time.Second

	// Keepalive defaults. The server accepts client pings every
	// DefaultKeepaliveMinTime, below the client's DefaultClientKeepaliveTime,
	// so grpcx clients are never disconnected for pinging too often.
	DefaultKeepaliveTime      = 2 * time.Minute
	DefaultKeepaliveTimeout   = 20 * time.Second
	DefaultKeepaliveMinTime   = 15 * time.Second
	DefaultMaxConnectionIdle  = 15 * time.Minute
	DefaultMaxConnectionGrace = 30 * time.Second
)
```

Server defaults applied when the matching ServerConfig field is <= 0.

```go
const ErrorDomain = "gofi.errs"
```

ErrorDomain is the ErrorInfo domain grpcx uses to carry an AppError.

```go
const RequestIDMetadata = "x-request-id"
```

RequestIDMetadata carries the request ID in both directions, the gRPC
counterpart of httpx.RequestIDHeader.

## Funções

### grpcx.BearerCredentials

```go
func BearerCredentials(source TokenSource, requireTLS bool) credentials.PerRPCCredentials
```

BearerCredentials returns per-RPC credentials that send the token from
source. requireTLS makes gRPC refuse to send it over a plaintext
connection; turn it off only inside a private network.

### grpcx.CodeOf

```go
func CodeOf(kind errs.ErrorKind) codes.Code
```

CodeOf maps an AppError kind to the gRPC status code, following the HTTP
status httpx.RespondError uses for the same kind.

### grpcx.FromError

```go
func FromError(err error) (errs.AppError, bool)
```

FromError converts an error returned by a gRPC call into an AppError. A
status carrying a grpcx ErrorInfo yields the AppError registered under that
code in this process (errs.GetErrorByCode) or, when the code is unknown
here, one rebuilt from the kind and message. Other statuses get the kind
that matches their code. ok is false when err is nil.

### grpcx.NewClient

```go
func NewClient(target string, cfg ClientConfig) (*grpc.ClientConn, error)
```

NewClient creates a client connection to target ("dns:///host:port"). The
connection is lazy: it connects on the first call. The caller closes it.

### grpcx.Status

```go
func Status(err error) error
```

Status converts err into a gRPC status error. A status error passes
through; context cancellation and deadline map to Canceled and
DeadlineExceeded; an AppError becomes CodeOf(kind) with its message and an
ErrorInfo detail carrying its code and kind, never its cause; any other
error becomes Internal with a generic message. A nil err returns nil.

## grpcx.AuthConfig

```go
type AuthConfig struct {
	// Validate checks the token from the "authorization: Bearer" metadata.
	Validate TokenValidator
	// PublicMethods are full-method prefixes served without a token
	// ("/pkg.Service/Method" or "/pkg.Service/"). Health checks are always
	// public; reflection is public only when listed here.
	PublicMethods []string
}
```

AuthConfig enables bearer-token authentication on the server.

## grpcx.ClientConfig

```go
type ClientConfig struct {
	// TLS customizes the server verification and enables mTLS. Nil with
	// Insecure false means TLS against the system roots.
	TLS *ClientTLSConfig

	// Insecure dials in plaintext (h2c). It must be set explicitly and is
	// meant only for private networks or a mesh that encrypts the hop.
	Insecure bool

	// Token sends "authorization: Bearer <token>" on every call. Over an
	// Insecure connection the token travels in plaintext.
	Token TokenSource

	// Timeout is applied to calls whose context has no deadline
	// (default DefaultCallTimeout; negative disables it).
	Timeout time.Duration

	// Retry enables gRPC retries for the listed codes. Off by default:
	// retrying is only safe for idempotent methods.
	Retry *RetryPolicy

	// LoadBalancing is the policy name (default DefaultLoadBalancing).
	LoadBalancing string

	// MaxRecvMsgSize and MaxSendMsgSize bound a single message
	// (default DefaultMaxMsgSize).
	MaxRecvMsgSize int
	MaxSendMsgSize int

	// KeepaliveTime and KeepaliveTimeout tune client pings (defaults above).
	KeepaliveTime    time.Duration
	KeepaliveTimeout time.Duration

	// UserAgent is prepended to gRPC's own user agent.
	UserAgent string

	// UnaryInterceptors and StreamInterceptors run after the built-in ones
	// (request ID, default timeout).
	UnaryInterceptors  []grpc.UnaryClientInterceptor
	StreamInterceptors []grpc.StreamClientInterceptor

	// DisableTelemetry turns off the OpenTelemetry stats handler.
	DisableTelemetry bool
}
```

ClientConfig configures NewClient. The zero value dials with TLS against
the system roots.

## grpcx.ClientTLSConfig

```go
type ClientTLSConfig struct {
	// CAFile is a PEM bundle of CAs trusted for the server; empty uses the
	// system roots.
	CAFile string
	// CertFile and KeyFile are the client certificate for mTLS (both or neither).
	CertFile string
	KeyFile  string
	// ServerName overrides the name verified in the server certificate.
	ServerName string
}
```

ClientTLSConfig verifies the server and optionally presents a client
certificate (mTLS).

## grpcx.HealthConfig

```go
type HealthConfig struct {
	// CheckTimeout bounds all readiness checks of one probe (default 2s).
	CheckTimeout time.Duration
	// CacheTTL reuses the last result (default 1s); concurrent probes share
	// one round of checks.
	CacheTTL time.Duration
}
```

HealthConfig tunes the grpc.health.v1 service.

## grpcx.KeepaliveConfig

```go
type KeepaliveConfig struct {
	// Time is the idle period after which the server pings the client.
	Time time.Duration
	// Timeout is how long the server waits for the ping ack.
	Timeout time.Duration
	// MinClientInterval is the shortest client ping interval accepted;
	// clients pinging faster are disconnected.
	MinClientInterval time.Duration
	// MaxConnectionIdle closes connections idle for this long.
	MaxConnectionIdle time.Duration
	// MaxConnectionAge closes connections after this age so clients
	// rebalance across replicas (0 = never).
	MaxConnectionAge time.Duration
	// MaxConnectionAgeGrace lets calls in flight finish after MaxConnectionAge.
	MaxConnectionAgeGrace time.Duration
}
```

KeepaliveConfig tunes server keepalive; zero fields use the defaults.

## grpcx.RetryPolicy

```go
type RetryPolicy struct {
	// MaxAttempts counts the first call (default 3, gRPC caps it at 5).
	MaxAttempts int
	// InitialBackoff and MaxBackoff bound the jittered exponential backoff
	// (defaults 100ms and 2s).
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	// Codes are the retryable status codes (default Unavailable).
	Codes []codes.Code
}
```

RetryPolicy is the gRPC retry policy applied to every method of the
connection.

## grpcx.Server

```go
type Server interface {
	grpc.ServiceRegistrar
	// AddHealthCheck registers a readiness check served by grpc.health.v1.
	AddHealthCheck(name string, check func(ctx context.Context) error)
	// ListenAndServe listens on Addr and serves until SIGINT/SIGTERM or
	// Shutdown, then reports NOT_SERVING, drains and stops gracefully. It
	// returns nil on a clean stop.
	ListenAndServe() error
	// Serve is ListenAndServe on a listener the caller opened, without
	// signal handling.
	Serve(ln net.Listener) error
	// Shutdown stops a running ListenAndServe/Serve and waits for it, or for
	// ctx. Calling it before serving makes the next serve return at once.
	Shutdown(ctx context.Context) error
	// GRPC returns the underlying *grpc.Server, for features grpcx does not
	// wrap.
	GRPC() *grpc.Server
}
```

Server is a gRPC server with the httpx server lifecycle. It is a
grpc.ServiceRegistrar, so generated Register*Server functions accept it.

### grpcx.NewServer

```go
func NewServer(config ServerConfig) (Server, error)
```

NewServer builds the server; it does not listen until ListenAndServe.

## grpcx.ServerConfig

```go
type ServerConfig struct {
	// Addr is the listen address (":9090").
	Addr string

	// TLS enables TLS (and mTLS with ClientCAFile) with the same hardening
	// and certificate reload as the httpx HTTP server. ALPN is h2 only.
	TLS *netx.TLSConfig

	// MaxRecvMsgSize and MaxSendMsgSize bound a single message
	// (default DefaultMaxMsgSize).
	MaxRecvMsgSize int
	MaxSendMsgSize int

	// MaxConcurrentStreams bounds the streams per connection (0 = gRPC default).
	MaxConcurrentStreams uint32

	// ConnectionTimeout bounds the connection handshake (TLS included).
	ConnectionTimeout time.Duration

	// Keepalive tunes server pings and connection ageing.
	Keepalive KeepaliveConfig

	// ShutdownTimeout bounds the graceful stop; streams still open after it
	// are cancelled.
	ShutdownTimeout time.Duration

	// DrainDelay is waited between reporting NOT_SERVING and closing the
	// listener (default DefaultDrainDelay; negative disables it).
	DrainDelay time.Duration

	// Health tunes the grpc.health.v1 service, which is always registered.
	Health HealthConfig

	// Reflection registers the server reflection service. Off by default: it
	// lists every service and method to anyone who can reach the port.
	Reflection bool

	// Auth validates a bearer token on every call except the public methods.
	// Health checks are always public.
	Auth *AuthConfig

	// UnaryInterceptors and StreamInterceptors run after the built-in ones
	// (recovery, request ID, logging, auth, error mapping), closest to the
	// handler.
	UnaryInterceptors  []grpc.UnaryServerInterceptor
	StreamInterceptors []grpc.StreamServerInterceptor

	// DisableTelemetry turns off the OpenTelemetry stats handler.
	DisableTelemetry bool
}
```

ServerConfig configures NewServer. The zero value serves plaintext h2c on
Addr with the defaults above.

### ServerConfig.Validate

```go
func (c ServerConfig) Validate() error
```

Validate reports a config NewServer cannot serve.

## grpcx.TokenSource

```go
type TokenSource func(ctx context.Context) (string, error)
```

TokenSource returns the bearer token for one call; it is called on every
RPC, so cache inside it when minting tokens is expensive.

## grpcx.TokenValidator

```go
type TokenValidator func(ctx context.Context, token string) (context.Context, error)
```

TokenValidator checks the bearer token of a call and returns the context
the handler runs with (typically carrying the caller's claims). Any error
rejects the call with Unauthenticated and a generic message; the error is
never sent to the caller.

