# sqln/pagination

`import "github.com/joaoprofile/gofi-sdk-go/sqln/pagination"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	DefaultPage          = uint16(0)
	DefaultLimit         = uint16(15)
	DefaultSortField     = "id"
	DefaultSortDirection = string(ASC)
)
```

## pagination.Page

```go
type Page[T any] struct {
	TotalPages       uint64 `json:"totalPages"`
	TotalElements    uint64 `json:"totalElements"`
	Size             uint64 `json:"size"`
	Number           uint64 `json:"number"`
	NumberOfElements uint64 `json:"numberOfElements"`
	Content          []T    `json:"content"`
}
```

## pagination.PageRequest

```go
type PageRequest struct {
	Page  uint16
	Limit uint16
	Order []Sort
}
```

### pagination.NewPageRequest

```go
func NewPageRequest(page uint16, limit uint16, order []Sort) *PageRequest
```

### pagination.NewPageRequestFromParams

```go
func NewPageRequestFromParams(page, limit uint16, sortField, sortDirection string) *PageRequest
```

NewPageRequestFromParams builds a PageRequest from explicit pagination parameters.
This is the coupling-free alternative to NewPageRequestFilter (which lives in the sqln root).

### PageRequest.GetOrder

```go
func (p *PageRequest) GetOrder() string
```

GetOrder renders the ORDER BY list; entries whose field is not a column reference are dropped.

## pagination.Sort

```go
type Sort struct {
	Field     string        `json:"field"`
	Direction SortDirection `json:"direction"`
}
```

### pagination.NewSort

```go
func NewSort(field string, direction SortDirection) Sort
```

## pagination.SortDirection

```go
type SortDirection string
```

```go
const (
	ASC  SortDirection = "ASC"
	DESC SortDirection = "DESC"
)
```

