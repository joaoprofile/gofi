# Workflow de implementação — os passos por camada

Os passos completos da implementação, agrupados pela camada do plano
(`gofi-plan`): cada papel lê só a sua seção (`gofi show` desta referência
lista as seções com as linhas). Sem plano, o `gofi-eng` faz todas, na ordem
em que estão aqui. A numeração é a de sempre, para as referências que a citam.

## Leitura

Todo papel de implementação, antes da sua camada.

```
1. Ler os deltas da mudança → requisitos e cenários; o design → entidades, dados, contratos, erros e o Mapa de requisitos
```

## Dados

Camada `data` — `gofi-eng-data`.

```
2. Criar model (entity + dto + query_dto se filtro dinâmico) seguindo boilerplates/model.md
4. Criar repository/{contexto}_repository.go (interface + impl no MESMO arquivo)
10. Criar `.migrations/{N}_{contexto}.up.sql` **e** `.down.sql` em **par** (regra absoluta #13 — `.claude/sdk/go/knowledge/migrations.md`). `.sql` puro é silenciosamente ignorado pelo `golang-migrate` (erro enganoso: `migration error: first .: file does not exist`). Se o design listar migrations sem `.up`/`.down` (§Modelo de dados), tratar como erro do design — pare e pergunte, porque a mudança aprovada não se corrige por conta própria. **Índices, `fillfactor` e tuning de autovacuum** derivam do perfil de acesso da tabela declarado pelo design (`cold` / `hot UPDATE` / `hot DELETE+INSERT` / `append-only`) + combinações de filtro previstas + workers cross-cutting (purge, archive). Padrão completo em `.claude/sdk/go/knowledge/postgres-index-strategy.md`. Se o design não declarar o perfil de uma tabela, **parar e perguntar** ao dev antes de inventar índices
```

## Domínio

Camada `domain` — `gofi-eng`.

```
3. Criar service/errors.go com todos os erros do contexto registrados
6. Criar service/{contexto}_service.go (interface + implementação — regra de domínio: persistência em batch, hidratação de tenancy, validação cross-aggregate, lookup, delete-by-policy). Se o contexto mistura CRUD + auth/IAM, criar também service/auth_service.go com os métodos auth no MESMO struct receiver + IAMSession + AuthInfra + constantes OAuth (ver "Service split CRUD vs Auth/IAM" em `.claude/sdk/<lang>/knowledge/service-bootstrap.md`)
6a. **Se** o contexto tem bridge/factory, coordena ≥2 domínios, transação multi-passo/saga, ou ≥2 transportes: criar `application/` com **uma struct por workflow orquestrador** (`{Aggregate}Application` com `Execute(ctx, ...)`). **Só workflows reais** entram em application (ingestão, saga, coordenação cross-domain) — ops simples de domain (lookup, delete-by-policy, CRUD trivial) ficam em `service/` e o caller (handler/scheduler) chama direto. Application chama `service` (não repository) e `bridge` via factory. Errors em `application/errors.go` (bridge/fetch/input) separados de `service/errors.go` (persistência/lookup). Tests da application mockam o service (handcraft). Tabela "o que mora em application vs service" em `.claude/expertise/ddd-architecture/application-vs-domain-service.md`.
8. Atualizar main.go (em `pathCmd` = `pathService/{projectName}/`, ex: `./backend/web-api/main.go` — **nunca** direto em `pathService`) — composition root: `LoadConfig` → declarar componentes com `gofi.New(name).With(componentes...)` → `Build()` → wiring (repository → service → handler, registrado com `http.Handlers(...)`) → `ListenAndServe`. Estrutura física e import paths em `.claude/sdk/go/knowledge/structure.md`; mecânica do orquestrador (`New/With/Build/ListenAndServe/Shutdown`, componentes, Runner) em `.claude/sdk/go/knowledge/gofi-orchestrator.md`. **Se o bootstrap inflar** (providers de auth/OAuth, adapters de SDK, workers, `run()` >~40 linhas úteis), aplicar split por responsabilidade no mesmo `package main`: `config.go` + `wire.go` + `iam.go` + um arquivo por worker/consumer + `config_test.go`. Regras invioláveis, ordem fixa e anti-padrões em `.claude/sdk/go/knowledge/service-bootstrap.md` — `LoadConfig(ctx, env)` retorna `(Config, error)` (testável), **`os.Getenv` só em `config.go`** (variável do SDK é lida pelo componente, não relida nem redeclarada no `Config`; ambiente em `configuration.md`), builders de `wire.go` recebem dependências por parâmetro, wiring só depois do `Build`, erro no wiring → `Shutdown` antes de sair, e só o `main` fataliza (sobre o erro de `run()`). **Se o serviço tem background workers (consumer, cron, scheduler, watcher):** cada um é **componente declarado no `With`**, dono do próprio ciclo de vida (`gofi.Runner`: `Start` no `Build`, `Run` no `ListenAndServe`, `Stop` no shutdown) — `main.go` **nunca** chama `Bootstrap`/`Schedule`/`Start`/`Register`/`Close` do worker nem sobe `go worker.Loop(ctx)`. Princípio geral em `.claude/sdk/go/knowledge/worker-bootstrap.md`; agendamento em `cronjob.md`; caso especializado consumer em `.claude/sdk/go/knowledge/consumer-bootstrap.md`
9. Atualizar manifest da linguagem (go.mod / Cargo.toml / pom.xml / etc.)
12. **Sincronizar `.env` na raiz do projeto** — adicionar variáveis ausentes com placeholders e avisar o dev nos "Próximos passos" (regra completa em `.claude/expertise/platform-delivery/env-file-management.md`)
```

## Integração

Camada `integration` — `gofi-integrate`.

```
5. Criar adapter/ se o contexto integra com SDK externo (IAM, etc.)
```

## API

Camada `api` — `gofi-eng-api`.

```
7. Criar handler/{contexto}_handler.go (+ middleware.go, auth_handler.go se aplicável) — handler chama **application** quando ela existir, senão service direto
```

## Testes

Do `gofi-test`, a partir dos cenários.

```
11. Escrever testes:
    - service/{contexto}_service_test.go com mock de repository handcraft
    - handler/{contexto}_handler_test.go com stub de service handcraft
```

## Fechar

Quem fecha a implementação — no loop, o loop reindexa depois de cada turno.

```
13. Índices: num turno do loop, nada — o loop reindexa depois do turno. Fora dele, `gofi index code` (código) e, se mexeu em spec ou PRD, `gofi index check` (faceta fora do léxico reprova); o índice de documentos se refaz sozinho na consulta. Depois a memória (ver `reference/memory-and-output.md` §"Atualização de memória ao concluir")
```

A ordem é guia, não rígida — ajuste se a mudança exigir; com plano, a ordem é a dele.
