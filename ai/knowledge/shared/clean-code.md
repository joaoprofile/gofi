# Clean Code — princípios cross-language

Aplicam-se a **todo agent** que escreve código (gofi-eng, gofi-qa em refactor,
qualquer outro futuro). Validados pelo dev como preferência permanente do
projeto. **gofi-qa audita.**

## Princípio fundamental

**Código limpo é código que se explica.** Identificadores bem escolhidos,
funções pequenas, fluxo direto. Comentário só quando o código não consegue
falar por si — e nesse caso, comentário explica **WHY**, nunca **WHAT**.

## Regras de comentários

### Quando NÃO comentar (default)

- ❌ Narração do que o código faz (`// loads the user from db`)
- ❌ Bloco doc no topo de struct/função óbvia (`// Pool struct represents a pool`)
- ❌ Repetição da regra que o nome já comunica (`// validates DTO` em `func (r Req) Validate()`)
- ❌ Comentário com referência à task/PR/issue (`// added for issue #123`)
- ❌ Comentário "TODO: melhorar isso" sem ação concreta
- ❌ Comentário "// removed legacy X" — código removido vive no git history
- ❌ Marcação de seções com banners (`// --- helpers ---`) em arquivos curtos

### Quando comentar (exceção)

Comentário tem que justificar sua existência. Use quando:

- ✅ Há um **constraint não-óbvio** (ex.: ordem das colunas casa com posicional)
- ✅ Há um **workaround** documentado (ex.: bug do driver, comportamento contra-intuitivo do SDK)
- ✅ Há uma **decisão de negócio** que o código não consegue carregar (ex.: "ledger é append-only por LGPD")
- ✅ TODO com **ação concreta** + condição clara (ex.: `// TODO(rbac-fino): trocar para RBACMiddleware quando user roles ficarem fine-grained`)
- ✅ Invariante de segurança/correção (ex.: "ctx deve ter *sql.Tx — falha silenciosa se chamado fora de tx")

Quando comentar, **lidere com WHY**, não com WHAT.

## Regras de código

### Funções

- **Nome diz o que faz** — se precisa de comentário pra explicar o nome, renomeie.
- **Pequenas e focadas** — uma responsabilidade. Quando uma função tem 2 responsabilidades, divide.
- **Early return** sobre `if/else` aninhado.
- **Sem flag arguments** — `func(silent bool)` vira duas funções.

### Estrutura

- **Construtores enxutos** — `New*()` faz wiring, não regra de negócio.
- **Interfaces consumidas** ficam no pacote do **consumidor**, não do produtor (DDD inversion).
- **Errors registrados em var de pacote** (errs.AppError) — sem `errors.New(...)` inline em service/handler.
- **Magic numbers** viram `const` no topo do arquivo, com nome explicativo.

### Tests

- **Subtests com nome em prosa** — `t.Run("returns 401 when claim missing", ...)`.
- **Mocks handcraft** — sem framework de mock externo (regra do projeto).
- **Stubs com campos**, mocks com `fn` fields — padrão já documentado.
- **Sem comentário descritivo de assertion** — o assert + nome do subtest já dizem.

### Naming

- **Verbos para funções** (`Get`, `Create`, `Validate`, `IsBookingOpen`).
- **Substantivos para tipos** (`Pool`, `LedgerWriter`).
- **Booleans com `Is`/`Has`/`Can`** — `IsValid`, `HasReservation`, `CanCancel`.
- **Constantes em `UPPER_SNAKE` só quando é enum/sentinel exposto** — caso contrário `MixedCase`.
- **Acronyms uppercase** — `ID`, `URL`, `CPF`, `JWT`, `HTTP`. Em compostos: `URLPath` (não `UrlPath`).

## Refactor antes de comentar

Se você está prestes a comentar:

1. **Renomear a variável/função** resolve? → faça.
2. **Extrair função pequena** com nome explicativo resolve? → faça.
3. **Mover a lógica** pra um lugar mais óbvio resolve? → faça.

Só comente quando 1, 2 e 3 não resolverem.

## Anti-padrões observados

```go
// LOAD user from db                                    ❌ narração
user := repo.FindByID(id)

// Insert is run via NewStatement so it picks up        ❌ narração + WHY
// *sql.Tx from ctx via SqlTxContextKey
err := sqln.NewStatement().Execute(ctx, q, args...)

// helper                                                ❌ banner inútil
// ---

// TODO: improve this                                    ❌ TODO sem ação
return result
```

```go
result := repo.FindByID(id)

err := sqln.NewStatement().Execute(ctx, q, args...)     ✅ código fala por si

return result
```

Quando o WHY for genuinamente não-óbvio:

```go
err := sqln.NewStatement().Execute(ctx, q, args...) // tx vem do ctx; ExecuteContext fora de tx silenciosamente bypassa  ✅
```

Uma linha, lidera com WHY.
