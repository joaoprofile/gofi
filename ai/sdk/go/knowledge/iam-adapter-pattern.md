---
name: iam-adapter-pattern
description: Ports próprios do iam — quando sair do componente para iam.New, contrato do SessionPort sobre storage existente, AuthPort para provedor externo, TokenPort assimétrico
sdk: v0.8.2
keywords: [iam, adapter, port, SessionPort, SessionRevoker, AuthPort, TokenPort, UserPort, TenantPort, iam.New, FromService, jwt.NewProvider, sessão de domínio]
---

# IAM adapter pattern — ports próprios só onde o default não serve

Caminho padrão: componente `iamc.New` com `UserPort`/`TenantPort` do projeto
(`iam.md`). Este arquivo cobre o que **sai** do padrão. Referência:
`.claude/sdk/go/api/iam-port.md`, `iam.md`, `iam-config.md`, `iam-provider-*.md`.

## Quando usar

| Situação | Port | Como |
|---|---|---|
| Usuários/tenants no banco do projeto | `UserPort`, `TenantPort` | **sempre** — adapter sobre o repository (`iam.md` §"Ports do projeto") |
| Sessões precisam morar num storage já existente | `SessionPort` (+ `SessionRevoker`) | adapter próprio + `iam.New` |
| Credencial validada por provedor externo (IdP corporativo, serviço de auth legado) | `AuthPort` | implementação própria + `iam.New` |
| RS256/ES256, `Audience`, chave em HSM | `TokenPort` | `jwt.NewProvider(jwt.Config{...})` — sem código próprio |
| Atributos de domínio no token/sessão | — | **não** é caso de adapter: `SelectTenantInput.Extra` → `Claims.Extra`/`Session.Extra` |

O motivo antigo para `TokenPort`/re-parse próprios (claims de domínio que o
SDK descartava) **não existe mais**: `types.Claims.Extra` faz round-trip no JWT.

## Montagem com `iam.New`

```go
func buildIAM(users *adapter.UserAdapter, sessions port.SessionPort, rbac port.RBACPort, ticketKey []byte) (*core.IAMService, error) {
    a := environment.Instance().Auth() // JWT_SECRET, JWT_ISSUER, *_TOKEN_TTL
    tokens, err := jwt.NewProvider(jwt.Config{
        Algorithm: jwt.HS256, Secret: a.JWTSecret,
        AccessTokenTTL: a.AccessTokenTTL, Issuer: a.Issuer,
    })
    if err != nil {
        return nil, fmt.Errorf("iam token provider: %w", err)
    }
    return iam.New(iam.Config{
        User: users, Tenant: users, Token: tokens, Session: sessions, RBAC: rbac,
        Security: iam.SecurityConfig{
            AccessTokenTTL:      a.AccessTokenTTL,
            RefreshTokenTTL:     a.RefreshTokenTTL,
            Issuer:              a.Issuer,
            TenantTicketSecret:  ticketKey, // ≥ 32 bytes, segredo próprio
            RequireTenantTicket: true,
        },
    })
}
```

- `iam.New` **não** deriva `TenantTicketSecret` (só `NewDefault`/componente
  derivam do `JWT_SECRET`). Sem ele, `AuthResult.Ticket` vem vazio e o passo
  `SelectTenant` fica sem prova de `Authenticate`. Defina-o.
- `Session` é obrigatório (`core.ErrSessionPortRequired`); TTLs acima do teto
  falham na montagem.
- Ports só são chamados em request. Se o adapter precisa de handle aberto por
  componente (`cache.Client()`, conexão), monte o iam **depois** do `Build` e
  passe direto ao middleware — não precisa ser componente. Montado antes,
  registre com `iamc.FromService(svc)`.

## `SessionPort` sobre storage existente

```go
type sessionAdapter struct{ repo repository.SessionRepository }

func (a *sessionAdapter) Get(ctx context.Context, id string) (*types.Session, error) {
    s, err := a.repo.Get(ctx, id)
    if errors.Is(err, repository.ErrSessionNotFound) {
        return nil, core.ErrSessionNotFound
    }
    return s, err
}

// RevokeIfActive makes refresh rotation atomic (port.SessionRevoker).
func (a *sessionAdapter) RevokeIfActive(ctx context.Context, id string) (bool, error) {
    return a.repo.MarkRevokedIfActive(ctx, id, time.Now()) // compare-and-set no storage
}
```

Contrato — cada item é um bug de segurança se violado:

- **`Save` nunca persiste `AccessToken` nem `RefreshToken`** (credenciais
  cruas). Persiste `RefreshTokenHash`, `Revoked`, `RevokedAt`, `ExpiresAt`,
  auditoria e `Extra`; expira o registro em `ExpiresAt`.
- **`Revoke` marca, não apaga.** A sessão revogada tem que continuar legível até
  `ExpiresAt`: é ela que detecta reuso de refresh token (revoga tudo). Apagar
  transforma roubo em simples "sessão não encontrada".
- **`Get` inexistente → `core.ErrSessionNotFound`.**
- **Implemente `RevokeIfActive`** (`port.SessionRevoker`) com operação atômica
  do storage; sem ele a rotação usa `Revoke` e dois refreshes concorrentes
  podem ambos vencer.
- `ListByUser` devolve só sessões ativas; `RevokeAllForUser` cobre todas.
- Campo de domínio da sessão sem par no SDK vai em `Session.Extra`
  (`map[string]string`) — não reaproveite `Module`/`AuthProvider` para outra coisa.

Sem storage prévio, **não escreva adapter**: `iam/provider/redis`
(`NewProvider`/`NewProviderWithClient`) e `iam/provider/memory` já cumprem o
contrato, inclusive `RevokeIfActive`.

## `AuthPort` para provedor externo

Substitui `Authenticate`/`SelectTenant`/`Logout`/`RefreshToken`/`ValidateToken`
inteiros. Handoff entre os passos por `AuthInput.Extra` → `AuthResult.Extra` →
o caller reinjeta em `SelectTenantInput.Extra`. `ValidateToken` continua
obrigado a checar a sessão (logout real). Com `Auth` próprio, `User`/`Tenant`
podem ser nil se o `AuthPort` os encapsula — mas `TenantMiddleware`/
`AssertAccess` por request dependem do `TenantPort`.

## Testes do adapter

- `SessionPort`: round-trip `Save`/`Get` sem token cru; `Revoke` mantém
  legível; `RevokeIfActive` devolve `false` na segunda chamada; not-found →
  `core.ErrSessionNotFound`.
- Fluxo completo sem infraestrutura: `iam.New` com
  `memory.NewTestProvider()` (sem goroutine/TTL) e `jwt.NewProvider` com
  segredo de 32 bytes de teste.

## Anti-padrões

- ❌ `TokenPort` próprio ou re-parse do JWT para carregar claims de domínio — use `Extra`.
- ❌ Métodos de port que "não se aplicam" devolvendo `nil` silencioso ou `panic` — erro explícito (`errors.New("<ctx>/adapter: X not supported")`).
- ❌ `User`/`Tenant`/`RBAC` noop só para compilar — sem login, deixe nil (resource server).
- ❌ Forkar provider do SDK — implemente o port.
- ❌ `iam.New` sem `TenantTicketSecret`.
