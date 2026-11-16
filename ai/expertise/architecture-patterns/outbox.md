# Outbox transacional

Mudança de estado que publica evento que não pode se perder.

O evento vira linha na mesma transação do estado e um relay o publica depois: troca o *dual write* por entrega ao menos uma vez, em ordem por chave.

## Quando usar

| Use quando | Não use quando → alternativa |
|---|---|
| Estado gravado gera evento para broker ou outro serviço, e os dois não compartilham transação | O efeito é no mesmo banco → mesma transação, sem evento |
| Evento perdido, ou evento de estado desfeito por rollback, é defeito de negócio | Perder evento é aceitável (métrica, invalidação de cache) → publicar depois do commit |
| O consumidor depende da ordem dos fatos de um mesmo agregado | O consumidor pode buscar o estado → consulta ou reconciliação periódica |
| | O estado já é um stream de eventos → `event-sourcing.md`: o stream faz o papel do outbox |

## O contrato do padrão

Cite como `outbox O<n>`.

- **O1 — Mesma transação.** A linha do evento entra na transação que grava o estado; rollback desfaz as duas. Nada é publicado no caminho da requisição.
- **O2 — Só o relay publica.** Lê linhas comitadas e pendentes, publica e marca `published_at` **depois** da confirmação do broker.
- **O3 — Ao menos uma vez.** Queda entre publicar e marcar republica: duplicata é esperada, perda nunca. O `id` do evento viaja na mensagem e é o mesmo na reentrega.
- **O4 — Consumidor idempotente.** Deduplica pelo `id` do evento, com `UNIQUE` na transação do efeito → `.claude/expertise/event-driven/executor-pattern.md` §Idempotência via UNIQUE em tabela.
- **O5 — Ordem por chave.** Eventos do mesmo `aggregate_id` saem na ordem de `seq`, com ele como chave de partição; o seguinte só sai depois da confirmação do anterior. Entre chaves não há ordem.
- **O6 — Fato autossuficiente.** Nome no passado, `type` + `schema_version`, valores do momento da gravação — nunca referência a linha que muda. Mudança incompatível é versão nova, publicada ao lado da antiga até os consumidores migrarem.
- **O7 — Retenção declarada.** Linha publicada é expurgada depois da janela; pendente, nunca.
- **O8 — Veneno isolado.** Esgotadas N tentativas com backoff, a linha fica marcada como falha, alerta e segura só a própria chave; as outras seguem. Reprocessar é operação declarada.

## O que cada camada implementa

| Camada (`layer` do plano) | O que existe |
|---|---|
| Dados (`data`) | Tabela `outbox` em migração par; repositório com `append(evento)` **na transação que grava o estado** — a do chamador, ou a do método que grava o agregado, pela convenção do SDK —, `pending(lote)` e `mark_published(id)`; índice só das pendentes, por `seq` |
| Domínio (`domain`) | O agregado diz qual evento nasce de cada mudança: tipo e payload. Não conhece tabela, relay nem broker |
| Aplicação (`domain`) | O caso de uso grava estado e evento na mesma transação e só então responde. Nunca chama publicador |
| API / transporte (`api`) | Nada do padrão: responde depois do commit, sem esperar a publicação |
| Integração / mensageria (`integration`) | **Relay** como worker, por polling das pendentes ou por leitura do log de transações (ex.: CDC): publica com `aggregate_id` como chave e `id` no cabeçalho; backoff, limite de tentativas, expurgo, métrica de pendentes e da idade da mais antiga. Com N instâncias, cada lote é reivindicado com bloqueio — duas nunca publicam a mesma chave. Consumidor conforme O4 |

| Coluna de `outbox` | Para quê |
|---|---|
| `id` | identidade do evento, única; chave de dedup no consumidor |
| `seq` | ordem de gravação, monotônica |
| `aggregate_type`, `aggregate_id` | de quem é o fato; ordenação e partição |
| `type`, `schema_version` | nome do evento e versão do esquema |
| `payload` | o fato serializado, com a tenancy do contexto |
| `occurred_at` | quando ocorreu |
| `published_at` | nulo = pendente |
| `attempt_count`, `last_error` | veneno (O8) |

## Como cada papel usa

