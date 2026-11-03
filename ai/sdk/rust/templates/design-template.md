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

> **Como esta mudança realiza os seus requisitos num backend Axum + sqlx.**
> Não o sistema inteiro — esse está no código e no grafo —, só o que muda.
> Cada decisão responde a um REQ dos deltas, citado pelo ID (`ctx/REQ-NN`
> quando o mesmo número existe em mais de um contexto da mudança). Regras e
> armadilhas das camadas: `.claude/sdk/rust/knowledge/`; esqueletos:
> `.claude/sdk/rust/boilerplates/`.

## Abordagem

{Como a mudança realiza os requisitos, em um parágrafo, e o que muda no grafo: os módulos e símbolos que `gofi impact` mostra.}

| Decisão | Valor |
|---|---|
| **Módulo** | `src/{contexto}/` · prefixo `{/api/v1}` |
| **Camadas tocadas** | {model/dto · repository · service · handler · adapters} |
| **Cache** | {TTL=Xs em {operação} \| não} |
| **Mensageria** | {publica `{evento}` \| consome `{tópico}` → {ação} \| não} |

**Contratos de camada** (só o que nasce ou muda):

```rust
impl {Contexto}Service {
    pub async fn create(&self, input: Create{Entidade}) -> Result<{Entidade}, {Contexto}Error>; // {REQ-NN}
}
```

## Modelo de dados

| Campo | Tipo Rust | Coluna SQL | Obrigatório | Requisito |
|---|---|---|---|---|
| {campo} | {i64 \| String \| …} | {coluna} | {sim \| não} | {REQ-NN} |

```sql
-- {AAAA}_{descrição} — up, e o down que a desfaz
ALTER TABLE {tabela} ADD COLUMN {coluna} {tipo};
```

| Consulta | Frequência | Índice |
|---|---|---|
| {por quem e por quê se busca} | {alta \| média \| baixa} | {índice, ou nenhum e por quê} |

## Contratos

| Operação | Método e rota | Auth | Requisitos | Respostas |
|---|---|---|---|---|
| {Operação} | `{MÉTODO} {prefixo}/{recurso}` | {extractor de auth \| não} | {REQ-NN} | {201 · 400 · 409} |

## Erros

| Variante | HTTP | Situação | Requisito |
|---|---|---|---|
| `{Contexto}Error::{Situação}` | {404 \| 409 \| 400} | {quando ocorre} | {REQ-NN} |

## Mapa de requisitos

> Todo requisito ADICIONADO ou MODIFICADO pela mudança, e a função que o
> realiza — onde vai o `// gofi:implements REQ-NN`. É daqui que o `gofi-plan`
> deriva as tarefas e o rastreio liga cenário a teste.

| Requisito | Componente |
|---|---|
| {REQ-NN} | `{Tipo}::{função}` em `src/{contexto}/{arquivo}.rs` |

## Decisões

### ADR-01 — {Título da decisão}

**Contexto:** {o que levou à decisão} · **Decisão:** {o que foi decidido} · **Consequências:** {o que se ganha e o que se paga}
