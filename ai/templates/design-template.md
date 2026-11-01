---
tipo: design
formato: 3                        # versão do formato do corpus — não invente outro valor
contexto: {contexto}
versao: "1.0"
status: proposto                  # aprovado com a mudança, por `gofi change approve`
keywords: [{até 12 termos}]
atualizado: {YYYY-MM-DD}
---
# Design — {mudanca}

> **Template neutro**, para uma linguagem sem pack. Quando o pack da linguagem
> tem o seu (`.claude/sdk/<lang>/templates/design-template.md`), `gofi change
> new` usa o dele: ele traz as camadas, os tipos e o catálogo de erros do
> stack, com as mesmas seções.
>
> O design diz **como esta mudança** realiza os seus requisitos — não o
> sistema inteiro, que já está no código e no grafo. Cada decisão responde a
> um REQ dos deltas, citado pelo ID (`ctx/REQ-NN` quando o mesmo número existe
> em mais de um contexto da mudança).

## Abordagem

{Como a mudança realiza os requisitos, em um parágrafo, e o que muda no grafo: os símbolos e contextos que `gofi impact` mostra.}

## Modelo de dados

{Tabelas, colunas e migração; o perfil de acesso e os índices que ele pede. Sem mudança de dados, diga isso.}

## Contratos

| Operação | Rota ou evento | Auth | Requisitos | Respostas |
|---|---|---|---|---|
| {operação} | {rota} | {auth} | {REQ-NN} | {códigos} |

## Erros

| Erro | Situação | Requisito |
|---|---|---|
| {erro} | {quando ocorre} | {REQ-NN} |

## Mapa de requisitos

> Todo requisito ADICIONADO ou MODIFICADO pela mudança, e onde ele vai morar.
> É daqui que o `gofi-plan` deriva as tarefas e o rastreio liga cenário a teste.

| Requisito | Componente |
|---|---|
| {REQ-NN} | {componente} |

## Decisões

### ADR-01 — {Título da decisão}

**Contexto:** {o que levou à decisão} · **Decisão:** {o que foi decidido} · **Consequências:** {o que se ganha e o que se paga}
