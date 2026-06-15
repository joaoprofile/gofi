# Naming de types canônicos do envelope Kafka — substantivo (inbound) vs gerúndio (outbound)

Aplica-se a qualquer projeto que use um envelope canônico unificado em
tópico Kafka (`marketplace.sync` ou equivalente) com campo discriminador
`type` (string) determinando o subtipo semântico de cada evento.

> **Princípio:** o nome do `type` carrega a **direção semântica** do evento.
> Substantivo factual = dado **inbound** (observação vinda de fora).
> Gerúndio/ação = processo **outbound** (comando emitido pelo nosso
> domínio para fora). Quando uma mesma "dimensão" tem ambos os pipelines,
> são **dois types distintos** no envelope — não 1 type discriminado por
> `source`.

---

## Regra

Ao declarar uma constante nova em `services/common/kafka/topics.go` (ou
arquivo equivalente) para o envelope canônico, escolha o nome conforme a
direção do evento:

| Direção | Significado | Forma do nome | Consumidor típico |
|---|---|---|---|
| **Inbound** | dado factual observado vindo de fora (sistema externo → nosso domínio) | **substantivo singular** (`Type{Snapshot}` onde `{Snapshot}` é o substantivo do dado observado) | adapter de sincronização (ex.: `services/adapter/{tech}/{ctx-in}/`) |
| **Outbound** | processo/comando emitido pelo nosso domínio para o sistema externo (nosso domínio → sistema externo) | **gerúndio / ação** (`Type{Acting}` onde `{Acting}` é o verbo no gerúndio) | adapter do contexto-dono (ex.: `services/adapter/{tech}/{ctx-out}/`) |

Exemplos genéricos:

```go
Type{Snapshot}  = "{snapshot}"   // inbound  — dado observado vindo do sistema externo
Type{Acting}    = "{acting}"     // outbound — ação executada pelo nosso domínio
```

---

## Quando uma "dimensão" tem ambos os pipelines

Quando o mesmo conceito do domínio (ex.: preço) tem dois fluxos com
direções opostas:

- **Outbound** — nosso domínio calcula e envia ao sistema externo
- **Inbound** — sistema externo nos avisa de uma mudança e atualizamos
  estado local

**Declarar dois types distintos no envelope**, um por direção:

```go
// services/common/kafka/topics.go
const (
    Type{Snapshot} = "{snapshot}"   // inbound  — dado observado do externo
    Type{Acting}   = "{acting}"     // outbound — ação executada pelo nosso domínio e enviada
)
```

Consumer groups distintos garantem que cada pipeline processe só o que
lhe cabe:

- `{tech}-{ctx-in}-sync-cg` consome `type={snapshot}` (inbound) → adapter
  `services/adapter/{tech}/{ctx-in}/` busca dado fresco do externo
  e atualiza estado local.
- `{tech}-{ctx-out}-sync-cg` consome `type={acting}` (outbound) → adapter
  `services/adapter/{tech}/{ctx-out}/` executa a ação e chama SDK do
  externo.

**Sem branching de dispatcher no consumer** — cada consumer é
single-purpose por construção.

---

## Anti-padrão MAJOR — 1 type com `source` discriminador

```go
// ❌ ERRADO
Type{Acting} = "{acting}" // inbound quando source=webhook; outbound quando source=scheduler

// Consumer precisa fazer:
if event.Source == "webhook" {
    // tratar como inbound
} else {
    // tratar como outbound
}
```

Por que é errado:

1. **Quebra o princípio "consumer filtra por `type`, não por `source`"** —
   a topologia Kafka assume que cada consumer group filtra por type
   (subscreve aos types relevantes). Discriminação por `source` força
   branching dentro do consumer e impossibilita ter consumer groups
   especializados por pipeline.

2. **Vaza ambiguidade para downstream** — o consumer de `type={acting}`
   recebe **tanto** comandos quanto observações; precisa entender que
   `source=webhook` significa coisa fundamentalmente diferente de
   `source=scheduler`. Acopla a semântica do type ao conjunto válido de
   sources, o que rompe encapsulamento.

3. **Impede escalar/desligar pipelines independentemente** — se você quer
   pausar o inbound (manter outbound funcionando), ou vice-versa, com 1
   type só não dá; com 2 types você pausa o consumer group correspondente.

4. **Confunde naming do contexto-dono** — `services/domain/{ctx-out}/` é o
   contexto outbound (executa a ação). Se `type={acting}` representa
   também o evento inbound, qual contexto é o dono? Vira ambiguidade
   arquitetural.

---

## Princípio generalizado

> *"Um type Kafka representa **um evento de uma direção** — não uma
> dimensão de domínio."*

Quando confundir, pergunte:

- "Esse evento é um **dado factual** que chegou de fora ou é um **comando**
  que o domínio emitiu?"
- "O consumer deveria **observar e refletir** o estado externo, ou
  **executar uma ação** definida pelo domínio?"

A resposta determina o nome (substantivo vs gerúndio) e o contexto-dono
do consumer.

---

## Aplicação por agent

- **Product Discovery (PD)**: quando o usuário falar de "atualizar X" ou
  "mudar X", sempre perguntar a direção do pipeline. Documentar no
  glossário a distinção quando ambos coexistem.
- **Spec Architect**: ao declarar evento novo no envelope, escolher o nome
  conforme a direção; declarar dois types quando ambos pipelines existem;
  rejeitar specs que descrevem 1 type discriminado por `source`.
- **Engineer**: ao adicionar constante em `kafka/topics.go`, incluir
  comentário inline `// inbound — ...` ou `// outbound — ...`; configurar
  consumer groups distintos quando ambos coexistem.
- **QA**: auditar que consumer code não tem branching por `source` para
  decidir semântica do evento; auditar que pares inbound/outbound têm
  consumer groups separados.

---

## Domínios onde isso aplica

A regra vale para qualquer "dimensão" de integração externa com pipelines
bidirecionais. Tabela com exemplos genéricos de pareamento:

| Dimensão (genérica) | Outbound (gerúndio — comando) | Inbound (substantivo — observação) |
|---|---|---|
| Valor monetário | `Type{Adjusting}` | `Type{Value}` |
| Quantidade física | `Type{Replenishing}` | `Type{Quantity}` |
| Recurso provisionado | `Type{Provisioning}` | `Type{Resource}` |
| Estado de entidade | `Type{Activating}` / `Type{Deactivating}` | `Type{State}` |

Quando só existe um lado (ex.: apenas outbound, sem feedback inbound),
declarar apenas um type — não inventar o par contrafactual.
