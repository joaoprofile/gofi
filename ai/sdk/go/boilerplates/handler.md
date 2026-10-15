---
name: handler-boilerplate
description: Esqueleto do handler Go sobre netx/httpx — RouterHandler, PublicRoutes/PrivateRoutes, parse de body/query/path, resposta e RespondError(w, r, appErr)
sdk: v0.2.6
---

# Boilerplate — Handler

> **`//gofi:context {contexto}`** abre a cláusula `package` (basta em um arquivo
> do pacote) — é o elo entre o símbolo no grafo e `specs/{contexto}/`. Ver
> `.claude/sdk/go/boilerplates/model.md`.

Servidor, ordem `Use → UseAuth → Handlers` e o que já vem ligado:
`.claude/sdk/go/knowledge/http-server.md`. Claims e auth:
`http-auth-middleware.md`; gate de permissão: `rbac.md`. Programa completo:
`examples/netx/api`.

```go
//gofi:context {contexto}
package handler

import (
	"net/http"

	"github.com/joaoprofile/gofi-sdk-go/iam/port"
	"github.com/joaoprofile/gofi-sdk-go/netx/httpx"

	authhandler "<module>/domain/{contexto-auth}/handler"
	"<module>/domain/{contexto}/model"
	"<module>/domain/{contexto}/service"
)

const resource = "entity" // RBAC resource of this context (matrix in the composition root)

type EntityHandler struct {
	svc  service.EntityService
	rbac port.RBACPort // iamSvc.RBAC()
}

func NewEntityHandler(svc service.EntityService, rbac port.RBACPort) *EntityHandler {
	return &EntityHandler{svc: svc, rbac: rbac}
}

// Handlers implements httpx.RouterHandler.
func (h *EntityHandler) Handlers() []*httpx.Route {
	return httpx.PrivateRoutes("/v1/entities",
		httpx.GET("/").To(h.getByFilter),
		httpx.GET("/{id}").To(h.getByID),
		httpx.POST("/").To(h.create),
		httpx.PUT("/{id}").To(h.update),
		httpx.DELETE("/{id}").To(h.delete),
	)
}

func (h *EntityHandler) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := authhandler.RequirePermission(w, r, h.rbac, resource, "create")
	if !ok {
		return // 401/403 already written
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
	httpx.JSON(w, http.StatusCreated, nil) // nil payload writes only the status
}

func (h *EntityHandler) update(w http.ResponseWriter, r *http.Request) {
	if _, ok := authhandler.RequirePermission(w, r, h.rbac, resource, "update"); !ok {
		return
	}
	id := httpx.GetPathParam("id", r)
	var req model.UpdateEntityRequest
	if err := httpx.ParseRequestBody(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if appErr := h.svc.Update(r.Context(), id, req); appErr.Exists() {
		httpx.RespondError(w, r, appErr)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *EntityHandler) delete(w http.ResponseWriter, r *http.Request) {
	if _, ok := authhandler.RequirePermission(w, r, h.rbac, resource, "delete"); !ok {
		return
	}
	if appErr := h.svc.Delete(r.Context(), httpx.GetPathParam("id", r)); appErr.Exists() {
		httpx.RespondError(w, r, appErr)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *EntityHandler) getByFilter(w http.ResponseWriter, r *http.Request) {
	claims, ok := authhandler.RequirePermission(w, r, h.rbac, resource, "read")
	if !ok {
		return
	}
	var filter model.EntityFilter
	if err := httpx.BindQueryParamsToStruct(r, w, &filter); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	filter.TenantID = claims.TenantID // always after binding: httpx treats form:"-" as a param named "-", not as skip
	page, appErr := h.svc.GetByFilter(r.Context(), filter)
	if appErr.Exists() {
		httpx.RespondError(w, r, appErr)
		return
	}
	httpx.Response(w, http.StatusOK, page)
}

func (h *EntityHandler) getByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := authhandler.RequirePermission(w, r, h.rbac, resource, "read"); !ok {
		return
	}
	entity, appErr := h.svc.GetByID(r.Context(), httpx.GetPathParam("id", r))
	if appErr.Exists() {
		httpx.RespondError(w, r, appErr)
		return
	}
	httpx.Response(w, http.StatusOK, entity)
}
```

