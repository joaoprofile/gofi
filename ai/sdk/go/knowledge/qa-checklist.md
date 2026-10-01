---
name: qa-checklist
description: Checklist de auditoria Go do gofi-qa contra o SDK v0.8.2 — ciclo de vida por componentes, Config tipado, secret://, UTC, sqln com allowlist, netx, iam, msq e observabilidade
sdk: v0.8.2
keywords: [qa, auditoria, checklist, gofi.New, componentes, config, secret, timezone, sqln, filtro-dinamico, netx, iam, msq, observabilidade]
---

# Checklist de Auditoria — Go (SDK v0.8.2)

Aplicado por `gofi-qa` em todo contexto Go implementado. Itens de PostgreSQL
(aggregate, statements, índices, dinheiro) ficam em `qa-checklist-postgres.md`
e se somam a este quando o contexto os toca. API citada:
`.claude/sdk/go/api/` (procure com `gofi find --in sdk "<símbolo>"`).

**Como verificar.** Cada item diz o que procurar. Consulte o grafo antes
(`gofi show <símbolo>` lista quem chama; `gofi find --in code "<termo>"`
acha o símbolo); a busca literal é `gofi find --text "<literal>" --in code`
ou `gofi find --regex "<re>" --in code` (`-i` ignora caixa), declarada.
Prova de ausência ("ninguém chama X") em grafo `fast` pede
`gofi index code --deep` ou a limitação declarada. Severidade ao fim de cada
item; a tabela está no final.

## 1. Conformidade com a Spec
- [ ] Todos os campos da entidade estão implementados
- [ ] Todas as operações listadas na spec existem
- [ ] Todas as regras de negócio (RN-*) estão implementadas
- [ ] HTTP status codes correspondem ao mapeado na spec
- [ ] Filtros de listagem se comportam como especificado

## 2. Ciclo de vida — `gofi.New` + componentes
Ver `gofi-orchestrator.md`, `service-bootstrap.md` e `database-connection.md`.

- [ ] **Serviço nasce em `gofi.New(nome).With(componentes...).Build()`** e
  termina em `svc.ListenAndServe()` (job sem HTTP: trabalho → `svc.Shutdown(ctx)`).
  Recursos vêm dos componentes (`database.New()`, `cache.New()`,
  `session.New()`, `messaging.New()`, `iam.New()`, `observability.New()`,
  `httpserver.New(port, cfg)`). Bootstrap manual num serviço que usa `gofi.New`
  é **MAJOR** — o recurso fica fora da ordem de start/close e do health:
  `gofi find --regex "obs\.Init\(|connection\.(NewConnection|SetGlobal)\(|sqln\.NewCacheRedis\(|msq\.(New|Open)\(|netx\.NewServer\(" --in code`;
  para identidade, `gofi find --text "gofi-sdk-go/iam\"" --in code` (o pacote
  raiz `iam`, com `iam.New`/`iam.NewDefault`, importado no lugar de
  `gofi/component/iam`, cujo `New` tem o mesmo nome). Segunda conexão de banco
  deliberada (`connection.NewConnection` usada com `WithConnection`) não conta.
  Exceção: recurso do chamador injetado por `database.FromDB`,
  `cache.FromClient`, `messaging.FromBroker`, `iam.FromService`,
  `observability.FromTelemetry`, `httpserver.FromServer` (quem abriu fecha).
- [ ] **Erro do `Build()` tratado** — `svc, err := ...Build(); if err != nil`
  encerra o `main`. `Build` devolve **todos** os erros de configuração
  juntos; ignorá-lo sobe o serviço com recurso nil. **BLOCKER**
- [ ] **Imports que registram providers presentes no `main`**: com
  `database.New()`, `_ ".../sqln/driver/<driver>"`; com `messaging.New()` sem
  `Broker`, `_ ".../msq/provider/<provider>"`; com variável
  `secret://awssm/...` ou `secret://ocivault/...`,
  `_ ".../base/secrets/awssm"` ou `.../ocivault`. Ausente: o `Build` falha no
  boot. **MAJOR** — `gofi find --regex "sqln/driver/|msq/provider/|base/secrets/" --in code`
- [ ] **Handle de componente lido depois do `Build`** — `db.DB()`,
  `identity.Service()`, `mq.Broker()`, `Telemetry()` do componente de
  observabilidade são nil antes. Handler
  ou consumer montado antes do `Build` com esse handle: **BLOCKER** (nil
  deref). Construtor de repositório não toca o banco (pode vir antes).
