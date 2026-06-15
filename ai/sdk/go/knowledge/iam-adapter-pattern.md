# IAM adapter pattern — `port.SessionPort` + `port.TokenPort` quando o domínio não bate com o SDK

## Quando usar

Quando o projeto:

1. Já tem `repository.SessionRepository` (Redis ou outro) com um `model.UserSession` que carrega campos de domínio (`CompanyID`, `ManagerID`, `Role`, `GumgaToken`...) que `iam/types.Session` **não** modela.
2. Já emite JWT manualmente em `auth_service.go` com custom claims (`name`/`companyId`/`managerId`/`role`) que `iam/provider/jwt.iamClaims` **não** modela.
3. Quer ativar `iamSvc.ValidateToken` para o middleware HTTP delegar revogação ao SDK **sem** migrar a emissão do token e a sessão para o SDK (Fase 2b).

Esse é o padrão "adapter mínimo": só o que o middleware precisa.

## Regra

`services/domain/{contexto}/adapter/` recebe **um arquivo por port**:

- `session_port.go` — implementa `port.SessionPort` traduzindo `types.Session` ↔ `model.UserSession`. `Save` é **stub explícito com erro** (o service do contexto cria sessões direto via repo).
- `token_port.go` — implementa `port.TokenPort.ParseToken` parseando o JWT manual e extraindo apenas o subconjunto que `types.Claims` carrega (`UserID`, `TenantID`, `SessionID`, `Issuer`). `IssueAccessToken`/`IssueRefreshToken` são **stubs explícitos com erro** (o service do contexto emite tokens direto).
- `adapter_test.go` — testes handcraft com mock de `repository.SessionRepository` em fn-fields.

`services/{service}/iam.go` (novo arquivo no `pathCmd`, ao lado de `config.go`/`wire.go`) constrói o `*iamcore.IAMService` via `iam.New(Config{Token, Session, Security: ...})`. **Não** passar `User`/`Tenant`/`RBAC` quando o middleware só usa `ValidateToken` — o SDK valida apenas `cfg.Session != nil`.

## Por quê

- **Não forkar o SDK.** O `iam/provider/jwt.iamClaims` é struct fechado sem `Extra map[string]any`. Implementar `port.TokenPort` no projeto contorna sem patch.
- **`port.SessionPort` é a interface mais limpa de adaptar.** O SDK só usa `Save`/`Get`/`Revoke`/`RevokeAllForUser`/`ListByUser`. Mapeia 1-pra-1 com o repo existente exceto pela conversão de struct.
- **Stubs explícitos > silenciosos.** Métodos não suportados retornam `errors.New("auth/adapter: X not supported (reason)")` — quem chamar por engano descobre na hora. Não use `panic` (mata o processo), não use `return nil` silencioso (esconde bug).
- **`Save` stub é seguro** porque o caminho `localAuth.SelectTenant` (que chamaria `Save`) só é exercitado se o middleware chamar `iamSvc.Authenticate` + `iamSvc.SelectTenant`. Como o middleware só usa `ValidateToken`, `Save` nunca é tocado em runtime.

## Padrão — `session_port.go`

```go
type sessionPort struct {
    repo       repository.SessionRepository
    revokedTTL time.Duration
}

func NewSessionPort(repo repository.SessionRepository, revokedTTL time.Duration) port.SessionPort {
    return &sessionPort{repo: repo, revokedTTL: revokedTTL}
}

func (a *sessionPort) Save(ctx context.Context, _ *types.Session) error {
    return errors.New("<ctx>/adapter: SessionPort.Save not supported (auth_service writes sessions directly)")
}

func (a *sessionPort) Get(ctx context.Context, sessionID string) (*types.Session, error) {
    sess, err := a.repo.Get(ctx, sessionID)
    if err != nil {
        if errors.Is(err, repository.ErrSessionNotFound) {
            return nil, iamcore.ErrSessionNotFound // traduz pro erro sentinel do SDK
        }
        return nil, err
    }
    return toTypesSession(sess), nil
}

func (a *sessionPort) Revoke(ctx context.Context, sessionID string) error {
    if err := a.repo.Revoke(ctx, sessionID, a.revokedTTL); err != nil {
        if errors.Is(err, repository.ErrSessionNotFound) {
            return iamcore.ErrSessionNotFound
        }
        return err
    }
    return nil
}

func (a *sessionPort) RevokeAllForUser(ctx context.Context, userID string) error {
    _, err := a.repo.RevokeAll(ctx, userID, a.revokedTTL)
    return err
}

func (a *sessionPort) ListByUser(ctx context.Context, userID string) ([]*types.Session, error) {
    ids, err := a.repo.ListByUserID(ctx, userID)
    if err != nil {
        return nil, err
    }
    out := make([]*types.Session, 0, len(ids))
    for _, id := range ids {
        sess, err := a.repo.Get(ctx, id)
        if err != nil {
            continue // sessão pode ter expirado entre lista e get
        }
        out = append(out, toTypesSession(sess))
    }
    return out, nil
}

// Tradução model.UserSession → types.Session. Só preenche o que o SDK
// efetivamente lê em ValidateToken (Revoked, ExpiresAt) + campos audit
// úteis. Campos de domínio sem destino no SDK (ManagerID, Role, GumgaToken)
// são descartados — o blob rico continua no Redis para quem precisar.
func toTypesSession(s *model.UserSession) *types.Session {
    return &types.Session{
        ID:               s.SessionID,
        UserID:           s.IAMUserID,
        TenantID:         s.CompanyID,
        RefreshTokenHash: s.RefreshTokenHash,
        ExpiresAt:        s.ExpiresAt,
        CreatedAt:        s.CreatedAt,
        LastUsedAt:       s.CreatedAt,
        Revoked:          s.RevokedAt != nil,
        RevokedAt:        s.RevokedAt,
        IPAddress:        s.IP,
        UserAgent:        s.UserAgent,
    }
}
```

