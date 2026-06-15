# Regras Absolutas — Go

Quinze regras invioláveis. Cada uma é Go-specific e nasceu de erro repetido
em review. **Auditadas pelo gofi-qa em todo contexto.**

1. **Nunca** `fmt.Println` em produção — sempre `logging.*` do `gofi/obs/logging`.
2. **Nunca** `*sql.DB` fora do pacote `gofi/sqln`.
3. **Nunca** retornar `error` puro do service — sempre `errs.AppError`.
4. **Nunca** mock de banco em testes — use mock de repository (handcraft).
5. **Nunca** dois arquivos em `repository/` — interface e implementação no mesmo arquivo.
6. **Nunca** adapters/factories em `repository/` — vão em `adapter/`.
7. **Nunca** `iam.NewDefault` quando há User/Tenant adapters — use `iam.New`.
8. **Nunca** `netx.RespondError` para 401/403 — use `netx.Error(w, http.StatusUnauthorized, err)` explicitamente.
9. **Nunca** `.ExecuteUniqueQuery()` ou `sqln.GetDB()` — use `.Execute()`, `.List()`, `.PagedList()`.
10. **Sempre** ler `memory/project.md` antes de qualquer ação (regra cross-agent).
11. **Sempre** atualizar `memory/contexts/{contexto}.md` ao concluir uma fase.
12. **Specs** ficam em `specs/{contexto}/` — **nunca** dentro de `.claude/`.
13. **Nunca** helpers de persistência do repo como funções de pacote — **todo helper que recebe `ctx context.Context` e executa SQL é método do receiver** (`func (r *xxxRepository) insertY(...)`). Função de pacote só para transformações **puras** sem `ctx` (ex.: `configArgs(e) []any`). Detalhes em `repository-aggregate-pattern.md` §"Helpers de persistência são MÉTODOS do receiver".
14. **Sempre** prepared statements no constructor — `*sql.Stmt` em campo do struct (`stmInsertX`, `stmUpdateX`, `stmDeleteX`), preparado **uma única vez** em `New{Contexto}Repository(ctx)`. Métodos usam `r.stmXxx.ExecContext(...)` (fora de tx) ou `ctx.Value(connection.SqlTxContextKey).(*sql.Tx).Stmt(r.stmXxx).ExecContext(...)` (dentro de tx). `sqln.NewStatement().Execute(ctx, sql, args...)` inline em cada chamada é **MAJOR**. Exceção: SQL **dinâmico** montado em runtime (filtro dinâmico) não pode ser preparado.
15. **`logging.Info` só no início/fim de fluxo de negócio** (1–2 por request/job, o fim com contadores). **Nunca** Info dentro de loop/por-página/por-mensagem ou de profiling/lifecycle de infra — isso é `Debug` (filtrado em prod, fora do Loki). **`logging.Fatal` só em bootstrap.** Pontos de fluxo usam `logging.FromContext(ctx)` (correlação com trace). Detalhes e padrão em `logging.md`.