- [ ] **Trabalho em background parado pelo ciclo de vida** — consumers via
  `msq.NewConsumerManager(mq.Broker())` (o `ListenAndServe` drena); loops e
  jobs como `gofi.Runner` (`Run`/`Stop`). `go loop()` solto, sem `Stop`
  nem cancelamento: **MAJOR** (shutdown corta trabalho no meio) —
  `gofi find --regex "^\s*go (func|\w+\()" --in code`
- [ ] **Componente próprio** (`Name`/`Stage`/`Start`) registra o que abre
  em `rt.OnClose` e o readiness em `rt.AddHealthCheck`. Ausente: **MAJOR**
- [ ] **Nada de `log.Fatal`/`os.Exit`/`logging.Fatal`/`panic` fora do
  `package main`** — biblioteca e camadas devolvem `error`.
  `gofi find --regex "log\.Fatal|os\.Exit\(|logging\.Fatal\(|panic\(" --in code`;
  ocorrência fora do `main` é **MAJOR**
- [ ] Deploy com probes (k8s): `httpserver.New(port, &netx.WSConfig{Health: &netx.HealthConfig{}})`
  — sem `Health` não há `/livez`/`/readyz` nem drenagem antes do shutdown.
  **MINOR** (MAJOR se a spec de infra declara as probes)

## 3. Configuração — `Config` tipado, variáveis e `secret://`
Ver `configuration.md` e `env-vars-standard.md`.

- [ ] **Sem `os.Getenv`/`os.LookupEnv`/`environment.Instance()` em código
  folha** (`domain/`, service, repository, handler, adapter). Quem lê ambiente
  é o `main` (ou um `config.go` do `package main`); o resto recebe `Config`
  tipado no construtor. `gofi find --regex "os\.(Getenv|LookupEnv)\(|environment\.Instance\(" --in code`
  — ocorrência fora do `main`: **MAJOR**
- [ ] Config do app lido **num lugar só**, validado no boot e devolvido como
  `(Config, error)` (`common.ParseStructAnnotationFunc` com tag `env` ou
  leitura explícita); valores do SDK vêm de `svc.Environment()`. Mesma
  variável lida em dois pontos: **MAJOR**
- [ ] Nomes de variável no padrão do SDK (`APP_*`, `DATABASE_*`, `CACHE_*`,
  `MESSAGING_*`, `BUCKET_*`, `MAIL_*`, `JWT_*`, `OAUTH_*`, `OTEL_*`, `LOG_*`,
  `TIMEZONE`, `ALLOWED_ORIGINS`). Sinônimo próprio (`DB_HOST`, `REDIS_ADDR`)
  para algo que o SDK já lê: **MAJOR**. Exceção de terceiro documentada na spec
- [ ] **Segredo nunca em texto** — nem literal no código, nem no manifesto,
  nem em `.env` versionado: `VAR=secret://<provider>/<nome>[#chave]` ou
  `VAR_FILE=/caminho`. `gofi find --regex -i "(password|secret|token|api_?key)\s*[:=]\s*\"[^\"]+\"" --in code`.
  Segredo literal: **BLOCKER**
- [ ] Variável **própria do app** com `secret://` é resolvida com
  `secrets.Resolve(ctx, v)` no `main` — o loader do SDK só resolve os campos
  de `environment.Environment`. Sem isso o valor chega como a string
  `secret://...`. **MAJOR**
- [ ] Produção sem `.env`: `GOFI_DOTENV=true`, `TLS_INSECURE_SKIP_VERIFY=true`
  ou `netx.ExposeErrorCause = true` fora de teste — `gofi find --regex "ExposeErrorCause\s*=\s*true|GOFI_DOTENV|TLS_INSECURE_SKIP_VERIFY"`.
  **MAJOR**

## 4. Tempo e fuso
- [ ] **Fuso do processo é UTC por padrão** (`TIMEZONE` vazio → UTC, aplicado
  pelo `Build`). Código não reatribui `time.Local`
  (`gofi find --regex "time\.Local\s*=" --in code` — **MAJOR**) nem supõe que
  `time.Now()` está no fuso de negócio: cálculo de "dia", "mês" ou janela de
  negócio usa `time.LoadLocation(nome)` com o nome vindo do `Config`.
  Fronteira de dia calculada no fuso do processo quando a spec fixa um fuso
  de negócio: **MAJOR**
- [ ] Timestamps persistidos e trafegados em UTC (`timestamptz`, RFC 3339).
  **MAJOR** se misturar fusos na mesma coluna
