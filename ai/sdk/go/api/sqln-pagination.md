# sqln/pagination

`import "github.com/joaoprofile/gofi-sdk-go/sqln/pagination"` · SDK gofi/v0.2.6 (3df9d20)

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

```go
const DefaultMaxLimit = uint16(100)
```

DefaultMaxLimit bounds the page size of requests built from client
parameters, so a client cannot ask for 65535 rows per page.

## Funções

### pagination.ClampLimit

```go
func ClampLimit(limit uint16, opts ...Option) uint16
```

ClampLimit returns limit, DefaultLimit when zero, capped at the maximum.

## pagination.Option

```go
type Option func(*options)
```

Option configures how a PageRequest is built from client parameters.

### pagination.WithMaxLimit

```go
func WithMaxLimit(n uint16) Option
```

WithMaxLimit replaces DefaultMaxLimit; 0 keeps the default.

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
func NewPageRequestFromParams(page, limit uint16, sortField, sortDirection string, opts ...Option) *PageRequest
```

NewPageRequestFromParams builds a PageRequest from explicit pagination parameters.
This is the coupling-free alternative to NewPageRequestFilter (which lives in the sqln root).
limit is capped at DefaultMaxLimit (see WithMaxLimit).
sortField is written into ORDER BY: GetOrder drops it unless it is a plain
column reference (driver.IsIdentifier). Prefer resolving request sort names
through an allowlist (filter.Mapping.SortColumn) so clients cannot sort by,
and so probe, columns they never see.

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

NewSort builds a sort entry. field is written into ORDER BY and is dropped
by GetOrder unless it is a plain column reference (driver.IsIdentifier);
resolve request values through an allowlist such as filter.Mapping.SortColumn.

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

