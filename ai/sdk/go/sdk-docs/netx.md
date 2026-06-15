# gofi/netx — HTTP Layer

## Servidor

```go
import "github.com/joaoprofile/gofi/netx"

server := netx.NewServer(netx.Config{
    Port: ":8080",
})
server.Start()
```

## Roteamento

```go
router := netx.NewRouter()
router.Post("/persons", handler.Create)
router.Get("/persons", handler.List)
router.Get("/persons/{id}", handler.GetByID)
router.Put("/persons/{id}", handler.Update)
router.Delete("/persons/{id}", handler.Delete)

server.Register(router)
```

## Request Helpers

```go
// Decodificar body JSON
var req model.CreatePersonRequest
if err := netx.DecodeJSON(r, &req); err != nil {
    netx.RespondError(w, errs.RegisterValidation("DECODE_ERROR", err.Error()).New())
    return
}

// Ler path param
id := netx.GetPathParam(r, "id")

// Ler query param
name := netx.GetQueryParam(r, "name")
```

## Response Helpers

```go
// Sucesso com body
netx.RespondJSON(w, http.StatusOK, body)

// Sucesso sem body
netx.RespondJSON(w, http.StatusCreated, nil)
netx.RespondJSON(w, http.StatusNoContent, nil)

// Erro estruturado — mapeia Kind → status HTTP
netx.RespondError(w, appErr)
```

### Mapeamento de Kind → HTTP Status

| AppError Kind  | HTTP Status |
|----------------|-------------|
| Validation     | 400         |
| NotFound       | 404         |
| Conflict       | 409         |
| Operation/default | 500     |

`RespondError` propaga `appErr.Details` no body quando não nil — campo `"details"` na resposta JSON.

## Middleware

```go
// Rate limiting
router.Use(netx.RateLimitMiddleware(cfg))

// CORS
router.Use(netx.CORSMiddleware(cfg))

// Tracing
router.Use(netx.TracingMiddleware())
```

## Versioning

```go
v1 := router.Group("/v1")
v1.Post("/persons", handler.Create)
```

## Testes de Handler

```go
// Simular request com path param (Go 1.22+)
r := httptest.NewRequest(http.MethodGet, "/persons/abc-123", nil)
r.SetPathValue("id", "abc-123")
w := httptest.NewRecorder()
handler.GetByID(w, r)
```
