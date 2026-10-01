---
name: iam
description: Módulo iam de ponta a ponta — montagem pelo componente, ports do projeto, login em dois passos, refresh com rotação, logout real, claims de domínio, hash de senha, auditoria
sdk: v0.8.2
keywords: [iam, login, Authenticate, SelectTenant, RefreshToken, Logout, LogoutAll, JWT_SECRET, UserPort, TenantPort, password, Argon2id, bcrypt, Extra, OnEvent, componente iam]
---

# IAM — autenticação com sessão real (`iam` + `gofi/component/iam`)

Referência: `.claude/sdk/go/api/iam*.md` e `gofi-component-iam.md`. Programa
completo (token e sessão, refresh, logout, RBAC): `examples/iam/login`.
Middleware HTTP: `http-auth-middleware.md`. Autorização: `rbac.md`. Login
social: `iam-social-login.md`. Ports sobre storage existente: `iam-adapter-pattern.md`.

## Modelo em uma tela

- Todo access token (JWT) é amarrado a uma **sessão** no `SessionPort`
  (`Claims.SessionID`). `ValidateToken` checa assinatura **e** sessão ativa →
  logout vale na hora, sem esperar o token expirar.
- Login em **dois passos**: `Authenticate` (senha → tenants + ticket, **sem**
  token) e `SelectTenant` (abre a sessão, emite access + refresh).
- Refresh com **rotação obrigatória**: cada refresh token vale uma vez.
  Reapresentar um já usado = suspeita de roubo → revoga **todas** as sessões
  do usuário (`types.EventSuspiciousActivity`).
- O projeto implementa **só** o que o SDK não sabe: `port.UserPort`,
  `port.TenantPort` e a matriz do `port.RBACPort`. JWT, sessão e rotação são do SDK.

## Montagem — componente do `gofi`

```go
import (
    iamc "github.com/joaoprofile/gofi-sdk-go/gofi/component/iam"
    iamconfig "github.com/joaoprofile/gofi-sdk-go/iam/config"
    "github.com/joaoprofile/gofi-sdk-go/iam/provider/rbac/roles"
)

identity := iamc.New(iamc.Config{
    User:    users,          // port.UserPort   — adapter sobre o repository
    Tenant:  users,          // port.TenantPort — tenants, módulos e roles do usuário
    RBAC:    roles.NewRBACProvider(roles.Config{Permissions: permissions}),
    OnEvent: auditEvent,     // trilha de auditoria
    Configure: func(c *iamconfig.DefaultConfig) {
        c.Security.RequireTenantTicket = true // SelectTenant só com o ticket do Authenticate
    },
})
svc, err := gofi.New("<service>").With(identity, server).Build()
// identity.Service() (*core.IAMService) só existe depois do Build.
```

Ambiente lido pelo componente (`config.IAM(env)`):

| Variável | Efeito |
|---|---|
| `JWT_SECRET` | HS256, **obrigatório**, ≥ 32 bytes — sem ele o `Build` falha |
| `JWT_ISSUER` | `iss` (default `gofi/iam`) |
| `ACCESS_TOKEN_TTL` | default 15m, **máx. 60m** (acima → `core.ErrAccessTokenTTLExceeded`) |
| `REFRESH_TOKEN_TTL` | duração da sessão; default 7 dias, **máx. 90 dias** |
| `JWT_KEY_ID`, `JWT_PREVIOUS_KEY_ID`, `JWT_PREVIOUS_SECRET` | rotação de chave: tokens da chave anterior valem até expirar |
| `CACHE_TYPE=redis` + `CACHE_*` | sessões em Redis (`iam/provider/redis`); senão em memória (`iam/provider/memory`) |
| `OAUTH_GOOGLE_*` | registra o IDP `google` (ver `iam-social-login.md`) |

- **Mais de uma réplica ⇒ `CACHE_TYPE=redis`.** Sessão em memória não é
  distribuída: logout numa réplica não vale na outra.
