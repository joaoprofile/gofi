---
name: http-auth-middleware
description: Middleware de autenticação HTTP sobre o iam — função netx.Middleware, Bearer ou cookie de sessão, sempre IAMService.ValidateToken, claims com chave própria
sdk: v0.8.2
keywords: [auth, middleware, UseAuth, ValidateToken, Bearer, cookie, sessão, BFF, vault, RefreshToken, ClaimsFromContext, CrossOriginProtection, 401]
---

# HTTP Auth Middleware — função, Bearer ou cookie de sessão, `ValidateToken` sempre

Setup do serviço iam, login, refresh e logout: `iam.md`. Gate de permissão:
`rbac.md`. Programa completo de referência: exemplo `examples/iam/login`
(`auth.go`) — `.claude/sdk/go/api/examples.md`.

## Regra

1. Middleware de autenticação é **função** que devolve `netx.Middleware`,
   fechada sobre o `*core.IAMService`. Nada de struct + construtor + `AsHandler()`.
2. Registrado com `UseAuth(...)` do componente `httpserver` — roda **só** nas
   rotas declaradas com `netx.PrivateRoutes`. **`UseAuth` antes de `Handlers`**
   (ver `http-server.md` §"Ordem de registro").
3. Toda credencial termina em **`svc.ValidateToken(ctx, token)`**: assinatura +
   expiração + sessão ativa no `SessionPort`. É isso que torna o logout
   imediato. Nunca parse o JWT à mão no middleware.
4. Aceita `Authorization: Bearer <jwt>` (apps, CLIs, serviço-a-serviço) **ou**
   cookie de sessão opaco (browser/BFF). Header tem prioridade.
5. Falha → `netx.Error(w, http.StatusUnauthorized, errUnauthenticated)` (JSON
   no mesmo formato dos demais erros), mensagem genérica — nunca o motivo.
6. Claims vão para o contexto com **chave do próprio pacote**; o mesmo arquivo
   exporta `ClaimsFromContext` e `WithClaims` (este último serve aos testes).

## Por quê

- **`iammw.AuthMiddleware` (`iam/middleware`) responde 401 em `text/plain`**
  (`http.Error`), aceita só Bearer e guarda as claims numa chave privada —
  handler test não consegue injetar claims sem montar um `IAMService` real.
  Serve a um resource server puro; para API com browser, escreva a função.
- **`ValidateToken` checa a sessão**, não só a assinatura: token de sessão
  revogada (logout, logout-all, rotação de refresh) é rejeitado antes de expirar.
- **Claims de domínio já vêm no token**: `types.Claims.Extra` (JSON `ext`) faz
  round-trip Issue → Parse. Não existe mais motivo para re-parse do JWT com
  struct própria (ver `iam.md` §"Claims de domínio").
- **JavaScript nunca vê token no modo sessão**: o cookie `HttpOnly` carrega só
  um id aleatório; os tokens ficam no servidor. XSS não rouba credencial.

## Padrão

```go
// handler/middleware.go of the context that manages authentication
// (imported elsewhere as authhandler "<module>/domain/{contexto-auth}/handler")
package handler

import (
    "context"
    "errors"
    "net/http"
    "strings"

    "github.com/joaoprofile/gofi-sdk-go/iam/core"
    "github.com/joaoprofile/gofi-sdk-go/iam/types"
    "github.com/joaoprofile/gofi-sdk-go/netx"
)

const sessionCookie = "sid"

var errUnauthenticated = errors.New("not authenticated")

type claimsKey struct{}

// Middleware guards netx.PrivateRoutes; register it with UseAuth before Handlers.
func Middleware(svc *core.IAMService, v *Vault) netx.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, err := authenticate(r, svc, v)
            if err != nil {
                netx.Error(w, http.StatusUnauthorized, errUnauthenticated)
                return
            }
            next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
        })
    }
}

func authenticate(r *http.Request, svc *core.IAMService, v *Vault) (*types.Claims, error) {
    if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
        return svc.ValidateToken(r.Context(), token)
    }
    if c, err := r.Cookie(sessionCookie); err == nil && v != nil {
        return v.Claims(r.Context(), svc, c.Value)
    }
    return nil, errUnauthenticated
}

func WithClaims(ctx context.Context, c *types.Claims) context.Context {
    return context.WithValue(ctx, claimsKey{}, c)
}

// ClaimsFromContext returns nil outside a private route.
func ClaimsFromContext(ctx context.Context) *types.Claims {
    c, _ := ctx.Value(claimsKey{}).(*types.Claims)
    return c
}
```

### Modo sessão (BFF) — vault + renovação transparente

O login em modo sessão guarda o `*types.Session` devolvido por `SelectTenant`
num vault do servidor, sob id aleatório de 32 bytes, e manda só o id no cookie.
A cada request:

```go
func (v *Vault) Claims(ctx context.Context, svc *core.IAMService, id string) (*types.Claims, error) {
    s, ok := v.get(id)
    if !ok {
        return nil, errUnauthenticated
    }
    claims, err := svc.ValidateToken(ctx, s.AccessToken)
    if !errors.Is(err, core.ErrTokenExpired) {
        return claims, err
    }
    // Access expirado: renova com o refresh guardado — uma vez só por id.
    s, err = v.renew(id, s, func(old *types.Session) (*types.Session, error) {
        return svc.RefreshToken(ctx, old.RefreshToken)
    })
    if err != nil {
        return nil, err
    }
    return svc.ValidateToken(ctx, s.AccessToken)
}
```

