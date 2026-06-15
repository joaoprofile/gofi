# Conhecimento — Paginação (gofi/sqln)

## Como funciona internamente

`sqln.NewPageRequest(page, limit, sorts)` usa page **0-indexed**:

```
page=0, limit=5 → OFFSET 0  LIMIT 5  (primeira página)
page=1, limit=5 → OFFSET 5  LIMIT 5  (segunda página)
page=2, limit=5 → OFFSET 10 LIMIT 5  (terceira página)
```

## Comportamento de limit=0

`NewPageRequest` substitui `limit=0` por `DefaultLimit` (15) automaticamente:

```go
func NewPageRequest(page uint16, limit uint16, order []Sort) *PageRequest {
    if limit == 0 {
        limit = DefaultLimit
    }
    // ...
}
```

Isso significa que omitir `limit` na query string resulta em 15 registros por página.

## Convenção do projeto

A API expõe `page` como **0-indexed** diretamente ao cliente:
- `GET /persons?page=0&limit=5` → primeira página
- `GET /persons?page=1&limit=5` → segunda página

Não fazer conversão no repository — repassar `f.Page` e `f.Limit` diretamente.

## Filtros e precedência

Quando múltiplos filtros são enviados, definir precedência explícita com `switch`:

```go
switch {
case f.CPF != "":
    q = q.Where(criteria.Eq("p.cpf", f.CPF))
case f.Name != "":
    q = q.Where(criteria.Contains("p.name", "%"+f.Name+"%"))
}
// sem default → retorna todos os registros paginados
```

CPF tem precedência sobre Nome. Sem filtro → lista todos com paginação.

## sqln.Page[T] — estrutura de resposta

```json
{
  "data": [...],
  "total": 10,
  "totalPages": 2,
  "currentPage": 0,
  "limit": 5
}
```

## Armadilhas conhecidas

- `page=1` com expectativa de primeira página causa confusão — documentar na spec
- `criteria.Contains` faz ILIKE no PostgreSQL — não é necessário fazer `.ToLower()` no valor
- A string `"%"+f.Name+"%"` já está correta para ILIKE — o critério recebe o valor com wildcards