- [ ] **Cron com horário fixo** (`cronjob.ScheduleConfig{Mode: cronjob.Fixed}`):
  `LocationName` IANA explícito quando o horário é de negócio — vazio usa
  `time.Local` (UTC por padrão). **MAJOR** se a spec fixa o horário no fuso de
  negócio; **MINOR** nos demais
- [ ] `cronjob.ScheduleJob` **panica** com config inválida (nome IANA
  desconhecido, hora fora da faixa). A base IANA vem embutida pelo
  `base/timezone`, que todo binário com `gofi.New` já linka; binário sem
  `gofi` que use `LocationName`/`time.LoadLocation` importa
  `_ "time/tzdata"`. Ausente: **MAJOR** (panic no boot em imagem sem tzdata)
- [ ] Job agendado com efeito colateral em serviço com mais de uma réplica usa
  `Locker` (`cronjob.RedisLocker`) + `Name`. Ausente com job não idempotente:
  **MAJOR**

## 5. Persistência `sqln` (genérico; PostgreSQL em `qa-checklist-postgres.md`)
Ver `persistence-rules.md`, `database-connection.md` e `transactions.md`.

- [ ] Entidade usa tags `db:"col"` — nunca `gofi:""` (não é mapeada). **MAJOR**
- [ ] Leitura por `sqln.Find[T]`, `sqln.FindFromCriteria[T]` ou
  `sqln.FindWithFilter[T]` + `.UniqueResult()`/`.Execute()`/`.List()`/
  `.PagedList()`/`.All()`. Escrita por `sqln.NewStatement().Execute(ctx, sql, args...)`
  (entra sozinha na transação do `ctx`). `*sql.DB` na mão no repository
  (`connection.DB()`, `connection.MustDB()`, `db.DB()`) é **MAJOR** — exceção:
  segundo banco via `statement.NewWithConnection`/`.WithConnection(conn)`
- [ ] `FindByID` devolve `(*T, error)` com `nil, nil` quando não acha
  (`UniqueResult` já faz isso); consultas de presença (`ExistsByXxx`) idem —
  ver `repository-primitive-return.md`. **MINOR**
- [ ] SQL só com placeholders (`$1`, `$2`…); nada de `fmt.Sprintf` com valor
  do cliente. `gofi find --regex "Sprintf\(\s*\"(SELECT|INSERT|UPDATE|DELETE)" -i --in code`.
  Valor de cliente interpolado: **BLOCKER**
- [ ] **Réplica de leitura** (`DATABASE_READ_HOST`): leitura fora de transação
  vai para a réplica. Ler para decidir uma escrita (check-then-act) ou reler o
  que acabou de gravar fora de `transaction.Execute`: **MAJOR** (lag)
- [ ] `transaction.Options{MaxRetries: n}` reexecuta `fn` inteira: dentro de
  `fn` não há publicação em fila, chamada HTTP, e-mail nem outro efeito fora
  do banco. Presente: **MAJOR**
- [ ] Paginação: `sqln.NewPageRequest(page, limit, sorts)`, `page` 0-indexed.
  O SDK não limita `limit` (até 65535): handler aplica teto. Sem teto em
  listagem pública: **MINOR**
- [ ] **Value objects aninhados** (ver `value-objects.md`): campo externo com
  `db:""`, sub-campos com `db:"col"`; multi-coluna = struct simples (o mapper
  expande); coluna única JSON = `sql.Scanner`/`driver.Valuer`. Colunas casam
  por nome; ordem só importa quando o SELECT não cobre todos os campos
  tagueados. **MAJOR** se o VO não é populado

## 6. Filtro dinâmico — allowlist (`sqln.FilterMapping`)
Ver `dynamic-filter.md` e `examples/sqln/filter-api`.

- [ ] **Todo filtro vindo do cliente passa por `sqln.FilterMapping`**:
  `sqln.BuildQuery(base, args, filters, mapping, nil)` +
  `sqln.NewPageRequestFilter(filters, mapping)` com **o mesmo** mapping e
  `sqln.FindWithFilter[T](ctx, q).WithPage(page).PagedList()`. Campo ou
  ordenação do cliente concatenado no SQL sem mapping: **BLOCKER** —
  `gofi show sqln.BuildQuery` e `gofi find --regex "FilterMapping|AllowColumns|filter\.Allow" --in code`
