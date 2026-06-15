# Bridge / Factory / Adapter — padrão para dimensão polimórfica externa

Use **quando** o contexto integra com **N implementações intercambiáveis** de
uma dimensão externa — marketplaces, payment gateways, shippers, identity
providers federados. Cada implementação compartilha o mesmo contrato de
operações, mas a escolha entre elas é dado de runtime (campo da entidade,
tenant, request).

**Não use** quando há **uma única implementação** que nunca vai ser trocada
(ex: o único provider de IAM do produto, o único JWT signer). Esse caso fica
em `services/domain/{ctx}/adapter/` — sem bridge, sem factory.

---

## Layout canônico

```
services/
  domain/{ctx}/
    bridge/{ctx}_bridge.go         ← interface — contrato puro
    factory/{ctx}_factory.go       ← registry tipada + Get(key) → Bridge
    model/                         ← entidades + DTOs cross-bridge (ProductCampaignSpec, …)
    application/                   ← use cases — workflow (resolve bridge → fetch → delega service)
    service/                       ← domain service — loop+hidratação+repo.Save, lookup, delete-by-policy
    repository/                    ← persistência

  adapter/{tech}/{ctx}/            ← top-level — uma pasta por (tech, ctx)
    {ctx}_bridge.go                ← implementa bridge.Bridge
    registry.go                    ← func Register(*factory.Factory) + const Key
    mapper.go                      ← DTO externo → model interno
    dto.go                         ← wire format do provider
    mapper_test.go
```

> **Application + service split é obrigatório quando há bridge/factory.** Use
> case (application) faz o workflow externo; domain service (service)
> faz a parte de domínio (hidratação, persistência). Detalhes em
> `.claude/knowledge/shared/application-vs-domain-service.md`.

Exemplo do shape:

```
services/domain/{ctx}/{bridge,factory,model,service,repository}/
services/adapter/{tech1}/{ctx}/
services/adapter/{tech2}/{ctx}/      ← adicionar tech nova: zero mudança em domain/{ctx}
```

---

## Regras de dependência (invioláveis)

1. **`bridge/` e `factory/` NÃO importam nada de `adapter/`.** Acoplamento é
   contra-direção apenas. Quebrar = import cycle inevitável.
2. **`adapter/{tech}/{ctx}/` importa só `domain/{ctx}/{bridge,model}` + libs
   externas.** Nunca importa `service/` nem `repository/` — adapter implementa
   o port, não consome o domínio.
3. **Registro do adapter na factory acontece SÓ no composition root** (ex:
   `{pathCmd}/wire.go`). **Não usar `init()` no adapter** — registro
   implícito é hostil a testes e impossível de desligar.
4. **Service depende de `*factory.Factory` (concreto), não da interface
   adapter.** Service nunca conhece adapters específicos. Para tests, criar
   factory real + registrar mock bridge.

---

## Templates

### `bridge/{ctx}_bridge.go`

```go
package bridge

import (
    "context"

    "<module>/services/common/contracts"
    "<module>/services/domain/{ctx}/model"
)

type {Ctx}Bridge interface {
    DoThing(ctx context.Context, account contracts.AccountInfo, spec model.{Ctx}Spec) ([]model.{Entity}, error)
}
```

Uma interface por contexto. Métodos múltiplos OK se compartilham mesmo
recurso/scope. Split em N interfaces (ISP) só se há implementação parcial
real (adapter A implementa `OpX` + `OpY`, adapter B implementa só `OpX`).

### `factory/{ctx}_factory.go`

