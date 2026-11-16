# Event sourcing

O histórico de eventos é a fonte da verdade: auditoria, consulta temporal.

Gravam-se os fatos, em ordem, por agregado; o estado atual é o resultado de aplicá-los. Toda leitura que não seja carregar um agregado é projeção (`cqrs.md`).

## Quando usar

| Use quando | Não use quando → alternativa |
|---|---|
| Provar como o estado chegou ali é requisito (disputa, conciliação, auditoria) | Basta saber quem mudou o quê → tabela de auditoria |
| Pede-se o estado num instante passado, ou leituras novas sobre fatos antigos | Estado atual e evento para fora → tabelas + `outbox.md` |
| O agregado tem transições ricas e fronteira clara | CRUD, cadastro, relatório: projeção, versão de evento e leitura eventual não se pagam |
| | A lei manda apagar o dado e não há estratégia de anonimização decidida |

Adote por agregado: um contexto pode ter um agregado event sourced e os demais em tabelas.

## O contrato do padrão

Cite como `event-sourcing E<n>`.

- **E1 — O stream é a verdade.** Um stream por agregado, só com inserção; nenhuma outra tabela é fonte do estado de escrita.
- **E2 — Evento é fato imutável.** Nome no passado, do domínio; nunca é alterado nem apagado. Erro se corrige com evento de compensação.
- **E3 — Concorrência otimista.** Todo append informa a versão esperada; `UNIQUE (stream_id, version)` recusa o segundo escritor, que recarrega e decide de novo ou devolve conflito.
- **E4 — Decidir não é aplicar.** `decidir(estado, comando) → eventos | erro` valida as invariantes; `aplicar(estado, evento) → estado` é pura, determinística, sem I/O nem relógio, e nunca recusa. Estado = `aplicar` em sequência sobre o stream.
- **E5 — Esquema versionado.** Evento leva `type` + `schema_version`; versão antiga passa por *upcaster* (função pura vN → vN+1) na leitura. O gravado não se migra no lugar.
- **E6 — Fato autossuficiente.** Como `outbox O6`, mais a causa nos metadados: id do comando (o mesmo comando não gera evento duas vezes), correlação, autor.
- **E7 — Snapshot é cache.** Guarda estado, `version` e versão do formato; é descartável e refeito do stream; formato incompatível é ignorado, não migrado.
- **E8 — Projeção derivada e reconstruível.** Lê por `position`, grava o checkpoint na transação da própria escrita, é idempotente e ordenada (`outbox O3`–`O5`) e pode ser apagada e refeita do zero.
- **E9 — Efeito externo fora do replay.** Notificar, chamar outro serviço e publicar acontecem uma vez, em reação ao evento novo (assinante com checkpoint ou outbox) — nunca ao reidratar nem ao reconstruir.

## O que cada camada implementa

| Camada (`layer` do plano) | O que existe |
|---|---|
| Dados (`data`) | Tabela de eventos só com `INSERT` — `UPDATE` e `DELETE` revogados quando o banco permite; store com `append(stream, versão_esperada, eventos)` atômico, `load(stream, desde_versão)` e `read_all(desde_posição, lote)`; snapshots; para cada projeção, tabela e checkpoint |
| Domínio (`domain`) | Agregado com `decidir` e `aplicar`, tipos de evento, upcasters. Não depende do store |
| Aplicação (`domain`) | Caso de uso do comando: carrega (snapshot + eventos seguintes), reidrata, `decidir`, `append` com a versão lida; em conflito recarrega e repete até o limite, depois devolve conflito |
| API / transporte (`api`) | Comando devolve id e versão; conflito de versão vira o erro de conflito; listar e filtrar, só por projeção |
| Integração / mensageria (`integration`) | Projetores e assinantes com checkpoint; publicação externa pelo contrato do outbox; reconstrução de projeção como operação executável |

| Coluna do stream | Para quê |
|---|---|
| `position` | ordem global, monotônica: cursor de projeções e assinantes |
| `stream_id` | tipo e id do agregado |
| `version` | sequência no stream, a partir de 1; `UNIQUE (stream_id, version)` |
| `type`, `schema_version` | nome do evento e versão do esquema |
| `payload` | o fato |
| `metadata` | id do comando, correlação, autor, tenancy |
| `occurred_at` | quando ocorreu |

Snapshot: `stream_id`, `version`, `state_version`, `state`. Checkpoint: `projection`, `position`.

## Como cada papel usa

