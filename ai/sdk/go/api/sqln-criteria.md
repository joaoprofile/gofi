# sqln/criteria

`import "github.com/joaoprofile/gofi-sdk-go/sqln/criteria"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	ASC  = "ASC"
	DESC = "DESC"
)
```

## Variáveis

```go
var ErrInvalidField = errors.New("criteria: invalid field")
```

ErrInvalidField is returned by Build when a predicate or order field is not
a column reference and was not marked Raw.

```go
var ErrInvalidValue = errors.New("criteria: invalid value")
```

ErrInvalidValue is returned by Build when a predicate value has the wrong shape.

## Funções

### criteria.BuildClause

```go
func BuildClause(predicates []Predicate, d driver.FilterDialect) (string, []any, error)
```

BuildClause compiles a predicate slice into a SQL WHERE fragment and its bound parameters.
It fails with ErrInvalidField when a field is not a column reference.

### criteria.BuildClauseAfter

```go
func BuildClauseAfter(args []any, predicates []Predicate, d driver.FilterDialect) (string, []any, error)
```

BuildClauseAfter numbers placeholders after args, the parameters the
surrounding query already binds, and returns args followed by the new ones.

## criteria.Order

```go
type Order struct {
	Field     string
	Direction string
	// contains filtered or unexported fields
}
```

Order defines a sort expression for a query (field + direction).

Field must be a column reference (see driver.IsIdentifier), so it may come
from a request; anything else fails Build with ErrInvalidField. Mark a
trusted SQL expression with Raw.

### criteria.Asc

```go
func Asc(field string) Order
```

Asc creates an ascending ORDER BY expression.

### criteria.Desc

```go
func Desc(field string) Order
```

Desc creates a descending ORDER BY expression.

### Order.Raw

```go
func (o Order) Raw() Order
```

Raw marks Field as a trusted SQL expression (e.g. COUNT(*)) written as is.
Trusted input only — never user data.

## criteria.Predicate

```go
type Predicate struct {
	// contains filtered or unexported fields
}
```

Predicate represents a single WHERE/HAVING condition or a logical connector (AND/OR).
Use the constructor functions below — never build Predicate literals directly.

The field must be a column reference (col, t.col, "Col"; see driver.IsIdentifier):
it is written into the SQL text, so anything else fails Build with ErrInvalidField.
Mark a trusted SQL expression such as COUNT(*) with Raw.

### criteria.And

```go
func And() Predicate
```

And inserts an explicit AND connector between adjacent predicates.
Adjacent predicates without any connector are implicitly ANDed, so And() is
only needed when mixing AND and OR within the same clause.

### criteria.Between

```go
func Between(field string, from, to any) Predicate
```

Between produces field BETWEEN $from AND $to.

### criteria.Contains

```go
func Contains(field string, value string) Predicate
```

Contains performs a case-insensitive substring match via the active dialect:
  - PostgreSQL → field ILIKE $n
  - All others → field LIKE $n

### criteria.DateAfter

```go
func DateAfter(field string, date time.Time) Predicate
```

DateAfter produces field > $date.

### criteria.DateBefore

```go
func DateBefore(field string, date time.Time) Predicate
```

DateBefore produces field < $date.

### criteria.DateBetween

```go
func DateBetween(field string, from, to time.Time) Predicate
```

DateBetween produces field BETWEEN $from AND $to.
Build fails when either value is zero or from is after to.

### criteria.DateEq

```go
func DateEq(field string, date time.Time) Predicate
```

DateEq produces field = $date.

### criteria.DateOnOrAfter

```go
func DateOnOrAfter(field string, date time.Time) Predicate
```

DateOnOrAfter produces field >= $date.

### criteria.DateOnOrBefore

```go
func DateOnOrBefore(field string, date time.Time) Predicate
```

DateOnOrBefore produces field <= $date.

### criteria.Eq

```go
func Eq(field string, value any) Predicate
```

Eq produces field = value.

### criteria.Group

```go
func Group(predicates ...Predicate) Predicate
```

