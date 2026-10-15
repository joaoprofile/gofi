---
name: full-text-search
description: Busca full-text no sqln (PostgreSQL e MySQL) — fts.MustIndex, migration gerada, criteria.Match/ByRelevance, SEARCH no filtro dinâmico, sugestão "você quis dizer", destaque seguro, política de conferência
sdk: v0.2.6
keywords: [full-text, fts, MustIndex, Spec, Match, ByRelevance, SelectRelevance, SelectHighlight, Highlight, Suggest, RefreshLexicon, tsvector, FULLTEXT, ErrIndexMissing, ErrEmptySearch, DATABASE_FULLTEXT_POLICY, busca]
---

# Busca full-text — `sqln/fts`

API: `.claude/sdk/go/api/sqln-fts.md`, `sqln-criteria.md`, `sqln-connection.md`,
`sqln.md` (`Match`, `ByRelevance`, `SuggestSearch`). Exemplo executável (job em
PostgreSQL, com a saída esperada): `examples/sqln/fulltext` — ver
`.claude/sdk/go/api/examples.md`.

## Regra — a busca só existe sobre um `fts.Index`

Não há API de full-text que receba nome de coluna. O índice é declarado **uma
vez**, em variável de pacote, no pacote que busca (o repositório); a migration
e toda query saem dessa mesma definição.

```go
var productSearch = fts.MustIndex("products", fts.Spec{
    Name: "ftx_products_search", // minúsculas, dígitos, '_' (máx. 59); único no schema
    Columns: []fts.Column{
        {Name: "name", Weight: fts.A},        // A > B > C > D; sem peso = D
        {Name: "description", Weight: fts.B},
    },
    Language: fts.Portuguese, // stopwords (+ stemming no PostgreSQL)
    Unaccent: true,           // "cafe" acha "café"
})
```

- `fts.MustIndex` valida **e registra** o índice (panic em spec inválida ou em
  declaração dupla com definições diferentes). `fts.NewIndex` só valida: use em
  índice montado em runtime e em teste.
- Nome de índice, tabela e colunas são identificadores simples; o termo
  buscado nunca é identificador — vai sempre como parâmetro.
- Campos úteis do `fts.Spec`: `Strategy` (`StoredVector` default, ou
  `ExpressionIndex` sem reescrever a tabela), `Synonyms` (grupos de
  equivalentes, só afetam query: mudar não pede migration), `Suggest`
  (lexicon para "você quis dizer"), `MinTokenSize` (MySQL
  `innodb_ft_min_token_size`), `Concurrently` (PostgreSQL, exige
  `ExpressionIndex`).

## Migration — renderizada, conferida por teste

```go
//go:generate go test -run TestMigrationIsCurrent -update .
up, down, err := fts.Migration(postgres.PostgresDialect{}, productSearch)
err = fts.WriteMigration(".migrations", 2, "products_search", dialect, productSearch)
```

- `fts.Migration` / `fts.Migrations` / `fts.WriteMigration` geram os arquivos
  `NNNNNN_*.up.sql`/`.down.sql` (um por passo; `Concurrently` sai em arquivo
  próprio). **Não se edita à mão.**
- Um teste do pacote compara `.migrations/` com `fts.Migration` e falha quando
  a definição mudou e a migration não (`go generate ./...` reescreve).
- O índice recebe um comentário `gofi-fts:v1:<hash do DDL>` — é com ele que a
  conferência detecta índice construído de outra definição.
- `fts.Migration` precisa do dialeto do banco do serviço
  (`postgres.PostgresDialect{}` ou `mysql.MySQLDialect{}`).

## Conferência — o serviço não sobe com índice faltando

| Quando | Falha | Como |
|---|---|---|
| `go test` | índice mudou e a migration não | teste de migration atual |
| `Build` do serviço | migration não aplicada ou de outra definição | `database.New()` lê o comentário de cada índice registrado logo após as migrations: `fts.ErrIndexMissing` / `fts.ErrIndexOutdated` / `fts.ErrServerConfig` derrubam o `Build` |
| 1ª busca | o mesmo, para índice criado em runtime | a conexão confere cada índice uma vez antes de buscar; falha é reconferida (aplicar a migration conserta sem reiniciar) |

`DATABASE_FULLTEXT_POLICY`: `verify` (default), `ensure` (cria o índice que
falta — só dev/teste; **recusado em `stage`/`prod`**) ou `trust` (não confere;
banco mockado). Em código: `connection.FullTextConfig{Policy, Indexes}` e
`connection.WithFullTextPolicy(fts.Trust)`.

Vários bancos: `database.New(database.WithFullText(idx...))` confere só os
índices dados. Query escrita à mão que busca num índice declara
`.RequireTextIndex(idx)` no `sqln.Find`.

## Buscar — `criteria.Match`, ordenar por relevância, paginar

```go
s := productSearch.Search(term).Mode(fts.Websearch) // Mode é opcional
q := criteria.From("products", "p").
    Select("p.id", "p.name").
    SelectRelevance(s, "relevance").
    Where(criteria.IsTrue("p.active"), criteria.Match(s))

order := []pagination.Sort{
    pagination.NewSort("relevance", pagination.DESC), // alias do SelectRelevance
    pagination.NewSort("p.id", pagination.ASC),       // desempate estável
}
page, err := sqln.FindFromCriteria[Hit](ctx, q).
    WithPage(pagination.NewPageRequest(0, 20, order)).PagedList()
if errors.Is(err, fts.ErrEmptySearch) { // só stopword/pontuação: página vazia, não erro
    return &sqln.Page[Hit]{}, nil
}
```

