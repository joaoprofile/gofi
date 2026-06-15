# gofi/iam — Autenticação e Autorização

## Setup Básico

```go
import "github.com/joaoprofile/gofi/iam"

// Config completa
iamSvc, err := iam.New(iam.Config{
    Security: iam.SecurityConfig{
        JWTSecret:       os.Getenv("JWT_SECRET"),
        TokenExpiry:     15 * time.Minute,
        RefreshExpiry:   7 * 24 * time.Hour,
    },
    User:    userRepo,    // port.UserPort
    Token:   tokenRepo,   // port.TokenPort
    Session: sessionRepo, // port.SessionPort
    Tenant:  tenantRepo,  // port.TenantPort (opcional)
    RBAC:    rbacRepo,    // port.RBACPort (opcional)
})

// Config default — para projetos simples
iamSvc, err := iam.New(iam.Config{
    Security: iam.DefaultConfig{
        JWTSecret: os.Getenv("JWT_SECRET"),
    },
})
```

## IDPs Sociais

```go
import (
    "github.com/joaoprofile/gofi/iam"
    googleidp "github.com/joaoprofile/gofi/iam/provider/google"
    msidp "github.com/joaoprofile/gofi/iam/provider/microsoft"
    oidcidp "github.com/joaoprofile/gofi/iam/provider/oidc"
)

iamSvc, err := iam.New(iam.Config{
    IDPs: map[string]iam.IDPConfig{
        "google":    googleidp.New(googleidp.Config{...}),
        "microsoft": msidp.New(msidp.Config{...}),
        "keycloak":  oidcidp.New(oidcidp.Config{...}),
    },
    // ...
})
```

## Middlewares HTTP

```go
import "github.com/joaoprofile/gofi/iam/middleware"

// Autenticação — extrai e valida Bearer token, injeta claims no context
router.Use(middleware.AuthMiddleware(iamSvc))

// RBAC — verifica permissão resource:action
router.Use(middleware.RBACMiddleware(iamSvc, "persons", "read"))

// Multi-tenancy — verifica acesso ao tenant das claims
router.Use(middleware.TenantMiddleware(iamSvc))
```

### Cadeia típica

```go
router.Group("/v1/persons",
    middleware.AuthMiddleware(iamSvc),
    middleware.TenantMiddleware(iamSvc),
).Get("/", handler.List)
```

## Claims no Handler

```go
import "github.com/joaoprofile/gofi/iam/middleware"

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
    claims := middleware.ClaimsFromContext(r.Context())
    if claims == nil {
        netx.RespondError(w, ErrUnauthorized.New())
        return
    }
    // claims.UserID, claims.TenantID, claims.Module, claims.Roles
}
```

## Tipos de Claims

```go
type Claims struct {
    UserID   string
    TenantID string
    Module   string
    Roles    []string
    // ...
}
```

## Sessão

O IAM suporta dois backends de sessão:
- **Memory** — para desenvolvimento/testes
- **Redis** — para produção com revogação real

```go
import (
    redisprovider "github.com/joaoprofile/gofi/iam/provider/redis"
    memprovider "github.com/joaoprofile/gofi/iam/provider/memory"
)

// Produção
session := redisprovider.NewSession(redisClient)

// Desenvolvimento
session := memprovider.NewSession()
```