- [ ] Mapping declarado como `var` do pacote, cada campo com `Column`,
  `Ops` (`sqln.Equality`/`sqln.Range`/`sqln.Text` ou lista explícita) e
  `Sortable` explícito. `sqln.AllowColumns`/`filter.Allow` (todo operador,
  toda ordenação, nome = coluna) em código novo: **MINOR**
- [ ] Mapping **não** expõe coluna sensível (hash de senha, token, segredo) nem
  a coluna de tenant. Presente: **BLOCKER**
- [ ] **Tenant fora do body.** `sqln.Filters.Tenant` é `any` **sem tag json**
  e o SDK não o usa: o tenant entra na base como placeholder
  (`... WHERE p.<tenant_col> = $1`, `args = []any{tenant}`) com o valor
  tirado dos claims. Tenant lido de `filters.Tenant` sem o handler
  sobrescrevê-lo depois do `ParseRequestBody` (o cliente manda `"tenant"` no
  JSON), ou tenant anexado em `filters.Filters` (o `OR` do cliente fura o
  isolamento dentro dos parênteses): **BLOCKER**. Tenant por `fmt.Sprintf` na
  base: **MAJOR**
- [ ] Base termina dentro do `WHERE` (predicado de tenant ou `WHERE 1=1`),
  como constante — o SDK anexa `AND ( … )`. **MAJOR** se não
- [ ] Handler traduz `errors.Is(err, sqln.ErrInvalidFilter)` em **400**.
  Cair em 500: **MAJOR**
- [ ] Endpoint de schema devolve o próprio `FilterMapping` (`Column` tem
  `json:"-"`). DTO próprio que copia `Column` para a resposta: **MAJOR**
  (expõe o schema)
- [ ] Símbolos anteriores à allowlist (`NewQueryBuild`, `QueryMapping`,
  `FieldMapping`, `.Validate(filters)`) não existem na v0.8.2: presença indica
  código não migrado. **BLOCKER** (não compila)

## 7. Lookup de enums no schema (só com filtro dinâmico)
Ver `lookup-endpoints.md`. `sqln.FilterField` carrega `FilterType`,
`SearchType` e `Content` como metadado de UI; não há endpoint `/status`.

- [ ] Campo enum com `FilterType` `search-multiple`/`search-single`
  (`"text"` num enum: **MAJOR**) e `SearchType` não vazio (**MAJOR**)
- [ ] `SearchType: "embedded"` ⇒ `Content` com constante exportada (nil ou map
  literal inline: **MAJOR**); api-path ⇒ `Content` nil (**MINOR**) e caminho
  relativo sem `/` inicial (`"/v1/..."` ou URL absoluta: **MAJOR**)
- [ ] Rota `getStatus` em código novo: **MAJOR**; em legado: **SUGGESTION**
- [ ] Enum declarado uma vez no pacote comum de enums do projeto (slice para
  `oneof`, map para `Content` e `IsValid`), reusado por todo mapping.
  Redeclaração paralela: **MAJOR**; `IsValidXxx` com switch duplicado:
  **MINOR**
- [ ] Schema serializa o `FilterField` inteiro. Filtrar campos no handler:
  **MAJOR**

## 8. Erros — `base/errs`
Ver `error-handling.md`.

- [ ] Erros do service em `errors.go`, registrados com `errs.Register*`
  (`RegisterValidation`, `RegisterNotFound`, `RegisterConflict`,
  `RegisterOperation`, `RegisterExternalError`, `RegisterUnauthorized`,
  `RegisterForbidden`). **MAJOR** se criados ad hoc
- [ ] Service devolve `errs.AppError`, nunca `error` puro. **MAJOR**
- [ ] Validação: `ErrXxxValidation.WithDetails(err)`; operação:
  `ErrXxxAction.Wrap(err)`. **MINOR**
- [ ] Not-found em Update detectado por `FindByID` nil **antes** do update.
  **MAJOR**

## 9. Validação — `base/validator`
Ver `validation.md`.

- [ ] DTO com `Validate()` chamando `v.ValidateStruct(r)`; `var v = validator.New()`
  no pacote. **MINOR**
- [ ] Tags `validate:"..."` cobrem as RNs de validação. **MAJOR**

## 10. HTTP servidor — `netx` + componente `httpserver`
Ver `http-auth-middleware.md`.

- [ ] Body: `netx.ParseRequestBody(w, r, &req)`; query:
  `netx.BindQueryParamsToStruct(r, w, &f)` (tag `form`); path:
  `netx.GetPathParam("id", r)`; resposta: `netx.Response(w, status, data)`.
  **MINOR**
