---
name: rbac
description: Autorização com o iam — matriz role → recurso → ações no roles provider, um helper único de gate no handler, regras dependentes do alvo no service
sdk: v0.8.2
keywords: [rbac, autorização, roles, permissão, Enforce, PermissionMap, NewRBACProvider, RequirePermission, 403, gate, handler, service]
---

# RBAC — matriz no `roles` provider, um helper de gate, alvo no service

**Aplica a:** todo handler que decide acesso num projeto que usa `iam`.
Referência: `.claude/sdk/go/api/iam-provider-rbac-roles.md`, `iam-port.md`.
Autenticação/claims: `http-auth-middleware.md`.

> Este arquivo descreve **o padrão técnico**. Quais roles têm qual permissão
> é decisão de produto e vive na spec do contexto — não aqui.

## De onde vêm as roles

`TenantPort.ListUserTenants` → `types.TenantAccess.Roles` do tenant escolhido
→ `Claims.Roles`. Recalculadas em `SelectTenant` **e em cada refresh**: mudar
a role no banco vale no próximo refresh (≤ `ACCESS_TOKEN_TTL`); para valer já,
`LogoutAll` do usuário.

## A matriz — uma só, na composition root

```go
// Role constants live in the context that owns roles (model); values come from the spec.
permissions := roles.PermissionMap{
    model.RoleA: {"entity": {"*"}, "report": {"read", "export"}},
    model.RoleB: {"entity": {"read"}},
}
rbac := roles.NewRBACProvider(roles.Config{Permissions: permissions})
identity := iamc.New(iamc.Config{User: users, Tenant: users, RBAC: rbac})
```

- `role → recurso → ações`. `"*"` só vale como **ação** (todas as ações do
  recurso); recurso é comparação exata — não há curinga de recurso.
- Handlers perguntam **permissão** (`recurso:ação`), nunca "qual é a role".
  Mudar quem pode o quê = editar a matriz, sem tocar em handler.
- ABAC/OPA/Cedar: implemente `port.RBACPort` (`Enforce`, `Permissions`) e
  passe no lugar do `roles.Provider` — os handlers não mudam.

## A regra única — um helper de gate

Todo gate de acesso no handler passa por **uma** função, no `handler/middleware.go` do contexto de auth
(o mesmo que exporta `ClaimsFromContext`):

```go
// RequirePermission writes 401 (no claims) or 403 (denied); the caller just returns when ok is false.
func RequirePermission(w http.ResponseWriter, r *http.Request, rbac port.RBACPort, resource, action string) (*types.Claims, bool) {
    c := ClaimsFromContext(r.Context())
    if c == nil {
        netx.Error(w, http.StatusUnauthorized, errUnauthenticated)
        return nil, false
    }
    if !rbac.Enforce(*c, resource, action) {
        netx.Error(w, http.StatusForbidden, errForbidden)
        return nil, false
    }
    return c, true
}
```

```go
func (h *EntityHandler) delete(w http.ResponseWriter, r *http.Request) {
    claims, ok := authhandler.RequirePermission(w, r, h.rbac, "entity", "delete")
    if !ok {
        return
    }
    // claims.TenantID, claims.UserID disponíveis
}
```

- O handler recebe o `port.RBACPort` no construtor (`iamSvc.RBAC()`), não o
  `*core.IAMService` inteiro.
- Rota só autenticada (qualquer role): basta `ClaimsFromContext` numa
  `netx.PrivateRoutes`; não invente permissão fictícia.
- `iamSvc.RBAC().Permissions(claims)` lista as permissões — use no `/me` para
  o front esconder ações (UX, não autoridade).

## Camadas de gate: handler vs service

| Camada | O que decide | Exemplo |
|---|---|---|
| **Handler** (`RequirePermission`) | permissão do ator, estática | "quem tem `entity:delete` chama este endpoint" |
| **Service** (regra de negócio + erro de domínio) | ator + estado/dono do **alvo** lido do banco | "RoleB só atua sobre registros próprios", hierarquia, ownership |

Gate **target-aware** não cabe no handler — precisa ler o registro-alvo.
Vive no service como função pura no model (`model.CanActOver(actor, target)`)
+ erro registrado (`errs.RegisterForbidden(...)` → 403 via `RespondError`).

Separar os dois gates evita TOCTOU entre middleware e handler, centraliza a
hierarquia na fonte da verdade e mantém o handler testável só com claims.

## Por que NÃO `iammw.RBACMiddleware`

- Rotas `netx` não aceitam middleware por rota (só `Cors`/`Timeouts`); o único
  middleware por grupo é o `UseAuth`. `RBACMiddleware(svc, resource, action)`
  não tem onde ser pendurado sem embrulhar a rota à mão.
- Lê as claims pela chave privada do `iammw` — com o middleware de auth do
  projeto (chave própria) ele responde 401 sempre.
- Responde `text/plain`, fora do formato de erro da API.

## Anti-padrões

- ❌ `if slices.Contains(claims.Roles, "RoleA")` inline — pergunte permissão via `RequirePermission`.
- ❌ Matriz espalhada (lista de roles em cada handler) ou duplicada por contexto.
- ❌ Helper de gate redefinido em cada contexto (`tenantIDFromClaims`, `errUnauthorized` locais) — importe do `handler` do contexto de auth.
- ❌ Hierarquia/ownership no handler (`if actor == X && target == Y`) — vai no service.
- ❌ `TenantID` do body/query quando há claims — sempre `claims.TenantID`.
- ❌ Confiar no gate do front; ele é espelho de UX.
- ❌ `// TODO(rbac)` deixando rota aberta — aplique o gate na hora.

## Testabilidade

`RequirePermission` depende só das claims no contexto e do `RBACPort`:

- claims via `authhandler.WithClaims(req.Context(), &types.Claims{Roles: []string{model.RoleB}})`;
- `RBACPort` real (`roles.NewRBACProvider` com a matriz de teste) — sem mock;
- sem claims → 401; role sem a permissão → 403; o service nem é chamado.
