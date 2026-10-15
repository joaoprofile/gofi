# sqln/driver

`import "github.com/joaoprofile/gofi-sdk-go/sqln/driver"` · SDK gofi/v0.2.6 (3df9d20)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	HighlightStart = "\uE000"
	HighlightEnd   = "\uE001"
)
```

Highlight markers wrap the matched words of TextHighlight. They are Unicode
private-use characters, absent from real text, so the markup can be split
from the text and the text escaped before any HTML is written.

```go
const (
	Eq             = "="
	NotEqual       = "!="
	Less           = "<"
	LessOrEqual    = "<="
	Greater        = ">"
	GreaterOrEqual = ">="
)
```

Comparison operators are valid for all databases.

```go
const (
	In    = "IN"
	NotIn = "NOT IN"
)
```

Membership operators test whether a value belongs to a set.

```go
const (
	Contains    = "LIKE"     // case-insensitive via dialect: ILIKE (PG) / LIKE (others)
	NotContains = "NOT LIKE" // case-insensitive NOT via dialect: NOT ILIKE (PG) / NOT LIKE (others)
	Like        = "~~"       // literal case-sensitive LIKE, all databases
	NotLike     = "!~~"      // literal case-sensitive NOT LIKE, all databases
)
```

```go
const (
	IsNull    = "IS NULL"
	IsNotNull = "IS NOT NULL"
)
```

Null Check Operators─
Null check operators do not require a bound value.

```go
const (
	IsTrue  = "IS TRUE"
	IsFalse = "IS FALSE"
)
```

Boolean Check Operators
Boolean check operators do not require a bound value.

```go
const (
	And = "AND"
	Or  = "OR"
)
```

Logical operators separate predicates within a filter list.

```go
const Between = "BETWEEN"
```

Between accepts:
  - a "startRFC3339|endRFC3339" string
  - a []time.Time{start, end}
  - a []any{start, end} where elements are time.Time

```go
const Group = "__group__"
```

Group Operator — internal sentinel used by Group predicate.
Never emitted verbatim; the builder wraps the inner clause in parentheses.

```go
const LikeEscapeChar = "!"
```

LikeEscapeChar is the escape character of the ESCAPE clause written for
literal LIKE matches. Unlike '\', '!' means the same inside a string
literal in every dialect (MySQL escapes '\' unless NO_BACKSLASH_ESCAPES).

```go
const LikeEscapeClause = " ESCAPE '" + LikeEscapeChar + "'"
```

LikeEscapeClause follows a LIKE whose pattern was escaped with EscapeLike.

```go
const Search = "SEARCH"
```

Search matches a full-text index (fts.Index); the dynamic filter accepts
it only on a field mapped to an index.

## Variáveis

```go
var AllowedOperators = map[string]struct{}{
	Eq: {}, NotEqual: {}, Less: {}, LessOrEqual: {}, Greater: {}, GreaterOrEqual: {},
	In: {}, NotIn: {},
	Contains: {}, NotContains: {},
	Like: {}, NotLike: {},
	Search:  {},
	Between: {},
	IsNull:  {}, IsNotNull: {},
	IsTrue: {}, IsFalse: {},
	Group: {},
}
```

AllowedOperators is the authoritative set of valid filter operator strings.
Used by the filter and criteria engines to reject unknown or injected operators.

## Funções

### driver.AccentForms

```go
func AccentForms(r rune) string
```

AccentForms returns the base letter of r and its accented forms ("aàá…"),
or "" when r has none.

### driver.EscapeLike

```go
func EscapeLike(d FilterDialect, s string) string
```

EscapeLike escapes s so that LIKE … ESCAPE '!' matches it literally in d.
Only the escape character, the wildcards and d's extra wildcards are
escaped: Oracle rejects the escape character before anything else.

### driver.FoldAccents

```go
func FoldAccents(s string) string
```

FoldAccents removes the accents of the Latin letters of a lower-case s.

### driver.IsIdentifier

```go
func IsIdentifier(s string) bool
```

IsIdentifier reports whether s is a column reference such as col, t.col,
"Col" or schema.t.col that is safe to write into SQL text from untrusted input.

### driver.IsSortDirection

```go
func IsSortDirection(d string) bool
```

IsSortDirection reports whether d is ASC/DESC with an optional NULLS FIRST/LAST.

### driver.SortDirection

```go
func SortDirection(d string) string
```

SortDirection normalizes d; anything that is not a valid direction falls back to ASC.

## driver.ArrayDialect

```go
type ArrayDialect interface {
	// ArrayMembership renders field IN / NOT IN for an array placeholder.
	ArrayMembership(field, param string, negate bool) string
}
```

ArrayDialect is implemented by dialects that bind a whole slice as one
array parameter, so IN lists keep the same SQL text for any length.

## driver.Dialect

```go
type Dialect interface {
	FilterDialect

	// offset is uint64 because it is page*limit: two uint16 multiplied overflow
	// past 65535 and silently wrap to a different page.
	BuildPagination(query string, order string, limit uint16, offset uint64) string

	BuildCount(query string) string
}
```

## driver.FilterDialect

```go
type FilterDialect interface {
	Param(index int) string
	Like(field string, param string) string
	NotLike(field string, param string) string
}
```

## driver.FullTextDialect

```go
type FullTextDialect interface {
	// TextSearchValue renders q as the search string of TextMatch; ok is
	// false when nothing searchable remains.
	TextSearchValue(idx *TextIndex, q TextQuery) (value string, ok bool)
	// TextMatch renders the condition matching q against idx, whose columns
	// are qualified by qualifier (an alias or table, may be empty). bind
	// binds a search string and returns its placeholder.
	TextMatch(idx *TextIndex, qualifier string, q TextQuery, bind func(any) string) (expr string, ok bool)
	// TextRank renders the relevance of q in idx; higher is better.
	TextRank(idx *TextIndex, qualifier string, q TextQuery, bind func(any) string) (expr string, ok bool)
	// TextHighlight renders the whole text of column, one of idx's, with
	// the words q matches wrapped in HighlightStart and HighlightEnd.
	TextHighlight(idx *TextIndex, qualifier, column string, q TextQuery, bind func(any) string) (expr string, ok bool)
	// TextIndexDDL renders the steps creating and dropping idx; comment is
	// stored on the index, where TextIndexLookup reads it back.
	TextIndexDDL(idx *TextIndex, comment string) ([]TextStep, error)
	// TextIndexLookup selects the comment of idx, or no row when idx does
	// not exist, usable, with the expected kind.
	TextIndexLookup(idx *TextIndex) (query string, args []any)
	// TextChecks lists the requirements of idx beyond the index itself.
	TextChecks(idx *TextIndex) []TextCheck
	// TextCandidates selects word, term and ndoc of up to 50 lexicon entries
	// whose term resembles key, through the lexicon's similarity index.
	TextCandidates(idx *TextIndex, key string, bind func(any) string) string
}
```

FullTextDialect is implemented by dialects with full-text search.

## driver.LikeWildcards

```go
type LikeWildcards interface {
	LikeWildcards() string
}
```

LikeWildcards is implemented by dialects whose LIKE has wildcard
characters besides % and _, such as SQL Server's [...] classes.

## driver.TextCheck

```go
type TextCheck struct {
	Query string
	Args  []any
	Want  string
	What  string // describes the requirement in errors
	// Index marks a part of the index itself (e.g. a MySQL weight group
	// index): a mismatch means the index is missing, not misconfigured.
	Index bool
}
```

TextCheck is a catalog query whose single string result must equal Want
for the index to be usable, e.g. a server setting the index depends on.

## driver.TextColumn

```go
type TextColumn struct {
	Name   string
	Weight TextWeight
}
```

TextColumn is one column of a full-text index.

## driver.TextGroup

```go
type TextGroup []TextTerm
```

TextGroup is a conjunction: every positive term must match, no negated term may.

## driver.TextIndex

```go
type TextIndex struct {
	Name         string
	Table        string // table or schema.table
	Columns      []TextColumn
	Language     string // text search language, e.g. "portuguese"
	Unaccent     bool   // fold accents (PostgreSQL unaccent, MySQL _ai_ collation)
	Strategy     TextStrategy
	VectorColumn string // PostgreSQL StoredVector: the generated column
	MinTokenSize int    // MySQL: innodb_ft_min_token_size the index was built with
	Concurrently bool   // build without blocking writes (PostgreSQL)
	Suggest      bool   // keep a lexicon of the indexed words for spelling suggestions
}
```

TextIndex is the validated definition of a full-text index. The same
definition renders the DDL and every query on it, so a query always reads
the expression or column list its index was built on. Build it with
fts.NewIndex, never by hand.

### TextIndex.LexiconTable

```go
func (idx *TextIndex) LexiconTable() string
```

LexiconTable is the table of the indexed words of idx (Suggest), next to
the indexed table: term (the folded word, key), word, ndoc (rows holding it).

### TextIndex.WeightGroups

```go
func (idx *TextIndex) WeightGroups() [][]TextColumn
```

WeightGroups returns the columns by effective weight, A first, or nil when
every column has the same weight.

## driver.TextQuery

```go
type TextQuery []TextGroup
```

TextQuery is a parsed search in disjunctive form: a row matches when any
group matches. Words carry no operator characters, so rendering them into
a search string cannot change its structure.

## driver.TextStep

```go
type TextStep struct {
	Up, Down []string
	Isolated bool
}
```

TextStep is a group of DDL statements that run together. An Isolated
step holds statements that cannot run inside a transaction block, such as
CREATE INDEX CONCURRENTLY: each gets a migration file of its own.

## driver.TextStrategy

```go
type TextStrategy uint8
```

TextStrategy selects how PostgreSQL materializes the searched document.

```go
const (
	// StoredVector adds a generated tsvector column and indexes it: ranking
	// reads the stored vector instead of parsing the text again per row.
	StoredVector TextStrategy = iota
	// ExpressionIndex indexes the tsvector expression itself: no new column
	// and no table rewrite, but ranking recomputes the vector per row.
	ExpressionIndex
)
```

## driver.TextTerm

```go
type TextTerm struct {
	Words  []string // lower-case letters and digits only
	Prefix bool     // matches words starting with the (single) word
	Negate bool     // excludes rows containing the term
	// Near, for two words, matches them at most Near positions apart, in
	// either order, instead of as an exact phrase.
	Near int
	// Synonyms are alternatives to the (single) word, each a word or a
	// phrase: the term matches when any of them does.
	Synonyms [][]string
}
```

TextTerm is one word or, with several words, one phrase of a search.

## driver.TextWeight

```go
type TextWeight byte
```

TextWeight ranks the matches of one indexed column; A ranks highest. The
zero value leaves the column unweighted, which ranks as D.

```go
const (
	WeightA TextWeight = 'A'
	WeightB TextWeight = 'B'
	WeightC TextWeight = 'C'
	WeightD TextWeight = 'D'
)
```

### TextWeight.Effective

```go
func (w TextWeight) Effective() TextWeight
```

Effective is w, or D for an unweighted column.

### TextWeight.Factor

```go
func (w TextWeight) Factor() string
```

Factor is the share of a match in a column of weight w, the defaults of
PostgreSQL's ts_rank ({0.1, 0.2, 0.4, 1.0} for D, C, B, A); dialects that
emulate weights use the same factors so both rank alike.