- [ ] Erro de service: `netx.RespondError(w, r, appErr)` — o `Kind` vira o
  status (404, 409, 400, 401, 403, 502, 500) e a causa só vai para o log.
  `netx.Error(w, status, err)` para erro fora de `AppError` (decode,
  middleware). Assinatura antiga `RespondError(w, appErr)`: **BLOCKER** (não
  compila); status escolhido à mão para um `AppError`: **MINOR**
- [ ] **`Use`/`UseAuth` antes de `Handlers`** e rota autenticada em
  `netx.PrivateRoutes`. Rota privada registrada antes do `UseAuth` (ou sem
  `UseAuth`) fica **sem** autenticação: **BLOCKER** — `gofi show` do `main`
- [ ] `WSConfig` explícito no que o endpoint pede: `AllowedOrigins` com as
  origens da spec (o componente não aplica `ALLOWED_ORIGINS` sozinho: o `main`
  passa `Environment.HTTP().AllowedOrigins` ou o valor do `Config`), `MaxBodyBytes` + `ReadTimeout`/`WriteTimeout` para upload,
  `RequestTimeout`, `CrossOriginProtection` com sessão por cookie. Upload
  grande sem subir o teto: **MAJOR**; cookie sem `CrossOriginProtection`:
  **MAJOR**
- [ ] Handler sem lógica de negócio; não acessa repository. **MAJOR**

## 11. HTTP cliente — `netx.NewClient`
- [ ] Cliente criado **uma vez** (`netx.NewClient(&netx.HttpClientConfig{Name, BaseURL, Timeout, Retries})`)
  e injetado; `Name` preenchido (nomeia os spans). Cliente por requisição:
  **MAJOR**
- [ ] **Retry só em método idempotente.** O cliente reexecuta GET/HEAD/OPTIONS/
  PUT/DELETE e 429; POST/PATCH só com header `Idempotency-Key` (via
  `req.SetHeader`) que o servidor honre. Laço de retry manual em volta de
  `Execute()` para POST/PATCH sem chave: **MAJOR** (efeito duplicado)
- [ ] `Retries: 0` **não** desliga retry (vira o padrão do SDK): quem precisa
  de uma tentativa só declara isso na spec e trata. `DisableRetryOn429: true`
  quando um rate limiter externo controla o ritmo. **MINOR**
- [ ] Assinatura por `req.SetSignature(signer)` (`netx/awssign` ou
  `netx.Signature` próprio) — nunca header de assinatura calculado fora do
  `Sign` (retry reenvia assinatura velha). **MAJOR**

## 12. Identidade — `iam`
Ver `iam.md`, `rbac.md` e `http-auth-middleware.md`.

- [ ] Serviço de identidade pelo componente `iam.New(iam.Config{User, Tenant, RBAC})`
  — configuração de token e sessão vem do ambiente (`JWT_SECRET` com 32+ bytes,
  por `secret://` em produção; `*_TOKEN_TTL`). Ajuste fino em `Configure`.
  **MAJOR** se montado à mão (ver §2)
- [ ] Rotas privadas passam por `middleware.AuthMiddleware(svc)` (ou middleware
  próprio que chama `ValidateToken`); permissão por
  `middleware.RBACMiddleware(svc, recurso, ação)` depois do auth; revogação de
  acesso ao tenant imediata com `middleware.TenantMiddleware(svc)`.
  **BLOCKER** se rota com dado de tenant não valida token
- [ ] **Tenant e usuário vêm dos claims** (`middleware.ClaimsFromContext(ctx)`),
  nunca de body, query ou header livre. **BLOCKER**
- [ ] Senha com `password.Hash` (Argon2id) e `password.Verify`;
  `password.NeedsRehash` no login migra hash bcrypt. Hash novo com bcrypt:
  **MAJOR**; senha em claro ou hash rápido (MD5/SHA): **BLOCKER**
- [ ] Mais de uma réplica ⇒ sessão em Redis (`CACHE_TYPE=redis`); em memória
  o logout não vale nas outras réplicas. **MAJOR**
- [ ] Login com seleção de tenant usa `Security.RequireTenantTicket`.
  **MINOR**

## 13. Mensageria — `msq`
Ver `messaging-msq.md` e `kafka-consumer-naming.md`.

- [ ] Broker pelo componente `messaging.New(...)`; consumers por
  `msq.NewConsumerManager(mq.Broker()).Register(cfg, handler).Start()`.
  **MAJOR** se fora do ciclo de vida (ver §2)
