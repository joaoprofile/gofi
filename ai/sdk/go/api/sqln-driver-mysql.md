# sqln/driver/mysql

`import "github.com/joaoprofile/gofi-sdk-go/sqln/driver/mysql"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Variáveis

```go
var ErrConcurrentIndex = errors.New("mysql: a FULLTEXT index cannot be added without blocking writes; schedule the migration or drop Concurrently")
```

ErrConcurrentIndex reports a full-text index asked to build without
blocking writes, which InnoDB cannot do.

## mysql.Driver

```go
type Driver struct{}
```

MySQL driver. To enable it, blank-import this package:

	import _ "github.com/joaoprofile/gofi-sdk-go/sqln/driver/mysql"

It links go-sql-driver/mysql and registers the MySQL migrations.

### Driver.DSN

```go
func (Driver) DSN(s connection.Settings) string
```

DSN builds a go-sql-driver/mysql DSN: user:pass@tcp(host:port)/dbname?tls=….
The database name is path-escaped and the driver splits the DSN at the
last '@' and '/', so the password may hold any character. The tls
parameter follows s.EffectiveSSLMode(): disable → false, prefer →
preferred, require → skip-verify, anything else → true (verify-full).

### Driver.Dialect

```go
func (Driver) Dialect() sqln_driver.Dialect
```

### Driver.MigrationConfig

```go
func (Driver) MigrationConfig(cfg connection.Config) connection.Config
```

MigrationConfig enables multiStatements on the migration pool only: the
query pools keep it off, so an injected "; …" can never run a second
statement there.

### Driver.Name

```go
func (Driver) Name() connection.DriverName
```

### Driver.Open

```go
func (Driver) Open(cfg connection.Config) (*sql.DB, error)
```

### Driver.ParseError

```go
func (Driver) ParseError(err error) error
```

ParseError hides the server message behind a generic one (see connection.Error).

### Driver.ValidateSettings

```go
func (Driver) ValidateSettings(s connection.Settings) error
```

ValidateSettings rejects what the DSN cannot express. A custom CA or client
certificate needs a tls.Config registered with mysql.RegisterTLSConfig and
a hand-built DSN with tls=<name>.

## mysql.MigrateDriver

```go
type MigrateDriver struct{}
```

### MigrateDriver.Instance

```go
func (MigrateDriver) Instance(db *sql.DB) (database.Driver, error)
```

Instance needs a pool opened with multiStatements=true, as
Driver.MigrationConfig sets, to run files holding several statements.

### MigrateDriver.Name

```go
func (MigrateDriver) Name() string
```

## mysql.MySQLDialect

```go
type MySQLDialect struct{}
```

### MySQLDialect.BuildCount

```go
func (MySQLDialect) BuildCount(query string) string
```

### MySQLDialect.BuildPagination

```go
func (MySQLDialect) BuildPagination(query string, order string, limit uint16, offset uint64) string
```

### MySQLDialect.Like

```go
func (MySQLDialect) Like(field string, param string) string
```

### MySQLDialect.NotLike

```go
func (MySQLDialect) NotLike(field string, param string) string
```

### MySQLDialect.Param

```go
func (MySQLDialect) Param(_ int) string
```

### MySQLDialect.TextCandidates

```go
func (MySQLDialect) TextCandidates(idx *driver.TextIndex, key string, bind func(any) string) string
```

TextCandidates uses the lexicon's ngram FULLTEXT index: terms sharing the
most n-grams with key come first.

### MySQLDialect.TextChecks

```go
func (MySQLDialect) TextChecks(idx *driver.TextIndex) []driver.TextCheck
```

TextChecks requires the token size searches assume when dropping words,
the weight group indexes TextRank reads and, with Unaccent, accent-
insensitive collations: MySQL folds accents only through the collation.

### MySQLDialect.TextHighlight

```go
func (MySQLDialect) TextHighlight(idx *driver.TextIndex, qualifier, column string, q driver.TextQuery, bind func(any) string) (string, bool)
```

TextHighlight wraps the matched words with REGEXP_REPLACE: MySQL has no
headline function. The pattern is built from the searched words (letters,
digits and marks only), each letter widened to its accented forms, as the
_ai_ collation compares them; whole words, a prefix marking the word it
starts. There is no
stemming, as in the search itself. Like the search, it skips negated
terms and the words InnoDB never matches.

### MySQLDialect.TextIndexDDL

```go
func (MySQLDialect) TextIndexDDL(idx *driver.TextIndex, comment string) ([]driver.TextStep, error)
```

TextIndexDDL adds the FULLTEXT index searched by TextMatch and, with
column weights, one index per weight group for TextRank. InnoDB builds one
FULLTEXT index per statement.

### MySQLDialect.TextIndexLookup

```go
func (MySQLDialect) TextIndexLookup(idx *driver.TextIndex) (string, []any)
```

TextIndexLookup finds a FULLTEXT index of that name on the table, in the
table's schema or the current database.

### MySQLDialect.TextMatch

```go
func (d MySQLDialect) TextMatch(idx *driver.TextIndex, qualifier string, q driver.TextQuery, bind func(any) string) (string, bool)
```

### MySQLDialect.TextRank

```go
func (d MySQLDialect) TextRank(idx *driver.TextIndex, qualifier string, q driver.TextQuery, bind func(any) string) (string, bool)
```

TextRank is the relevance MATCH returns. With column weights, it sums the
relevance in each weight group's own index times the weight's factor.

### MySQLDialect.TextSearchValue

```go
func (MySQLDialect) TextSearchValue(idx *driver.TextIndex, q driver.TextQuery) (string, bool)
```

TextSearchValue renders q in boolean mode syntax: +word, -word, word*,
"phrase" and (group) (group) for alternatives. Words hold only letters,
digits and marks, so no operator can come from the search string.

