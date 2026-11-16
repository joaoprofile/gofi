---
pack: architecture-patterns
title: Padrões de arquitetura
summary: outbox transacional, event sourcing e CQRS
applies_when:
  symbols: ["*Outbox*", "*EventStore*", "*Projector*", "*ReadModel*", "*Upcaster*"]
  entities: [outbox, event sourcing, event-sourcing, event store, cqrs, read model, modelo de leitura, projeção de leitura, dual write]
  paths: ["**/*outbox*", "**/*Outbox*", "**/*outbox*/**", "**/*eventstore*/**", "**/*event_store*/**", "**/*readmodel*/**", "**/*read_model*/**"]
serves: [spec, eng, qa]
---

# Padrões de arquitetura

Três padrões independentes de linguagem e de broker. Cada módulo tem as mesmas seções: quando usar, contrato numerado, o que cada camada implementa, como cada papel usa, sensores e armadilhas — abra só a que a tarefa pede.

| Seção | Quando |
|---|---|
| `outbox.md` | mudança de estado que publica evento que não pode se perder |
| `event-sourcing.md` | o histórico de eventos é a fonte da verdade: auditoria, consulta temporal |
| `cqrs.md` | leitura com forma, carga ou escala diferente da escrita |

## Qual padrão

| Problema | Padrão | Custa | Não adote quando |
|---|---|---|---|
| Gravar estado e publicar evento, sem perder evento nem publicar o que não foi gravado | Outbox | tabela, relay, consumidor idempotente, publicação com atraso | o efeito cabe na mesma transação; perder evento é aceitável |
| Provar como o estado chegou ali; estado num instante passado; leituras novas sobre fatos antigos | Event sourcing | projeções, versão de evento, leitura eventual, curva da equipe | basta auditoria de quem mudou o quê; CRUD sem regra de transição |
| Consultas que o modelo de escrita não atende em forma ou carga | CQRS | dois modelos, projetor, atraso de leitura a declarar | índice, view ou cache resolvem; a leitura é o próprio agregado |

Adote por agregado ou contexto, nunca "no sistema", e o mais barato que resolve.

## Como combinam

- **Outbox sozinho** — estado em tabelas, eventos para fora. O caso comum.
- **CQRS sem event sourcing** — escrita em tabelas; o modelo de leitura é atualizado na transação do comando ou pelos eventos do outbox.
- **Event sourcing ⇒ CQRS** — o stream não se consulta por campo: toda listagem é projeção, logo há lado de leitura.
- **Event sourcing + outbox** — o que sai para outros serviços é publicado por relay sob o contrato do outbox; o stream faz o papel da tabela.

## Como chega a cada papel

O `gofi-spec` decide, e o **design da mudança nomeia o padrão e cita o caminho do módulo** (ex.: `.claude/expertise/architecture-patterns/outbox.md`) em §Abordagem, com os parâmetros em §Decisões; os cenários de §Sensores do módulo entram nos deltas. Quem planeja, implementa, testa e audita segue a citação do design: a sua linha em §Como cada papel usa, as tarefas por §O que cada camada implementa, o código e os achados pelo contrato numerado (`outbox O1`, `event-sourcing E3`, `cqrs C5`).