| Papel | Faz |
|---|---|
| `gofi-spec` | Design: §Abordagem nomeia **event sourcing**, os agregados que o adotam, e cita `.claude/expertise/architecture-patterns/event-sourcing.md`; §Decisões fixa o catálogo de eventos (nome, campos, versão), a política de conflito e a de snapshot, as projeções com a consistência de cada uma (`cqrs C5`), a reconstrução (quem dispara, o que a leitura mostra enquanto roda), retenção e arquivamento do stream, e o tratamento de dado pessoal. Deltas: os cenários de §Sensores |
| `gofi-plan` | `data` (stream, snapshot, projeções e checkpoint) → `domain` (eventos, `decidir`/`aplicar`, upcasters, caso de uso) → `integration` (projetores, assinantes, reconstrução) → `api` |
| `gofi-eng-data` | Stream com a unicidade de E3, append atômico de N eventos, leitura por stream e por posição; nada que altere ou apague evento |
| `gofi-eng` | `decidir` e `aplicar` separados (E4); upcaster a cada versão nova (E5); repetição limitada no conflito (E3) |
| `gofi-eng-api` | Devolve id e versão; mapeia o conflito; nenhuma rota lista pelo stream |
| `gofi-integrate` | Projetor com checkpoint transacional e reconstrução (E8); efeito externo só em evento novo (E9) |
| `gofi-test` | Um teste por cenário de §Sensores; o agregado se testa como DADO eventos · QUANDO comando · ENTÃO eventos |
| `gofi-qa` | Confere E1–E9; `rule` do achado: `event-sourcing E<n>`. **Blocker:** evento alterado ou apagado (E2), append sem versão esperada (E3), I/O, relógio ou recusa em `aplicar` (E4), efeito externo no replay (E9). **Major:** evento sem versão, ou versão nova sem upcaster (E5), snapshot lido como verdade (E7), projeção sem checkpoint transacional ou sem reconstrução (E8) |

## Sensores

**`test` e `trace`** — cenários que o delta traz (forma e cobrança como em `outbox.md` §Sensores):

| Cenário | Forma |
|---|---|
| Reidratação | DADO o histórico de `<agregado>` · QUANDO é carregado · ENTÃO o estado é o do último fato, com e sem snapshot |
| Invariante | DADO um estado que proíbe `<comando>` · QUANDO ele chega · ENTÃO é recusado E nenhum evento é gravado |
| Conflito | DADO dois comandos sobre a mesma versão · QUANDO concorrem · ENTÃO um grava E o outro decide sobre o estado novo ou recebe conflito |
| Comando repetido | DADO `<comando>` já aceito · QUANDO chega com o mesmo id · ENTÃO nenhum evento novo é gravado |
| Compensação | DADO um fato registrado errado · QUANDO é corrigido · ENTÃO o histórico ganha o fato de correção E mantém o original |
| Evento antigo | DADO um evento na versão anterior do esquema · QUANDO o agregado é carregado · ENTÃO o estado é o esperado |
| Reconstrução | DADO a projeção apagada · QUANDO é reconstruída · ENTÃO fica igual à anterior E nenhum efeito externo se repete |
| Instante passado (se for requisito) | DADO o histórico · QUANDO se pede o estado em T · ENTÃO vale só o ocorrido até T |

**`graph`** — sintaxe e limites em `outbox.md` §Sensores. Os packs já negam `model -> repository`; para o que `decidir` e `aplicar` não podem tocar:

```yaml
graph:
  rules:
    - "deny: import \"<pacote de banco ou de rede>\" from model"   # a camada onde o agregado mora
```

## Armadilhas

- **Editar ou apagar evento** — invalida todo estado e projeção derivados; corrija com compensação.
- **Validar em `aplicar`** — regra nova faz o histórico antigo falhar ao reidratar.
- **I/O, relógio ou aleatório em `aplicar`** — o replay dá outro estado.
- **Efeito externo no replay** — reconstruir reenvia notificação e cobrança.
- **Evento como diff de CRUD** (`<Agregado>Atualizado` com o registro inteiro) — perde a intenção; nomeie o fato do domínio.
- **Projeção que não se reconstrói** — corrigida à mão, escrita por dois lugares ou dependente de dado fora do stream.
- **Cursor de `position` pulando transação atrasada** — a armadilha do cursor de `outbox.md`: avance só até onde não há transação aberta, ou atribua a posição em série.
- **Snapshot como verdade** — lido sem conferir o formato, ou usado sem os eventos seguintes.
- **Consultar o stream por campo** — listagem e filtro são de projeção.
- **Stream sem fim** — agregado de vida longa sem snapshot nem fechamento de período.
- **Dado pessoal no evento** — imutável não combina com apagamento: guarde referência a um cofre ou cifre com chave descartável por titular.
- **Event sourcing no sistema todo** — é decisão por agregado.
