# sqln/fts

`import "github.com/joaoprofile/gofi-sdk-go/sqln/fts"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package fts defines full-text indexes and the searches that read them.

An Index is the single definition of a full-text index: the migration that
creates it and every query that searches it are rendered from it, so the
searched expression (PostgreSQL) or column list (MySQL) is always the one
the index was built on. There is no full-text API taking a column name: a
search exists only on an Index, and before a search runs, the connection
checks that the index exists in the database as defined (see Verify).

	var ProductSearch = fts.MustIndex("products", fts.Spec{
	    Name:     "ftx_products_search",
	    Columns:  []fts.Column{{Name: "name", Weight: fts.A}, {Name: "description", Weight: fts.B}},
	    Language: fts.Portuguese,
	    Unaccent: true,
	})

	criteria.From("products", "p").
	    Where(criteria.Match(ProductSearch.Search(term).Mode(fts.Prefix)))

## Constantes

```go
const (
	A = driver.WeightA
	B = driver.WeightB
	C = driver.WeightC
	D = driver.WeightD
)
```

```go
const (
	// StoredVector (default) adds a generated tsvector column and indexes it.
	// Ranking is cheap; adding the column rewrites the table.
	StoredVector = driver.StoredVector
	// ExpressionIndex indexes the tsvector expression: no column and no
	// rewrite, but ranking parses the text again for every matched row.
	ExpressionIndex = driver.ExpressionIndex
)
```

```go
const (
	MaxTermLength = 256 // characters
	MaxTerms      = 16  // words and phrases
	MaxNear       = 10  // positions of a "two words"~N proximity search
)
```

Bounds of a search string.

```go
const DefaultMinTokenSize = 3
```

DefaultMinTokenSize is MySQL's default innodb_ft_min_token_size.

```go
const (

	// SuggestThreshold is the least trigram similarity of a suggestion,
	// pg_trgm's default similarity_threshold.
	SuggestThreshold = 0.3
)
```

Lexicon bounds: shorter words are not corrected nor suggested, longer
ones do not fit the lexicon.

## Variáveis

```go
var (
	// ErrInvalidIndex reports an Index definition that cannot be built.
	ErrInvalidIndex = errors.New("fts: invalid index")
	// ErrInvalidSearch reports a search above the length or term bounds.
	ErrInvalidSearch = errors.New("fts: invalid search")
	// ErrEmptySearch reports a search with no searchable word left, e.g. only
	// punctuation, stopwords or words shorter than the MySQL token size.
	// Callers usually skip the search filter or return an empty result.
	ErrEmptySearch = errors.New("fts: empty search")
	// ErrUnsupported reports a dialect without full-text search.
	ErrUnsupported = errors.New("fts: full-text search not supported by the dialect")
	// ErrIndexMissing reports an index absent from the database: apply the
	// migration rendered by Migration.
	ErrIndexMissing = errors.New("fts: full-text index missing")
	// ErrIndexOutdated reports an index created from a different definition:
	// the Index changed and its migration was not applied.
	ErrIndexOutdated = errors.New("fts: full-text index outdated")
	// ErrServerConfig reports a server setting the index depends on, such as
	// MySQL's innodb_ft_min_token_size, differing from the definition.
	ErrServerConfig = errors.New("fts: server configuration does not match the index")
)
```

## Funções

### fts.Apply

```go
func Apply(ctx context.Context, p Policy, db Querier, d driver.Dialect, indexes ...*Index) error
```

Apply runs policy p on the indexes: Check, Create or nothing.

### fts.Check

```go
func Check(ctx context.Context, db Querier, d driver.Dialect, indexes ...*Index) error
```

Check verifies that each index exists in db as defined in d, and that the
server settings it depends on match.

### fts.Create

```go
func Create(ctx context.Context, db Querier, d driver.Dialect, indexes ...*Index) error
```

Create creates each missing index with the statements of Migration, then
checks all of them; an outdated index is reported, never dropped.

### fts.Migration

```go
func Migration(d driver.Dialect, indexes ...*Index) (up, down string, err error)
```

Migration renders the indexes as the bodies of a single up and down
migration file, run by sqln/migrate like any other (on MySQL, connection
enables multiStatements for the migration pool). A Concurrently index
needs several files: use Migrations or WriteMigration.

### fts.RefreshLexicon

```go
func RefreshLexicon(ctx context.Context, db *sql.DB, d driver.Dialect, idx *Index) (int, error)
```

RefreshLexicon rebuilds the lexicon of idx (Spec.Suggest) from the indexed
columns: every word of 3 to 64 characters, keyed without accents, with the
number of rows holding it. It reads the whole table, so run it from a job,
after bulk loads or periodically, not per request. Both databases get the
same lexicon: the words are split by the same rules as searches.

### fts.Statements

```go
func Statements(d driver.Dialect, indexes ...*Index) (up, down []string, err error)
```

Statements renders the statements creating (up) and dropping (down) the
indexes in d, in order; down undoes up in reverse.

### fts.WriteMigration

```go
func WriteMigration(dir string, version uint64, name string, d driver.Dialect, indexes ...*Index) error
```

WriteMigration writes <version>_<name>.up.sql and .down.sql into dir, the
version zero-padded to six digits, in the golang-migrate layout read by
sqln/migrate; when Migrations renders several files, the next ones take
the following versions and the names <name>_2, <name>_3…. Call it from a
generator (go:generate) or a test, and commit the files: the database gets
the index through the normal migration path. Existing files are replaced.

## fts.Column

```go
type Column struct {
	Name   string
	Weight Weight
}
```

Column is one indexed column.

## fts.File

```go
type File struct {
	Up, Down string
}
```

File is the body of one up and one down migration file.

### fts.Migrations

```go
func Migrations(d driver.Dialect, indexes ...*Index) ([]File, error)
```

Migrations renders the migration files of the indexes in d: consecutive
statements share a file, and each statement that cannot run inside a
transaction block (CREATE INDEX CONCURRENTLY) gets its own, since a
migration file runs as one.

## fts.Fragment

```go
type Fragment struct {
	Text  string
	Match bool
}
```

Fragment is a run of text, matched or not.

## fts.Highlight

```go
type Highlight string
```

Highlight is a text whose matched words are wrapped in the private-use
markers driver.HighlightStart and driver.HighlightEnd, as
criteria.Query.SelectHighlight returns it. Scan it into a field of this
type and render it with HTML or Fragments: never write it to a page as is.
The text comes from the database unescaped, so it may hold markup.

### Highlight.Excerpt

```go
func (h Highlight) Excerpt(maxWords int) Highlight
```

Excerpt cuts h to about maxWords words around its first match (the start
of the text when nothing matched), marking each cut with "…". Whitespace
collapses to single spaces.

### Highlight.Fragments

```go
func (h Highlight) Fragments() []Fragment
```

Fragments splits h into matched and unmatched runs, without markers.

### Highlight.HTML

```go
func (h Highlight) HTML() string
```

HTML escapes the text and wraps each match in <mark>…</mark>: the only
markup in the result is the one HTML adds.

### Highlight.Text

```go
func (h Highlight) Text() string
```

Text returns h without markers.

## fts.Index

```go
type Index struct {
	// contains filtered or unexported fields
}
```

Index is a validated full-text index definition. Declare each one once
with MustIndex, as a package-level variable (in the repository that
searches it, for instance), and share it between the migration and the
queries.

### fts.MustIndex

```go
func MustIndex(table string, spec Spec) *Index
```

MustIndex declares an index: it validates spec like NewIndex and registers
the index (see Registered), so the service checks it at startup. Call it
at package level; it panics on an invalid spec, and on a table and name
already declared with another definition.

### fts.NewIndex

```go
func NewIndex(table string, spec Spec) (*Index, error)
```

NewIndex validates spec for table (name or schema.name, unquoted). It does
not register the index (see MustIndex): use it for indexes built at run
time and in tests.

### fts.Registered

```go
func Registered() []*Index
```

Registered returns the indexes declared with MustIndex in the packages
linked into the binary, sorted by table and name. gofi's database
component checks them when the service starts.

### Index.Comment

```go
func (i *Index) Comment(d driver.Dialect) (string, error)
```

Comment returns the comment the migration stores on the index for d: a
hash of its DDL, so Check detects an index built from another definition.
Concurrently only changes how the index is built, not what it holds, so
it is left out of the hash.

### Index.Definition

```go
func (i *Index) Definition() *driver.TextIndex
```

Definition returns a copy of the validated definition the dialects render.

### Index.Name

```go
func (i *Index) Name() string
```

Name returns the index name.

### Index.Search

```go
func (i *Index) Search(term string) Search
```

Search returns a Websearch over the index for term, which may come from a
request: it is always bound as a parameter and reduced to plain words.

### Index.Table

```go
func (i *Index) Table() string
```

Table returns the indexed table as given to NewIndex.

## fts.Language

```go
type Language string
```

Language selects stopwords and, on PostgreSQL, stemming. MySQL has no
stemmer: searches drop the language's stopwords (PostgreSQL's lists), so
both ignore the same words, but "cafés" does not find "café" there.

```go
const (
	Simple     Language = "simple" // no stemming, no stopwords
	Portuguese Language = "portuguese"
	English    Language = "english"
	Spanish    Language = "spanish"
	French     Language = "french"
	German     Language = "german"
	Italian    Language = "italian"
	Dutch      Language = "dutch"
)
```

## fts.Mode

```go
type Mode uint8
```

Mode selects how a search string is read.

```go
const (
	// Websearch (default): words must all match; "quoted text" is a phrase,
	// "two words"~N finds them at most N positions apart in either order
	// (N up to MaxNear), -word or -"phrase" excludes, and OR separates
	// alternatives. Words of a Spec.Synonyms group match each other.
	Websearch Mode = iota
	// AllTerms: every word must match; no operators.
	AllTerms
	// AnyTerm: at least one word must match; no operators.
	AnyTerm
	// Prefix: every word must match as a word prefix, for search-as-you-type.
	Prefix
	// Phrase: the words must appear together, in order.
	Phrase
)
```

## fts.Policy

```go
type Policy uint8
```

Policy decides what a connection does before running a search on an index.

```go
const (
	// Verify (default) checks the catalog once per index and connection and
	// fails the search with ErrIndexMissing, ErrIndexOutdated or
	// ErrServerConfig when the index is not there as defined.
	Verify Policy = iota
	// Ensure creates a missing index, then verifies it. Meant for development
	// and tests; production indexes belong in migrations (see Migration).
	Ensure
	// Trust runs searches without checking, e.g. against a mocked database.
	Trust
)
```

## fts.Querier

```go
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
```

Querier is the part of *sql.DB, *sql.Tx and *sql.Conn the checks and
suggestions use.

## fts.Search

```go
type Search struct {
	// contains filtered or unexported fields
}
```

Search is a search string on an Index. Build it with Index.Search and pass
it to criteria.Match, criteria.ByRelevance or Query.SelectRelevance.

### fts.Suggest

```go
func Suggest(ctx context.Context, db Querier, d driver.Dialect, s Search) (Search, bool, error)
```

Suggest corrects the words of s missing from the lexicon of its index
(Spec.Suggest) with the most similar indexed word, and reports whether it
changed any. Show the corrected Search's Term ("did you mean …") and run
it, typically when s found nothing. Only positive, whole words of 3 or more
characters are corrected; the rest of the search string is kept as typed.
The database proposes candidates through the lexicon's similarity index
(pg_trgm on PostgreSQL, ngram on MySQL) and the choice is the same on
both: the highest trigram similarity, as pg_trgm computes it, of at least
SuggestThreshold, then the most frequent word.

### Search.Index

```go
func (s Search) Index() *Index
```

Index returns the searched index.

### Search.Mode

```go
func (s Search) Mode(m Mode) Search
```

Mode returns s read in mode m.

### Search.On

```go
func (s Search) On(qualifier string) Search
```

On qualifies the indexed columns with an alias. criteria resolves it from
the query's FROM and JOIN tables; set it when the table is joined twice or
when building a bare clause (criteria.BuildClause). Trusted input only.

### Search.Qualifier

```go
func (s Search) Qualifier() string
```

Qualifier returns the alias set by On.

### Search.Query

```go
func (s Search) Query() (driver.TextQuery, error)
```

Query parses the search string. It fails with ErrInvalidSearch above the
bounds and with ErrEmptySearch when no word remains.

### Search.Term

```go
func (s Search) Term() string
```

Term returns the search string.

### Search.Value

```go
func (s Search) Value(d driver.FilterDialect) (string, error)
```

Value renders the search string bound to the match and rank expressions of d.

## fts.Spec

```go
type Spec struct {
	// Name of the index: lower-case letters, digits and '_', at most 59
	// characters. It must be unique in the schema.
	Name    string
	Columns []Column
	// Language defaults to Simple.
	Language Language
	// Unaccent folds accents: PostgreSQL through the unaccent extension,
	// which the migration creates; MySQL through the column collation, which
	// the check requires to be accent-insensitive (_ai_, or a legacy _ci).
	Unaccent bool
	// Strategy defaults to StoredVector.
	Strategy Strategy
	// VectorColumn names the StoredVector column; defaults to Name + "_tsv".
	VectorColumn string
	// MinTokenSize is the innodb_ft_min_token_size of the MySQL server,
	// DefaultMinTokenSize when zero. Shorter words are dropped from searches:
	// MySQL would otherwise require a word it never indexed and match nothing.
	MinTokenSize int
	// Synonyms lists groups of equivalent words or phrases, e.g.
	// {"notebook", "laptop"}: a searched word also matches the other members
	// of its group, on both databases. They apply to queries only, so
	// changing them needs no migration.
	Synonyms [][]string
	// Suggest keeps a lexicon of the indexed words (table <Name>_lex, created
	// by the migration and filled by RefreshLexicon), from which Suggest
	// corrects misspelled searches. Name is then limited to 58 characters.
	Suggest bool
	// Concurrently builds the index without blocking writes (PostgreSQL
	// CREATE INDEX CONCURRENTLY, in a migration file of its own). It needs
	// ExpressionIndex, since StoredVector rewrites the table anyway. MySQL
	// cannot: rendering its migration fails.
	Concurrently bool
}
```

Spec describes a full-text index.

## fts.Strategy

```go
type Strategy = driver.TextStrategy
```

Strategy selects how PostgreSQL materializes the searched document.

## fts.Weight

```go
type Weight = driver.TextWeight
```

Weight ranks the matches of a column; A ranks highest, an unweighted
column ranks as D. PostgreSQL labels the vector; MySQL adds one FULLTEXT
index per weight group and sums their relevance with the same factors.