```go
package factory

import (
    "fmt"
    "sync"

    "<module>/services/domain/{ctx}/bridge"
)

type BridgeBuilder func() bridge.{Ctx}Bridge

// Factory cacheia 1 bridge por key. Bridge é stateless (HTTP client + rate
// limiter), thread-safe — múltiplos consumers/workers da mesma key
// compartilham o MESMO HTTP client.
//
// **Por que cache é obrigatório:** sem cache, cada Get() chama build() que
// cria HTTP client novo com rate limiter próprio → N consumers × M workers =
// N×M clients independentes, cada um com seu budget local. Mas o serviço
// externo (marketplace, IDP, gateway) **limita por TOKEN, não por client** →
// estoura rate limit real. Bug latente que só explode quando split de
// consumer multiplica o ataque (ver `worker-bootstrap.md` § split por type).
type Factory struct {
    registry map[uint8]BridgeBuilder
    cache    sync.Map // key: uint8, value: bridge.{Ctx}Bridge (envelopada por decorators)
}

func New() *Factory {
    return &Factory{registry: make(map[uint8]BridgeBuilder)}
}

func (f *Factory) Register(key uint8, build BridgeBuilder) {
    f.registry[key] = build
}

// Get devolve a bridge **cacheada por key** (lazy — construída no 1º Get).
// LoadOrStore é atômico: se 2 goroutines chegam ao mesmo Get inicial, ambas
// chamam build(), mas só a 1ª Store ganha; as outras descartam o build()
// extra e usam o cached. Sem race.
func (f *Factory) Get(key uint8) (bridge.{Ctx}Bridge, error) {
    if cached, ok := f.cache.Load(key); ok {
        return cached.(bridge.{Ctx}Bridge), nil
    }
    build, ok := f.registry[key]
    if !ok {
        return nil, fmt.Errorf("{ctx} bridge not registered for key %d", key)
    }
    // Envelopa com decorators (observabilidade etc.) ANTES de cachear —
    // todos os callers compartilham o mesmo wrapper instrumentado.
    wrapped := obs.WrapBridge(build(), keyLabel(key))
    actual, _ := f.cache.LoadOrStore(key, wrapped)
    return actual.(bridge.{Ctx}Bridge), nil
}
```

- Chave tipada pela dimensão (uint8 para marketplaceID, string para slug,
  etc.) — escolher o tipo que combina com o campo da `AccountInfo`/entidade.
- `BridgeBuilder` é **factory de instância** chamada **uma única vez** por key
  (após o 1º Get, cache devolve). Se o adapter mantém state interno (cache
  de auth token, conexão persistente, etc.), o state vira singleton por key —
  é o que queremos.
- Sem locks no `registry`: write-once no boot, read-only depois (Register
  só roda em `wire.go`). `cache` usa `sync.Map` por ser **read-heavy** (raro
  novo Get após warmup).
- **Anti-padrão crítico:** factory sem cache. Single consumer pode aguentar
  (1 client/process), mas qualquer split de consumer (split por type no
  mesmo binário ou separação em binários distintos) escala o ataque
  linearmente sem ninguém perceber até o serviço externo devolver `429`.

### Decorators no factory — observabilidade, retry, circuit breaker

A bridge envolvida por decorator no `Get()` mantém o adapter **zero-acoplado**
a cross-cutting concerns. Padrão recomendado:

```go
// Decorator 1: observabilidade (MeteredBridge captura latência/status/reason)
wrapped := obs.WrapBridge(build(), keyLabel(key))

// Decorator 2 (opcional): retry com backoff exponencial
wrapped = retry.WrapBridge(wrapped, retry.Config{...})

// Decorator 3 (opcional): circuit breaker
wrapped = cb.WrapBridge(wrapped, cb.Config{...})

f.cache.LoadOrStore(key, wrapped)
```

Cada decorator implementa `bridge.{Ctx}Bridge` envolvendo o `inner` da camada
abaixo. Adapters reais (`adapter-A`, `adapter-B`, etc.) **não conhecem**
observability/retry/cb — ganham tudo via composição no `Get()`. Vide
`observability-otel.md` para o template completo do `MeteredBridge` (decorator
de instrumentação).

**Princípio:** 1 lugar (factory) configura cross-cutting; N adapters ficam
limpos. **Anti-padrão:** instrumentar dentro do adapter — viola DRY e cria
divergência entre adapters quando alguém esquece de adicionar.

### `adapter/{tech}/{ctx}/registry.go`

```go
package {ctx}

import (
    "<module>/services/domain/{ctx}/bridge"
    "<module>/services/domain/{ctx}/factory"
)

const Key uint8 = 2  // marketplaceID, slug, …

func Register(f *factory.Factory) {
    f.Register(Key, func() bridge.{Ctx}Bridge { return NewBridge() })
}
```

- Package name = mesmo do domínio (`{ctx}`, não `{tech}{Ctx}`) —
  import com alias no wire (`{tech}{Ctx}Pkg "<module>/services/adapter/{tech}/{ctx}"`).
- Const da chave fica no adapter, não no factory — adapter é dono da sua
  identidade.

### `application/{aggregate}_application.go` — workflow orquestrador

