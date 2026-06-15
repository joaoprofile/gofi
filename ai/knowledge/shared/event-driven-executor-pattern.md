# Event-Driven Executor — decider/executor split

Princípio cross-agent: quando um workflow envolve **decidir** uma ação e
**aplicar** essa ação em sistema externo, separar o pipeline em dois agents
(`decider` + `executor`) trocando mensagens por broker (Kafka/Pub-Sub/SQS).
Aplica-se a qualquer cenário onde a decisão é determinística sobre estado
local e a aplicação envolve I/O externo com latência/falha.

**gofi-spec** declara o split na §4 (Arquitetura) quando aplicável; **gofi-eng**
implementa respeitando o contrato de evento; **gofi-qa** audita idempotência
e completude do ciclo terminal.

---

## Quando usar

Aplicar **quando todas** as condições abaixo se aplicam:

1. **Decisão é função pura sobre estado local** — pode ser tomada sem
   chamar sistema externo (lê banco/cache local; calcula resultado).
2. **Aplicação envolve I/O externo** com latência variável, falhas
   parciais e/ou regras do sistema externo que podem rejeitar a operação.
3. **Decisão sem efeito não é problema** — múltiplas decisões idênticas em
   curto prazo são absorvidas por idempotência; o executor é tolerante a
   replay.
4. **Decisão tem ciclo de vida** com estados terminais (`APPLIED` / `FAILED` /
   `STALE`) consultáveis por auditoria.

Cenários típicos: aplicar config externa, sincronizar estado para fora,
participar de campanha externa, publicar/despublicar recurso, integrar com
gateway de pagamento, ajustar inventário externo, …

**Não usar** quando:

- Decisão e aplicação são **idênticas e síncronas** (CRUD local + side effect
  trivial) — overhead injustificado.
- **Estado externo é fonte da verdade** e local é só cache — usar
  reconciler pattern (worker periódico que diff entre desejado/atual)
  em vez de event-driven.
