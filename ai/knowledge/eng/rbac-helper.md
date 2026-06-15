# RBAC — `RequireRole` helper centralizado no contexto `user`

**Aplica a:** todo handler que decide acesso por role num projeto que usa
`gofi/iam`. **Source of truth do código:** o pacote `handler` do contexto
`user` — é lá que o helper é definido; demais contextos importam.

> Este arquivo descreve **o padrão técnico**. A matriz por endpoint
> (quais roles entram em qual rota) é decisão de produto e vive na spec
> do contexto + memória do projeto — não aqui.

---

## A regra única

Toda decisão de acesso por role passa por **uma única função** importada
do contexto `user`:

```go
import (
    userhandler "<module>/domain/user/handler"
    usermodel   "<module>/domain/user/model"
)

func (h *FooHandler) doSomething(w http.ResponseWriter, r *http.Request) {
    id, ok := userhandler.RequireRole(w, r, usermodel.RoleA, usermodel.RoleB)
    if !ok {
        return // RequireRole já escreveu 401 (sem claims) ou 403 (role fora da lista)
    }
    // id.TenantID, id.UserID, id.ActorRole disponíveis
    ...
}
```

- **Sem args** (`RequireRole(w, r)`) = qualquer role autenticado passa.
  Use em endpoints abertos a todos os autenticados (leitura compartilhada,
  endpoints `/me`, etc.).
- **Com args** = só os roles listados passam; demais recebem 403.
- **Retorno**: `(claims, ok)` — o handler **só** segue se `ok == true`.
  O helper é responsável por escrever a resposta de erro; o handler
  apenas faz `return` quando `ok == false`.

---

## Camadas de gate: handler vs service

| Camada | O que decide | Exemplo |
|---|---|---|
| **Handler** (`RequireRole`) | role do ator vs lista estática de roles permitidas | "só RoleA e RoleB podem chamar este endpoint" |
| **Service** (regra de negócio + erro de domínio) | role do ator + estado/role do **alvo** lido do banco | "RoleB só pode atuar sobre RoleC", hierarquia, ownership |

Gates **target-aware** (decisão depende de quem é o alvo da operação)
**não cabem no handler** — precisam ler o registro-alvo. Eles vivem no
service como uma função pura no model (ex.: `model.CanActOver(actor, target)`)
+ um erro de domínio (ex.: `ErrRoleHierarchy` mapeado a 409 ou 403).

Manter os dois gates separados:

- evita TOCTOU entre middleware e handler;
- centraliza hierarquia na fonte da verdade (`UserService` + função pura no `usermodel`);
- mantém o handler magro e testável só com claims, sem mock de banco.

---

## Por que NÃO usar `RBACMiddleware` do `gofi/iam`

O middleware do SDK valida binário `claims.Roles ∋ allowed-for-resource:action`
**sem ver o target** da operação. Qualquer regra que dependa do alvo
("role X só atua sobre role Y", "owner-only", "tenant scoping com exceção")
precisa ler o registro do banco — só faz sentido no service.

Pipeline canônico do projeto:

1. **Auth middleware** do SDK na cadeia de rotas — extrai/valida claims;
2. **`RequireRole`** no início do handler — gate role-only;
3. **Service** — qualquer gate target-aware via função pura no model + erro de domínio.

Não há `RBACMiddleware` montado em paralelo — duplicaria o gate e pode divergir.

---

## Por que o helper vive em `domain/user/handler`

O contexto `user` é o que **define** o enum de roles e é conceitualmente
upstream dos demais contextos. Centralizar `RequireRole` lá:

- Elimina helpers duplicados em cada contexto (`tenantIDFromClaims`,
  `tenantAndUserFromClaims`, `errUnauthorized`, etc. espalhados por
  `pool/handler`, `order/handler`, …).
- Garante que mudar a hierarquia (adicionar/renomear/remover role) toca
  **um** ponto — o helper + o enum.
- Mantém o contrato: o `user` é quem importa `iammw` (middleware do SDK)
  e conhece a estrutura das claims; demais contextos só consomem o
  helper, sem se acoplar à API do `iammw`.

Demais contextos importam `userhandler.RequireRole` e `usermodel.Role*`,
nunca redefinem helpers locais.

---

## Anti-padrões observados (não fazer)

- ❌ `tenantID, err := tenantIDFromClaims(r)` (ou similar) em handler
  novo — usar `RequireRole`.
- ❌ `if claims.Roles[0] != "X"` inline — usar
  `RequireRole(w, r, usermodel.RoleX, ...)`.
- ❌ Helper duplicado em outro contexto (`tenantIDFromClaims` em
  `pool/handler`, `errUnauthorized` em `order/handler`) — importar do
  `user/handler`.
- ❌ Adicionar `RBACMiddleware` na cadeia de rotas em paralelo ao
  `RequireRole` — duplica gate e pode divergir.
- ❌ Validar hierarquia/ownership em handler
  (`if actorRole == "X" && target == "Y" return 403`) — esse check vive
  **no service** via função pura no model + erro de domínio.
- ❌ Acessar `iammw.ClaimsFromContext(r)` direto em handler de outro
  contexto — só o `user/handler` (auth handler) tem motivo legítimo
  para isso; demais usam `RequireRole`.
- ❌ TODO `// TODO(rbac): trocar middleware depois` deixando o handler
  aberto — substituir por `RequireRole` na hora.

---

## Testabilidade

`RequireRole` é puro: depende só de `r` (claims no contexto) e da lista
de roles. Nos `handler_test.go` de outros contextos:

- monte o request com claims via helper de teste do `user` (ex.:
  `userhandler.WithTestClaims(r, claims)`);
- não precisa mockar service só para testar o gate role-only — o gate
  acontece **antes** de qualquer chamada ao service.

---

## Frontend espelha o gate (UX, não autoridade)

Frontends costumam ter helpers tipo `canActOver(actor, target)` ou
`canManageX(role)` para **double-gate visual**: rota protegida +
visibilidade condicional de botões/campos.

**Backend continua sendo a fonte da verdade.** UI esconder o botão é só
UX — o handler ainda valida via `RequireRole` (e o service via função
pura no model + erro de domínio). Nunca confiar em check feito só no
cliente.