- **`renew` serializa por id** (mutex; com várias instâncias, lock distribuído —
  `session-store.md`): requests concorrentes com o mesmo token expirado
  reaproveitam o resultado. Refresh token usado duas vezes é tratado pelo iam
  como **roubo** — revoga todas as sessões do usuário.
- **Vault compartilhado entre instâncias** (Redis) em produção; em memória só
  com uma réplica. `types.Session.RefreshToken` tem tag `json:"-"`: ao
  serializar o vault, grave `AccessToken`/`RefreshToken` em campos próprios.
- Implementação completa (put/get/delete/renew): `examples/iam/login/auth.go`.

### Cookie helpers — no mesmo arquivo, sem `secure bool`

```go
// cookieSecure is false only in local/test environments (http://localhost).
func cookieSecure() bool { return !environment.IsLocalEnvironment() }

func setSessionCookie(w http.ResponseWriter, id string, expiresAt time.Time) {
    http.SetCookie(w, &http.Cookie{
        Name: sessionCookie, Value: id, Path: "/", Expires: expiresAt,
        HttpOnly: true, Secure: cookieSecure(), SameSite: http.SameSiteLaxMode,
    })
}

func clearSessionCookie(w http.ResponseWriter) {
    http.SetCookie(w, &http.Cookie{
        Name: sessionCookie, Path: "/", MaxAge: -1,
        HttpOnly: true, Secure: cookieSecure(), SameSite: http.SameSiteLaxMode,
    })
}
```

- Cookie de sessão **exige** `netx.WSConfig{CrossOriginProtection: true}` no
  servidor (rejeita POST cross-site por `Sec-Fetch-Site`/`Origin` — CSRF).
- Os campos `Cookie*` de `iamconfig.SecurityConfig` são só defaults
  declarativos: o SDK **não** escreve cookie nenhum. Quem seta é o handler.

### Revogação de acesso ao tenant por request (opcional)

Quando remover acesso precisa valer antes do token expirar, acrescente após
`ValidateToken`:

```go
if err := svc.Tenant().AssertAccess(r.Context(), claims.UserID, claims.TenantID, claims.Module); err != nil {
    netx.Error(w, http.StatusForbidden, errForbidden)
    return
}
```

(É o que `iammw.TenantMiddleware` faz — mas ele lê a chave de claims do
`iammw`, não a sua.) Custa uma consulta ao `TenantPort` por request.

## Wiring

```go
identity := iamc.New(iamc.Config{User: users, Tenant: users, RBAC: rbac}) // gofi/component/iam
server := httpserver.New(":8080", &netx.WSConfig{CrossOriginProtection: true})

svc, err := gofi.New("<service>").With(identity, server).Build()
if err != nil {
    log.Fatal(err)
}

// identity.Service() é nil antes do Build: auth e rotas entram depois dele.
vault := authhandler.NewVault()
server.UseAuth(authhandler.Middleware(identity.Service(), vault)). // antes de Handlers
    Handlers(authHandler, entityHandler)

if err := svc.ListenAndServe(); err != nil {
    log.Fatal(err)
}
```

Handlers **não** recebem o middleware; só leem `authhandler.ClaimsFromContext(r.Context())`.

## Anti-padrões

```go
// ❌ Handlers antes de UseAuth — as rotas privadas já registradas ficam SEM auth
server.Handlers(h).UseAuth(mw)

// ❌ Rota que exige login declarada com PublicRoutes
netx.PublicRoutes("/v1", netx.GET("/me").To(h.me))

// ❌ Parse manual do JWT no middleware (pula a checagem de sessão revogada)
tok, _ := jwt.Parse(raw, keyFunc)

// ❌ Re-parse do token para recuperar claims de domínio — use Claims.Extra

// ❌ Token (access ou refresh) em cookie legível por JS ou em localStorage

// ❌ Resposta revelando o motivo ("token expired", "session revoked") — 401 genérico

// ❌ Struct AuthMiddleware + NewAuthMiddleware + AsHandler(); ponteiro do middleware no handler

// ❌ os.Getenv no middleware; Secure do cookie vindo de parâmetro/Config do handler
```

## Checklist

- [ ] `Middleware(svc, …) netx.Middleware` — função, fechada sobre `*core.IAMService`
- [ ] Toda credencial passa por `svc.ValidateToken`; Bearer tem prioridade sobre cookie
- [ ] 401 via `netx.Error`, mensagem genérica
- [ ] `ClaimsFromContext` + `WithClaims` com chave própria, no mesmo arquivo
- [ ] Modo sessão: cookie `HttpOnly` só com id opaco; renovação serializada por id; vault compartilhado com >1 réplica
- [ ] `CrossOriginProtection: true` quando há cookie de sessão
- [ ] `cookieSecure()` interno (`!environment.IsLocalEnvironment()`)
- [ ] `UseAuth` depois do `Build` e **antes** de `Handlers`
- [ ] Rotas autenticadas em `netx.PrivateRoutes`