- **Decisão depende de feedback síncrono** do sistema externo (ex: "preciso
  do ID gerado pelo externo antes de poder decidir mais") — fluxo síncrono
  é mais simples.

---

## Arquitetura

```
┌─────────────┐                                ┌──────────────┐
│   Decider   │ ── produz evento decisão ───→  │  Tópico      │
│  (worker)   │   (decision_id UUID v7,       │  Kafka/Bus   │
│             │    action, target, snapshot)  │              │
└─────────────┘                                └──────┬───────┘
       │                                              │
       │ lê estado local                              │ partition key = entity_id
       │ (config, snapshot)                           │
       │                                              ↓
       │                                       ┌──────────────┐
       │                                       │   Executor   │ ── chama externo via
       │                                       │   (worker)   │    bridge/adapter
       │                                       │              │
       │                                       └──────┬───────┘
       │                                              │
       │              ┌───────────────────────────────┘
       │              │
       │              ↓
       │       INSERT INTO {ctx}_execution
       │       (decision_id UNIQUE) ON CONFLICT DO NOTHING
       │              │
       │              ↓ se não conflito
       │       Re-valida estado local + guard rails
       │              │
       │              ↓ se ainda válido
       │       Chama adapter (com retry)
       │              │
       │              ↓ sucesso
       │       Materializa estado local em tx
       │       + status = APPLIED
       │              │
       ↓              ↓
   Reavalia periodicamente; eventualmente emite
   nova decisão se estado mudar
```

### Contrato do evento

Campos **mínimos** que o evento carrega:

| Campo | Finalidade |
|-------|------------|
| `decision_id` | UUID v7 único por decisão; chave de idempotência |
| `entity_id` | Entidade-alvo (anúncio, usuário, pedido…); também a partition key |
| `action` | Enum UPPER_SNAKE_CASE com a operação (`APPLY_X` / `REVERT_X` / `SWITCH_X` …) |
| `target_*` | Identificador do recurso a aplicar (campanha, plano…) |
| `previous_*` | Identificador do recurso anterior (em ações que substituem) |
| `expected_*` | Valores que o decider calculou e o executor deve transmitir ao externo |
| `decided_at` | Timestamp da decisão — usado para cutoff de idade |
| `tenant_ids` | Identificadores de tenancy do projeto (ex.: `tenant_id` / `org_id` / `account_id`) para tenancy nos logs |
| `reason` | Motivo da decisão (auditoria) |
| `snapshot` | Estado lido pelo decider no momento da decisão (auditoria) |

`decision_id` UUID v7 (não v4) — time-ordered, ajuda particionamento de
tabela e ordenação cronológica natural.

### Partition key = entity_id

Garante **ordem por entidade**: dois eventos do mesmo `entity_id` chegam ao
executor em ordem cronológica (`APPLY_X` antes do `SWITCH_X` subsequente).
Eventos de entidades diferentes paralelizam em consumers distintos da
mesma partition group.

---

## Idempotência via UNIQUE em tabela

Padrão de proteção contra reentrega ("at-least-once" do broker):

### Tabela `{ctx}_execution`

| Campo | Finalidade |
|-------|------------|
| `decision_id` | **UNIQUE** (chave de idempotência) |
| `entity_id` | Entidade sob execução |
| `tenant_ids` | Tenancy + expurgo de churn |
| `action` | Ação solicitada |
| `target_*`, `previous_*` | Dados da decisão |
| `expected_*` | Valor esperado pelo decider |
| `status` | `PENDING` / `APPLIED` / `FAILED` / `STALE` |
| `attempt_count` | Número de tentativas feitas |
| `last_error` | Código curto do último erro |
| `decided_at` | Timestamp do evento original (cutoff de idade) |
| `started_at` | Quando o executor pegou |
| `completed_at` | Desfecho final |

### Padrão de consumo

```
1. INSERT INTO {ctx}_execution (decision_id, entity_id, ..., status=PENDING)
   ON CONFLICT (decision_id) DO NOTHING

2. Se INSERT não inseriu (conflito) → decisão já está sendo ou já foi processada
   → commit offset Kafka, log informativo ALREADY_PROCESSED
   → nada mais a fazer

3. Se INSERT inseriu → segue o fluxo normal
```

**Por quê banco e não cache:**
- `UNIQUE` no banco é exclusão **real**, mesmo com N instâncias do executor
  consumindo em paralelo.
- Cache (Redis SETNX) é mais rápido mas perde a garantia em failover/eviction.
- Banco serve simultaneamente como **auditoria perene** (cache não).

---

## Re-validação antes de aplicar

Entre decider emitir e executor consumir podem passar segundos a minutos
(broker lag, retry, fila). O estado local pode ter mudado — campanha
expirou, preço mudou, regra de produto foi alterada. **Re-validar é
barato e evita chamada inútil ao sistema externo.**

### O que re-validar

Para ações que dependem de estado mutável:

- **Existência e elegibilidade** do recurso-alvo (campanha ainda elegível,
  plano ainda ativo, etc.).
- **Guard rails do domínio** ainda passam (preço mínimo, margem, limite).
- Reusar o **mesmo serviço de domínio** que o decider usou — não duplicar
  lógica.

### Decisão sobre falha de re-validação

| Resultado | Ação |
|-----------|------|
| Tudo ok | Chama o externo |
| Recurso não-elegível | `status = STALE`, `last_error = target_not_eligible` |
| Guard rail falhou | `status = STALE`, `last_error = guard_rail_failed` |
| Idade > cutoff (default 24h) | `status = STALE`, sem nem re-validar (proteção contra burst após incidente) |

`STALE` é **estado terminal** — executor não retenta, não emite evento de
volta. **Decider reavalia naturalmente** no próximo ciclo (scheduler/trigger)
e emite nova decisão se ainda fizer sentido.

---

## Retry transient vs permanent

Erros do sistema externo classificados:

| Classe | Códigos típicos | Ação |
|--------|----------------|------|
| **Transient** | timeout, 5xx, 429 (rate limit), erros de rede | Retry com backoff exponencial |
| **Permanent** | 4xx (exceto 429), regra do externo violada | Sem retry; `FAILED` imediato |
| **Auth** | 401 (token expirado) | Renova token via context externo; conta como 1 retry transient |

### Backoff sugerido

5 tentativas com intervalos 1s, 4s, 16s, 64s, 256s (~5min total). Após a
5ª falha transient → `FAILED` com `last_error = max_retries_exceeded`.

Cada tentativa gera entrada no log estruturado com `attempt_count`, response
code, latência, classe do erro.

---

## Materialização atomic do estado local

Após sucesso no externo, executor escreve no banco local em **transação
única**:

```
BEGIN
  UPDATE {ctx}_execution SET status=APPLIED, completed_at=now() WHERE decision_id=...
  INSERT INTO {entity}_to_{target} (...)  -- materializa a junction concreta
  -- ou DELETE em ações de saída
  -- ou DELETE+INSERT em ações de switch (mesma tx)
COMMIT
```

A junction local (`{entity}_{target}` — ex.: `user_subscription`,
`account_plan`, …) é **alterada pelo executor**, não pelo decider. Decider
só decide; executor aplica e materializa.

### Por que atomic local

Se o INSERT na junction falhar depois do externo aceitar, ficamos com
divergência entre local e externo. Tx única garante "tudo ou nada" no lado
local. A próxima sincronização (ingestão periódica) é o backup contra
divergência rara.

---

## Fire-and-forget vs loop fechado

### Fire-and-forget (recomendado por simplicidade)

Executor **não publica evento de confirmação** de volta para o decider.
Ciclo fecha em `{ctx}_execution` + log. Decider naturalmente reavalia
no próximo trigger e emite nova decisão se necessário.

**Vantagens:**
- Sem loop de eventos entre dois agents.
- Decider não precisa consumer adicional.
- Estado vive em um lugar só (`{ctx}_execution`).

**Trade-off:**
- Decider não sabe **em real-time** se a aplicação foi efetiva — descobre no
  próximo ciclo de reavaliação.

### Loop fechado (quando o produto precisa de confirmação)

Executor publica `{ctx}.execution.applied` ou `.failed`. Decider consome
para atualizar estado do ciclo.

**Vantagens:**
- Decider tem confirmação rápida; pode decidir baseado nela.

**Trade-off:**
- Dois tópicos, mais infra, dois consumers.
- Estado distribuído entre tabelas e eventos.

**Recomendação:** começar com fire-and-forget. Migrar para loop fechado só
se aparecer demanda concreta (UI que precisa de "aplicado em real-time",
métrica que depende de feedback rápido).

---

## Sem DLQ separada nesta primeira rodada

Falhas finais (`FAILED`) ficam na própria tabela `{ctx}_execution`.
Reprocessamento manual (UI admin, worker dedicado) entra em **rodada
futura** quando virar dor real.

**Quando justifica DLQ:**
- Volume alto de falhas exigindo reprocessamento humano em batch.
- Necessidade de "reler eventos antigos" sem replay do tópico todo.
- Equipe de suporte/eng com fluxo formal de "investigar e reprocessar".

Até lá, métrica `{ctx}_execution_failed_total` por `last_error` cobre o
caso de uso.

---

## Como cada agent usa isso

### gofi-spec

- **§4 Arquitetura** — declarar quando o contexto usa o padrão executor:
  - Decider e executor são **dois workers separados** do mesmo bounded context.
  - Tópico Kafka entre eles (nome + partition key + payload).
  - Tabela `{ctx}_execution` com `decision_id` UNIQUE.
- **§3.4 Migrations** — incluir migration de `{ctx}_execution`.
- **§3.1 Operações** — operações do executor são workflow application
  (chamam bridge/factory + service + transação local), não service direto.
- **§4 Cache/Mensageria** — perfil de write da tabela de execução é
  `hot DELETE+INSERT` típico (decisões expirando + novas chegando).

### gofi-eng

- Implementar consumer Kafka com partition key respeitada.
- Tabela `{ctx}_execution` com `decision_id` UUID v7 UNIQUE; INSERT ON CONFLICT.
- Re-validação reusa o serviço de domínio (não duplica lógica).
- Bridge **estende** com métodos de escrita (`Apply*`, `Revert*`) — ver
  `.claude/sdk/<lang>/knowledge/bridge-factory-adapter-pattern.md` §"Bridge
  com operações de escrita".
- Transação local atomic para status + materialização.
- Status terminais `APPLIED`/`FAILED`/`STALE` + estado de trabalho `PENDING`.

### gofi-qa

- Auditar `INSERT ON CONFLICT` antes de qualquer chamada externa
  (idempotência por banco, não por cache).
- Auditar que `STALE` é terminal — executor não retenta nem emite evento.
- Auditar que `decision_id` é UUID v7 (não v4) — ver
  `.claude/knowledge/shared/id-types.md`.
- Auditar que partition key do producer = partition key esperada pelo
  consumer (ordering).
- Auditar classificação transient vs permanent no retry.
- Auditar log estruturado por tentativa (não só por desfecho).

---

## Bridge do decider vs bridge do executor — **DUAS bridges, NÃO uma**

Quando o contexto adota o split decider/executor, o adapter por marketplace
(ou por integração externa) tipicamente implementa **duas bridges
separadas**, com naturezas opostas:

| Bridge | Quem usa | Faz I/O externo? | Sinal de método |
|---|---|---|---|
| **`DecisionBridge`** (ou similar) | Decider | **NÃO** — operações puras sobre estado local | `MapToCategory(extType, extSubType) (Category, error)`, `ApplyMarketplaceRestrictions(ad, pool) []Eligible` — **sem `ctx`** |
| **`ExecutionBridge`** (ou similar) | Executor | **SIM** — chamadas HTTP/gRPC ao sistema externo | `AdhereToCampaign(ctx, token, target)`, `ExitFromCampaign(ctx, token, target)` — **com `ctx`** |

**Por que separar e não uma só:**

- **Decisão é determinística sobre snapshot.** Se a bridge da decisão fizer
  I/O ao externo, quebra o princípio do snapshot consistente — o decider
  deixaria de ser decider e viraria um híbrido frágil.
- **Latência.** Decider tipicamente processa N entidades por ciclo (100k+/h
  em alguns casos). Round-trip HTTP por entidade é proibitivo.
- **Falha de rede não pode derrubar decisão.** Decider precisa decidir com
  o que tem; falha de rede só pode acontecer no executor (que retenta).
- **Tipagem reflete o contrato.** Bridge da decisão **sem `ctx` e sem
  retorno de `error` por I/O** é a forma mais explícita de garantir que o
  adapter não tente fazer chamada externa ali.

`gofi-spec` declara as duas bridges no contrato §0.1 com cláusula explícita
"DecisionBridge é puro, sem I/O". `gofi-eng` implementa respeitando os tipos
(sem `ctx`, sem `error` de rede). `gofi-qa` audita: se um `decision_bridge.go`
de adapter importa `net/http` ou faz chamada externa, é violação do padrão.

### Sinal de "bridge inflada"

Se você se pegar adicionando `ctx context.Context` ou retornando `error` por
falha de rede em métodos da `DecisionBridge`, **pare**: é sinal de que esse
método pertence à `ExecutionBridge`. Move pra lá; decider vive com a
informação que tem no snapshot.

---

## Processor (scheduler-driven) mora no DOMÍNIO, não no binário cron

Quando o decider é acionado por cron periódico (em vez de consumer Kafka
reativo), o `Processor` que itera entidades + chama o use case + emite
evento Kafka **mora no domínio**, não no binário do scheduler.

Layout canônico (Go):

```
services/domain/{ctx}/scheduler/
  model/        — DTOs específicos do scheduler (ex.: EligibleEntity)
  processor/    — {ctx}_processor.go implementa scheduler.Processor
  repository/   — {ctx}_pending_repository.go (lista pendências por marketplace/tenant)
```

```
services/{cron-binary}/
  main.go       — só build do gofi.New
  wiring.go     — IMPORTA scheduler.NewRunner + Processor do domínio,
                  monta runner por marketplace/dimensão
```

**Princípio:** o binário cron do projeto é **só composition root**. Toda
lógica de domínio (qual query lista pendências, o que fazer com cada
entidade, como montar payload do evento) **mora no domínio**. O binário
só wira.

**Por que:** o mesmo Processor pode rodar em (i) binário cron, (ii) handler
HTTP de "force re-evaluate" (futuro), (iii) consumer Kafka reativo (fase
2). Se a lógica vive no binário cron, dois desses casos viram cópia de
código.

`gofi-spec` lista o `scheduler/{processor,repository,model}/` em §8 do
contexto; `gofi-eng` implementa lá; `gofi-qa` aponta como violação se
encontrar `*_processor.go` no binário cron.

---

## Anti-padrões

- **Sem `decision_id` único** — Kafka reentrega, executor aplica 2x. Sempre
  UUID v7 + UNIQUE.
- **`DecisionBridge` com `ctx` ou chamada HTTP** — vira "executor disfarçado",
  quebra snapshot consistente e introduz latência por entidade. Se precisa
  de I/O, é `ExecutionBridge`.
- **`*_processor.go` no binário cron** — viola separation domain/binário.
  Processor mora em `services/domain/{ctx}/scheduler/processor/`.
- **Re-validação ausente** — executor aplica decisão velha (campanha
  expirou entre decider e ele). Result: chamada externa inútil, ruído no
  log, ou pior: ação aplicada em estado que o produto considera inválido.
- **Idempotência só em cache** — perde garantia em failover. UNIQUE no
  banco é a fonte da verdade.
- **Executor emitindo evento de volta sem necessidade** — overhead de loop
  fechado quando fire-and-forget atende. Migrar só com demanda.
- **Executor decidindo** — se executor faz lógica de "qual ação aplicar",
  é decider escondido. Decider produz decisão completa; executor só executa.
- **Materialização fora da tx do status** — se INSERT na junction falha
  depois do UPDATE de APPLIED, fica divergência. Sempre tx única.
- **Decisões muito antigas processadas sem cutoff** — após incidente,
  consumer pode encontrar burst de eventos com horas de idade. Sem cutoff
  (default 24h), executor chama externo para decisões que o produto não
  quer mais.
- **Sem classificação de erro** — todo erro tratado como retry → bate
  permanent (400) e fica tentando até max retries; ou todo erro como
  permanent → perde recuperação de transient (timeout) que recuperaria
  naturalmente.

---

## Referência cruzada

- `.claude/sdk/<lang>/knowledge/bridge-factory-adapter-pattern.md` §"Bridge
  com operações de escrita" — como a bridge cresce de read para read+write.
- `.claude/knowledge/shared/application-vs-domain-service.md` — decider e
  executor são ambos `application/` workflows; bridge é consumida em ambos.
- `.claude/knowledge/shared/id-types.md` — UUID v7 para `decision_id`.
- `.claude/sdk/<lang>/knowledge/postgres-index-strategy.md` — perfil de
  write da tabela `{ctx}_execution` (hot DELETE+INSERT ou append-only com
  expurgo, conforme política).
