# Application service vs Domain service

Princípio cross-agent: **gofi-spec** declara qual camada possui cada operação,
**gofi-eng** implementa respeitando o split, **gofi-qa** audita a separação.

## Conceitos

### Domain service (`service/`)

**Comportamento de domínio que não cabe numa entidade** mas é policy do
negócio: validação cross-aggregate, hidratação de campos de tenancy,
cálculo de regra que envolve múltiplas entidades, persistência em
batch com consistência de domínio. Não conhece transporte, não conhece
workflow externo, não chama integrações.

Sinais que algo é domain service:
- Lê/escreve via repository
- Aplica regra de domínio (validação, hidratação, normalização)
- Pode ser reusado por N workflows diferentes
- Pura no sentido de domínio (sem chamada a sistemas externos)

### Application service (`application/`)

**Use case** — coordena a unidade de intenção do sistema:
"ingerir dados de fonte externa", "publicar entidade", "executar decisão
recebida via evento". Resolve qual bridge/integração usar, chama a bridge,
delega persistência ao domain service, publica eventos, gerencia transação.

Sinais que algo é application service:
- Coordena bridge/factory/ports externos
- Tem boundary de transação, idempotência, retry, saga
- Servido por ≥1 transporte (HTTP, scheduler, Kafka consumer, CLI)
- 1:1 com intenção do usuário/sistema (não é regra de domínio sozinha)

## Regra de quando criar cada camada

| Situação | service/ | application/ |
|---|---|---|
| Contexto CRUD trivial (sem bridge, sem workflow multi-step) | ✅ | ❌ (orquestração mora no service) |
| Contexto com bridge/factory (dimensão polimórfica externa) | ✅ | ✅ |
| Contexto que coordena ≥2 domínios | ✅ (de cada domínio) | ✅ (orquestrador) |
| Workflow com transação multi-passo, outbox, saga | ✅ | ✅ (donos da tx) |
| Mesmo use case servido por ≥2 transportes | ✅ | ✅ (use case agnóstico de transporte) |
| Persistência simples + lógica de domínio em entity | ✅ (com hidratação/validação) | ❌ se workflow é trivial |

**Heurística rápida:** se a operação envolve **bridge externa**, **transação
complexa**, **coordenação cross-domain** ou **múltiplos transportes**,
extraia `application/`. Se é só "validar + persistir + ler", fica em
`service/` e dispensa `application/`.

> **Não fabricar camada vazia.** Se o contexto tem 5 endpoints CRUD sem
> bridge e sem workflow complexo, `application/` seria 5 passthroughs de
> 3 linhas — overhead sem retorno. Documente a decisão na spec.

## Direção de dependência (inviolável)

```
handler → application → service → repository
                      ↘ bridge (port externo)
              ↘ factory → bridge
```

- **application importa service e bridge/factory** (camada que orquestra).
- **service importa repository e model** (camada de domínio).
- **service NUNCA importa application nem bridge.** Domain service não
  conhece use case nem integração externa — se precisa, é sinal de que a
  responsabilidade subiu pra application.
- **Handler importa application** (e não service direto), exceto em
  contextos sem application/ (CRUD trivial — handler chama service direto).

Quebrar esses ifs vira import cycle ou layering inversion.

## Responsabilidade de erros por camada

| Camada | Tipos de erro |
|---|---|
| `application/errors.go` | bridge/factory (`*_BRIDGE_NOT_FOUND`), fetch externa (`*_FETCH_*_FAILED`), validação de input do use case |
| `service/errors.go` | persistência (`*_PERSIST_*_FAILED`), lookup (`*_LOOKUP_*_FAILED`), regra de domínio violada (`*_VALIDATION_FAILED`) |
| `repository` | retorna erros crus do driver/SQL — service traduz pra `errs.AppError` |

**Não duplicar.** Cada código de erro mora numa só camada. Quando a fronteira
é ambígua, a regra é: o erro pertence à camada que **gerou** ele (não à
camada que o leu).

## Como cada agent usa isso

### gofi-spec
- Em §3.1 (Operações), marcar cada operação como **use case** (application)
  ou **domain operation** (service direto).
- Em §3.2 (Contratos), declarar interfaces das duas camadas separadamente
  quando ambas existirem.