```go
type {Aggregate}Application interface {
    Execute(ctx context.Context, account contracts.AccountInfo) errs.AppError
}

type {aggregate}Application struct {
    bridges *factory.Factory
    service service.{Ctx}Service
}

func New{Aggregate}Application(bridges *factory.Factory, svc service.{Ctx}Service) {Aggregate}Application {
    return &{aggregate}Application{bridges: bridges, service: svc}
}

func (a *{aggregate}Application) Execute(ctx context.Context, account contracts.AccountInfo) errs.AppError {
    bridge, err := a.bridges.Get(account.MarketplaceID)
    if err != nil { return Err{Ctx}BridgeNotFound.Wrap(err, account.MarketplaceID) }

    items, err := bridge.FetchThing(ctx, account)
    if err != nil { return Err{Ctx}Fetch.Wrap(err) }

    return a.service.Create{Aggregate}s(ctx, account, items)
}
```

- Application **não toca repository**. Delega persistência ao domain service.
- Resolve bridge no início; falha cedo se factory não tem entrada.
- Erros: `Err{Ctx}BridgeNotFound`/`Err{Ctx}Fetch` ficam em `application/errors.go`.
- **Só workflows reais** entram em application (ingestão, saga, coordenação). Lookup/delete-by-policy/CRUD trivial **ficam no service** — caller chama direto.

### `service/{ctx}_service.go` — domain service (persistência + hidratação)

```go
func (s *{ctx}Service) Create{Aggregate}s(ctx context.Context, owner contracts.AccountInfo, items []model.{Aggregate}) errs.AppError {
    var lastErr errs.AppError
    for _, it := range items {
        it.CoreAccountID = owner.AccountID  // hidratação de tenancy = domain policy
        if err := s.repo.Save(ctx, it); err != nil {
            lastErr = Err{Ctx}Persist.Wrap(err)
            logging.Error("...", slog.Any("error", err), slog.String("id", it.ExternalID))
        }
    }
    return lastErr
}
```

- Service hidrata campos de tenancy (`CoreAccountID`, `CoreCompanyID`,
  `CoreMarketplaceID`) **antes** de persistir — adapter não conhece esses
  campos do schema interno; application só passa o `account`.
- Loop com `lastErr` mantém o batch (`continue on error`); semântica
  "best-effort" típica de ingestão. Se a regra for fail-fast, use `return`.
- Erros: `Err{Ctx}Persist`/`Err{Ctx}Lookup` ficam em `service/errors.go`.

### `wire.go` — composition root

```go
type {ctx}Wiring struct {
    {workflowA}Application {ctxAppPkg}.{WorkflowA}Application
    {workflowB}Application {ctxAppPkg}.{WorkflowB}Application
    service                {ctxSvcPkg}.{Ctx}Service  // exposto pra ops simples (get, delete-by-policy)
}

func build{Ctx}Wiring(repos {ctx}Repos) {ctx}Wiring {
    bridges := {ctxFactoryPkg}.New()
    {tech1Pkg}.Register(bridges)
    // {tech2Pkg}.Register(bridges)   ← adicionar adapter = uma linha
    svc := {ctxSvcPkg}.New{Ctx}Service(repos.{aggregateA}, repos.{aggregateB})
    return {ctx}Wiring{
        {workflowA}Application: {ctxAppPkg}.New{WorkflowA}Application(bridges, svc),
        {workflowB}Application: {ctxAppPkg}.New{WorkflowB}Application(bridges, svc),
        service:                svc,
    }
}
```

- Service construído **uma vez**, injetado em cada application + exposto pra ops simples.
- Application só pra workflows com bridge (ex: ingest, execute external); lookup/delete vai direto pelo `service`.
- Registro de adapter na factory é **uma linha** por tech.

---

## Testes

### Application test — usa factory real + mockBridge + mockService

```go
type mockBridge struct { items []model.{Entity}; err error; calls int }
type mockService struct {
    createCalls int
    createItems []model.{Aggregate}
    createErr   errs.AppError
}

func (m *mockService) Create{Aggregate}s(_, _, items) errs.AppError {
    m.createCalls++; m.createItems = items
    return m.createErr
}

func newTestFactory(b bridge.{Ctx}Bridge) *factory.Factory {
    f := factory.New()
    f.Register(testKey, func() bridge.{Ctx}Bridge { return b })
    return f
}
```

Application **mocka service** (não repository). Reusa o tipo concreto
`*factory.Factory` (sem extrair interface só pra teste).

### Service test — mockRepo handcraft (repository da camada de baixo)

```go
type mockCampaignRepo struct {
    saved   []model.{Aggregate}
    saveErr error
}
```

Service mocka **repository**, não conhece bridge.

### Adapter test — só mapper, sem rede