- Sem `User`/`Tenant` o serviço só valida token e revoga sessão (resource
  server); login devolve `core.ErrLoginPortsRequired`.
- `Configure` recebe o `DefaultConfig` já preenchido do ambiente —
  `c.Security` nunca é nil ali. Use-o para `Audience`, `VerifyIssuer`,
  `ClockSkew`, `IDPs` extras. Nunca leia `JWT_*` à mão.
- Precisa trocar `SessionPort`/`TokenPort`/`AuthPort`? Monte com `iam.New`
  e injete com `iamc.FromService(svc)` — `iam-adapter-pattern.md`.

## Ports do projeto

```go
// port.UserPort
func (a *userAdapter) FindByEmail(ctx context.Context, email string) (*types.User, error) {
    u, err := a.repo.FindByEmail(ctx, email)
    if err != nil || u == nil {
        return nil, core.ErrInvalidCredentials // inexistente = mesma resposta de senha errada
    }
    return toIAMUser(u), nil // PasswordHash, Active
}

func (a *userAdapter) ValidatePassword(ctx context.Context, userID, pw string) error {
    u, err := a.repo.FindByID(ctx, userID)
    if err != nil || u == nil {
        return core.ErrInvalidCredentials
    }
    if err := password.Verify(u.PasswordHash, pw); err != nil {
        return core.ErrInvalidCredentials
    }
    if password.NeedsRehash(u.PasswordHash) { // bcrypt legado ou Argon2id fraco
        if h, err := password.Hash(pw); err == nil {
            _ = a.repo.UpdatePasswordHash(ctx, userID, h)
        }
    }
    return nil
}

// port.TenantPort
func (a *userAdapter) ListUserTenants(ctx context.Context, userID string) ([]types.TenantAccess, error)
func (a *userAdapter) AssertAccess(ctx context.Context, userID, tenantID, module string) error // core.ErrTenantAccessDenied
```

- `TenantAccess.Roles` é a **fonte das roles** que vão para `Claims.Roles` —
  recalculadas a cada `SelectTenant` e a cada refresh.
- `AssertAccess` roda no `SelectTenant` e pode rodar por request
  (`http-auth-middleware.md` §"Revogação de acesso ao tenant"). Claims nunca
  são a única fonte de verdade de acesso.
- `FindOrCreateByExternalIdentity` só é chamado no login social; sem IDP,
  devolva erro explícito.
- Serviço single-tenant: um `types.Tenant` fixo e um `Module` fixo — o fluxo
  em dois passos continua igual.

## Hash de senha

| Pacote | Uso |
|---|---|
| `iam/provider/password` | **padrão** — `Hash` (Argon2id, `DefaultParams` OWASP), `Verify` (Argon2id **e** bcrypt, tempo constante), `NeedsRehash` |
| `iam/provider/bcrypt` | legado — `HashDefault` (custo 12), `Compare`; custo < 12 só em teste (`MinCost`) |

- Migração bcrypt → Argon2id é no login: `Verify` aceita os dois,
  `NeedsRehash` diz quando regravar (código acima).
- `password.ErrInvalid` não distingue senha errada de hash ilegível — de
  propósito. Não tente distinguir.
- Para e-mail inexistente o SDK gasta uma comparação **bcrypt custo 12**
  (equaliza tempo). Com hashes Argon2id a latência de "e-mail inexistente" e
  "senha errada" difere; proteja o endpoint de login com rate limit
  (`netx.WSConfig.RateLimiter`).

## Login, refresh e logout

```go
res, err := iamSvc.Authenticate(ctx, port.AuthInput{Email: in.Email, Password: in.Password})
if err != nil {
    return ErrAuthInvalidCredentials.New() // RegisterUnauthorized — mensagem única
}
if len(res.Tenants) == 0 {
    return ErrAuthTenantDenied.New() // RegisterForbidden
}
session, err := iamSvc.SelectTenant(ctx, port.SelectTenantInput{
    UserID:    res.UserID,
    TenantID:  chosenTenantID, // um só tenant: res.Tenants[0].Tenant.ID
    Module:    module,
    Ticket:    res.Ticket,     // obrigatório com RequireTenantTicket
    IPAddress: clientIP, UserAgent: r.UserAgent(),
})
// session.AccessToken, session.RefreshToken (cru, só agora), session.ExpiresAt (fim da sessão)
```

