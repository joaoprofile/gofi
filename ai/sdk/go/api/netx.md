# netx

`import "github.com/joaoprofile/gofi-sdk-go/netx"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

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
const DefaultMaxBodyBytes int64 = 10 << 20 // 10 MB
```

DefaultMaxBodyBytes is the request body cap applied when no explicit limit
is configured.

```go
const ErrMsgOnQueryParameter = "error on parsing query parameters %w"
```

```go
const RequestIDKey contextKey = "request_id"
```

## Variáveis

```go
var (
	ErrUnsupportedBodyType = errors.New("unsupported body type")
	ErrTooManyRequests     = errors.New("too many requests")
	ErrResponseIsNil       = errors.New("response is nil")
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
var ExposeErrorCause = false
```

ExposeErrorCause returns wrapped error causes in ErrorResponse.Cause.
Keep false in production: causes may carry SQL or driver details.

```go
var PrivateNetworks = []string{
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10",
	"127.0.0.0/8", "169.254.0.0/16", "::1/128", "fc00::/7", "fe80::/10",
}
```

PrivateNetworks are trusted as proxies when WSConfig.TrustedProxies is nil:
in-cluster ingresses and cloud load balancers reach the pod from these ranges.

## Funções

### netx.BindQueryParamsToStruct

```go
func BindQueryParamsToStruct(r *http.Request, w http.ResponseWriter, tStruct any) error
```

BindQueryParamsToStruct populates tStruct from the URL query parameters.
Each exported field is matched by its `form` tag, falling back to the lowercased
field name. tStruct must be a non-nil pointer to a struct.

### netx.BlockUnsafeMethods

```go
func BlockUnsafeMethods(next http.Handler) http.Handler
```

BlockUnsafeMethods rejects TRACE (information disclosure) and CONNECT
(proxy tunnel abuse) before they reach application handlers.

### netx.Error

```go
func Error(w http.ResponseWriter, statusCode int, err error)
```

### netx.ErrorDetails

```go
func ErrorDetails(w http.ResponseWriter, statusCode int, message string, details any)
```

### netx.GetPathParam

```go
func GetPathParam(param string, r *http.Request) string
```

GetPathParam returns the value of the named URL path parameter, unchanged.

### netx.GetQueryParam

```go
func GetQueryParam(filter string, r *http.Request) string
```

Query / path parameters

GetQueryParam returns the value of the named URL query parameter, unchanged.

### netx.GetRequestID

```go
func GetRequestID(ctx context.Context) string
```

GetRequestID returns the request ID stored in the context by LoggingMiddleware.

### netx.JSON

```go
func JSON(w http.ResponseWriter, statusCode int, jsonData []byte)
```

### netx.LimitBody

```go
func LimitBody(next http.Handler) http.Handler
```

LimitBody caps request bodies at DefaultMaxBodyBytes to prevent memory
exhaustion attacks. Kept for callers that do not need a custom cap.

### netx.LogIPBlocked

```go
func LogIPBlocked(r *http.Request, clientName string)
```

LogIPBlocked logs a blocked IP attempt at Warn level.

### netx.LogInvalidAPIKey

```go
func LogInvalidAPIKey(r *http.Request)
```

LogInvalidAPIKey logs an invalid API key attempt at Warn level.

### netx.LogRateLimit

```go
func LogRateLimit(r *http.Request)
```

LogRateLimit logs a rate-limit event at Warn level with trace context.

### netx.LogServerError

```go
func LogServerError(r *http.Request, err error)
```

LogServerError logs an application error at Error level with trace context.

### netx.ParseRequestBody

```go
func ParseRequestBody(_ http.ResponseWriter, r *http.Request, tStruct any) error
```

ParseRequestBody decodes the JSON request body into tStruct.
tStruct must be a non-nil pointer to a struct; otherwise ErrInvalidStruct is returned.

### netx.ReadBody

```go
func ReadBody(r *http.Request) ([]byte, error)
```

Body parsing

ReadBody reads and returns the full request body.
The body is always closed, even when the read fails.

