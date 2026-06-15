# Boilerplate — Handler Test

```go
package handler

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/joaoprofile/gofi/base/errs"
    "github.com/stretchr/testify/assert"

    "github.com/org/service/src/person/model"
    "github.com/org/service/src/person/service"
)

// Stub com campos de resultado — não funções como no service test
type stubPersonService struct {
    createErr      errs.AppError
    updateErr      errs.AppError
    getByIDResult  *model.Person
    getByIDErr     errs.AppError
    getByFilterRes model.PersonResponse
    getByFilterErr errs.AppError
}

func (s *stubPersonService) Create(_ context.Context, _ model.CreatePersonRequest) errs.AppError {
    return s.createErr
}
func (s *stubPersonService) Update(_ context.Context, _ int64, _ model.UpdatePersonRequest) errs.AppError {
    return s.updateErr
}
func (s *stubPersonService) GetByID(_ context.Context, _ int64) (*model.Person, errs.AppError) {
    return s.getByIDResult, s.getByIDErr
}
func (s *stubPersonService) GetByFilter(_ context.Context, _ model.PersonFilter) (model.PersonResponse, errs.AppError) {
    return s.getByFilterRes, s.getByFilterErr
}

// POST — body obrigatório
func TestPersonHandler_Create(t *testing.T) {
    validBody := `{"tenantId":"...","name":"João","cpf":"123.456.789-00","email":"j@j.com","phone":"11999999999","origin":"SITE"}`

    t.Run("returns 201 on success", func(t *testing.T) {
        h := NewPersonHandler(&stubPersonService{})
        req := httptest.NewRequest(http.MethodPost, "/api/v1/persons", strings.NewReader(validBody))
        rr := httptest.NewRecorder()
        h.create(rr, req)
        assert.Equal(t, http.StatusCreated, rr.Code)
    })

    t.Run("returns 400 on invalid JSON body", func(t *testing.T) {
        h := NewPersonHandler(&stubPersonService{})
        req := httptest.NewRequest(http.MethodPost, "/api/v1/persons", strings.NewReader(`{invalid`))
        rr := httptest.NewRecorder()
        h.create(rr, req)
        assert.Equal(t, http.StatusBadRequest, rr.Code)
    })

    t.Run("returns 409 on conflict", func(t *testing.T) {
        h := NewPersonHandler(&stubPersonService{createErr: service.ErrPersonConflict.New()})
        req := httptest.NewRequest(http.MethodPost, "/api/v1/persons", strings.NewReader(validBody))
        rr := httptest.NewRecorder()
        h.create(rr, req)
        assert.Equal(t, http.StatusConflict, rr.Code)
    })

    t.Run("returns 500 on service error", func(t *testing.T) {
        h := NewPersonHandler(&stubPersonService{createErr: service.ErrPersonCreate.New()})
        req := httptest.NewRequest(http.MethodPost, "/api/v1/persons", strings.NewReader(validBody))
        rr := httptest.NewRecorder()
        h.create(rr, req)
        assert.Equal(t, http.StatusInternalServerError, rr.Code)
    })
}

// GET com path param int64
func TestPersonHandler_GetByID(t *testing.T) {
    t.Run("returns 200 on success", func(t *testing.T) {
        h := NewPersonHandler(&stubPersonService{getByIDResult: &model.Person{ID: 1}})
        req := httptest.NewRequest(http.MethodGet, "/api/v1/persons/1", nil)
        req.SetPathValue("id", "1")  // compatível com netx.GetPathParam → r.PathValue
        rr := httptest.NewRecorder()
        h.getByID(rr, req)
        assert.Equal(t, http.StatusOK, rr.Code)
    })

    t.Run("returns 400 on invalid ID", func(t *testing.T) {
        h := NewPersonHandler(&stubPersonService{})
        req := httptest.NewRequest(http.MethodGet, "/api/v1/persons/abc", nil)
        req.SetPathValue("id", "abc")
        rr := httptest.NewRecorder()
        h.getByID(rr, req)
        assert.Equal(t, http.StatusBadRequest, rr.Code)
    })

    t.Run("returns 404 when not found", func(t *testing.T) {
        h := NewPersonHandler(&stubPersonService{getByIDErr: service.ErrPersonNotFound.New()})
        req := httptest.NewRequest(http.MethodGet, "/api/v1/persons/99", nil)
        req.SetPathValue("id", "99")
        rr := httptest.NewRecorder()
        h.getByID(rr, req)
        assert.Equal(t, http.StatusNotFound, rr.Code)
    })
}

// GET com query params
func TestPersonHandler_GetByFilter(t *testing.T) {
    t.Run("returns 200 on success", func(t *testing.T) {
        h := NewPersonHandler(&stubPersonService{})
        req := httptest.NewRequest(http.MethodGet, "/api/v1/persons?page=0&limit=10", nil)
        rr := httptest.NewRecorder()
        h.getByFilter(rr, req)
        assert.Equal(t, http.StatusOK, rr.Code)
    })

    t.Run("returns 500 on service error", func(t *testing.T) {
        h := NewPersonHandler(&stubPersonService{getByFilterErr: service.ErrPersonQuery.New()})
        req := httptest.NewRequest(http.MethodGet, "/api/v1/persons", nil)
        rr := httptest.NewRecorder()
        h.getByFilter(rr, req)
        assert.Equal(t, http.StatusInternalServerError, rr.Code)
    })
}
```

## Padrões Obrigatórios

- Stub com **campos de resultado** — não funções (diferente do service test que usa `fn` fields)
- `req.SetPathValue("param", val)` para simular path params — compatível com `netx.GetPathParam`
- Cobrir apenas **status code** — não inspecionar corpo da resposta
- Um `t.Run` por caso: happy path, input inválido, service error
- Sem `init()` de logging — handler não loga diretamente
- Sem banco de dados — o stub isola o handler completamente