| Operação | Chamada | Observação |
|---|---|---|
| Renovar | `iamSvc.RefreshToken(ctx, refreshToken)` | devolve **nova** sessão; a anterior é revogada |
| Logout (este dispositivo) | `iamSvc.Logout(ctx, claims.SessionID)` | tokens da sessão rejeitados na hora |
| Logout de todos | `iamSvc.LogoutAll(ctx, claims.UserID)` | |
| Dispositivos ativos | `iamSvc.ListSessions(ctx, claims.UserID)` | |
| Validar (middleware) | `iamSvc.ValidateToken(ctx, accessToken)` | `core.ErrTokenExpired` sinaliza hora de renovar |

Entrega ao cliente — escolha pelo tipo de cliente, não por gosto:

| Modo | Cliente guarda | Envia | Para |
|---|---|---|---|
| Token | access + refresh | `Authorization: Bearer` | app mobile, CLI, serviço-a-serviço |
| Sessão (BFF) | cookie `HttpOnly` com id opaco | o cookie | browser — tokens ficam no servidor |

- Resposta do modo token: `{access_token, refresh_token, token_type: "Bearer", expires_in}`.
- Refresh token cru **nunca** é persistido nem logado (o SDK grava só o
  SHA-256). `types.Session.RefreshToken` tem `json:"-"`.
- Erros do iam são traduzidos no service de auth para `errs.AppError`
  registrados (`RegisterUnauthorized`/`RegisterForbidden`) — o cliente vê uma
  mensagem genérica; o detalhe fica no `OnEvent`/log.

## Claims de domínio (`Extra`)

Atributo do domínio que precisa viajar no token (id de unidade, rótulo de
papel externo): passe em `SelectTenantInput.Extra` (`map[string]string`). O
SDK copia para `Claims.Extra` (JSON `ext`) e `Session.Extra`, e **preserva na
rotação**:

```go
session, err := iamSvc.SelectTenant(ctx, port.SelectTenantInput{ /* … */ Extra: map[string]string{"unitId": unitID}})
// no handler:
unitID, _ := authhandler.ClaimsFromContext(r.Context()).Extra["unitId"].(string)
```

- Só dado **não sensível e estável durante a sessão** — o JWT é legível.
- `Extra` não substitui `TenantID`/`Roles`: tenant e papéis têm campo próprio.
- O fluxo social (`IDPSelectTenant`) **não** propaga `Extra` em v0.8.2.

## Auditoria (`OnEvent`)

`OnEvent func(ctx, types.IAMEvent)` recebe login, falha, refresh, logout,
negação de tenant e `security.suspicious`. Nunca traz senha nem token.
`types.EventTokenValidated` dispara a **cada request** — filtre antes de logar.
Eventos com `Error != nil` ou `EventSuspiciousActivity` → Warn.

## Anti-padrões

- ❌ `jwt.Parse`/emissão de JWT à mão no service de auth — o `TokenPort` é do SDK.
- ❌ Sessão em memória com mais de uma réplica.
- ❌ Mensagens diferentes para e-mail inexistente, senha errada e conta inativa.
- ❌ `SelectTenant` com `TenantID` vindo do cliente sem passar pelo ticket/`AssertAccess`.
- ❌ Guardar refresh token cru no banco, em log ou em `Session.Extra`.
- ❌ Reenviar o mesmo refresh token em paralelo (duas abas, retry) — revoga tudo; serialize.
- ❌ `iamc.New` e depois tentar trocar o `SessionPort` — para isso é `iam.New` + `FromService`.