### netx.RespondError

```go
func RespondError(w http.ResponseWriter, r *http.Request, appErr errs.AppError)
```

### netx.Response

```go
func Response(w http.ResponseWriter, statusCode int, data any)
```

### netx.SecurityHeaders

```go
func SecurityHeaders(next http.Handler) http.Handler
```

### netx.ValidateRequest

```go
func ValidateRequest(next http.Handler) http.Handler
```

ValidateRequest rejects requests that carry both Content-Length and
Transfer-Encoding, a classic HTTP request-smuggling vector.

## netx.CorsConfig

```go
type CorsConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           string
}
```

### netx.DefaultCORSConfig

```go
func DefaultCORSConfig() CorsConfig
```

## netx.ErrorResponse

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

## netx.HandlerRegistry

```go
type HandlerRegistry struct {
	// contains filtered or unexported fields
}
```

### netx.NewHandlerRegistry

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

## netx.HealthConfig

```go
type HealthConfig struct {
	LivenessPath  string        // default "/livez"
	ReadinessPath string        // default "/readyz"
	CheckTimeout  time.Duration // default 2s for all readiness checks
}
```

HealthConfig enables liveness and readiness endpoints. They are served ahead
of every middleware, so probes never hit auth, rate limiting or logging.

## netx.HttpClient

```go
type HttpClient struct {
	Name              string
	BaseURL           string
	Retries           uint8
	RetrySleep        time.Duration
	DisableRetryOn429 bool
	Client            *http.Client
	RateLimit         int
	// contains filtered or unexported fields
}
```

### netx.NewClient

```go
func NewClient(config *HttpClientConfig) (*HttpClient, error)
```

## netx.HttpClientConfig

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
}
```

## netx.HttpError

```go
type HttpError struct {
	Message string   `json:"message"`
	Err     string   `json:"error"`
	Status  int      `json:"status"`
	Cause   []string `json:"cause"`
}
```

### netx.FromError

```go
func FromError(err error) *HttpError
```

### netx.NewAPIError

```go
func NewAPIError(status int, message string, err error) *HttpError
```

### netx.ParseError

```go
func ParseError(jsonStr string) (*HttpError, error)
```

### HttpError.Error

```go
func (e *HttpError) Error() string
```

## netx.HttpServer

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

### netx.NewServer

```go
func NewServer(config *WSConfig) HttpServer
```

## netx.Middleware

```go
type Middleware func(http.Handler) http.Handler
```

### netx.CORSMiddleware

```go
func CORSMiddleware(config CorsConfig) Middleware
```

### netx.ClientIPMiddleware

```go
func ClientIPMiddleware(trustedProxies []string) Middleware
```

ClientIPMiddleware resolves the client IP once per request. X-Forwarded-For is
honoured only when the TCP peer is a trusted proxy: nil trusts PrivateNetworks,
an empty slice trusts none. Invalid CIDRs panic.

### netx.LimitBodyWithMax

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

### netx.LoggingMiddleware

```go
func LoggingMiddleware() Middleware
```

LoggingMiddleware logs only errors and security-relevant responses.
2xx and 3xx responses are intentionally silent — logs are persisted and
request-level noise would dominate the cost and signal-to-noise ratio.

Logged conditions:
  - 5xx  → Error  (server fault, always logged)
  - 401/403/429 → Warn  (access denied, rate limited — security signal)

### netx.NewRedisRateLimiter

```go
func NewRedisRateLimiter(cfg RedisRateLimiterConfig) Middleware
```

NewRedisRateLimiter returns a Middleware that enforces per-client rate limits
using a sliding-window algorithm backed by the provided RateLimiterBackend.

Standard rate-limit response headers (X-RateLimit-Limit, X-RateLimit-Remaining,
X-RateLimit-Reset) are written on every response. Retry-After is added only
when the limit is exceeded.

### netx.NewStressControlMiddleware

```go
func NewStressControlMiddleware(cfg StressControlConfig) Middleware
```

NewStressControlMiddleware limits the number of concurrent requests served
simultaneously. Route-specific limits are checked first by prefix; all
other requests fall through to the default limiter.

## netx.Module

```go
type Module interface {
	Register(reg *HandlerRegistry)
}
```

## netx.RateLimitPlan

```go
type RateLimitPlan struct {
	Rate  int // requests per second
	Burst int // maximum burst size
}
```

RateLimitPlan defines the allowed request rate and burst for a named client plan.

## netx.RateLimiterBackend

```go
type RateLimiterBackend interface {
	Allow(ctx context.Context, key string, limit redis_rate.Limit) (*redis_rate.Result, error)
}
```

RateLimiterBackend abstracts the rate-limiting backend, decoupling the
middleware from a concrete Redis client and making it fully testable without
a live Redis instance.

### netx.NewRedisBackend

```go
func NewRedisBackend(client redis.UniversalClient) RateLimiterBackend
```

NewRedisBackend wraps a redis.UniversalClient into a RateLimiterBackend using
a sliding-window algorithm. Call this at the composition root where the Redis
client is already wired.

## netx.RedisRateLimiterConfig

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
}
```