Group wraps one or more predicates in parentheses, producing (pred1 AND/OR pred2 ...).
Use it to isolate OR branches from surrounding AND conditions, e.g.:
Produces: WHERE p.company_id = $1 AND (p.title ILIKE $2 OR p.sku = $3)

TODO: Create tests to validate Group within Group

### criteria.Gt

```go
func Gt(field string, value any) Predicate
```

Gt produces field > value.

### criteria.Gte

```go
func Gte(field string, value any) Predicate
```

Gte produces field >= value.

### criteria.In

```go
func In(field string, values any) Predicate
```

In produces field IN (v1, v2, …).
values must be a slice: []string, []int, []int64, []int32, []float64, or []any.

### criteria.IsFalse

```go
func IsFalse(field string) Predicate
```

IsFalse produces field IS FALSE (no bound parameter).

### criteria.IsNotNull

```go
func IsNotNull(field string) Predicate
```

IsNotNull produces field IS NOT NULL (no bound parameter).

### criteria.IsNull

```go
func IsNull(field string) Predicate
```

IsNull produces field IS NULL (no bound parameter).

### criteria.IsTrue

```go
func IsTrue(field string) Predicate
```

IsTrue produces field IS TRUE (no bound parameter).

### criteria.Like

```go
func Like(field string, value string) Predicate
```

Like performs a case-sensitive LIKE match: field LIKE $n (all databases).

### criteria.Lt

```go
func Lt(field string, value any) Predicate
```

Lt produces field < value.

### criteria.Lte

```go
func Lte(field string, value any) Predicate
```

Lte produces field <= value.

### criteria.Ne

```go
func Ne(field string, value any) Predicate
```

Ne produces field != value.

### criteria.NotContains

```go
func NotContains(field string, value string) Predicate
```

NotContains is the negated form of Contains:
  - PostgreSQL → field NOT ILIKE $n
  - All others → field NOT LIKE $n

### criteria.NotIn

```go
func NotIn(field string, values any) Predicate
```

NotIn produces field NOT IN (v1, v2, …).

### criteria.NotLike

```go
func NotLike(field string, value string) Predicate
```

NotLike performs a case-sensitive NOT LIKE match: field NOT LIKE $n (all databases).

### criteria.Or

```go
func Or() Predicate
```

Or inserts an OR connector between adjacent predicates.

### Predicate.Raw

```go
func (p Predicate) Raw() Predicate
```

Raw marks the field as a trusted SQL expression (e.g. COUNT(*), lower(u.name))
written as is. Trusted input only — never user data.

### Predicate.Substring

```go
func (p Predicate) Substring() Predicate
```

Substring makes a Contains, NotContains, Like or NotLike value literal
text matched anywhere in the field: its wildcards are escaped for the
dialect, it is wrapped in %…% and ESCAPE '!' is appended. Use it for user
input, so "%" or "_" cannot turn a search into a full scan pattern.

## criteria.Query

```go
type Query struct {
	// contains filtered or unexported fields
}
```

Query is a declarative, dialect-aware SQL query builder.

Use From() as the entry point and chain clauses fluently.
Call Build(dialect) to obtain the SQL string and bound parameters for direct execution,
or BuildBase(dialect) when the ORDER BY / LIMIT / OFFSET are handled externally
(e.g., via manager pagination).

Example (direct use):

	q := criteria.From("users", "u").
	    Select("u.id", "u.name", "u.email").
	    LeftJoin("orders", "o", "o.user_id = u.id").
	    Where(
	        criteria.Eq("u.active", true),
	        criteria.And(),
	        criteria.In("u.role", []string{"admin", "moderator"}),
	    ).
	    GroupBy("u.id", "u.name").
	    Having(criteria.Gt("COUNT(o.id)", 0).Raw()).
	    OrderBy(criteria.Asc("u.name"), criteria.Desc("u.id")).
	    Limit(15).Offset(0)

	sql, params, err := q.Build(dialect)

