# netx/httpx

`import "github.com/joaoprofile/gofi-sdk-go/netx/httpx"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package httpx serves and calls HTTP APIs: a chi-based server with hardened
TLS (netx.TLSConfig), request IDs, error-only logging, CORS, cross-origin
protection, rate limiting, health endpoints, OpenTelemetry and graceful
shutdown, and a client with retries, request signing and safe response
parsing.

gofi services declare the server with the gofi/component/httpserver
component.

## Constantes

```go
const (
	ErrServiceNotAvailable   = "service not available"
	ErrResponseWithEmptyBody = "response returned with empty body and %d status code"
	ErrConfigNotProvided     = "error config not provided"
)
```

```go
const (
	// DefaultMaxResponseBytes caps a decoded success body.
	DefaultMaxResponseBytes int64 = 10 << 20
	// DefaultMaxRetryAfter caps the wait a server can impose via Retry-After.
	DefaultMaxRetryAfter = 30 * time.Second
)
```

Client limits applied when the matching field is <= 0.

```go
const (
	DefaultReadTimeout = 10 * time.Second
	// DefaultWriteTimeout exceeds DefaultRequestTimeout so handlers that use their
	// whole budget can still write the response.
	DefaultWriteTimeout   = 35 * time.Second
	DefaultIdleTimeout    = 60 * time.Second
	DefaultRequestTimeout = 30 * time.Second
	// DefaultShutdownTimeout stays below Kubernetes' default 30s grace period.
	DefaultShutdownTimeout = 25 * time.Second
	// DefaultDrainDelay is applied when Health is enabled, so endpoints observe
	// the failed readiness before connections close.
	DefaultDrainDelay = 5 * time.Second
)
```

Server timeouts applied when the matching WSConfig field is <= 0.

```go
const (
	// DefaultStressTimeout is how long a request waits for a slot before 503.
	DefaultStressTimeout = 100 * time.Millisecond
	// DefaultStressBufferBytes is how much of a request body is read before a
	// slot is taken.
	DefaultStressBufferBytes int64 = 64 << 10
)
```

Stress control defaults, applied when StressControlConfig leaves a field <= 0.

```go
const (

	// AnyOrigin allows every origin; it requires AllowCredentials=false.
	AnyOrigin = "*"
)
```

```go
const DefaultHSTSMaxAge = 2 * 365 * 24 * time.Hour
```

DefaultHSTSMaxAge is two years, the preload list minimum.

```go
const DefaultMaxBodyBytes int64 = 10 << 20 // 10 MB
```

DefaultMaxBodyBytes is the request body cap applied when no explicit limit
is configured.

```go
const ErrMsgOnQueryParameter = "error on parsing query parameters %w"
```

```go
const RequestIDHeader = "X-Request-Id"
```

RequestIDHeader carries the request ID in both directions.

```go
const TrustPrivateNetworks = "private"
```

TrustPrivateNetworks is a TrustedProxies entry that expands to
PrivateNetworks(), so "private" can be set from an env var.

## Variáveis

```go
var (
	ErrUnsupportedBodyType = errors.New("unsupported body type")
	ErrTooManyRequests     = errors.New("too many requests")
	ErrResponseIsNil       = errors.New("response is nil")
	// ErrResponseTooLarge is returned when a success body exceeds
	// HttpClient.MaxResponseBytes.
	ErrResponseTooLarge = errors.New("httpx: response body too large")
	// ErrURLOutsideBase is returned when the request path would change the
	// scheme, host or base path of HttpClient.BaseURL (e.g. "@evil.com").
	ErrURLOutsideBase = errors.New("httpx: request URL escapes the client base URL")
)
```

```go
var (
	ErrFailedToCreateRequest         = "failed to create request: %w"
	ErrSignatureRequest              = "signature request error: %w"
	ErrOnExecuteRequest              = "failed to execute request: %w"
	ErrRequestFailed                 = "request failed: %w"
	ErrFailedToReadResponseBody      = "failed to read response body: %w"
	ErrFailedToUnmarshalResponseBody = "failed to unmarshal response body: %w"
	ErrUnexpectedStatusCode          = "unexpected status code: %d | body: %v"
	ErrRequestExceededToStatus429    = "request exceeded retries due to status 429"
	ErrRateLimitedRetryDisabled      = "request rate limited (retry on 429 disabled)"
	ErrRateLimiting                  = "rate limiting error: %w"
	ErrUnexpectedDuringRetries       = "unexpected error during retries"
)
```

```go
var (
	ErrJSONEncoder       = errors.New("error encoding data")
	ErrReadBody          = errors.New("error reading request body")
	ErrJSONUnmarshal     = errors.New("JSON structure error")
	ErrInvalidStruct     = errors.New("provided type is not a valid struct")
	ErrMissingQueryParam = errors.New("missing query parameter")
)
```

```go
var (
	ErrUnsupportedMediaType = errors.New("unsupported media type")
	ErrBodyTooLarge         = errors.New("request body too large")
	ErrInvalidJSON          = errors.New("invalid JSON body")
	ErrInvalidQueryParam    = errors.New("invalid query parameter")
)
```

Causes wrapped by RequestError, for errors.Is.

```go
var ErrRedirectBlocked = errors.New("httpx: redirect to a different origin blocked")
```

ErrRedirectBlocked is returned when a server redirects to another scheme or
host: following it would resend custom credential headers (X-API-Key,
X-Amz-Security-Token, ...) to a third party. Same-origin redirects and the
http→https upgrade on the same host are followed.

## Funções

### httpx.BindQueryParamsToStruct

```go
func BindQueryParamsToStruct(r *http.Request, _ http.ResponseWriter, tStruct any) error
```

BindQueryParamsToStruct populates tStruct from the URL query parameters.
Each exported field is matched by its `form` tag ("-" skips it), falling
back to the lowercased field name; unexported fields are ignored. tStruct
must be a non-nil pointer to a struct. A value that does not parse or does
not fit the field is a 400 *RequestError naming the parameter, never
echoing the value.

### httpx.BlockUnsafeMethods

```go
func BlockUnsafeMethods(next http.Handler) http.Handler
```

BlockUnsafeMethods rejects TRACE (information disclosure) and CONNECT
(proxy tunnel abuse) before they reach application handlers.

### httpx.DefaultCORSAllowedHeaders

```go
func DefaultCORSAllowedHeaders() []string
```

DefaultCORSAllowedHeaders are the request headers a preflight may ask for
when CorsConfig.AllowedHeaders is empty.

### httpx.DefaultMaxConcurrent

```go
func DefaultMaxConcurrent() int
```

DefaultMaxConcurrent returns the default slot count: 64 per GOMAXPROCS, at
least 256. Handlers are mostly I/O-bound (DB, downstream HTTP), so the cap
guards memory and backend fan-out rather than CPU; a few dozen slots would
turn ordinary latency spikes into 503s.

### httpx.Error

```go
func Error(w http.ResponseWriter, statusCode int, err error)
```

Error writes err as an ErrorResponse. A *RequestError (ParseRequestBody,
BindQueryParamsToStruct) sets its own status and client-safe message. For
status >= 500 the client gets a generic message and err is only logged.

### httpx.ErrorDetails

```go
func ErrorDetails(w http.ResponseWriter, statusCode int, message string, details any)
```

ErrorDetails writes message and details; for status >= 500 both are
replaced by a generic message and logged.

### httpx.GetPathParam

```go
func GetPathParam(param string, r *http.Request) string
```

GetPathParam returns the value of the named URL path parameter, unchanged.

### httpx.GetQueryParam

```go
func GetQueryParam(filter string, r *http.Request) string
```

Query / path parameters

GetQueryParam returns the value of the named URL query parameter, unchanged.

### httpx.JSON

```go
func JSON(w http.ResponseWriter, statusCode int, jsonData []byte)
```

### httpx.LimitBody

```go
func LimitBody(next http.Handler) http.Handler
```

LimitBody caps request bodies at DefaultMaxBodyBytes to prevent memory
exhaustion attacks. Kept for callers that do not need a custom cap.

### httpx.LogIPBlocked

```go
func LogIPBlocked(r *http.Request, clientName string)
```

LogIPBlocked logs a blocked IP attempt at Warn level.

### httpx.LogInvalidAPIKey

```go
func LogInvalidAPIKey(r *http.Request)
```

LogInvalidAPIKey logs an invalid API key attempt at Warn level.

### httpx.LogRateLimit

```go
func LogRateLimit(r *http.Request)
```

LogRateLimit logs a rate-limit event at Warn level with trace context.

### httpx.LogServerError

```go
func LogServerError(r *http.Request, err error)
```

LogServerError logs an application error at Error level with trace context.

### httpx.ParseRequestBody

```go
func ParseRequestBody(w http.ResponseWriter, r *http.Request, tStruct any, opts ...BodyOption) error
```

ParseRequestBody decodes a single JSON value from the request body into
tStruct, a non-nil pointer to a struct (ErrInvalidStruct otherwise). It
requires Content-Type application/json (or application/*+json), rejects
unknown fields (see AllowUnknownFields) and trailing data. Client errors
are *RequestError: 415 for the media type, 413 for an oversized body and
400 for invalid JSON, with generic messages safe to return.

### httpx.PrivateNetworks

```go
func PrivateNetworks() []string
```

PrivateNetworks returns the private, loopback, link-local and CGNAT ranges
in-cluster ingresses and cloud load balancers reach the pod from. Trusting
them lets any internal peer spoof X-Forwarded-For, so opt in only when every
such peer is a proxy.

### httpx.ReadBody

```go
func ReadBody(r *http.Request) ([]byte, error)
```

Body parsing

ReadBody reads and returns the full request body.
The body is always closed, even when the read fails.

### httpx.Recoverer

```go
func Recoverer(next http.Handler) http.Handler
```

Recoverer turns a handler panic into a generic 500 and logs the panic with
the request ID, trace and stack. http.ErrAbortHandler is re-raised, so
net/http aborts the response as intended.

### httpx.RespondError

```go
func RespondError(w http.ResponseWriter, r *http.Request, appErr errs.AppError)
```

### httpx.Response

```go
func Response(w http.ResponseWriter, statusCode int, data any)
```

### httpx.SecurityHeaders

```go
func SecurityHeaders(next http.Handler) http.Handler
```

SecurityHeaders sets defensive response headers against clickjacking, XSS,
MIME sniffing and caching of API responses, plus HSTS over TLS.

### httpx.ValidateRequest

```go
func ValidateRequest(next http.Handler) http.Handler
```

ValidateRequest rejects requests that carry both Content-Length and
Transfer-Encoding, a classic HTTP request-smuggling vector.

## httpx.BodyOption

```go
type BodyOption func(*bodyOptions)
```

BodyOption tunes ParseRequestBody.

### httpx.AllowUnknownFields

```go
func AllowUnknownFields() BodyOption
```

AllowUnknownFields accepts JSON fields the target struct does not declare.

### httpx.WithMaxBodyBytes

```go
func WithMaxBodyBytes(n int64) BodyOption
```

WithMaxBodyBytes caps the body read by ParseRequestBody (default
DefaultMaxBodyBytes); the server-wide WSConfig.MaxBodyBytes still applies.

## httpx.CorsConfig

```go
type CorsConfig struct {
	// AllowedOrigins are exact scheme://host[:port] origins, or AnyOrigin
	// ("*", only with AllowCredentials=false).
	AllowedOrigins []string
	// AllowedMethods are checked against Access-Control-Request-Method;
	// empty uses DefaultCORSConfig's.
	AllowedMethods []string
	// AllowedHeaders bound Access-Control-Allow-Headers; empty uses
	// DefaultCORSAllowedHeaders.
	AllowedHeaders   []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           string
}
```

CorsConfig is a CORS policy. A route policy (RouteBuilder.Cors) replaces
the global one for that route, preflights included.

### httpx.DefaultCORSConfig

```go
func DefaultCORSConfig() CorsConfig
```

DefaultCORSConfig is the global policy; it allows no origin until
AllowedOrigins is set.

### CorsConfig.Validate

```go
func (c CorsConfig) Validate() error
```

Validate rejects origins that are not scheme://host[:port] (subdomain
wildcards are not supported) and AnyOrigin combined with AllowCredentials.

## httpx.ErrorResponse

```go
type ErrorResponse struct {
	StatusCode int    `json:"code"`
	Message    string `json:"message"`
	ErrorCode  string `json:"errorCode,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Cause      string `json:"cause,omitempty"`
	Details    any    `json:"details,omitempty"`
}
```

## httpx.HSTSConfig

```go
type HSTSConfig struct {
	// Disabled omits the header, e.g. when the ingress sets it.
	Disabled bool
	// MaxAge defaults to DefaultHSTSMaxAge when <= 0.
	MaxAge time.Duration
	// ExcludeSubDomains drops includeSubDomains.
	ExcludeSubDomains bool
	// Preload adds preload. Opt in only when every subdomain serves HTTPS:
	// leaving the preload list takes months.
	Preload bool
}
```

HSTSConfig configures Strict-Transport-Security. The zero value sends
max-age=63072000; includeSubDomains.

## httpx.HandlerRegistry

```go
type HandlerRegistry struct {
	// contains filtered or unexported fields
}
```

### httpx.NewHandlerRegistry

```go
func NewHandlerRegistry(lc *observer.Registry) *HandlerRegistry
```

### HandlerRegistry.Handlers

```go
func (r *HandlerRegistry) Handlers() []RouterHandler
```

### HandlerRegistry.Register

```go
func (r *HandlerRegistry) Register(h ...RouterHandler)
```

## httpx.HealthConfig

```go
type HealthConfig struct {
	LivenessPath  string        // default "/livez"
	ReadinessPath string        // default "/readyz"
	CheckTimeout  time.Duration // default 2s for all readiness checks
	// CacheTTL reuses the last readiness result (default 1s); concurrent
	// probes share one round of checks, so /readyz cannot flood dependencies.
	CacheTTL time.Duration
}
```

HealthConfig enables liveness and readiness endpoints. They are served ahead
of every middleware, so probes never hit auth, rate limiting or logging.

## httpx.HttpClient

```go
type HttpClient struct {
	Name              string
	BaseURL           string
	Retries           uint8
	RetrySleep        time.Duration
	DisableRetryOn429 bool
	Client            *http.Client
	RateLimit         int
	// MaxResponseBytes and MaxRetryAfter: see HttpClientConfig.
	MaxResponseBytes int64
	MaxRetryAfter    time.Duration
	// contains filtered or unexported fields
}
```

### httpx.NewClient

```go
func NewClient(config *HttpClientConfig) (*HttpClient, error)
```

## httpx.HttpClientConfig

```go
type HttpClientConfig struct {
	Name       string
	BaseURL    string
	Timeout    time.Duration
	Retries    uint8
	RetrySleep time.Duration
	RateLimit  int
	// DisableRetryOn429 stops the retry loop from swallowing a 429: the status is
	// returned to the caller as-is, immediately. Retries on 5xx and network errors
	// are unaffected. Set it when an external rate limiter owns the pacing —
	// retrying in place holds the worker for the whole backoff and issues requests
	// the limiter never authorized, which fights the limiter instead of helping it.
	DisableRetryOn429 bool
	// MaxResponseBytes caps a success body; larger ones fail with
	// ErrResponseTooLarge. <= 0 uses DefaultMaxResponseBytes.
	MaxResponseBytes int64
	// MaxRetryAfter caps the Retry-After wait of a 429. <= 0 uses
	// DefaultMaxRetryAfter.
	MaxRetryAfter time.Duration
}
```

## httpx.HttpError

```go
type HttpError struct {
	Message string   `json:"message"`
	Err     string   `json:"error"`
	Status  int      `json:"status"`
	Cause   []string `json:"cause"`
}
```

### httpx.FromError

```go
func FromError(err error) *HttpError
```

### httpx.NewAPIError

```go
func NewAPIError(status int, message string, err error) *HttpError
```

### httpx.ParseError

```go
func ParseError(jsonStr string) (*HttpError, error)
```

### HttpError.Error

```go
func (e *HttpError) Error() string
```

## httpx.HttpServer

```go
type HttpServer interface {
	Use(middleware ...Middleware)
	UseAuth(authMiddleware Middleware)
	AddHandlers(handlers ...RouterHandler)
	// ListenAndServe serves until SIGINT/SIGTERM or Shutdown, then fails
	// readiness, drains and stops gracefully. It returns nil on a clean stop.
	ListenAndServe() error
	// Shutdown stops a running ListenAndServe and waits for it to finish, or
	// for ctx. Calling it before ListenAndServe makes ListenAndServe return.
	Shutdown(ctx context.Context) error
	// AddHealthCheck registers a readiness check (requires WSConfig.Health).
	AddHealthCheck(name string, check func(ctx context.Context) error)
}
```

### httpx.NewServer

```go
func NewServer(config *WSConfig) HttpServer
```

NewServer builds the server. It panics on an invalid config; call
WSConfig.Validate first to get an error instead.

## httpx.Middleware

```go
type Middleware func(http.Handler) http.Handler
```

### httpx.CORSMiddleware

```go
func CORSMiddleware(config CorsConfig) Middleware
```

CORSMiddleware applies config to every response and answers preflights.
It panics on an invalid config (see CorsConfig.Validate).

### httpx.ClientIPMiddleware

```go
func ClientIPMiddleware(trustedProxies []string) Middleware
```

ClientIPMiddleware resolves the client IP once per request. X-Forwarded-For is
honoured only when the TCP peer is in trustedProxies (CIDRs, or
TrustPrivateNetworks); nil or empty trusts no proxy and uses the TCP peer.
Invalid CIDRs panic.

### httpx.LimitBodyWithMax

```go
func LimitBodyWithMax(maxBody int64) Middleware
```

LimitBodyWithMax caps request bodies at maxBody bytes. A maxBody <= 0 falls
back to DefaultMaxBodyBytes, so a missing configuration never leaves the
server without a ceiling.

Requests announcing an oversized Content-Length are rejected with 413 before
the handler runs, so the caller gets an honest error instead of a parse
failure surfacing deep inside the handler. Chunked requests carry no length
upfront: those stay capped by MaxBytesReader, which fails on read.

### httpx.LoggingMiddleware

```go
func LoggingMiddleware() Middleware
```

LoggingMiddleware logs only errors and security-relevant responses.
2xx and 3xx responses are intentionally silent — logs are persisted and
request-level noise would dominate the cost and signal-to-noise ratio.

Logged conditions:
  - 5xx  → Error  (server fault, always logged)
  - 401/403/429 → Warn  (access denied, rate limited — security signal)

### httpx.NewRedisRateLimiter

```go
func NewRedisRateLimiter(cfg RedisRateLimiterConfig) Middleware
```

NewRedisRateLimiter returns a Middleware that enforces per-client rate limits
using a sliding-window algorithm backed by the provided RateLimiterBackend.

Standard rate-limit response headers (X-RateLimit-Limit, X-RateLimit-Remaining,
X-RateLimit-Reset) are written on every response. Retry-After is added only
when the limit is exceeded. Requests without a known API key are bucketed by
client IP (IPv6 by /64). An invalid cfg (nil Backend, a plan with Rate or
Burst <= 0) panics: it is a wiring error, like an invalid CIDR.

### httpx.NewStressControlMiddleware

```go
func NewStressControlMiddleware(cfg StressControlConfig) Middleware
```

NewStressControlMiddleware limits the number of concurrent requests served
simultaneously. Route-specific limits are checked first by prefix; all
other requests fall through to the default limiter.

### httpx.SecurityHeadersWith

```go
func SecurityHeadersWith(cfg SecurityHeadersConfig) Middleware
```

SecurityHeadersWith is SecurityHeaders with HSTS settings; HSTS is sent over
TLS, or when a trusted proxy reports X-Forwarded-Proto: https.

## httpx.Module

```go
type Module interface {
	Register(reg *HandlerRegistry)
}
```

## httpx.RateLimitPlan

```go
type RateLimitPlan struct {
	Rate  int // requests per second
	Burst int // maximum burst size
}
```

RateLimitPlan defines the allowed request rate and burst for a named client plan.

## httpx.RateLimiterBackend

```go
type RateLimiterBackend interface {
	Allow(ctx context.Context, key string, limit redis_rate.Limit) (*redis_rate.Result, error)
}
```

RateLimiterBackend abstracts the rate-limiting backend, decoupling the
middleware from a concrete Redis client and making it fully testable without
a live Redis instance.

### httpx.NewRedisBackend

```go
func NewRedisBackend(client redis.UniversalClient) RateLimiterBackend
```

NewRedisBackend wraps a redis.UniversalClient into a RateLimiterBackend using
a sliding-window algorithm. Call this at the composition root where the Redis
client is already wired.

## httpx.RedisRateLimiterConfig

```go
type RedisRateLimiterConfig struct {
	// Backend is the rate-limiting implementation. Use NewRedisBackend to build
	// one from a redis.UniversalClient.
	Backend RateLimiterBackend

	// Plans maps API keys to specific rate-limit plans. Only keys listed here get
	// their own bucket; any other X-API-Key is limited by client IP.
	Plans map[string]RateLimitPlan

	// IsKnownAPIKey optionally accepts keys outside Plans; they get their own
	// bucket with the Default plan. Unaccepted keys are limited by client IP.
	IsKnownAPIKey func(key string) bool

	// Default is the plan applied to requests whose API key is absent or has no
	// matching entry in Plans.
	Default RateLimitPlan

	// KeyPrefix namespaces rate-limit keys in Redis. Defaults to "rl".
	KeyPrefix string

	// FailClosed rejects requests with 503 while the backend fails. The default
	// (false) lets them through; either way the failure is logged, at most
	// once per backendErrorLogInterval.
	FailClosed bool
}
```

RedisRateLimiterConfig holds the configuration for the rate-limiter middleware.
Backend must be provided by the caller — Redis is never created internally.

## httpx.Request

```go
type Request[T any] struct {
	Ctx        context.Context
	Client     *HttpClient
	HttpMethod string
	Url        string
	Headers    map[string]string
	Body       RequestBody
	Retries    int

	// ResponseHeaders carries the headers of the last response Execute() saw —
	// on success, and also on a 429 surfaced by DisableRetryOn429. Lets the caller
	// read transport metadata (a provider-returned rate limit, for instance)
	// without the SDK knowing the semantics of any particular header. Nil until
	// Execute() has run.
	ResponseHeaders http.Header
	// contains filtered or unexported fields
}
```

### httpx.NewRequest

```go
func NewRequest[T any](ctx context.Context, client *HttpClient, method, path string) *Request[T]
```

NewRequest targets client.BaseURL + path. A path that would leave the base
URL (another scheme, host, userinfo or a "../" above the base path) makes
Execute fail with ErrURLOutsideBase.

### Request.Execute

```go
func (r *Request[T]) Execute() (*T, error)
```

### Request.SetBody

```go
func (r *Request[T]) SetBody(body any)
```

### Request.SetHeader

```go
func (r *Request[T]) SetHeader(key, value string)
```

### Request.SetSignature

```go
func (r *Request[T]) SetSignature(signature Signature)
```

## httpx.RequestBody

```go
type RequestBody any
```

## httpx.RequestBodyResult

```go
type RequestBodyResult struct {
	Reader   io.Reader
	JSONData []byte
	Err      error
}
```

## httpx.RequestError

```go
type RequestError struct {
	Status  int
	Message string
	Err     error
}
```

RequestError is a client error. Error returns Message, which is safe to
send to the client; Err keeps the detailed cause for logs (errors.Unwrap).

### RequestError.Error

```go
func (e *RequestError) Error() string
```

### RequestError.Unwrap

```go
func (e *RequestError) Unwrap() error
```

## httpx.Route

```go
type Route struct {
	// contains filtered or unexported fields
}
```

### httpx.PrivateRoutes

```go
func PrivateRoutes(prefix string, builders ...*RouteBuilder) []*Route
```

### httpx.PublicRoutes

```go
func PublicRoutes(prefix string, builders ...*RouteBuilder) []*Route
```

## httpx.RouteBuilder

```go
type RouteBuilder struct {
	// contains filtered or unexported fields
}
```

### httpx.DELETE

```go
func DELETE(rootPath string) *RouteBuilder
```

### httpx.GET

```go
func GET(rootPath string) *RouteBuilder
```

### httpx.PATCH

```go
func PATCH(rootPath string) *RouteBuilder
```

### httpx.POST

```go
func POST(rootPath string) *RouteBuilder
```

### httpx.PUT

```go
func PUT(rootPath string) *RouteBuilder
```

### RouteBuilder.Build

```go
func (rb *RouteBuilder) Build() *Route
```

### RouteBuilder.Cors

```go
func (rb *RouteBuilder) Cors(config *CorsConfig) *RouteBuilder
```

Cors gives the route its own CORS policy, replacing the global one;
AddHandlers panics when it is invalid (see CorsConfig.Validate).

### RouteBuilder.Timeouts

```go
func (rb *RouteBuilder) Timeouts(read, write time.Duration) *RouteBuilder
```

Timeouts overrides the server-wide read and write deadlines for this route
alone, so a service can accept a long upload without relaxing ReadTimeout
for every other route. Either argument may be zero to keep the server-wide
value for that direction.

Only the connection deadlines can be extended this way. WSConfig.RequestTimeout
is a context deadline: downstream code can shorten it, never lengthen it, so a
route that needs a long handler budget still requires a matching server-wide
RequestTimeout.

### RouteBuilder.To

```go
func (rb *RouteBuilder) To(function handlerFunc) *RouteBuilder
```

## httpx.RouteLimit

```go
type RouteLimit struct {
	Path          string
	MaxConcurrent int
	Timeout       time.Duration
}
```

RouteLimit defines per-route concurrency constraints.

## httpx.RouterHandler

```go
type RouterHandler interface {
	Handlers() []*Route
}
```

## httpx.SecurityHeadersConfig

```go
type SecurityHeadersConfig struct {
	HSTS HSTSConfig
	// TrustedProxies (CIDRs or TrustPrivateNetworks) may report HTTPS with
	// X-Forwarded-Proto; invalid entries panic.
	TrustedProxies []string
}
```

SecurityHeadersConfig configures SecurityHeadersWith.

## httpx.Signature

```go
type Signature interface {
	Sign(originalRequest *http.Request, body []byte) (*http.Request, error)
}
```

Signature signs an outgoing request; body is the exact payload sent.
AWS SigV4 lives in the netx/awssign module.

## httpx.StressControlConfig

```go
type StressControlConfig struct {
	// DefaultMaxConcurrent caps requests in flight outside RouteLimits;
	// <= 0 uses DefaultMaxConcurrent().
	DefaultMaxConcurrent int
	// DefaultTimeout is the wait for a slot; <= 0 uses DefaultStressTimeout.
	DefaultTimeout time.Duration
	// RouteLimits give path prefixes their own pool, e.g. uploads, so slow
	// routes cannot drain the default pool.
	RouteLimits []RouteLimit
	// BufferBodyBytes of each request body are read before a slot is taken,
	// so clients trickling a small body never hold a slot. Bodies above it
	// take the slot once the prefix arrived and are then bounded only by the
	// read timeouts. <= 0 uses DefaultStressBufferBytes.
	BufferBodyBytes int64
}
```

StressControlConfig holds global and per-route concurrency settings. The
limits are process-wide, shared by every client, not per client: pair them
with the rate limiter for per-client fairness.

## httpx.WSConfig

```go
type WSConfig struct {
	ServerPort string

	// AllowedOrigins sets the origins of the global CORS policy, which is
	// otherwise DefaultCORSConfig. Ignored when CORS is set.
	AllowedOrigins []string

	// CORS replaces the global CORS policy, e.g. AnyOrigin without credentials.
	CORS *CorsConfig

	// TrustedProxies lists the CIDRs of load balancers allowed to set
	// X-Forwarded-For. nil or empty trusts none: the TCP peer is the client.
	// The entry TrustPrivateNetworks ("private") expands to PrivateNetworks().
	TrustedProxies []string

	// StressControl overrides the default concurrency limits. When nil, the
	// defaults apply: DefaultMaxConcurrent() slots shared by all clients,
	// DefaultStressTimeout wait, DefaultStressBufferBytes buffered per body.
	StressControl *StressControlConfig

	// RateLimiter configures per-client rate limiting. When nil, rate limiting
	// is disabled. Build the Backend field with NewRedisBackend(client) at the
	// composition root where the Redis client is available.
	RateLimiter *RedisRateLimiterConfig

	// MaxBodyBytes caps request bodies, in bytes. A value <= 0 falls back to
	// DefaultMaxBodyBytes. Services accepting large uploads must raise this
	// cap together with ReadTimeout and WriteTimeout: the body cap is useless
	// while the transfer cannot fit in the read budget.
	MaxBodyBytes int64

	// ReadTimeout bounds reading the whole request, body included. A value
	// <= 0 falls back to DefaultReadTimeout. ReadHeaderTimeout stays fixed at
	// 5s regardless — it is what guards against Slowloris, so relaxing this
	// field for slow uploads does not weaken that defense.
	ReadTimeout time.Duration

	// WriteTimeout bounds the response write. The deadline is armed right
	// after the request headers are read, so it also covers the time spent
	// receiving the body. A value <= 0 falls back to DefaultWriteTimeout.
	WriteTimeout time.Duration

	// IdleTimeout bounds keep-alive connections between requests. A value
	// <= 0 falls back to DefaultIdleTimeout.
	IdleTimeout time.Duration

	// RequestTimeout bounds the per-request context passed to handlers. A
	// value <= 0 falls back to DefaultRequestTimeout.
	RequestTimeout time.Duration

	// ShutdownTimeout bounds graceful shutdown. A value <= 0 falls back to
	// DefaultShutdownTimeout; keep it below the pod's termination grace period.
	ShutdownTimeout time.Duration

	// DrainDelay is waited after readiness fails and before shutdown starts.
	// 0 uses DefaultDrainDelay when Health is enabled, otherwise no delay.
	DrainDelay time.Duration

	// Health enables /livez and /readyz; nil disables them.
	Health *HealthConfig

	// H2C serves HTTP/2 without TLS next to HTTP/1.1, for meshes and load
	// balancers that speak HTTP/2 to the pod. It cannot be combined with TLS.
	H2C bool

	// TLS serves HTTPS (HTTP/2 and HTTP/1.1), optionally with mTLS; nil serves
	// plain HTTP. Invalid settings make ListenAndServe fail.
	TLS *netx.TLSConfig

	// DisableCrossOriginProtection turns off the CSRF check. By default every
	// route rejects cross-origin unsafe requests (http.CrossOriginProtection,
	// by Sec-Fetch-Site/Origin) except from the origins its CORS policy
	// allows; same-origin requests and clients sending neither header pass.
	DisableCrossOriginProtection bool

	// HSTS configures Strict-Transport-Security; it is only sent over TLS or
	// behind a trusted proxy reporting X-Forwarded-Proto: https.
	HSTS HSTSConfig

	// ExposeErrorCause returns wrapped error causes in ErrorResponse.Cause
	// (RespondError). Keep false in production: causes may carry SQL or
	// driver details.
	ExposeErrorCause bool
}
```

### WSConfig.Validate

```go
func (c *WSConfig) Validate() error
```

Validate reports the settings NewServer would panic on.

