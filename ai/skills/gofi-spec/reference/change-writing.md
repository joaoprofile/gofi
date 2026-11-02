# Escrita da mudança — requisitos, delta, proposta e design

O formato e o ciclo estão na decisão 0003; o `gofi spec check <id>` verifica
tudo o que segue, sem modelo. Esta referência diz **como escrever bem** o que o
check só sabe verificar na forma.

## Requisitos e cenários

### Forma

```markdown
### Requisito: REQ-04 — Pedido enviado não pode ser cancelado
O sistema NÃO DEVE cancelar um pedido QUANDO ele já foi enviado.

#### Cenário: REQ-04.1 — Cancelar pedido enviado
- DADO um pedido enviado
- QUANDO o cliente pede o cancelamento
- ENTÃO recebe recusa
- E o pedido não muda
```

### O que é um bom requisito

- **Uma obrigação por requisito**, com DEVE ou NÃO DEVE. Dois verbos
  independentes são dois requisitos.
- **EARS quando há gatilho**: "QUANDO {evento}, o sistema DEVE {resposta}";
  "ENQUANTO {estado}, o sistema DEVE …"; "SE {condição indesejada}, ENTÃO o
  sistema DEVE …". Sem gatilho, a forma direta: "O sistema DEVE …".
- **Observável de fora**: o que um cliente da API, um usuário ou outro
  contexto percebe. "Grava na tabela X" é design; "o pedido aparece como
  cancelado" é requisito.
- **Invariante do domínio é requisito NÃO DEVE** — é a regra de negócio de
  sempre (o antigo RN), agora com cenário.
- **Não funcional é requisito como os outros**, mensurável: "O sistema DEVE
  responder a listagem em até 300 ms no p95 com 10 mil pedidos".

### O que é um bom cenário

- **Um comportamento por cenário**: um QUANDO. Caminho feliz e cada recusa
  são cenários separados (REQ-04.1, REQ-04.2…).
- **DADO concreto** — o estado mínimo que torna o cenário verdadeiro, sem
  detalhe que não muda o resultado.
- **ENTÃO verificável por um teste** — um resultado, não "funciona
  corretamente". Efeito colateral que importa vai em `- E …`.
- O cenário é **lido pelo `gofi-test` sem olhar o código**: se ele não basta
  para escrever o teste, falta algo nele.

### IDs

- Número novo é o **próximo livre do contexto**: o maior entre a spec viva
  (`gofi show specs/{ctx}/spec.md`) e as mudanças abertas do contexto
  (`gofi change list`), mais um. Duas mudanças abertas não podem adicionar o
  mesmo número — o check recusa.
- Cenário `REQ-NN.n` pertence a `REQ-NN`; numere a partir de `.1`.
- Removido, o número fica vago. Nunca renumere para "fechar buracos": o
  rastreio entre código e teste (`gofi:implements`, `gofi:tests`) usa o ID.

## O delta — `deltas/<ctx>.md`

- **ADICIONADO** — requisitos novos, inteiros, com os cenários.
- **MODIFICADO** — o requisito **inteiro como ficará**, com **todos** os seus
  cenários, inclusive os que não mudaram. Nunca "igual ao anterior, exceto…": o
  arquivamento substitui o bloco, sem interpretar.
- **REMOVIDO** — o título e `**Por quê:** …`. O teste que cobria o requisito
  sai junto, na implementação.
- **Propósito** — só para contexto que ainda não tem spec viva: vira o
  `## Propósito` dela. Num contexto que já tem, apague a seção.
- `depende_de: [...]` no frontmatter do delta troca o da spec viva ao arquivar —
  declare quando a mudança faz este contexto passar a consumir outro.
- Seção sem requisito sai do arquivo.

## Correção de defeito

Uma correção também é uma mudança: o defeito mostra um cenário que o contrato
não tinha.

- O requisito que o código descumpre entra em **MODIFICADO**, inteiro, com um
  cenário novo que **reproduz o defeito** (`REQ-NN.k — {o caso que quebrava}`):
  DADO o estado que dispara o bug, QUANDO a ação, ENTÃO o comportamento certo.
  É esse cenário que vira o teste de regressão.
- O requisito não existia (o código faz algo que ninguém especificou e está
  errado): **ADICIONADO**, com o cenário.
- A proposta diz o defeito em "Por quê" e o impacto pelo grafo.
- O design traz só o que a correção muda; `design: n/a` só se nenhum código
  muda — o que numa correção é raro.

## A proposta — `proposal.md`

- **Por quê** — o problema ou a oportunidade, 2–3 frases; cite o PRD quando
  houver.
- **O que muda** — uma linha por requisito, por contexto:
  "pedidos: REQ-07 novo (limite por cliente); REQ-04 modificado (…)".
- **Impacto** — o alcance real, do grafo: `gofi show ctx:{ctx}` (vizinhos,
  tabelas, pacotes) e os símbolos que a mudança toca. Contexto que depende
  deste e muda de comportamento é citado aqui — e pode pedir um delta também.
- **Fora desta mudança** — o que fica para outra.
- `contextos: [...]` lista **todo** contexto com delta; `depende_de` lista
  mudanças abertas que precisam ser aceitas antes desta.
- Mudança sem código a mudar (só remover um requisito, corrigir texto de
  cenário): `design: n/a`.

## O design — `design.md`

O template é o do pack da linguagem (`.claude/sdk/<lang>/templates/`): as
seções são as mesmas em todo pack, o conteúdo é do stack.

- **Só o que esta mudança faz** — o sistema inteiro está no código e no grafo.
  Não descreva de novo o que existe; cite o símbolo (`gofi show <símbolo>`).
- **Abordagem** — como os requisitos são realizados e o que muda no grafo.
- **Modelo de dados** — migração (par up/down) e índices pelo perfil de acesso.
- **Contratos** — rotas e eventos novos ou alterados, cada um com os REQ que
  realiza.
- **Erros** — do catálogo do SDK/framework, cada um com o REQ que o pede.
- **Mapa de requisitos** — **todo** REQ ADICIONADO ou MODIFICADO e o
  componente que o realiza (onde vai o `gofi:implements`). Numa mudança de
  vários contextos, `ctx/REQ-NN` quando o número existe em mais de um.
- **Decisões** — ADR só do que a mudança decide e alguém poderia decidir
  diferente.
- Princípios do projeto (`project.md` §Princípios) são portões: o design que
  viola um diz por quê, em ADR, ou não passa na revisão.
- Conteúdo obrigatório por camada → `reference/spec-writing-rules.md`.
