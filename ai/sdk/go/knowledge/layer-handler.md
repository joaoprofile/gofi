---
name: layer-handler
description: Resumo da camada handler em Go — RouterHandler, PublicRoutes/PrivateRoutes, gate de permissão, parse, resposta, RespondError(w, r, appErr) e teste com stub
sdk: v0.2.18
keywords: [camada, handler, httpx, RouterHandler, PrivateRoutes, PublicRoutes, RespondError, RequirePermission, ParseRequestBody, BindQueryParamsToStruct, stub]
---

# Camada Handler — resumo

Leia este primeiro; o documento completo só na situação da última seção.

## O que faz e o que não faz

**Faz:** declara rotas, aplica o gate de permissão, faz parse de
body/query/path, chama o service e traduz a resposta. Tenant e usuário saem
**sempre** das claims.

**Não faz:** lógica de negócio; SQL; chamar repository; `Validate()` de DTO de
Create/Update; `switch` de status por `appErr.Kind`; logar o erro que
`RespondError` já loga; validar filtro dinâmico.

## Padrão

```go
//gofi:context {contexto}
package handler

const resource = "entity" // recurso RBAC do contexto

type EntityHandler struct {
	svc  service.EntityService // interface
	rbac port.RBACPort
}

func NewEntityHandler(svc service.EntityService, rbac port.RBACPort) *EntityHandler {
	return &EntityHandler{svc: svc, rbac: rbac}
}

func (h *EntityHandler) Handlers() []*httpx.Route { // implementa httpx.RouterHandler
	return httpx.PrivateRoutes("/v1/entities",
		httpx.GET("/{id}").To(h.getByID),
		httpx.POST("/").To(h.create),
	)
}

func (h *EntityHandler) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := authhandler.RequirePermission(w, r, h.rbac, resource, "create")
	if !ok {
		return // 401/403 já escritos
	}
	var req model.CreateEntityRequest
	if err := httpx.ParseRequestBody(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if appErr := h.svc.Create(r.Context(), claims.TenantID, req); appErr.Exists() {
		httpx.RespondError(w, r, appErr)
		return
	}
	httpx.JSON(w, http.StatusCreated, nil) // nil escreve só o status
}
```

Listagem por query string:

```go
var filter model.EntityFilter
if err := httpx.BindQueryParamsToStruct(r, w, &filter); err != nil {
	httpx.Error(w, http.StatusBadRequest, err)
	return
}
filter.TenantID = claims.TenantID // sempre depois do bind
```

| Função | Uso |
|---|---|
| `httpx.ParseRequestBody(w, r, &req)` | JSON do body |
| `httpx.BindQueryParamsToStruct(r, w, &f)` | query → struct, tag `form` (atenção à ordem: `r, w`) |
| `httpx.GetPathParam("id", r)` | path param `{id}` |
| `httpx.GetQueryParam("q", r)` | um query param |
| `httpx.Response(w, status, data)` | serializa `data` em JSON |
| `httpx.JSON(w, status, nil)` | só status |
| `httpx.Error(w, status, err)` | erro que não é `AppError` (parse, auth) |
| `httpx.ErrorDetails(w, status, msg, details)` | 400 com detalhes montados aqui |
| `httpx.RespondError(w, r, appErr)` | `AppError` → status pelo Kind |

## Regras que mais causam erro

- **`httpx.RespondError(w, r, appErr)`** para todo erro de service, inclusive
  401/403. A assinatura antiga `(w, appErr)` não existe mais. Sempre passe o
  `r` do request.
- **Sempre `appErr.Exists()`** antes de continuar.
- **Não existem** `RespondJSON`, `DecodeJSON` nem `ProtectedRoutes`.
- **`RequirePermission` é helper do projeto**, no `handler/middleware.go` do
  contexto de auth, e não do SDK; `ok == false` → só `return`. Sem RBAC:
  `claims := authhandler.ClaimsFromContext(r.Context())`, não-nil em rota
  privada. Sem `slices.Contains(claims.Roles, ...)` inline.
- **`form:"-"` não pula o bind:** o `httpx` trata como parâmetro chamado `-`.
  Atribua tenant **depois** do bind.
- **Query:** nunca `strconv.Atoi(q.Get(...))`; bind em struct com tag `form`. O
  bind não roda as tags `validate`.
- **`PrivateRoutes` só protege** com `UseAuth` registrado **antes** de
  `Handlers` no bootstrap. Rota sem login é decisão da spec; handler de rota
  pública não lê claims.
- **Público + privado no mesmo handler:** `append(public, private...)`.
- **Path param** de UUID de `tenant`/`user`: valide com `uuid.Parse` e responda
  400 quando malformado.
- **Create** com insert simples responde `201` sem corpo.
- **Filtro dinâmico:** `POST /schemas` devolve o `FilterMapping`; `POST /query`
  faz parse de `sqln.Filters` e passa o tenant das claims como argumento.
  Filtro default (`filters.Add`) usa **nome de API**, só com
  `len(filters.Filters) == 0`.
- **Sucesso:** `Response` com dado, `JSON(w, status, nil)` sem corpo. Path
  param na rota é `{paramName}`.

## Testes

`{contexto}_handler_test.go` em todo handler, com **stub** de service (campos
de resultado) e método chamado direto, com `httptest`, sem servidor. Use
`req.SetPathValue("id", "1")`. Happy path verifica **só o status code**. Rota
com gate cobre 401 e 403.

## Quando abrir o detalhe

| Documento | Abra quando |
|---|---|
| `.claude/sdk/go/boilerplates/handler.md` | for criar o handler do zero |
| `.claude/sdk/go/boilerplates/handler-test.md` | for escrever o teste |
| `.claude/sdk/go/knowledge/handler-test-pattern.md` | houver dúvida do que cobrir ou de injetar claims |
| `.claude/sdk/go/knowledge/api-endpoint-rules.md` | o endpoint fugir do CRUD (índice das regras) |
| `.claude/sdk/go/knowledge/error-handling.md` | houver dúvida de corpo de erro ou mapeamento |
| `.claude/sdk/go/knowledge/http-server.md` | for mexer em ordem `Use → UseAuth → Handlers` |
| `.claude/sdk/go/knowledge/http-auth-middleware.md` | precisar de claims ou middleware |
| `.claude/sdk/go/knowledge/rbac.md` | for definir gate ou matriz de permissões |
| `.claude/sdk/go/knowledge/read-endpoints.md` | for GET agregado para o front |
| `.claude/sdk/go/knowledge/dynamic-filter.md` | houver `/schemas` + `/query` |
| `.claude/sdk/go/knowledge/lookup-endpoints.md` | houver dropdown de filtro |
| `.claude/sdk/go/knowledge/report-export.md` | houver export XLSX/CSV |
| `.claude/sdk/go/api/netx-httpx.md` | precisar de função fora da tabela |