| Papel | Faz |
|---|---|
| `gofi-spec` | Design: §Abordagem nomeia **outbox** e cita `.claude/expertise/architecture-patterns/outbox.md`; §Decisões fixa eventos e tópico, chave de ordenação, garantia (ao menos uma vez), atraso máximo de publicação, retenção, tentativas e o destino do veneno; §Modelo de dados traz a tabela. Deltas: os cenários de §Sensores |
| `gofi-plan` | `data` (tabela, repositório) → `domain` (evento, gravação na transação) → `integration` (relay, expurgo, consumidor); `api` só se o contrato muda |
| `gofi-eng-data` | Tabela, índice de pendentes e `append` que entra na transação do estado — nunca abre outra |
| `gofi-eng` | Estado e evento numa transação (O1); `id` e payload completos (O6); nenhum import de broker |
| `gofi-eng-api` | Responde após o commit; não promete entrega |
| `gofi-integrate` | Relay (O2, O3, O5, O7, O8) e consumidor idempotente (O4) |
| `gofi-test` | Um teste por cenário de §Sensores; a falha é injetada por mock do repositório e do publicador |
| `gofi-qa` | Confere O1–O8 no código; `rule` do achado: `outbox O<n>`. **Blocker:** publicação fora do relay ou antes do commit (O1, O2), marcar antes da confirmação (O2), consumidor sem dedup (O4). **Major:** ordem por chave não garantida (O5), payload por referência ou sem versão (O6), sem expurgo (O7), sem limite de tentativas (O8) |

## Sensores

**`test` e `trace`** — o delta traz estes cenários, nos termos do domínio e observáveis de fora; cada um vira teste e o `trace` cobra o par cenário → teste.

| Cenário | Forma |
|---|---|
| Atomicidade | DADO uma falha ao gravar `<agregado>` · QUANDO a operação termina em erro · ENTÃO o estado não mudou E nenhum `<Evento>` é entregue |
| Broker fora | DADO o broker indisponível · QUANDO `<agregado>` muda · ENTÃO a operação conclui E `<Evento>` é entregue quando o broker volta |
| Entrega repetida | DADO `<Evento>` já processado · QUANDO chega de novo · ENTÃO o efeito não se repete |
| Ordem | DADO dois eventos do mesmo `<agregado>` · QUANDO são publicados · ENTÃO chegam na ordem em que ocorreram |
| Veneno | DADO um evento que sempre falha ao publicar · QUANDO esgota as tentativas · ENTÃO fica isolado e visível à operação E os de outros agregados seguem |

**`graph`** — regra de projeto no `.gofi.yaml`. Os nomes de camada são os do pack da linguagem (`service`, `repository`, `model`, `adapter`…); regra com outro nome é erro.

```yaml
graph:
  rules:
    - "deny: import \"<pacote do cliente do broker>\" from service"   # o serviço não publica
    - "deny: service -> adapter"                                    # nem chama o publicador
```

A segunda vale para o projeto inteiro: só quando nenhum serviço chama adapter direto. As duas pegam o atalho; *dual write* atrás de uma interface, só o cenário de atomicidade e a auditoria.

## Armadilhas

- **Dual write** — gravar e publicar em sequência: a queda entre os dois perde o evento ou publica estado que não existe.
- **Publicar antes do commit**, mesmo "dentro" da transação — o rollback deixa o evento no broker.
- **Relay com cursor `seq > último`** — transação que comita atrasada fica abaixo do cursor e nunca sai. Selecione por pendente (`published_at` nulo).
- **Marcar publicado antes da confirmação** — a queda no meio perde o evento em silêncio.
- **Consumidor não idempotente** — a duplicata de O3 vira efeito repetido.
- **Outbox sem expurgo** — a tabela cresce, o índice degrada, o relay atrasa.
- **Evento com referência mutável** ("leia o registro X") — o consumidor vê o estado de depois, não o do fato.
- **Dois relays na mesma chave** — sem bloqueio de lote, reordenam.
- **Contar com ordem global** — só existe por chave.
- **Esperar a publicação para responder** — recria o acoplamento ao broker que o padrão tirou.
- **Segredo ou dado pessoal no payload** — fica na tabela e no broker pelo tempo da retenção.
