# iam/middleware

`import "github.com/gofi-labs/gofi-sdk-go/iam/middleware"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Funções

### middleware.AuthInterceptor

```go
func AuthInterceptor(svc *core.IAMService) grpc.UnaryServerInterceptor
```

AuthInterceptor is the gRPC equivalent of AuthMiddleware for unary RPCs.
Validates the token from the "authorization: Bearer" metadata entry.
Injects the claims into the context for use in gRPC handlers.

### middleware.AuthMiddleware

```go
func AuthMiddleware(svc *core.IAMService) func(http.Handler) http.Handler
```

AuthMiddleware extracts and validates the Bearer token.
On success, injects the validated claims into the context.
On failure, responds with 401 and terminates the handler chain.

### middleware.AuthStreamInterceptor

```go
func AuthStreamInterceptor(svc *core.IAMService) grpc.StreamServerInterceptor
```

AuthStreamInterceptor is the gRPC equivalent of AuthMiddleware for streaming RPCs.

### middleware.ClaimsFromContext

```go
func ClaimsFromContext(ctx context.Context) *types.Claims
```

ClaimsFromContext extracts the claims injected by AuthMiddleware.
Returns nil if not found, meaning the request did not pass through AuthMiddleware.

### middleware.RBACMiddleware

```go
func RBACMiddleware(svc *core.IAMService, resource, action string) func(http.Handler) http.Handler
```

RBACMiddleware checks whether the claims authorize the given resource and action.
Must be chained after AuthMiddleware. Responds with 403 on failure.

### middleware.TenantMiddleware

```go
func TenantMiddleware(svc *core.IAMService) func(http.Handler) http.Handler
```

TenantMiddleware verifies in the TenantPort whether the user still has access to the tenant in the claims.
Ensures that access removal takes immediate effect without relying on token expiry.
Must be chained after AuthMiddleware.