- `criteria.ByRelevance(s)` ordena direto pela relevância (sem coluna no
  SELECT); `SelectRelevance(s, alias)` expõe o valor para ordenar por alias e
  devolver ao cliente. Os valores **não são comparáveis entre bancos** — a
  ordem é.
- `Index.Search(term).On("p")` fixa o alias da tabela (tabela unida duas vezes,
  ou `criteria.BuildClause`); o `criteria` resolve sozinho a partir de
  FROM/JOIN quando é inequívoco.
- Pelo atalho do pacote raiz: `sqln.Match(s)`, `sqln.ByRelevance(s)`,
  `sqln.FullTextIndex`, `sqln.FullTextSearch`.
- O termo pode vir do request: é limitado (`fts.MaxTermLength` 256 caracteres,
  `fts.MaxTerms` 16) e reduzido a palavras; excesso → `fts.ErrInvalidSearch`.

### Modos (`Search.Mode`)

| Modo | Lê o texto como |
|---|---|
| `fts.Websearch` (default) | palavras (todas), `"frase"`, `"duas palavras"~N` (até `fts.MaxNear` = 10 posições, qualquer ordem), `-palavra`/`-"frase"` exclui, `a OR b`; sinônimos do `Spec.Synonyms` |
| `fts.AllTerms` / `fts.AnyTerm` | todas / ao menos uma palavra, sem operadores |
| `fts.Prefix` | cada palavra como prefixo — busca ao digitar |
| `fts.Phrase` | as palavras juntas e em ordem |

## No filtro dinâmico

Campo do `sqln.FilterMapping` com `Search: idx` aceita a condição `SEARCH` —
ver `dynamic-filter.md` §"Busca full-text". `BuildQuery` devolve os índices e
`FindWithFilter` os confere.

## Destaque dos trechos — `fts.Highlight`

```go
type Hit struct {
    ID      int64         `db:"id"`
    Snippet fts.Highlight `db:"snippet"`
}
q.SelectHighlight(s, "description", "snippet") // coluna do índice
hit.Snippet.Excerpt(30).HTML()                  // "… a <mark>café</mark> &lt;b&gt;…"
```

O banco marca os trechos com caracteres privados e devolve o texto **sem
escapar**: `HTML()` escapa tudo e acrescenta só `<mark>`; `Fragments()` dá os
trechos (casado ou não) para outros renderizadores. **Nunca escreva o valor
cru numa página.** O destaque é avaliado depois de `ORDER BY … LIMIT`: só as
linhas da página pagam por ele.

## "Você quis dizer" — `Spec.Suggest`

Com `Suggest: true` a migration cria o lexicon `<nome>_lex` (PostgreSQL:
`pg_trgm`; MySQL: FULLTEXT `ngram`). `fts.RefreshLexicon(ctx, db, dialect, idx)`
o preenche — rode em **job**, após carga em massa ou periodicamente, nunca por
request (lê a tabela inteira). Busca sem resultado:

```go
if page.TotalElements == 0 {
    if fixed, ok, err := sqln.SuggestSearch(ctx, s); err == nil && ok {
        // "Mostrando resultados para <fixed.Term()>": rode `fixed`
    }
}
```

Só palavras positivas de 3+ caracteres são corrigidas; o resto do texto
permanece como digitado. Com `Suggest` o nome do índice é limitado a 58
caracteres.

## PostgreSQL × MySQL — mesma `Spec`, mesmas queries

`DATABASE_DRIVER=mysql` só muda o que é renderizado.

| | PostgreSQL | MySQL |
|---|---|---|
| Match | `tsvector @@ to_tsquery`, GIN | `MATCH … AGAINST (? IN BOOLEAN MODE)`, `FULLTEXT` |
| Pesos A–D | `setweight`, `ts_rank_cd` | um `FULLTEXT` por grupo de peso, relevância somada com os mesmos fatores |
| Acentos | configuração `unaccent` (a migration cria a extensão) | collation insensível a acento — exigida pela conferência |
| Stemming | sim | **não** (`cafés` não acha `café`) |
| Stopwords | lista do idioma | a mesma lista + a do InnoDB |
| Criar sem bloquear escrita | `Concurrently: true` (com `ExpressionIndex`) | impossível — a migration recusa |

Palavras abaixo de `innodb_ft_min_token_size` são descartadas das buscas no
MySQL (`Spec.MinTokenSize` deve refletir o servidor).

## Anti-padrões

- `ILIKE '%termo%'`/`LIKE` para busca textual em tabela grande — use o índice.
- `to_tsvector`/`MATCH … AGAINST` escritos à mão, ou concatenar o termo na
  query (o SDK vincula parâmetro e valida identificador).
- Editar a migration gerada, ou criar o índice por `ensure`/DDL manual em
  `stage`/`prod`.
- `fts.NewIndex` no lugar de `fts.MustIndex` em índice de produção (não é
  conferido no startup).
- Escrever `fts.Highlight` direto no HTML/JSON de página.
- Rodar `fts.RefreshLexicon` por request.
- Comparar o valor de `relevance` entre PostgreSQL e MySQL.
