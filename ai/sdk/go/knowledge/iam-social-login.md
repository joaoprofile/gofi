---
name: iam-social-login
description: Login social com o iam — Google, Microsoft Entra e OIDC genérico; state + PKCE em cookie temporário, callback, vínculo de identidade externa e abertura da sessão
sdk: v0.8.2
keywords: [iam, IDP, OAuth, OIDC, Google, Microsoft, Entra, PKCE, state, IDPInitFlow, IDPHandleCallback, IDPSelectTenant, FindOrCreateByExternalIdentity]
---

# Login social — `IDPInitFlow` → `IDPHandleCallback` → `IDPSelectTenant`

Pré-requisito: `iam.md` (ports, sessão, entrega token/cookie). Referência:
`.claude/sdk/go/api/iam-provider-{google,microsoft,oidc}.md`, `iam-core.md`.

## Registro dos provedores

| Provedor | Como registrar | Nome no fluxo |
|---|---|---|
| Google | `OAUTH_GOOGLE_CLIENT_ID`, `OAUTH_GOOGLE_CLIENT_SECRET`, `OAUTH_GOOGLE_REDIRECT_URI` — o componente registra sozinho | `"google"` |
| Microsoft Entra | `Configure`: `c.IDPs = append(c.IDPs, iamconfig.IDPConfig{Provider: "microsoft", ClientID: …, ClientSecret: …, RedirectURI: …})` | `"microsoft"` |
| OIDC genérico | `Configure`: `iamconfig.IDPConfig{Provider: "oidc", IssuerURL: …, ClientID: …, ClientSecret: …, RedirectURI: …, Scopes: …}` (`IssuerURL` obrigatório) | `"oidc"` |

- `IDPConfig.Provider` aceita só `google`, `microsoft`, `oidc` — outro valor
  falha no `Build` (`unknown IDP provider`).
- Opções que o `IDPConfig` não carrega exigem montar o port e usar `iam.New` +
  `iamc.FromService`:
  - `google.Config.HostedDomain` — restringe a um domínio Workspace (`google.ErrHostedDomainMismatch`);
  - `microsoft.Config.TenantID` (`"common"` é o default; `"organizations"`,
    `"consumers"` ou o id do diretório) e `AllowedTenants`;
  - `oidc.Config.ValidateClaims`/`ValidateIssuer`/`AuthParams`, ou **mais de um** OIDC (`oidc.New(name, cfg)`).

  ```go
  idps := map[string]port.IDPAuthPort{
      "microsoft": microsoft.New(microsoft.Config{ClientID: id, ClientSecret: secret, RedirectURI: cb, TenantID: dirID}),
  }
  svc, err := iam.New(iam.Config{User: users, Tenant: users, Token: tokens, Session: sessions, RBAC: rbac, IDPs: idps})
  ```

- Client secret vem da config do projeto (secret reference), nunca literal.

## Fluxo

### 1. Início — `GET /auth/{provider}/login` (rota pública)

```go
u, err := iamSvc.IDPInitFlow(r.Context(), provider, redirectURI, nil)
if err != nil { // core.ErrProviderNotFound para nome não registrado
    netx.Error(w, http.StatusNotFound, errUnknownProvider)
    return
}
setIDPStateCookie(w, u.State, u.CodeVerifier) // HttpOnly, SameSite=Lax, 10 min
http.Redirect(w, r, u.URL, http.StatusFound)
```

O SDK gera `state`, PKCE S256 e o `nonce` (derivado do state). **O projeto
guarda** `State` + `CodeVerifier` num cookie temporário: `HttpOnly`,
`Secure` fora do local, **`SameSite=Lax`** (o retorno do provedor é navegação
cross-site — `Strict` descarta o cookie), `MaxAge` ≈ 10 min, `Path` restrito à
rota de callback. O SDK não escreve esse cookie.

### 2. Callback — `GET /auth/{provider}/callback` (rota pública)

```go
st, ok := readIDPStateCookie(r)
clearIDPStateCookie(w) // uso único, sucesso ou falha
if !ok {
    netx.Error(w, http.StatusUnauthorized, errUnauthenticated)
    return
}
res, err := iamSvc.IDPHandleCallback(r.Context(), provider, port.IDPCallbackInput{
    Code:          netx.GetQueryParam("code", r),
    State:         netx.GetQueryParam("state", r),
    ExpectedState: st.State,        // ≠ → core.ErrInvalidIDPState (CSRF)
    CodeVerifier:  st.CodeVerifier, // PKCE
    RedirectURI:   redirectURI,
})
if err != nil {
    netx.Error(w, http.StatusUnauthorized, errUnauthenticated)
    return
}
session, err := iamSvc.IDPSelectTenant(r.Context(), provider, port.SelectTenantInput{
    UserID: res.UserID, TenantID: chosenTenantID, Module: module, Ticket: res.Ticket,
    IPAddress: clientIP, UserAgent: r.UserAgent(),
})
// entrega igual ao login local (modo sessão é o natural para browser)
```

O SDK valida state, troca o code, valida o `id_token` (JWKS, issuer,
audience, nonce), chama `UserPort.FindOrCreateByExternalIdentity` e lista os
tenants. `res.IDPUser` traz o perfil normalizado (`Email`, `EmailVerified`,
`Name`, `PictureURL`).

### 3. `FindOrCreateByExternalIdentity` — o único código do projeto

- **Idempotente**; chave = `(Provider, ExternalID)`. Nunca o e-mail.
- **Não funda** com conta local existente só porque o e-mail coincide — o
  port não recebe `EmailVerified`; fusão automática por e-mail é tomada de
  conta. Vínculo com conta existente é fluxo explícito (usuário logado
  vincula a identidade).
- Microsoft: `ExternalID` já é `tid`+`oid` (estável entre registros de app).
- Usuário novo sem nenhum tenant → `ListUserTenants` vazio → o handler
  responde 403 (ou fluxo de onboarding), nunca abre sessão sem tenant.

## Anti-padrões

- ❌ State/verifier em query string, `localStorage` ou cookie legível por JS.
- ❌ Reaproveitar o cookie de state (não limpar no callback).
- ❌ `IDPSelectTenant` sem o `Ticket` de `IDPHandleCallback`.
- ❌ Contar com `SelectTenantInput.Extra` no fluxo social — não é propagado em v0.8.2.
- ❌ `IDPConfig{Provider: "github"}` — não suportado; exige port próprio (`port.IDPAuthPort`).
