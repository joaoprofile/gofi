# HTTP Auth Middleware — função simples, cookies + Bearer, env-driven

## Regra

Middleware HTTP de autenticação é **função**, não struct. Lê config via
`environment.Instance().Auth()`. Aceita o access token por **`Authorization:
Bearer`** ou pelo cookie `access_token` — sem forkar serviço browser vs.
máquina. Cookie helpers (`setRefreshCookie`, `clearRefreshCookie`,
`readRefreshCookie`, `cookieSecure`) vivem no **mesmo arquivo** do middleware,
não duplicados no handler.

## Por quê

- **Struct + constructor inflam o tipo do bootstrap** — `main.go` passa a
  carregar `*AuthMiddleware`, `MiddlewareConfig`, `NewAuthMiddleware` e
  `AsHandler()` quando uma única função já basta. Função é mais leve, e
  alinha com a assinatura `netx.Middleware = func(http.Handler) http.Handler`.
- **`MiddlewareConfig{JWTSigningKey, JWTIssuer}` duplica `environment.Auth()`.**
  Quem builda o middleware tem que ler env e enfiar no struct — mais um
  ponto onde alguém pode chamar `os.Getenv` à mão e divergir do padrão.
- **Cookie `secure bool` passado como parâmetro a cada `setRefreshCookie`/
  `clearRefreshCookie` chamado pelo handler** vira lixo: o handler precisa
  carregar `Config{CookieSecure: ...}`, e a verdade já mora em
  `APP_ENVIRONMENT`. Centralizar em `cookieSecure()` = `!environment.
  IsLocalEnvironment()` elimina o parâmetro.
- **Suporte a cookie de access via wrapper** é gratuito: ler `access_token`
  cookie e setar `Authorization: Bearer` antes de validar permite que browser
  use cookie HttpOnly (proteção contra XSS) sem o servidor ter dois fluxos
  de extração de token.

## Padrão

### Assinatura — variante standalone (sem iamcore)

Usada quando o projeto **não tem** `iamcore.IAMService` wired (Fase inicial,
ou contexto que prefere parse local).

```go
// Função, devolve netx.Middleware. Constrói uma vez (fora do closure)
// a AuthConfig lida do environment singleton.
func AuthMiddleware(sessionRepo repository.SessionRepository) netx.Middleware {
    cfg := environment.Instance().Auth()
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            promoteCookieToBearer(r)
            claims, err := authenticate(r, cfg, sessionRepo)
            if err != nil {
                netx.Error(w, http.StatusUnauthorized, err)
                return
            }
            ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
            // Demais valores de contexto cross-domain (ex: auth.ACCOUNT_AUTH)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Assinatura — variante delegada ao SDK (preferida quando há `iamcore.IAMService`)

Quando o projeto constrói `iamSvc := iam.New(...)` com adapters de
`SessionPort` e `TokenPort`, o middleware **delega** validação ao SDK e
ainda assim **preserva o `*model.Claims` rico** com um passo de
enriquecimento (re-parse). Use quando o JWT carrega custom claims
(`name`/`<TenantID>`/`<ParentID>`/role) que `iam/types.Claims` não modela.

```go
func AuthMiddleware(iamSvc *iamcore.IAMService) netx.Middleware {
    cfg := environment.Instance().Auth()
    sdkInner := iammw.AuthMiddleware(iamSvc) // valida assinatura + sessão via SDK

    enrich := func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token, err := extractBearer(r)
            if err != nil { netx.Error(w, http.StatusUnauthorized, err); return }
            claims, err := parseRichClaims(token, cfg) // mesma chave; só desserialização
            if err != nil { netx.Error(w, http.StatusUnauthorized, err); return }
            ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
            ctx = context.WithValue(ctx, auth.ACCOUNT_AUTH, claimsToAccountAuth(claims))
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }

    return func(next http.Handler) http.Handler {
        chained := sdkInner(enrich(next))
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            promoteCookieToBearer(r) // antes do SDK — o cookie vira Bearer header
            chained.ServeHTTP(w, r)
        })
    }
}
```

**Pipeline real**: `promoteCookieToBearer → sdkInner (iamSvc.ValidateToken: signature+session+revocation) → enrich (re-parse para *model.Claims rico) → next handler`.

**Por que duas passadas no token?** O SDK só conhece `iam/types.Claims`
(minimal). Como `iam/provider/jwt.iamClaims` é struct fechado sem campo
`Extra`, a desserialização do SDK descarta `name`/`companyId`/`managerId`/
`role` silenciosamente. Reparsear com a mesma chave (HMAC verify trivial)
mantém o `*model.Claims` rico sem alterar o SDK. Migração para emissão
via SDK (Fase 2b) exige PR ao gofi-sdk-go estendendo `iamClaims` com
custom claims.

### Wrapper cookie → Bearer

```go
const cookieAccessToken = "access_token"

func promoteCookieToBearer(r *http.Request) {
    if r.Header.Get("Authorization") != "" {
        return // header tem prioridade — cliente máquina não é afetado
    }
    c, err := r.Cookie(cookieAccessToken)
    if err != nil || c.Value == "" {
        return
    }
    r.Header.Set("Authorization", "Bearer "+c.Value)
}
```

### Cookie helpers — sem `secure bool`

```go
const (
    cookieRefreshToken = "<refresh-cookie-name>"
    refreshCookiePath  = "<refresh-cookie-path>" // ex: "/v1/auth/refresh"
)

