# CQRS

Leitura com forma, carga ou escala diferente da escrita.

Comandos mudam estado sobre o modelo de escrita; consultas leem modelos de leitura derivados dele. Não exige event sourcing nem banco separado.

## Quando usar

| Use quando | Não use quando → alternativa |
|---|---|
| A consulta pede forma que o modelo de escrita não tem: junção de agregados, totais, busca | A leitura é o próprio agregado → repositório e DTO |
| Carga ou escala de leitura muito acima da de escrita | Poucas consultas lentas → índice, view ou cache |
| A escrita é event sourced: não há tabela de estado para consultar | Nenhum atraso de leitura é aceitável → nível 1, ou nível 2 na transação do comando |
| Necessidade de tela está deformando as regras de escrita | |

Níveis — adote o mais baixo que resolve:

| Nível | O que separa | Consistência |
|---|---|---|
| 1 | Só o código: comandos passam pelo agregado; consultas leem as mesmas tabelas direto para DTO | imediata |
| 2 | Tabela de leitura no mesmo banco, atualizada na transação do comando ou por projetor | imediata ou eventual |
| 3 | Armazenamento de leitura próprio, alimentado por eventos | eventual |

## O contrato do padrão

Cite como `cqrs C<n>`.

- **C1 — Comando muda estado e devolve só confirmação, id e versão.** Nunca dado de domínio lido.
- **C2 — Consulta não muda estado.** Sem efeito observável; repetível e cacheável.
- **C3 — Um modelo de escrita por agregado, N de leitura.** Invariantes só na escrita; cada modelo de leitura serve a uma consulta e é desnormalizado para ela.
- **C4 — Leitura é derivada.** Só a projeção escreve nela — o projetor, ou a transação do comando no nível 2 —, e nenhum comando decide sobre leitura eventual, que é dado atrasado, sem decisão registrada no design.
- **C5 — Consistência declarada.** Para cada consulta: imediata, ou eventual com atraso máximo; e como o cliente lê o que acabou de escrever (tabela abaixo).
- **C6 — Projeção idempotente, ordenada e reconstruível.** Como `event-sourcing E8`; sem event sourcing, os eventos vêm do outbox (`outbox O3`–`O5`).
- **C7 — Lados isolados.** Consulta não alcança agregado nem repositório de escrita; comando não alcança DTO de leitura. A ponte são os eventos — no nível 1, as tabelas.

| Ler o que escreveu (C5) | Como |
|---|---|
| Versão mínima | O comando devolve a versão; a consulta a recebe e espera, ou responde "ainda não", até a projeção alcançá-la |
| Retorno do comando | O cliente atualiza a tela com a confirmação; a consulta vem depois |
| Projeção na transação | Aquele modelo de leitura é gravado junto com o comando (nível 2) |
| Estado pendente | A interface mostra "em processamento" até a leitura refletir |

## O que cada camada implementa

| Camada (`layer` do plano) | O que existe |
|---|---|
| Dados (`data`) | Uma tabela de leitura por consulta, desnormalizada, com a `version` ou `position` de origem e índices pelo perfil da consulta; `checkpoint (projection, position)`; repositório de leitura separado do de escrita. Reconstruir = esvaziar e reprocessar |
| Domínio (`domain`) | Agregado e invariantes, só no caminho dos comandos; não conhece DTO de leitura |
| Aplicação (`domain`) | Um caso de uso por comando (carrega, decide, grava, emite evento) e um por consulta (lê o modelo de leitura e devolve DTO, sem regra de negócio) |
| API / transporte (`api`) | Rotas de comando devolvem confirmação, id e versão — assíncrono responde "aceito" (ex.: HTTP 202), não "feito"; rotas de consulta só leem e, quando eventuais, expõem a versão ou a hora da última atualização |
| Integração / mensageria (`integration`) | Projetor: consome os eventos, grava por upsert guardado pela versão, avança o checkpoint na mesma transação; reconstrução; métrica de atraso. No nível 1 não existe |

## Como cada papel usa

