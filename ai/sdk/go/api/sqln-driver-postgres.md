# sqln/driver/postgres

`import "github.com/joaoprofile/gofi-sdk-go/sqln/driver/postgres"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package postgres registers the PostgreSQL driver, built on pgx/v5.

## Variáveis

```go
var ErrPasswordFuncNeedsVerifiedTLS = errors.New("postgres: Config.Password requires sslmode verify-ca or verify-full")
```

ErrPasswordFuncNeedsVerifiedTLS is returned by Open when Config.Password
(an IAM token) would travel without a verified TLS connection.

## postgres.Driver

```go
type Driver struct{}
```

### Driver.DSN

```go
func (Driver) DSN(s connection.Settings) string
```

DSN builds a key-value connection string with every value quoted, so no
value can add keys. sslmode is s.EffectiveSSLMode(): verify-full unless the
host is local. StatementTimeout becomes the statement_timeout runtime
parameter (not accepted by PgBouncer in transaction mode).

### Driver.Dialect

```go
func (Driver) Dialect() driver.Dialect
```

### Driver.Name

```go
func (Driver) Name() connection.DriverName
```

### Driver.Open

```go
func (Driver) Open(cfg connection.Config) (*sql.DB, error)
```

Open returns a pgx-backed *sql.DB. Unless the DSN sets
default_query_exec_mode, queries use cache_describe: one round trip without
named prepared statements, so PgBouncer in transaction mode keeps working.
With Config.Password the DSN must verify the server certificate: an IAM
token sent to an unverified server can be replayed by whoever receives it.

### Driver.ParseError

```go
func (Driver) ParseError(err error) error
```

ParseError hides the server message behind a generic one (see connection.Error).

## postgres.MigrateDriver

```go
type MigrateDriver struct{}
```

### MigrateDriver.Instance

```go
func (MigrateDriver) Instance(db *sql.DB) (database.Driver, error)
```

### MigrateDriver.Name

```go
func (MigrateDriver) Name() string
```

## postgres.PostgresDialect

```go
type PostgresDialect struct{}
```

### PostgresDialect.ArrayMembership

```go
func (PostgresDialect) ArrayMembership(field, param string, negate bool) string
```

ArrayMembership binds IN lists as one array: the SQL text is the same for
any list length. <> ALL keeps NOT IN's NULL semantics.

### PostgresDialect.BuildCount

```go
func (PostgresDialect) BuildCount(query string) string
```

COUNT(*) rather than COUNT(tb.*): both count the same rows (an all-NULL row
included), but naming the composite row forces the planner to materialize it,
and it then gives up eliminating the unique-key LEFT JOINs nothing projects.

### PostgresDialect.BuildPagination

```go
func (PostgresDialect) BuildPagination(query string, order string, limit uint16, offset uint64) string
```

### PostgresDialect.Like

```go
func (PostgresDialect) Like(field string, param string) string
```

### PostgresDialect.NotLike

```go
func (PostgresDialect) NotLike(field string, param string) string
```

### PostgresDialect.Param

```go
func (PostgresDialect) Param(index int) string
```

### PostgresDialect.TextCandidates

```go
func (PostgresDialect) TextCandidates(idx *driver.TextIndex, key string, bind func(any) string) string
```

TextCandidates uses pg_trgm's % operator, served by the GIN trigram index
(pg_trgm.similarity_threshold, 0.3 by default).

### PostgresDialect.TextChecks

```go
func (PostgresDialect) TextChecks(idx *driver.TextIndex) []driver.TextCheck
```

TextChecks requires the lexicon of Suggest; the configuration and the
extensions are dependencies of the index or lexicon, which cannot exist
without them.

### PostgresDialect.TextHighlight

```go
func (d PostgresDialect) TextHighlight(idx *driver.TextIndex, qualifier, column string, q driver.TextQuery, bind func(any) string) (string, bool)
```

TextHighlight uses ts_headline with the index configuration, so it marks
what the search matches: stems ("cafés" for "cafe") and folded accents.
PostgreSQL evaluates it after ORDER BY … LIMIT: only the page pays for it.

### PostgresDialect.TextIndexDDL

```go
func (PostgresDialect) TextIndexDDL(idx *driver.TextIndex, comment string) ([]driver.TextStep, error)
```

TextIndexDDL creates, for Unaccent, the extension and a configuration
copied from the language whose words pass through unaccent before the
stemmer; the configuration is shared and kept on down. A Concurrently
index is built in an isolated step: CREATE INDEX CONCURRENTLY cannot run
inside a transaction block, so its comment follows in a step of its own.

### PostgresDialect.TextIndexLookup

```go
func (PostgresDialect) TextIndexLookup(idx *driver.TextIndex) (string, []any)
```

TextIndexLookup finds a valid GIN index of that name on the table,
resolved through the search_path like the queries are. A CONCURRENTLY
build that failed leaves an invalid index, which reads as missing.

### PostgresDialect.TextMatch

```go
func (d PostgresDialect) TextMatch(idx *driver.TextIndex, qualifier string, q driver.TextQuery, bind func(any) string) (string, bool)
```

### PostgresDialect.TextRank

```go
func (d PostgresDialect) TextRank(idx *driver.TextIndex, qualifier string, q driver.TextQuery, bind func(any) string) (string, bool)
```

TextRank uses cover density, which rewards matched words close together;
the column weights apply through the setweight labels of the document.

### PostgresDialect.TextSearchValue

```go
func (PostgresDialect) TextSearchValue(_ *driver.TextIndex, q driver.TextQuery) (string, bool)
```

TextSearchValue renders q in to_tsquery syntax. Words hold only letters,
digits and marks, so quoting them as 'word' is enough; stopwords are left
to to_tsquery, which drops them.