## Padrão — `token_port.go`

```go
type tokenPort struct {
    cfg environment.AuthConfig
}

func NewTokenPort() port.TokenPort {
    return &tokenPort{cfg: environment.Instance().Auth()}
}

func (t *tokenPort) IssueAccessToken(_ types.Claims) (string, error) {
    return "", errors.New("<ctx>/adapter: TokenPort.IssueAccessToken not supported (auth_service mints tokens directly)")
}

func (t *tokenPort) IssueRefreshToken(_ types.Claims) (string, error) {
    return "", errors.New("<ctx>/adapter: TokenPort.IssueRefreshToken not supported (auth_service mints tokens directly)")
}

func (t *tokenPort) ParseToken(tokenStr string) (*types.Claims, error) {
    tok, err := jwt.Parse(tokenStr, func(j *jwt.Token) (any, error) {
        if j.Method.Alg() != "HS256" {
            return nil, iamcore.ErrTokenInvalid
        }
        return t.cfg.JWTSecret, nil
    })
    if err != nil || tok == nil || !tok.Valid {
        return nil, iamcore.ErrTokenInvalid
    }
    mc, ok := tok.Claims.(jwt.MapClaims)
    if !ok {
        return nil, iamcore.ErrTokenInvalid
    }
    sessionID := asString(mc["sessionId"])
    iamUserID := asString(mc["iamUserId"])
    if sessionID == "" || iamUserID == "" {
        return nil, iamcore.ErrTokenInvalid
    }
    return &types.Claims{
        UserID:    iamUserID,
        TenantID:  asString(mc["companyId"]),
        SessionID: sessionID,
        Issuer:    asString(mc["iss"]),
    }, nil
}
```

**Custom claims do projeto** (`name`, `companyId`, `managerId`, `role`, `sub`) **não** entram em `types.Claims` — o middleware HTTP reparseia o token na enrichment phase (ver `http-auth-middleware.md` §"Assinatura — variante delegada ao SDK").

## Padrão — `iam.go` no `pathCmd`

```go
package main

import (
    authAdapterPkg "<module>/services/domain/<ctx>/adapter"

    "github.com/joaoprofile/gofi/iam"
    iamcore "github.com/joaoprofile/gofi/iam/core"
)

func buildIAM(cfg Config, repos <ctx>Repos) (*iamcore.IAMService, error) {
    sessionPort := authAdapterPkg.NewSessionPort(repos.session, cfg.RevokedSessionTTL)
    tokenPort := authAdapterPkg.NewTokenPort()

    iamSvc, err := iam.New(iam.Config{
        Token:   tokenPort,
        Session: sessionPort,
        Security: iam.SecurityConfig{
            AccessTokenTTL:  cfg.Auth.AccessTokenTTL,
            RefreshTokenTTL: cfg.Auth.RefreshTokenTTL,
            Issuer:          cfg.Auth.Issuer,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("build iam: %w", err)
    }
    return iamSvc, nil
}
```

`main.go` chama `iamSvc, err := buildIAM(cfg, authRepos)` antes de
`buildAuthMiddleware(iamSvc)`, fataliza em erro via `logging.Fatal`.

## Anti-padrões

```go
// ❌ Forkar o jwt provider do SDK para suportar custom claims — divergência permanente
// ❌ panic em métodos não suportados — derruba o processo numa rota não-coberta
func (a *sessionPort) Save(ctx context.Context, _ *types.Session) error {
    panic("not implemented")
}

// ❌ return nil silencioso — esconde bug futuro
func (a *sessionPort) Save(ctx context.Context, _ *types.Session) error {
    return nil
}

// ❌ Passar User/Tenant/RBAC mockados só pra satisfazer assinatura do SDK
iam.New(iam.Config{
    Token: ..., Session: ...,
    User:   noopUserPort{},   // não use — SDK só valida Session!=nil
    Tenant: noopTenantPort{},
    RBAC:   noopRBACPort{},
})

// ❌ Adapter inflado tentando carregar TODOS os campos de domínio em types.Session.
//    Module := s.ManagerID; AuthProvider := s.Role  ← abuso semântico que confunde quem lê depois
```

## Checklist

- [ ] `adapter/session_port.go` traduz `model.UserSession → types.Session` mantendo só `ID`/`UserID`/`TenantID`/`Revoked`/`RevokedAt`/`ExpiresAt` (+ audit se útil)
- [ ] `repository.ErrSessionNotFound` → `iamcore.ErrSessionNotFound` (erro sentinel do SDK)
- [ ] `Save` retorna `errors.New(...)` explícito — nunca `panic`, nunca `return nil`
- [ ] `adapter/token_port.go` valida HS256 + segredo de `environment.Auth()`; lê `iamUserId`/`sessionId` (rejeita se vazios)
- [ ] `Issue*` retornam `errors.New(...)` explícito — emissão fica em `auth_service.go`
- [ ] `services/{cmd}/iam.go` constrói via `iam.New(Config{Token, Session, Security})` — sem User/Tenant/RBAC mockados
- [ ] `main.go` fataliza erro de `buildIAM` via `logging.Fatal`
- [ ] `adapter/adapter_test.go` com `mockSessionRepo` handcraft (fn-fields) cobrindo: `Get` not-found, `Get` live, `Get` revoked, `Revoke`/`RevokeAll` propagação de TTL, `ListByUser` skip-on-error, `Save` retorna erro, `ParseToken` happy path / tampered / missing-required
- [ ] Middleware HTTP usa variante delegada ao SDK + re-parse (ver `http-auth-middleware.md`)