- Gate de permissão: `authhandler.RequirePermission` (`rbac.md`) — escreve
  401/403 e o handler só faz `return`. Tenant e usuário **sempre** das claims,
  nunca de body/query.
- Recurso sem RBAC (só login): troque o gate por
  `claims := authhandler.ClaimsFromContext(r.Context())` (não-nil em rota privada).

## Rotas públicas e privadas no mesmo handler

```go
func (h *EntityHandler) Handlers() []*httpx.Route {
	public := httpx.PublicRoutes("/v1/entities", httpx.GET("/{id}/public").To(h.getPublic))
	private := httpx.PrivateRoutes("/v1/entities", httpx.POST("/").To(h.create))
	return append(public, private...)
}
```

`PrivateRoutes` só protege se o `UseAuth` foi registrado **antes** de
`Handlers` (`http-server.md`). Rota sem login é decisão da spec, não default;
handler de rota pública não lê claims.

## Filtro Dinâmico — getSchema + getDynamicQuery

Adicionado em `Handlers()` quando o contexto expõe filtro dinâmico (regras
completas: `.claude/sdk/go/knowledge/dynamic-filter.md`):

```go
import (
	"github.com/joaoprofile/gofi-sdk-go/sqln"

	"<module>/common/enums"
)

// Rotas (dentro de Handlers())
httpx.POST("/schemas").To(h.getSchema),
httpx.POST("/query").To(h.getDynamicQuery),

func (h *EntityHandler) getSchema(w http.ResponseWriter, _ *http.Request) {
	httpx.Response(w, http.StatusOK, model.EntityFilterMapping)
}

func (h *EntityHandler) getDynamicQuery(w http.ResponseWriter, r *http.Request) {
	claims, ok := authhandler.RequirePermission(w, r, h.rbac, resource, "read")
	if !ok {
		return
	}
	var filters sqln.Filters
	if err := httpx.ParseRequestBody(w, r, &filters); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if len(filters.Filters) == 0 { // default only when the client sent no filter
		filters.Add(sqln.NewFilter("status", sqln.Eq, enums.EntityStatusActive)) // API name from the mapping
	}
	page, appErr := h.svc.GetByDynamicQuery(r.Context(), claims.TenantID, &filters)
	if appErr.Exists() {
		httpx.RespondError(w, r, appErr)
		return
	}
	httpx.Response(w, http.StatusOK, page)
}
```

**Regras:**
- A allowlist (`sqln.FilterMapping`) é aplicada no repository; filtro inválido
  volta do service como 400 — o handler não valida filtro
- Filtro default (`filters.Add(...)`) usa **nome de API** do mapping, só quando `len(filters.Filters) == 0`
- Tenant das claims como argumento; `filters.Tenant` nunca é lido
- Rotas são `POST` — body JSON (schema sem body, query com `sqln.Filters`)

## Funções httpx utilizadas

| Função | Uso |
|--------|-----|
| `httpx.ParseRequestBody(w, r, &req)` | Decodifica JSON do body (ponteiro para struct) |
| `httpx.BindQueryParamsToStruct(r, w, &filter)` | Query params → struct (tag `form:`; sem tag, nome do campo minúsculo) |
| `httpx.GetPathParam("id", r)` | Path param `{id}` (`r.PathValue`) |
| `httpx.GetQueryParam("q", r)` | Um query param |
| `httpx.Response(w, status, data)` | Serializa `data` em JSON |
| `httpx.JSON(w, status, []byte)` | Escreve bytes JSON prontos; `nil` = só status |
| `httpx.Error(w, status, err)` | Erro HTTP simples (parse, auth) |
| `httpx.ErrorDetails(w, status, msg, details)` | Erro HTTP com `details` |
| `httpx.RespondError(w, r, appErr)` | `errs.AppError` → status pelo Kind + corpo padronizado |

## Regras

- Handler não contém lógica de negócio
- Sempre checar `appErr.Exists()` antes de continuar
- Rotas em `Handlers()` com `httpx.PublicRoutes` / `httpx.PrivateRoutes` (não existe `ProtectedRoutes`)
- `RespondError` sempre com o `r` do request
- Path params com `{paramName}` na rota