Não testar `bridge.FetchThing` (depende de HTTP). Testar **mapper puro**
(`mapDTO → model`) e helpers (`normalize*`, `pick*`). Integration test do
HTTP fica fora do unit test.

---

## Bridge com operações de escrita (read + write)

A bridge tipicamente começa **read-only** (fetch/sync de dados externos) e
pode crescer para **read + write** (aplicar ações no sistema externo) quando
o contexto evolui de "ingestão" para "ingestão + execução". Esse é um
padrão comum quando se separa **decider** (decide) de **executor** (age) —
o decider produz eventos com decisões; o executor consome e chama as
operações de escrita da bridge.

### Estratégias para crescer a bridge

**A — Interface única que cresce (recomendado quando todos os adapters
suportam todas as operações):**

```go
type {Ctx}Bridge interface {
    // Leitura (consumida por workflows de ingestão)
    Fetch{X}s(ctx, owner) ([]model.{X}, error)
    Fetch{Y}s(ctx, owner, spec) ([]model.{Y}, error)

    // Escrita (consumida por workflows de execução)
    Apply{Z}(ctx, owner, entity, params) error
    Revert{Z}(ctx, owner, entity) error
}
```

- **Todos os adapters implementam todos os métodos.** Adapter que ainda não
  suporta um método retorna erro estável (ex: `ErrOperationNotSupported`)
  até a implementação real chegar.
- **Vantagem:** uma única factory, uma única bridge, callers misturam read
  e write sem branching.
- **Desvantagem:** adapter incompleto carrega métodos "no-op com erro"
  até virar funcional. Aceitável quando o gap é temporário.

**B — Duas interfaces separadas (ISP estrita; usar quando há adapters
genuinamente read-only ou genuinamente write-only):**

```go
type {Ctx}Reader interface {
    Fetch{X}s(ctx, owner) ([]model.{X}, error)
}

type {Ctx}Writer interface {
    Apply{Z}(ctx, owner, entity, params) error
    Revert{Z}(ctx, owner, entity) error
}
```

- **Duas factories** (`{ctx}ReaderFactory`, `{ctx}WriterFactory`) **ou** uma
  factory com dois métodos `GetReader(key)` / `GetWriter(key)`.
- Adapter implementa só o que faz sentido (`type {Tech}Reader struct{...}`).
- **Usar só quando** há motivo real — adapter analítico (só lê) coexistindo
  com adapter transacional (só escreve). Em projeto onde **todos** os
  adapters fazem read e write, B é overhead.

### Padrão recomendado: A com erros estáveis

Comece com interface única. Se aparecer um adapter realmente read-only (ou
write-only) **e a divergência for permanente**, refatore para B. Não comece
com B "por elegância" — viola YAGNI.

### Quando read e write estão em workflows diferentes

A bridge cresce, **mas o uso fica separado**:

- `application/{workflowA}_application.go` chama **só** os métodos de leitura
  (sync, ingestão, validação periódica).
- `application/{workflowB}_application.go` chama **só** os métodos de
  escrita (aplicar decisão recebida via evento, etc.).
- Domain service de cada workflow não conhece os métodos do outro lado.

Isso preserva o single-responsibility por workflow sem dividir a interface.

### Implicação para o adapter

```go
// services/adapter/{tech}/{ctx}/{ctx}_bridge.go

type bridge struct { client *netx.HttpClient }

func (b *bridge) Fetch{X}s(ctx, owner) ([]model.{X}, error) { /* GET /api/... */ }
func (b *bridge) Apply{Z}(ctx, owner, entity, params) error  { /* POST /api/... */ }
func (b *bridge) Revert{Z}(ctx, owner, entity) error         { /* DELETE /api/... */ }
```

- Adapter agora tem **endpoints de leitura e de escrita**.
- Mapper continua sendo o coração testável; rede continua fora dos unit tests.
- Tratamento de erro do adapter de escrita exige classificação **transient
  vs permanent** (`5xx`/`429`/timeout vs `4xx`/regra externa violada) —
  caller (executor) lê o tipo do erro para decidir retry. Detalhes do
  padrão executor em `.claude/knowledge/shared/event-driven-executor-pattern.md`.

---

## Template canônico do adapter HTTP

Quando o adapter integra com **API HTTP externa** (marketplaces, gateways de
pagamento, shippers), este é o shape mínimo. Desvios exigem justificativa
explícita — a uniformidade é o que permite trocar adapters/marketplaces sem
reler tudo.

### Anatomia (ordem dos elementos no arquivo)

