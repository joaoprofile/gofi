---
tipo: design
formato: 3                        # versão do formato do corpus — não invente outro valor
contexto: {contexto}
versao: "1.0"
status: proposto                  # aprovado com a mudança, por `gofi change approve`
entidades: [{tabelas que esta mudança cria ou altera}]
keywords: [{até 12 termos}]
atualizado: {YYYY-MM-DD}
---
# Design — {mudanca}

> **Como esta mudança realiza os seus requisitos em Go, sobre o gofi-sdk-go.**
> Não o sistema inteiro — esse está no código e no grafo —, só o que muda.
> Cada decisão responde a um REQ dos deltas, citado pelo ID (`ctx/REQ-NN`
> quando o mesmo número existe em mais de um contexto da mudança). Regras e
> armadilhas das camadas: `.claude/sdk/go/knowledge/`; esqueletos:
> `.claude/sdk/go/boilerplates/`.

## Abordagem

{Como a mudança realiza os requisitos, em um parágrafo, e o que muda no grafo: os pacotes e símbolos que `gofi impact` mostra.}

| Decisão | Valor |
|---|---|
| **Serviço** | {nome, como na tabela Serviços de project.md} · prefixo `{/api/v1}` |
| **Camadas tocadas** | {model · repository · service · application · handler · adapter} |
| **Cache** | {Redis TTL=Xs em {operação} \| não} |
| **Mensageria** | {publica `{evento}` em `{tópico}` \| consome `{tópico}` → {ação} \| não} |
| **Filtro dinâmico** | {sim — `{Ctx}FilterMapping` \| não} |

**Contratos de camada** (só o que nasce ou muda):

```go
type {Contexto}Service interface {
    Create(ctx context.Context, req model.Create{Entidade}Request) (*model.{Entidade}, errs.AppError) // {REQ-NN}
}
```

## Modelo de dados

| Campo | Tipo Go | Coluna SQL | Obrigatório | Requisito |
|---|---|---|---|---|
| {Campo} | {tipo} | {coluna} | {sim \| não} | {REQ-NN} |

```sql
-- {AAAA}_{descrição}.up.sql — e o .down.sql que a desfaz
ALTER TABLE {tabela} ADD COLUMN {coluna} {tipo};
```

| Consulta | Frequência | Índice |
|---|---|---|
| {por quem e por quê se busca} | {alta \| média \| baixa} | {índice, ou nenhum e por quê} |

## Contratos

| Operação | Método e rota | Auth | Requisitos | Respostas |
|---|---|---|---|---|
| {Operação} | `{MÉTODO} {prefixo}/{recurso}` | {Bearer \| não} | {REQ-NN} | {201 · 400 · 409} |

## Erros

| Var | Kind | Code | Situação | Requisito |
|---|---|---|---|---|
| `Err{C}{Situação}` | {NotFound \| Conflict \| Validation \| Operation} | `{C}_{CODE}` | {quando ocorre} | {REQ-NN} |

## Mapa de requisitos

> Todo requisito ADICIONADO ou MODIFICADO pela mudança, e o símbolo Go que o
> realiza — onde vai o `// gofi:implements REQ-NN`. É daqui que o `gofi-plan`
> deriva as tarefas e o rastreio liga cenário a teste.

| Requisito | Componente |
|---|---|
| {REQ-NN} | `{pacote}.{Tipo}.{Método}` |

## Decisões

### ADR-01 — {Título da decisão}

**Contexto:** {o que levou à decisão} · **Decisão:** {o que foi decidido} · **Consequências:** {o que se ganha e o que se paga}
