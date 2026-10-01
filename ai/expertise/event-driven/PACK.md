---
pack: event-driven
title: Arquitetura orientada a eventos
summary: decider/executor, idempotência, retry transitório vs permanente, naming de types do envelope
applies_when:
  imports: [github.com/gofi-labs/gofi-sdk-go/msq]
  symbols: ["*Consumer", "*Producer", "*Executor", "*Decider"]
  intents: [consumir, publicar, integrar, agendar, sincronizar]
  entities: [evento, mensagem, tópico, fila]
serves: [spec, eng, qa]
---

# Arquitetura orientada a eventos

Padrões de eventos independentes de broker. O uso do SDK de mensageria fica em `sdk/<lang>/knowledge/` (`messaging-msq`, `consumer-bootstrap`, `kafka-consumer-naming`).

| Seção | Quando |
|---|---|
| `executor-pattern.md` | split decider/executor, contrato do evento, idempotência, retry |
| `kafka-type-naming.md` | nome do type no envelope: substantivo (inbound) vs gerúndio (outbound) |