- Aplicar a heurística antes de propor estrutura — se nenhum critério de
  `application/` é satisfeito, **declarar explicitamente** "sem camada
  application — operações ficam em domain service".

### gofi-eng
- Implementar respeitando o split declarado pela spec.
- Se a spec for ambígua, **perguntar antes de criar arquivos** — não
  inventar nem service/ nem application/.
- Tests de cada camada usam mocks da camada imediatamente abaixo:
  - service test mocka repository
  - application test mocka service (não mocka repository nem bridge
    diretamente, exceto pra cobrir a parte de bridge)

### gofi-qa
- Auditar que application **não chama repository direto** (vazamento de
  camada). Application chama service; service chama repository.
- Auditar que service **não importa** bridge/factory/application.
- Auditar que erros estão na camada certa (persistência em service, bridge
  em application).
- Auditar que tests da application mockam service (e não repository) —
  test passando direto na repository é sinal de service inflado/inexistente.

## Templates

### Domain service típico

```go
type {Ctx}Service interface {
    Create{Aggregate}s(ctx context.Context, owner Owner, items []model.{Aggregate}) errs.AppError
    Get{Aggregate}s(ctx context.Context, owner Owner) ([]model.{Aggregate}, errs.AppError)
    Delete{Aggregate}sByPolicy(ctx context.Context) errs.AppError
}

func (s *{ctx}Service) Create{Aggregate}s(ctx, owner, items) errs.AppError {
    var lastErr errs.AppError
    for _, it := range items {
        it.OwnerID = owner.ID  // hidratação de tenancy = domain policy
        if err := s.repo.Save(ctx, it); err != nil {
            lastErr = Err{Ctx}Persist.Wrap(err)
            logging.Error(...)
        }
    }
    return lastErr
}
```

### Application use case típico

```go
type Ingest{Ctx}UseCase struct {
    bridges *factory.Factory
    service service.{Ctx}Service
}

func (u *Ingest{Ctx}UseCase) Execute(ctx, owner) errs.AppError {
    bridge, err := u.bridges.Get(owner.MarketplaceID)
    if err != nil { return Err{Ctx}BridgeNotFound.Wrap(err, owner.MarketplaceID) }

    items, err := bridge.Fetch(ctx, owner)
    if err != nil { return Err{Ctx}Fetch.Wrap(err) }

    return u.service.Create{Aggregate}s(ctx, owner, items)
}
```

Use case canônico: **3 blocos** (resolve bridge, chama bridge, delega ao
service). Se ficar maior, é sinal de coordenação não-trivial — explicita
saga/transação na spec antes de implementar.

## Granularidade do application/

Três convenções possíveis:

**A. Uma struct por use case orquestrador** (padrão recomendado):
- `application/{aggregate}_application.go` → `{Aggregate}Application` interface com `Execute(ctx, ...)`
- **Só workflows reais** entram em application: ingestão, saga, coordenação cross-domain, transação multi-passo
- Ops simples de domain (CRUD direto, lookup, delete-by-policy) **ficam no service** e são chamadas diretas pelo caller — sem application wrapper redundante
- Vantagem: application/ vira mapa fiel dos workflows do contexto; CRUD trivial não polui a lista
- Desvantagem: caller (handler/scheduler) tem dois pontos de entrada (application pra workflow, service pra ops simples)

**B. Uma struct por use case incluindo CRUD** (Vaughn Vernon estrito):
- Cada operação do contexto = uma struct
- Vantagem: simétrico, todos os callers passam por application
- Desvantagem: para CRUD trivial, application vira passthrough de 3 linhas

**C. Uma struct por aggregate com N métodos** (Evans/Fowler):
- `application/{ctx}_application.go` → interface com N métodos
- Vantagem: menos arquivos
- Desvantagem: deps são união (todos os métodos compartilham), atrapalha quando workflows têm deps heterogêneas

**Padrão recomendado: A**. Um arquivo `application/{workflow}_application.go`
por workflow orquestrador real, cada um com interface `{Workflow}Application`
+ método `Execute(ctx, ...)`. Ops simples de domínio (`Get*ByOwner`,
`DeleteExpired*`, CRUD trivial) ficam em `service/` e o caller (handler /
scheduler / consumer) chama direto, sem application wrapper redundante.