// cookieSecure devolve false em dev/test (browser aceita sobre http://localhost)
// e true em stage/prod. Lê APP_ENVIRONMENT via o singleton — nunca os.Getenv.
func cookieSecure() bool { return !environment.IsLocalEnvironment() }

func setRefreshCookie(w http.ResponseWriter, value string, expiresAt time.Time) {
    http.SetCookie(w, &http.Cookie{
        Name:     cookieRefreshToken,
        Value:    value,
        Path:     refreshCookiePath,
        Expires:  expiresAt,
        MaxAge:   maxAgeUntil(expiresAt),
        HttpOnly: true,
        Secure:   cookieSecure(),
        SameSite: http.SameSiteStrictMode,
    })
}

func clearRefreshCookie(w http.ResponseWriter) {
    http.SetCookie(w, &http.Cookie{
        Name:     cookieRefreshToken,
        Value:    "",
        Path:     refreshCookiePath,
        MaxAge:   -1,
        HttpOnly: true,
        Secure:   cookieSecure(),
        SameSite: http.SameSiteStrictMode,
    })
}

func readRefreshCookie(r *http.Request) string {
    c, err := r.Cookie(cookieRefreshToken)
    if err != nil {
        return ""
    }
    return c.Value
}

func maxAgeUntil(t time.Time) int {
    secs := int(time.Until(t).Seconds())
    if secs < 1 {
        return 1
    }
    return secs
}
```

### Claims do contexto

Continua devolvendo `*model.Claims` rico (com campos de domínio — `Name`,
`<TenantID>`, `<ParentID>` etc.), **não** o `*types.Claims` minimal do SDK.
Custom claims do domínio vivem no JWT emitido pelo `auth_service`:

```go
func ClaimsFromContext(r *http.Request) (*model.Claims, bool) {
    c, ok := r.Context().Value(claimsCtxKey).(*model.Claims)
    return c, ok
}
```

### Revogação de sessão

Permanece no middleware — `sessionRepo.Get(ctx, sessionID)` + check em
`session.RevokedAt != nil`. Mantém o controle local enquanto não há
`iamcore.IAMService` wired no projeto.

## Wiring no `main.go` / `auth.go`

```go
// Apenas o sessionRepo é necessário. JWT_SECRET/JWT_ISSUER vêm do env.
func buildAuthMiddleware(sessionRepo authRepoPkg.SessionRepository) netx.Middleware {
    return authHandlerPkg.AuthMiddleware(sessionRepo)
}

// Handler também perde a referência ao middleware — handlers nunca dependem
// do AuthMiddleware, só do contexto que ele injeta.
func buildAuthHandler(cfg Config, svc authSvcPkg.AuthService) *authHandlerPkg.AuthHandler {
    return authHandlerPkg.NewAuthHandler(svc, authHandlerPkg.Config{ /* sem CookieSecure */ })
}

// main.go usa direto, sem .AsHandler():
authMw := buildAuthMiddleware(authRepos.session)
api.HttpServer().UseAuth(authMw)
```

## Anti-padrões

```go
// ❌ Struct + constructor + MiddlewareConfig redundante com environment.Auth()
type AuthMiddleware struct {
    cfg         MiddlewareConfig
    sessionRepo repository.SessionRepository
}
func NewAuthMiddleware(cfg MiddlewareConfig, sessionRepo repository.SessionRepository) *AuthMiddleware { ... }
func (m *AuthMiddleware) AsHandler() netx.Middleware { ... }

// ❌ Handler tem ponteiro para o middleware
type AuthHandler struct { svc Service; mw *AuthMiddleware; cfg Config }

// ❌ Cookie helper recebe `secure bool` — flag duplica APP_ENVIRONMENT
func setRefreshCookie(w http.ResponseWriter, value string, expiresAt time.Time, secure bool)
func clearRefreshCookie(w http.ResponseWriter, secure bool)

// ❌ Config do handler carrega CookieSecure replicando env
type Config struct { CookieSecure bool; ... }

// ❌ Handler lê cookie direto em vez de usar readRefreshCookie helper
cookie, err := r.Cookie("iam_rt")
if err != nil || cookie.Value == "" { ... }

// ❌ os.Getenv em qualquer ponto do middleware ou wiring (sempre environment.Instance())
secret := os.Getenv("JWT_SECRET")
```

## Checklist

- [ ] `AuthMiddleware(repos…) netx.Middleware` — função, não struct
- [ ] `cfg := environment.Instance().Auth()` capturado **fora** do closure
- [ ] `promoteCookieToBearer(r)` antes da extração — header tem prioridade
- [ ] `cookieSecure()` interno; helpers de cookie **sem** parâmetro `secure bool`
- [ ] Cookie helpers no mesmo arquivo do middleware (não duplicar no handler)
- [ ] Revogação preservada (`sessionRepo.Get` + check `RevokedAt`) enquanto não há iamcore wired
- [ ] `ClaimsFromContext` continua devolvendo `*model.Claims` rico
- [ ] Handler **não tem** field para o middleware; só consome contexto
- [ ] `Config` do handler **não tem** `CookieSecure`
- [ ] `main.go` usa `api.HttpServer().UseAuth(authMw)` direto, sem `.AsHandler()`
- [ ] Zero `os.Getenv` no middleware e no wiring
