---
name: absolute-rules
description: Regras invioláveis de código Go com gofi — auditadas pelo gofi-qa em todo contexto
sdk: v0.8.2
keywords: [regras absolutas, logging, sqln, errs.AppError, repository, adapter, iam, netx.RespondError, os.Getenv, log.Fatal, componentes, ciclo de vida, cron]
---

# Regras Absolutas — Go

Regras invioláveis. Cada uma é Go-specific e nasceu de erro repetido em
review. **Auditadas pelo gofi-qa em todo contexto.**

1. **Nunca** `fmt.Println`/`log.Printf` em produção — sempre `logging.*` de `github.com/joaoprofile/gofi-sdk-go/obs/logging` (exceção: `main()` reportando o erro de `run()`).
2. **Nunca** `*sql.DB` fora do `sqln` — nada de `sql.Open`, `db.DB()` ou `connection.DB()` em repository/service. Handle `*sql.DB` só no `main`/componente (`database.FromDB`, `metrics.ObserveDBStats`). Ver `database-connection.md`.
3. **Nunca** retornar `error` puro do service — sempre `errs.AppError`.
4. **Nunca** mock de banco em testes — use mock de repository (handcraft).
5. **Nunca** dois arquivos em `repository/` — interface e implementação no mesmo arquivo.
6. **Nunca** adapters/factories em `repository/` — vão em `adapter/`.
7. **IAM sobe pelo componente** `gofi/component/iam`: `iam.New(iam.Config{User, Tenant, RBAC, OnEvent})` — ele lê `JWT_*`/`*_TOKEN_TTL`/`OAUTH_GOOGLE_*` e falha o `Build` sem `JWT_SECRET`. **Nunca** montar o serviço IAM lendo variáveis à mão; `iam.FromService(svc)` só quando precisa de provider que o `DefaultConfig` não expressa. Ver `service-bootstrap.md` § IAM.
8. **Erro de service sempre por `netx.RespondError(w, r, appErr)`** — inclusive 401/403, que nascem de `errs.RegisterUnauthorized`/`errs.RegisterForbidden`. `netx.Error(w, status, err)` só para erro que não é `AppError`. Nunca montar status HTTP à mão a partir de `appErr.Kind`.
9. **Leitura só por `sqln.Find[T]` / `FindFromCriteria` / `FindWithFilter`** + `.List()`, `.UniqueResult()`, `.PagedList()`, `.All()`. **Nunca** `ExecuteListQuery(db)`/`ExecuteUniqueResultQuery(db)`/`ExecutePagedQuery(db)` passando `*sql.DB` (não existe `sqln.GetDB`).
10. **Sempre** ler `memory/project.md` antes de qualquer ação (regra cross-agent).
11. **Sempre** atualizar `memory/contexts/{contexto}.md` ao concluir uma fase.
12. **Specs** ficam em `specs/{contexto}/` — **nunca** dentro de `.claude/`.
13. **Nunca** helpers de persistência do repo como funções de pacote — **todo helper que recebe `ctx context.Context` e executa SQL é método do receiver** (`func (r *{contexto}Repository) insertY(...)`). Função de pacote só para transformação **pura** sem `ctx`. Detalhes em `persistence-rules.md`.
14. **SQL de escrita só pelo `sqln`** — `sqln.NewStatement()` (ou `*sql.Stmt` preparado pelo próprio repo, conforme `persistence-rules.md`) e transação por `transaction.New(opts).Execute(ctx, fn)`. Dentro da tx, `statement` e `sqln.Find*` usam a tx do `ctx` sozinhos; `*sql.Stmt` preparado fora da tx só participa via `tx, ok := connection.TxFrom(ctx)` + `tx.Stmt(stm)`. Ver `transactions.md`.
15. **`logging.Info` só no início/fim de fluxo de negócio** (1–2 por request/job, o fim com contadores). **Nunca** Info dentro de loop/por página/por mensagem ou de lifecycle de infra — isso é `Debug`. Pontos de fluxo usam `logging.FromContext(ctx)`. Ver `logging.md`.
16. **Só o `main` encerra o processo.** `log.Fatal` apenas em `main()` sobre o erro de `run()`; **nunca** `logging.Fatal`, `log.Fatal`, `os.Exit` ou `panic` de config/boot em construtor, repository, service, handler, worker ou consumer — devolva `error`. Erro entre `Build` e `ListenAndServe` passa por `svc.Shutdown` antes de sair.
17. **Recurso e trabalho de fundo são componentes gofi.** Banco, cache, broker, IAM, HTTP via `gofi/component/*`; worker é `gofi.Runner` e consumer é `gofi.Component`, declarados no `With`. **Nunca** `defer x.Close()`, `go x.Loop()` ou chamada de método de worker no `main`. Ver `gofi-orchestrator.md`, `worker-bootstrap.md`, `consumer-bootstrap.md`.
18. **Zero `os.Getenv` fora de `config.go`**; variável do SDK nunca é relida nem redeclarada no `Config` do projeto; nomes conforme `env-vars-standard.md`; segredo só por `secret://` ou `X_FILE`. Ver `configuration.md`.
19. **Cron com horário de negócio tem fuso explícito e config validada** — `cronjob.Fixed` com `LocationName` vindo de config; hora/minuto/fuso validados antes de `cronjob.ScheduleJob` (que panica com config inválida). Ver `cronjob.md`.