```go
package {ctx}

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "time"

    "<module>/services/domain/integration/token"
    {ctx}Bridge "<module>/services/domain/{ctx}/bridge"
    "github.com/joaoprofile/gofi/base/errs"
    "github.com/joaoprofile/gofi/netx"
    "github.com/joaoprofile/gofi/obs/logging"
)

// 1. URL/path constants + tuning constants
const (
    {tech}BaseURL    = "https://api.{tech}.com"
    {tech}{Op}Path   = "/some/path/%s"

    bridgeHTTPTimeout    = 5 * time.Second
    bridgeHTTPRetries    = 3
    bridgeHTTPRetrySleep = 1 * time.Second
    bridgeHTTPRateLimit  = 50
)

// 2. Error variables — uma por outcome estável; registro estático.
//    Convenção: ErrTechCtxOperationOutcome (TRANSIENT/PERMANENT/RATELIMITED/AUTH/...).
var (
    Err{Tech}{Ctx}{Op}Transient   = errs.RegisterExternalError("{TECH}_{CTX}_{OP}_TRANSIENT",    "{tech} {ctx} {op}: transient failure (5xx/timeout/network)")
    Err{Tech}{Ctx}{Op}RateLimited = errs.RegisterExternalError("{TECH}_{CTX}_{OP}_RATE_LIMITED", "{tech} {ctx} {op}: rate limited (429)")
    Err{Tech}{Ctx}{Op}Auth        = errs.RegisterExternalError("{TECH}_{CTX}_{OP}_AUTH",         "{tech} {ctx} {op}: unauthorized (401)")
    Err{Tech}{Ctx}{Op}Permanent   = errs.RegisterExternalError("{TECH}_{CTX}_{OP}_PERMANENT",    "{tech} {ctx} {op}: permanent failure (4xx)")
)

// 3. Wire DTOs — privados, só do adapter. Domain NÃO importa.
type {op}Body struct {
    PromotionID string  `json:"promotion_id"`
    DealPrice   float64 `json:"deal_price"`
    // ...
}

type {op}Response struct {
    ID string `json:"id"`
    // ...
}

// 4. Struct + constructor — devolve a interface do DOMAIN, não o tipo concreto.
type bridge struct {
    client *netx.HttpClient
}

func NewBridge() {ctx}Bridge.{Op}Bridge {
    client, err := netx.NewClient(&netx.HttpClientConfig{
        Name:       "{Tech}{Ctx}Bridge",
        BaseURL:    {tech}BaseURL,
        Timeout:    bridgeHTTPTimeout,
        Retries:    bridgeHTTPRetries,
        RetrySleep: bridgeHTTPRetrySleep,
        RateLimit:  bridgeHTTPRateLimit,
    })
    if err != nil {
        logging.Fatal("{tech}.{ctx}.bridge.client_create_failed", slog.Any("error", err))
    }
    return &bridge{client: client}
}

// 5. Métodos — token é argumento separado; entrada é DTO do domain; saída é (*Result, errs.AppError)
func (b *bridge) {Op}(ctx context.Context, tkn *token.Token, in {ctx}Bridge.{Op}Input) (*{ctx}Bridge.{Op}Result, errs.AppError) {
    url := fmt.Sprintf({tech}{Op}Path, in.SkuMarketplace)
    req := netx.NewRequest[{op}Response](ctx, b.client, http.MethodPost, url)
    req.SetHeader("Authorization", "Bearer "+tkn.AccessToken)
    req.SetBody(&{op}Body{ /* mapping */ })

    resp, err := req.Execute()
    if err != nil {
        return nil, classify{Op}Error(err)
    }
    return &{ctx}Bridge.{Op}Result{ExternalID: resp.ID}, errs.AppError{}
}

// 6. Classificador local — HTTP status → AppError tipado.
//    Adapter CONHECE a API externa, então classifica aqui (não na application).
func classify{Op}Error(err error) errs.AppError {
    httpErr := netx.FromError(err)
    status := 0
    if httpErr != nil {
        status = httpErr.Status
    }
    switch {
    case status == http.StatusUnauthorized:
        return Err{Tech}{Ctx}{Op}Auth.Wrap(err)
    case status == http.StatusTooManyRequests:
        return Err{Tech}{Ctx}{Op}RateLimited.Wrap(err)
    case status >= 500 || status == 0:  // 0 = timeout/network
        return Err{Tech}{Ctx}{Op}Transient.Wrap(err)
    default:
        return Err{Tech}{Ctx}{Op}Permanent.Wrap(err)
    }
}
```

