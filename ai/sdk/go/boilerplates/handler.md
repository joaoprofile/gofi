# Boilerplate — Handler

```go
package handler

import (
	"net/http"

	"github.com/joaoprofile/examples/api/src/person/model"
	"github.com/joaoprofile/examples/api/src/person/service"
	"github.com/joaoprofile/gofi/netx"
)

type PersonHandler struct {
	svc service.PersonService
}

func NewPersonHandler(svc service.PersonService) *PersonHandler {
	return &PersonHandler{svc: svc}
}

func (h *PersonHandler) Handlers() []*netx.Route {
	return netx.PublicRoutes("/v1",
		netx.GET("/persons").To(h.getByFilter),
		netx.GET("/persons/{id}").To(h.getByID),
		netx.POST("/persons").To(h.create),
		netx.PUT("/persons/{id}").To(h.update),
		netx.DELETE("/persons/{id}").To(h.delete),
	)
}

func (h *PersonHandler) create(w http.ResponseWriter, r *http.Request) {
	var req model.CreatePersonRequest
	if err := netx.ParseRequestBody(w, r, &req); err != nil {
		netx.Error(w, http.StatusBadRequest, err)
		return
	}
	if appErr := h.svc.Create(r.Context(), req); appErr.Exists() {
		netx.RespondError(w, appErr)
		return
	}
	netx.JSON(w, http.StatusCreated, nil)
}

func (h *PersonHandler) update(w http.ResponseWriter, r *http.Request) {
	id := netx.GetPathParam("id", r)
	var req model.UpdatePersonRequest
	if err := netx.ParseRequestBody(w, r, &req); err != nil {
		netx.Error(w, http.StatusBadRequest, err)
		return
	}
	if appErr := h.svc.Update(r.Context(), id, req); appErr.Exists() {
		netx.RespondError(w, appErr)
		return
	}
	netx.JSON(w, http.StatusNoContent, nil)
}

func (h *PersonHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := netx.GetPathParam("id", r)
	if appErr := h.svc.Delete(r.Context(), id); appErr.Exists() {
		netx.RespondError(w, appErr)
		return
	}
	netx.JSON(w, http.StatusNoContent, nil)
}

func (h *PersonHandler) getByFilter(w http.ResponseWriter, r *http.Request) {
	var filter model.PersonFilter
	if err := netx.BindQueryParamsToStruct(r, w, &filter); err != nil {
		netx.Error(w, http.StatusBadRequest, err)
		return
	}
	page, appErr := h.svc.GetByFilter(r.Context(), filter)
	if appErr.Exists() {
		netx.RespondError(w, appErr)
		return
	}
	netx.Response(w, http.StatusOK, page)
}

func (h *PersonHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := netx.GetPathParam("id", r)
	person, appErr := h.svc.GetByID(r.Context(), id)
	if appErr.Exists() {
		netx.RespondError(w, appErr)
		return
	}
	netx.Response(w, http.StatusOK, person)
}
```

## Filtro Dinâmico — getSchema + getDynamicQuery

Adicionado em `Handlers()` quando o contexto expõe filtro dinâmico:

```go
import "github.com/joaoprofile/gofi/sqln"

// Rotas (dentro de Handlers())
netx.POST("/persons/schemas").To(h.getSchema),
netx.POST("/persons/query").To(h.getDynamicQuery),

func (h *PersonHandler) getSchema(w http.ResponseWriter, r *http.Request) {
    qm := model.PersonQueryMapping()
    netx.Response(w, http.StatusOK, map[string]interface{}{
        "allowedSortingFields": qm.AllowedSortingFields,
        "allowedFields":        qm.AllowedFields,
        "operators":            qm.Operators,
        "logicalOperators":     qm.LogicalOperators,
    })
}

func (h *PersonHandler) getDynamicQuery(w http.ResponseWriter, r *http.Request) {
    filters := &sqln.Filters{}
    if err := netx.ParseRequestBody(w, r, filters); err != nil {
        netx.Error(w, http.StatusBadRequest, err)
        return
    }
    if err := model.PersonQueryMapping().Validate(filters); err != nil {
        netx.Error(w, http.StatusBadRequest, err)
        return
    }
    if len(filters.Filters) == 0 {
        filters.Add(sqln.NewFilter("p.active", sqln.Eq, true))
    }
    // TODO: filters.Tenant = claims.TenantID (quando IAM configurado)
    result, appErr := h.svc.GetByDynamicQuery(r.Context(), filters)
    if appErr.Exists() {
        netx.RespondError(w, appErr)
        return
    }
    netx.Response(w, http.StatusOK, result)
}
```

**Regras:**
- `queryMapping.Validate(filters)` no handler, **antes** de chamar o service
- Filtro default (`filters.Add(...)`) no handler quando `len(filters.Filters) == 0`
- `filters.Tenant` injetado pelo handler (do JWT), nunca do body
- Rotas são `POST` — body JSON para ambas (schema não tem body, query tem `sqln.Filters`)

## Funções netx utilizadas

| Função | Uso |
|--------|-----|
| `netx.ParseRequestBody(w, r, &req)` | Decodifica JSON do body |
| `netx.BindQueryParamsToStruct(r, w, &filter)` | Preenche struct com query params (tags `form:`) |
| `netx.GetPathParam("id", r)` | Lê path param `{id}` |
| `netx.JSON(w, status, body)` | Responde com JSON |
| `netx.Response(w, status, body)` | Responde com JSON (alias) |
| `netx.Error(w, status, err)` | Responde com erro HTTP simples |
| `netx.RespondError(w, appErr)` | Responde com AppError estruturado |

## Regras

- Handler não contém lógica de negócio
- Sempre checar `appErr.Exists()` antes de continuar
- Rotas definidas em `Handlers()` usando `netx.PublicRoutes` ou `netx.ProtectedRoutes`
- Path params com `{paramName}` na rota