RedisRateLimiterConfig holds the configuration for the rate-limiter middleware.
Backend must be provided by the caller — Redis is never created internally.

## netx.Request

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

### netx.NewRequest

```go
func NewRequest[T any](ctx context.Context, client *HttpClient, method, path string) *Request[T]
```

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

## netx.RequestBody

```go
type RequestBody any
```

## netx.RequestBodyResult

```go
type RequestBodyResult struct {
	Reader   io.Reader
	JSONData []byte
	Err      error
}
```

## netx.Route

```go
type Route struct {
	// contains filtered or unexported fields
}
```

### netx.PrivateRoutes

```go
func PrivateRoutes(prefix string, builders ...*RouteBuilder) []*Route
```

### netx.PublicRoutes

```go
func PublicRoutes(prefix string, builders ...*RouteBuilder) []*Route
```

## netx.RouteBuilder

```go
type RouteBuilder struct {
	// contains filtered or unexported fields
}
```

### netx.DELETE

```go
func DELETE(rootPath string) *RouteBuilder
```

### netx.GET

```go
func GET(rootPath string) *RouteBuilder
```

### netx.PATCH

```go
func PATCH(rootPath string) *RouteBuilder
```

### netx.POST

```go
func POST(rootPath string) *RouteBuilder
```

### netx.PUT

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

## netx.RouteLimit

```go
type RouteLimit struct {
	Path          string
	MaxConcurrent int
	Timeout       time.Duration
}
```

RouteLimit defines per-route concurrency constraints.

## netx.RouterHandler

```go
type RouterHandler interface {
	Handlers() []*Route
}
```

## netx.Signature

```go
type Signature interface {
	Sign(originalRequest *http.Request, body []byte) (*http.Request, error)
}
```

Signature signs an outgoing request; body is the exact payload sent.
AWS SigV4 lives in the netx/awssign module.

## netx.StressControlConfig

```go
type StressControlConfig struct {
	DefaultMaxConcurrent int
	DefaultTimeout       time.Duration
	RouteLimits          []RouteLimit
}
```

StressControlConfig holds global and per-route concurrency settings.

## netx.WSConfig

```go
type WSConfig struct {
	ServerPort     string
	AllowedOrigins []string

	// TrustedProxies lists the CIDRs of load balancers allowed to set
	// X-Forwarded-For. nil trusts PrivateNetworks; an empty slice trusts none.
	TrustedProxies []string

	// StressControl overrides the default concurrency limits. When nil,
	// sensible defaults (50 concurrent, 20 ms timeout) are applied.
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
	// balancers that speak HTTP/2 to the pod.
	H2C bool

	// CrossOriginProtection rejects cross-origin unsafe requests (CSRF) using
	// Sec-Fetch-Site/Origin; AllowedOrigins without wildcards stay trusted.
	CrossOriginProtection bool
}
```