### Regras invioláveis do adapter HTTP

1. **Token vai como argumento separado** (`tkn *token.Token`), **nunca**
   dentro do `Input` struct. Separa credencial de dados da operação;
   application resolve token via `token.TokenResolver`. Anti-padrão:
   `type AdhereInput struct { Token string; ... }` — viola separação.
2. **Return canônico é `(*Result, errs.AppError)`** — pointer no result,
   `errs.AppError` (não `error`). Empty struct `errs.AppError{}` = sucesso.
   Em erro: `return nil, ErrXxx.Wrap(err)`. Anti-padrão: `(Result, error)` —
   perde a tipagem rica de `AppError` (code, kind, details).
3. **Input/Output structs moram no DOMAIN (`bridge/`)**, não no adapter.
   Todos os adapters compartilham a mesma forma; o wire format JSON
   (`{op}Body`/`{op}Response`) é detalhe do adapter (privado).
4. **Erros registrados via `errs.RegisterExternalError("CODE", "msg")`** no
   topo do arquivo — código UPPER_SNAKE com prefixo `{TECH}_{CTX}_{OP}_`
   + sufixo do outcome (`TRANSIENT`/`PERMANENT`/`AUTH`/`RATE_LIMITED`).
   Caller mapeia código → semântica de retry/event_code.
5. **Adapter classifica HTTP status → AppError tipado**, não devolve `error`
   cru com status separado. Application/executor consome o **código** do
   `AppError`, não o `int` do status. Anti-padrão: bridge devolve
   `(Result, error)` + caller faz `if status >= 500 ...` — duplica
   conhecimento da API em dois lugares.
6. **Constructor devolve a interface do domain** (`{ctx}Bridge.{X}Bridge`),
   nunca o tipo concreto `*bridge`. Mantém o domain como dono do contrato.
7. **Client criado uma vez no constructor**; falha de criação é
   `logging.Fatal` — sem client funcional, não há razão para o processo
   subir. Anti-padrão: `return nil` no constructor de erro.
8. **URL paths e tuning de HTTP são constantes no topo** — `bridgeHTTPTimeout`,
   `bridgeHTTPRetries`, `bridgeHTTPRetrySleep`, `bridgeHTTPRateLimit`. Mudar
   tuning é PR rastreável, não env var.
9. **Wire DTOs (`{op}Body`/`{op}Response`) são privados** (lowercase) e ficam
   no mesmo arquivo do adapter. Não vazam pro domain.
10. **Header de auth é único e padronizado** — `req.SetHeader("Authorization", "Bearer "+tkn.AccessToken)`.
    Outras formas (cookie, HMAC) viram helper se aparecerem; default é Bearer.
11. **`url := fmt.Sprintf(path, args...)`** — path tem placeholders `%s`/`%d`;
    nunca concat de strings com `+`. Query params estáticos ficam no path
    constant; dinâmicos viram `url.Values.Encode()`.

### Precedente no repo

Procurar implementações canônicas via grep no projeto:
- `services/adapter/{tech}/{ctx}/bridge.go` — implementação canônica
  (operação única ou múltipla, retorno `(*Result, errs.AppError)`).
- `services/adapter/{tech}/{ctx}/execution_bridge.go` — adapter com
  múltiplas operações + classificação multi-outcome.

---

## Quando NÃO usar (anti-padrões)

- **Implementação única hoje, "talvez no futuro" tenha mais.** YAGNI —
  começa com adapter direto em `domain/{ctx}/adapter/` e só extrai pro padrão
  quando o segundo adapter aparecer de verdade.
- **Dimensão interna, não externa.** Strategy entre algoritmos (`fast` vs
  `accurate`, `v1` vs `v2`) é polimorfismo de domínio, não bridge — resolve
  com interface no service ou type switch.
- **Adapter precisa ler/escrever no banco do domínio.** Sinal de que é
  service disfarçado. Adapter integra com **externo**; persistência interna
  fica no service do domínio.

---

## Setup obrigatório de tests com logging

Service com `logging.Error(...)` panica em test sem inicializar o logger.
Cada package de service que loga precisa de:

```go
// service/setup_test.go
package service

import (
    "context"
    "os"
    "testing"

    "github.com/joaoprofile/gofi/obs/logging"
)

func TestMain(m *testing.M) {
    _ = logging.InitGlobal(context.Background(), logging.Config{ServiceName: "{ctx}-test"})
    os.Exit(m.Run())
}
```
