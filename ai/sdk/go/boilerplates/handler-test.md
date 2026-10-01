---
name: handler-test-boilerplate
description: Esqueleto do teste de handler Go — stub de service com campos de resultado, httptest, SetPathValue, claims via WithClaims, RBAC real, só status code
sdk: v0.8.2
---

# Boilerplate — Handler Test

Regras: `.claude/sdk/go/knowledge/handler-test-pattern.md`. Casa com
`handler.md` (handler com `svc` + `port.RBACPort`, gate por
`authhandler.RequirePermission`).

```go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gofi-labs/gofi-sdk-go/base/errs"
	"github.com/gofi-labs/gofi-sdk-go/iam/provider/rbac/roles"
	"github.com/gofi-labs/gofi-sdk-go/iam/types"
	"github.com/gofi-labs/gofi-sdk-go/sqln"

	authhandler "<module>/domain/{contexto-auth}/handler"
	"<module>/domain/{contexto}/model"
	"<module>/domain/{contexto}/service"
)

// Stub with result fields — not fn fields as in the service test.
type stubEntityService struct {
	createErr      errs.AppError
	updateErr      errs.AppError
	deleteErr      errs.AppError
	getByIDResult  *model.Entity
	getByIDErr     errs.AppError
	getByFilterRes *sqln.Page[model.Entity]
	getByFilterErr errs.AppError
}

func (s *stubEntityService) Create(_ context.Context, _ string, _ model.CreateEntityRequest) errs.AppError {
	return s.createErr
}
func (s *stubEntityService) Update(_ context.Context, _ string, _ model.UpdateEntityRequest) errs.AppError {
	return s.updateErr
}
func (s *stubEntityService) Delete(_ context.Context, _ string) errs.AppError {
	return s.deleteErr
}
func (s *stubEntityService) GetByID(_ context.Context, _ string) (*model.Entity, errs.AppError) {
	return s.getByIDResult, s.getByIDErr
}
func (s *stubEntityService) GetByFilter(_ context.Context, _ model.EntityFilter) (*sqln.Page[model.Entity], errs.AppError) {
	return s.getByFilterRes, s.getByFilterErr
}

// Real RBAC with a test matrix — no mock.
var testRBAC = roles.NewRBACProvider(roles.Config{Permissions: roles.PermissionMap{
	"RoleA": {"entity": {"*"}},
	"RoleB": {"entity": {"read"}},
}})

// as puts claims in the context the way the auth middleware does.
func as(req *http.Request, role string) *http.Request {
	return req.WithContext(authhandler.WithClaims(req.Context(),
		&types.Claims{UserID: "u1", TenantID: "t1", Roles: []string{role}}))
}

func newHandler(svc *stubEntityService) *EntityHandler { return NewEntityHandler(svc, testRBAC) }

// POST — body required
func TestEntityHandler_Create(t *testing.T) {
	validBody := `{"name":"Name","email":"a@b.com"}`
	post := func(body string) *http.Request {
		return as(httptest.NewRequest(http.MethodPost, "/v1/entities", strings.NewReader(body)), "RoleA")
	}

	t.Run("returns 201 on success", func(t *testing.T) {
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{}).create(rr, post(validBody))
		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("returns 400 on invalid JSON body", func(t *testing.T) {
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{}).create(rr, post(`{invalid`))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 400 on validation error", func(t *testing.T) {
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{createErr: service.ErrEntityValidation.New()}).create(rr, post(validBody))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 409 on conflict", func(t *testing.T) {
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{createErr: service.ErrEntityConflict.New()}).create(rr, post(validBody))
		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{createErr: service.ErrEntityCreate.New()}).create(rr, post(validBody))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("returns 401 without claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/entities", strings.NewReader(validBody))
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{}).create(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("returns 403 without permission", func(t *testing.T) {
		req := as(httptest.NewRequest(http.MethodPost, "/v1/entities", strings.NewReader(validBody)), "RoleB")
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{}).create(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

// GET with path param
func TestEntityHandler_GetByID(t *testing.T) {
	get := func(id string) *http.Request {
		req := as(httptest.NewRequest(http.MethodGet, "/v1/entities/"+id, nil), "RoleB")
		req.SetPathValue("id", id) // netx.GetPathParam reads r.PathValue
		return req
	}

	t.Run("returns 200 on success", func(t *testing.T) {
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{getByIDResult: &model.Entity{ID: "1"}}).getByID(rr, get("1"))
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{getByIDErr: service.ErrEntityNotFound.New("99")}).getByID(rr, get("99"))
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

// GET with query params
func TestEntityHandler_GetByFilter(t *testing.T) {
	list := func(query string) *http.Request {
		return as(httptest.NewRequest(http.MethodGet, "/v1/entities"+query, nil), "RoleB")
	}

	t.Run("returns 200 on success", func(t *testing.T) {
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{}).getByFilter(rr, list("?page=0&limit=10"))
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("returns 400 on malformed query", func(t *testing.T) {
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{}).getByFilter(rr, list("?page=abc"))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		rr := httptest.NewRecorder()
		newHandler(&stubEntityService{getByFilterErr: service.ErrEntityQuery.New()}).getByFilter(rr, list(""))
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
```

## Padrões Obrigatórios

- Stub com **campos de resultado** — não funções (diferente do service test que usa `fn` fields)
- Assinaturas do stub idênticas à interface do service
- Claims via `authhandler.WithClaims` (helper do middleware de auth) e
  `RBACPort` real (`roles.NewRBACProvider` com matriz de teste) — sem
  `IAMService`, sem mock de RBAC
- `req.SetPathValue("param", val)` para path params — `netx.GetPathParam` lê `r.PathValue`
- Cobrir apenas **status code** — não inspecionar corpo da resposta
- Um `t.Run` por caso: happy path, input inválido, erro do service, 401/403 do gate
- Sem `init()` de logging — handler não loga diretamente
- Sem banco — o stub isola o handler completamente
