---
name: document-versioning
description: "Política de versão de PRD e spec — a versão conta estados de produção, não edições. Lida por gofi-pd, gofi-spec, gofi-eng e gofi-qa antes de tocar o frontmatter de qualquer documento."
---

# Versão de documento conta estados de produção, não edições

Cross-agent, cross-language. Vale para **PRD** (`prd/`) e **spec** (`specs/`).

## A regra

> **A versão de um documento é o número de comportamentos distintos que aquele
> documento já colocou em produção.** Não é contador de edições, não é
> registro de esforço, não marca revisão.

Consequência direta: um documento reescrito quinze vezes antes de virar código
continua em **1.0**. Um documento intocado por um ano, cuja solução está em
produção e agora vai mudar, sobe para **1.1**.

## O ciclo

```
nasce 1.0 ──── editado, refinado, reescrito, invertido ──── continua 1.0
   │
   └── vai para produção ──── continua 1.0
                                │
                                └── passa a descrever comportamento
                                    diferente, que irá para produção ──── 1.1
```

**1.0 é a baseline e é onde a maioria dos documentos vive.** Só sai de 1.0 o
documento cuja solução já rodou em produção e vai rodar diferente.

## Quando bumpar

Uma única condição, e as três partes precisam valer juntas:

1. a solução descrita **está em produção**; **e**
2. a edição faz o documento descrever um **comportamento diferente** do que está
   em produção; **e**
3. esse comportamento novo **irá para produção**.

Incremento:
- **MINOR** (`1.0 → 1.1`) — mudança de comportamento dentro do mesmo escopo.
- **MAJOR** (`1.x → 2.0`) — o documento passa a descrever uma solução que não é
  mais a mesma. Na prática é raro: reformulação desse tamanho costuma virar
  **documento novo** (sufixo `-v2` no nome), que nasce em 1.0. Ver "Reformulação".

## Quando NÃO bumpar

Nenhum destes bumpa, e nenhum entra no histórico:

| Situação | Por quê |
|---|---|
| Documento ainda não implementado, seja qual for a mudança | Não há estado de produção anterior a comparar |
| Implementado, mergeado, **mas ainda não em produção** | Produção é o gatilho, não o merge |
| Corrigir texto, reorganizar seção, melhorar redação | Não muda comportamento |
| Acrescentar ADR que documenta decisão **já tomada** | Está registrando o passado, não decidindo o futuro |
| Corrigir erro factual do documento (ele descrevia errado o que já existe) | O comportamento em produção não muda — o documento é que estava errado |
| Nota de superseded apontando para um documento novo | É sinalização, não mudança |
| Detalhar mais, acrescentar exemplo, expandir tabela | Não muda comportamento |
| QA aprovar sem exigir mudança | Aprovar não é mudar |
| QA exigir correção de código que ainda não foi a produção | Continua sendo o primeiro estado de produção |

## O teste

> *"Esta edição vai fazer alguém mudar código que já está rodando em produção?"*

Não → não bumpa. É a pergunta inteira; não há segunda.

## Reformulação: documento novo, não MAJOR

Quando o modelo muda a ponto de o documento anterior deixar de descrever a mesma
solução, a convenção é **criar um documento novo** com sufixo (`-v2`), que nasce
em 1.0, e deixar o antigo:

- na versão em que está — ganhar nota de superseded **não** bumpa;
- como **as-built** enquanto a reformulação não estiver em produção. É ele que
  descreve o que está rodando, e é ele que alguém precisa ler para entender o
  sistema de hoje.

Isso preserva a propriedade que dá valor à versão: `1.5` num documento significa
cinco mudanças de comportamento em produção, e não "foi mexido cinco vezes".

## O que carrega a informação que a versão não carrega

| Pergunta | Campo |
|---|---|
| Em que fase este documento está? | `status` no frontmatter |
| Quando foi tocado pela última vez? | `atualizado` no frontmatter |
| Quem mudou, quando e por quê? | **git** |
| Quais comportamentos já foram a produção? | `## Histórico de versões` — uma linha por versão |

`atualizado` é atualizado **sempre**, inclusive nas edições que não bumpam. É
ele que responde "isto está fresco?", que era o papel que a versão vinha
acumulando indevidamente.

## Histórico de versões

Uma linha por **versão**, nunca por edição:

```markdown
| Versão | Data | Mudança |
|--------|------|---------|
| v1.0 | {data} | Baseline consolidada. |
```

Documento em 1.0 tem **uma** linha, sempre — a baseline —, por mais que tenha
sido reescrito no caminho. Edição que não bumpa não acrescenta linha.

## Memória de contexto é outra coisa

`.claude/memory/contexts/{contexto}.md` **não** segue esta política. Ele não é
contrato com consumidor: é o rastreador de estado do contexto, e o `versao` dele
acompanha a evolução do contexto (fase concluída, spec criada, código
implementado). Regra própria em `memory-protocol.md`. Não confundir `versao` do
contexto com `versao_prd`/`versao_spec`, que são **ponteiros** para a versão
própria de cada documento e seguem esta política.