Table, alias, join ON, Select and GroupBy are written verbatim: trusted input
only, never user data. Predicate and order fields are validated as column
references (see Predicate) and may come from a request.

Example (paged via manager — ORDER BY goes in PageRequest, not here):

	sqln.FindFromCriteria[User](ctx,
	    criteria.From("users", "u").
	        Where(criteria.Eq("u.active", true)),
	).WithPage(pageRequest).PagedList()

### criteria.From

```go
func From(table, alias string) *Query
```

From creates a new Query rooted at the given table with an optional alias.
Pass an empty string for alias when none is needed.
Both are written verbatim: trusted input only, never user data.

### Query.Build

```go
func (q *Query) Build(d driver.FilterDialect) (string, []any, error)
```

Build compiles the full query (SELECT … ORDER BY … LIMIT … OFFSET …) into a
SQL string and the corresponding bound parameters, using the provided dialect
for placeholder format and LIKE behaviour.

Pass the returned parameters directly to database/sql query functions.
It fails with ErrInvalidField when a predicate or order field is not a column
reference, so a request-supplied name never reaches the SQL text.

### Query.BuildBase

```go
func (q *Query) BuildBase(d driver.FilterDialect) (string, []any, error)
```

BuildBase compiles the query without ORDER BY, LIMIT, and OFFSET.
Used internally by the manager when dialect-level pagination wraps the base query
(manager.WithPage + PagedList). Prefer Build for direct execution.

### Query.GroupBy

```go
func (q *Query) GroupBy(fields ...string) *Query
```

GroupBy appends fields to the GROUP BY clause.
Fields are written verbatim: trusted input only, never user data.

### Query.Having

```go
func (q *Query) Having(predicates ...Predicate) *Query
```

Having appends predicates to the HAVING clause.
Implicit AND rules are the same as Where.

### Query.Join

```go
func (q *Query) Join(table, alias, on string) *Query
```

Join adds an INNER JOIN clause.
alias may be empty. table, alias and on are written verbatim: trusted input only, never user data.

### Query.LeftJoin

```go
func (q *Query) LeftJoin(table, alias, on string) *Query
```

LeftJoin adds a LEFT JOIN clause; arguments are trusted input only, as in Join.

### Query.LeftJoinLateral

```go
func (q *Query) LeftJoinLateral(subquery, alias string) *Query
```

LeftJoinLateral adds a LEFT JOIN LATERAL clause over a subquery correlated with the
outer row. The ON condition is always TRUE: a lateral subquery expresses its own
correlation in its WHERE, so an outer ON would only ever be redundant.
subquery is the parenthesized SELECT, e.g. "(SELECT x FROM t WHERE t.id = p.id LIMIT 1)".
subquery and alias are written verbatim: trusted input only, never user data.

### Query.Limit

```go
func (q *Query) Limit(n int) *Query
```

Limit sets the maximum number of rows to return (LIMIT n).
Uses standard SQL syntax (PostgreSQL, MySQL).
For Oracle / SQL Server, prefer manager.WithPage + PagedList().

### Query.Offset

```go
func (q *Query) Offset(n int) *Query
```

Offset sets the number of rows to skip before returning results (OFFSET n).

### Query.OrderBy

```go
func (q *Query) OrderBy(orders ...Order) *Query
```

OrderBy appends sort expressions.
Use criteria.Asc(field) and criteria.Desc(field) as constructors; fields that
are not column references fail Build unless marked Raw.

### Query.RightJoin

```go
func (q *Query) RightJoin(table, alias, on string) *Query
```

RightJoin adds a RIGHT JOIN clause; arguments are trusted input only, as in Join.

### Query.Select

```go
func (q *Query) Select(fields ...string) *Query
```

Select specifies the columns included in the SELECT clause.
If never called, SELECT * is generated.
Fields are SQL expressions written verbatim: trusted input only, never user data.

### Query.Where

```go
func (q *Query) Where(predicates ...Predicate) *Query
```

Where appends predicates to the WHERE clause.
Adjacent predicates without an explicit And()/Or() connector are implicitly joined with AND.