| Papel | Faz |
|---|---|
| `gofi-spec` | Design: §Abordagem nomeia **CQRS** e o nível, e cita `.claude/expertise/architecture-patterns/cqrs.md`; §Decisões fixa, por consulta, o modelo de leitura, a consistência com o atraso máximo, como se lê o que escreveu e a reconstrução; §Contratos separa rotas de comando e de consulta. Deltas: os cenários de §Sensores, com o atraso como requisito mensurável |
| `gofi-plan` | `data` (tabelas de leitura, checkpoint) → `domain` (casos de uso de comando e de consulta) → `integration` (projetor, reconstrução) → `api` |
| `gofi-eng-data` | Tabela de leitura por consulta com índice pelo perfil; repositório de leitura próprio |
| `gofi-eng` | Comando sem retorno de domínio (C1); consulta sem escrita nem regra (C2); nenhum comando lê modelo de leitura (C4) |
| `gofi-eng-api` | Rotas separadas; "aceito" quando assíncrono; versão na resposta do comando e na da consulta eventual |
| `gofi-integrate` | Projetor conforme C6, com métrica de atraso |
| `gofi-test` | Um teste por cenário de §Sensores |
| `gofi-qa` | Confere C1–C7; `rule` do achado: `cqrs C<n>`. **Blocker:** consulta que escreve (C2), modelo de leitura gravado fora da projeção, ou comando decidindo sobre leitura eventual sem decisão no design (C4), projeção não idempotente (C6). **Major:** comando que devolve dado lido (C1), consistência não declarada ou não cumprida (C5), projeção sem reconstrução (C6), consulta que alcança o modelo de escrita (C7) |

## Sensores

**`test` e `trace`** — cenários que o delta traz (forma e cobrança como em `outbox.md` §Sensores):

| Cenário | Forma |
|---|---|
| Comando | DADO `<agregado>` · QUANDO `<comando>` é aceito · ENTÃO a resposta traz confirmação, id e versão |
| Consulta sem efeito | DADO um estado · QUANDO a consulta se repete · ENTÃO o resultado é o mesmo E nada muda |
| Atraso | DADO `<comando>` aceito · QUANDO passa o atraso máximo · ENTÃO a consulta o reflete |
| Ler o que escreveu | DADO `<comando>` aceito na versão V · QUANDO a consulta pede no mínimo V · ENTÃO traz a mudança, ou a resposta declarada de "ainda não" |
| Evento repetido | DADO um evento já projetado · QUANDO chega de novo · ENTÃO o modelo de leitura não muda |
| Fora de ordem | DADO um evento mais antigo que o já projetado · QUANDO chega · ENTÃO não sobrescreve o mais novo |
| Reconstrução | DADO o modelo de leitura esvaziado · QUANDO é reconstruído · ENTÃO fica igual ao anterior |

**`graph`** — sintaxe e limites em `outbox.md` §Sensores. Os packs não têm camada de leitura, então C7 não vira regra de camada: confira com `gofi path <consulta> <agregado>`, que deve voltar vazio com o grafo em `deep`. Lado de leitura em contexto próprio: `"deny: new-cross-context-edge"` recusa a aresta nova para o contexto de escrita em toda mudança que não o declare.

## Armadilhas

- **"Só desta vez" no modelo de escrita** — a consulta que lê o agregado ou a tabela de escrita vira a regra e prende o esquema de escrita às telas.
- **Surpresa ao ler o que escreveu** — a tela consulta logo depois do comando e mostra o estado antigo: C5 não foi decidido.
- **Comando que devolve a entidade** — obriga a ler o modelo e acopla os lados.
- **Comando decidindo sobre leitura atrasada** — unicidade ou saldo conferidos numa projeção deixam passar o duplicado.
- **Projeção por incremento** (`total = total + x`) — dobra na reentrega; grave valor absoluto, guardado pela versão.
- **Upsert sem comparar versão** — evento atrasado sobrescreve o mais novo.
- **Checkpoint fora da transação** — a queda entre os dois perde ou repete eventos.
- **Um modelo de leitura para todas as telas** — volta a ser o modelo de escrita, com junções.
- **Regra de negócio na consulta** — a invariante calculada em dois lugares diverge.
- **Projeção sem reconstrução** — um defeito no projetor vira dado errado para sempre.
- **Banco de leitura separado sem necessidade** — nível 3 onde o 1 bastava.
- **Atraso sem métrica** — ninguém percebe que a leitura parou.