- [ ] `cfg := msq.DefaultConsumeConfig(topic)` com `MaxRetries`,
  `RetryBackoff`, `DeadLetterTopic` e `HandlerTimeout` definidos para handler
  que pode falhar. `MaxRetries > 0` sem `DeadLetterTopic` devolve o Nack ao
  broker (redelivery sem fim): **MAJOR**. Sem `HandlerTimeout`: **MINOR**
- [ ] Resultado certo: `msq.Ack` processado; `msq.Nack` falha transitória;
  `msq.Ignore` falha permanente (payload inválido — retry não conserta).
  `Nack` em erro de `msq.UnpackMessage`: **MAJOR** (mensagem venenosa)
- [ ] Handler idempotente (entrega pelo menos uma vez; o DLQ ou o retry
  repetem). Efeito duplicado em reprocessamento: **MAJOR**
- [ ] Existe consumer ou alerta para o tópico de DLQ (lê
  `msq.HeaderDLQOriginalTopic`/`HeaderDLQError`/`HeaderDLQAttempts`).
  **MINOR**
- [ ] Erro de `msq.NewMessage`/`msq.NewMessageWithTopic` e de `SendMessage`
  tratado. Ignorado: **MAJOR**

## 14. Observabilidade e logging
Ver `observability-otel.md` e `logging.md`.

- [ ] Telemetria pelo componente `observability.New()` (`OTEL_EXPORTER_OTLP_ENDPOINT`);
  `obs.Init` à mão: **MAJOR** (ver §2)
- [ ] Sem span/middleware de tracing próprio em volta de rota `netx`, chamada
  do `netx.HttpClient` ou consumer `msq` — o SDK já cria o span e propaga o
  contexto. Duplicado: **MINOR**
- [ ] Instrumentos (`obs/metrics`: `NewInt64Counter`, `NewFloat64Histogram`,
  ...) criados **uma vez** no boot e injetados. Criados por requisição:
  **MAJOR**. Atributo de alta cardinalidade (ID, e-mail) ou chamado `job`/
  `instance` (a série é descartada): **MAJOR**
- [ ] Log por `logging.*`/`logging.FromContext(ctx)` com atributos `slog`;
  nunca `fmt.Println`/`log.*` fora do `main` —
  `gofi find --regex "fmt\.Print|log\.(Print|Fatal|Panic)" --in code`. **MAJOR**
- [ ] `Info` só no início/fim de fluxo; nada de `Info` por item de loop ou por
  mensagem. **MINOR**
- [ ] Sem dado sensível em log (senha, token, documento completo). **BLOCKER**

## 15. Estrutura
Ver `structure.md`.

- [ ] `main.go` em `pathCmd` (`./src/{projectName}/main.go`); `go.mod`,
  `domain/` e `.migrations/` em `pathService`; `go.work` na raiz com
  `use ./src`. **MINOR**
- [ ] Imports do SDK com os paths de módulo da v0.8.2
  (`github.com/joaoprofile/gofi-sdk-go/gofi`, `.../sqln`, `.../netx`, ...).
  Path antigo do orquestrador na raiz (`github.com/joaoprofile/gofi-sdk-go` sem
  sufixo): **BLOCKER** (não resolve)
- [ ] Separação de camadas: service não conhece `http.ResponseWriter`/
  `http.Request`; repository não conhece DTOs. **MAJOR**

## 16. Testabilidade e qualidade
- [ ] Service recebe interface de repository; handler, interface de service.
  **MAJOR**
- [ ] Service test cobre sucesso, validação, not-found, erro de repo; handler
  test cobre sucesso, decode error, erro do service mapeado. Mocks handcraft,
  sem framework. **MAJOR**
- [ ] Sem dead code, magic string repetida, TODO sem rastreio. **MINOR**
- [ ] `go vet` e `golangci-lint` limpos. **MINOR**

---

## Severidade

| Nível | Quando |
|-------|--------|
| **BLOCKER** | Impede funcionamento correto ou abre falha de segurança: SQL injection, tenant do cliente, rota privada sem auth, panic, código que não compila |
| **MAJOR** | Viola o SDK ou introduz bug latente: bootstrap manual, `os.Getenv` em código folha, retry não idempotente, `Nack` em mensagem venenosa |
| **MINOR** | Desvio de convenção sem impacto funcional: nome fora de padrão, `Info` demais |
| **SUGGESTION** | Melhoria opcional: extrair constante, refinar mensagem |