### Como decidir o que mora em application/

| Operação | Mora em |
|---|---|
| Ingestão externa (fetch via bridge + persist via service) | `application/` |
| Saga / transação multi-passo / outbox / coordenação cross-domain | `application/` |
| Lookup simples (`Find*ByAccount`, `GetMe`) | `service/` direto |
| Delete-by-policy (`DeleteExpired*`, `PurgeBy*`) | `service/` direto |
| CRUD trivial (Create, Rename, ChangeStatus) | `service/` direto |
| Operação que só hidrata + persiste sem fetch externo | `service/` direto |

**Heurística:** se o método chama bridge/factory ou coordena 2+ services,
vai pra application. Senão fica no service e o caller acessa direto.

## Anti-padrões

- **Service chamando bridge/factory** — vira "application disfarçada", quebra
  isolamento de domínio. Solução: extrair pra application/.
- **Application chamando repository direto** — bypass do service, perde a
  hidratação/validação. Solução: passar pelo service.
- **Domain service sem nada de domínio** — só envelope sobre repo. Sinal de
  modelo anêmico; ou move a lógica pra service real, ou remove a camada
  (handler → repo direto) se for CRUD genuinamente vazio.
- **Application com lógica de domínio** — validação, normalização,
  hidratação que não envolve recurso externo. Sinal: o código existiria
  igual se o transporte mudasse. Solução: descer pro service.
- **Duplicação de erros entre camadas** — `Err{Ctx}Save` em service E
  application. Sinal de que a tradução de erro está incoerente. Cada código
  mora em uma camada só.

---

## Recompute de agregado derivado (rollup denormalizado) — owner, best-effort, set-based

Quando um contexto **A** (ingestão de um fato — vendas, eventos, transações)
precisa manter um **compilado denormalizado** (rollup de janela móvel: total,
contagem, participação %) que **fisicamente vive numa tabela de outro contexto
B** (o read-model/agregado que dashboards leem), valem três regras:

1. **Propriedade é por-coluna, não por-tabela.** O contexto que **calcula** o
   rollup é o **escritor exclusivo** daquelas colunas; o contexto dono da
   tabela só **lê**. Documente isso (ADR + spec dos dois lados). A escrita
   cross-context é uma **exceção consciente** ao "cada contexto escreve só na
   sua tabela" — justificada porque o cálculo depende do raw que só A tem, e
   nenhum outro produtor toca essas colunas (sem contenção de escrita).

2. **Best-effort, fora da transação do raw.** O recompute roda **após** a
   persistência do fato cru (fonte da verdade). Se falhar: `WARN` + métrica,
   **não** derruba o pipeline / não dá Nack — o raw já está persistido e um
   job periódico (re-sync) recompila no próximo ciclo. Recompute derivado
   **nunca** deve fazer o consumer perder o trabalho de ingestão já feito.
   Roda **sempre** que a ingestão da entidade-pai termina com sucesso (mesmo
   com 0 itens novos), porque janelas móveis têm **roll-off temporal** que
   precisa ser reaplicado independentemente de novos fatos.

3. **Recompute set-based de todo o escopo, não reset+N updates.** Um único
   `UPDATE ... FROM (CTE de agregação)` recompila o escopo inteiro (tenant/
   conta) numa passada. Pontos críticos:
   - **Roll-off**: `LEFT JOIN` do agregado + filtro `(entrou no agregado OR
     rollup atual != 0)` zera quem saiu da janela e pula linhas que já são 0 e
     continuam 0 (sem write desnecessário).
   - **% de participação muda para TODOS quando o total do escopo muda** — não
     dá pra pular linhas não-zero "que não tiveram fato novo".
   - **Guard de divisão por zero** quando o denominador (total do escopo) é 0.
   - Evita o anti-padrão legado "zera tudo + atualiza linha-a-linha em N
     goroutines".

Camada: o **service** de A expõe `Refresh{X}Rollup(ctx, scopeID)`; a
**application** chama logo após o `Upsert`/persist. O `repository` de A faz o
`UPDATE` (prepared stmt). Semântica de cada métrica (o que conta como total,
contagem = unidades vs ocorrências, base da %) é **decisão de negócio** — vive
na spec/PRD, não aqui.
