# AGENTS.md — projeto gofi

Instruções do projeto para qualquer agente de código (Claude Code, Codex,
Copilot, Cursor…), carregadas antes da primeira mensagem: como ler o projeto,
quais skills existem e o que só a pessoa decide. O detalhe de cada regra vive
nos protocolos de `.claude/expertise/harness-protocols/`, lidos quando a
tarefa pede.

> **Um arquivo só.** Não crie `CLAUDE.md`, `.claude/CLAUDE.md` nem
> `CLAUDE.local.md` no projeto. Instrução pessoal vai em `~/.claude/CLAUDE.md`.

## Leis comuns a todas as skills

Valem para toda skill; cada `SKILL.md` só acrescenta as dela (`Lei comum 3`).

1. **Portável; o específico mora fora.** Skill e knowledge levam só o que
   serviria, sem mudar uma palavra, a outro projeto com o mesmo SDK. Fato do
   projeto (entidade, role, endpoint, valor de negócio) vai para spec, memória
   ou institucional.
2. **Toda busca passa pelo grafo.** Símbolo ou documento: `gofi find`
   (`--in code`); quem chama: `gofi show`; A até B: `gofi path`; o que depende:
   `gofi impact`; texto exato: `gofi find --text`/`--regex`. `Grep`, `Glob`,
   `grep`/`rg` não varrem o projeto (o guard recusa). O arquivo que o grafo, a
   mudança ou um achado apontou — e tudo em `.claude/` —, leia direto.
   Protocolo: `.claude/expertise/harness-protocols/graph-retrieval.md`.
3. **Feche o que você mexeu.** Memória do contexto com
   `gofi memory write <contexto>` (o arquivo inteiro pela entrada padrão);
   mexeu em código, `gofi index code`; em spec ou PRD, `gofi index check`.
   Num turno do loop, o loop reindexa depois do turno.
4. **Versão conta produção, não edição.** PRD e spec ficam em `1.0` até a
   solução estar em produção; editar não bumpa nem entra no Histórico. A versão
   sobe uma vez, quando o fluxo fecha (mudança arquivada) — e é o CLI que a
   sobe. Política: `.claude/expertise/harness-protocols/document-versioning.md`.
5. **Bug fix ou mudança de comportamento leva teste de regressão** na mesma
   entrega: reproduz o cenário, falha sem o fix, passa com ele.
6. **Só se implementa o que está documentado.** O que a spec (ou o PRD) não
   diz vira pergunta à pessoa — ou, num turno do loop, pergunta gravada no
   arquivo de decisão e turno encerrado.
7. **Documento e memória só recebem o combinado.** Nada de rascunho,
   suposição, alternativa descartada, TODO, "a definir", marca de fase (✅,
   data, nome de agente) ou proveniência. Dúvida vira pergunta.

## Skills disponíveis

| Comando | Função |
|---------|--------|
| `/gofi-pd` | Discovery — PRD a partir do problema |
| `/gofi-spec` | Contrato — escreve a mudança (proposta, deltas, design) |
| `/gofi-run` | Entrega — leva uma mudança aprovada ao loop e repassa as decisões à pessoa |
| `/gofi-status` | Onde cada contexto está e o próximo passo |
| `/gofi-migrate` | Traz specs, PRDs e memória do formato antigo para o atual |

Os **papéis internos** (planejar, implementar por camada, testar, reparar,
triar, auditar, documentar) ficam em `.claude/gofi/roles/` e o loop os
despacha pelos fatos da mudança — `gofi run <mudança> --explain` diz quem e
por quê.

Pipeline: `/gofi-pd → /gofi-spec → a pessoa aprova → /gofi-run`.

**O contrato.** `specs/{contexto}/spec.md` é a verdade viva de um contexto:
requisitos `### Requisito: REQ-NN` (DEVE / NÃO DEVE) e cenários
`#### Cenário: REQ-NN.n` (DADO / QUANDO / ENTÃO). Ela não se edita: todo
trabalho é uma **mudança**, criada por `gofi change new <id> --context <ctx>`
(nunca à mão), aprovada pela pessoa, implementada pelo loop e aplicada à spec
viva no arquivamento. Contexto ainda no formato 2 (`sdd-*.md`) migra antes
(`/gofi-migrate`). Valide com `gofi spec check`.

**Decisões da pessoa** — as únicas cinco; nenhum agente as toma por ela:

| Decisão | Quando | Como | Trava? |
|---|---|---|---|
| Revisar | antes de aprovar | achados de `gofi review <mudança>` — rodado em primeiro plano, que a revisão leva minutos e a pessoa acompanha o progresso: corrige ou diz que não procede | não |
| Aprovar | contrato pronto | ferramenta `approve`, ou `gofi change approve` / `gofi spec approve` no terminal dela; editar depois desfaz o selo | sim |
| Responder o portão | a execução parou numa decisão não coberta | `gofi answer` (ou os cartões do painel) | sim |
| Decidir a emenda | `impasse` com proposta em `.gofi/proposals/` | edita a mudança e aprova de novo, ou escreve outra | sim |
| Aceitar a lição | execução aceita com retrabalho | `gofi learn accept` / `reject` | não |

Aprovar: quando a pessoa disser na conversa que aprova, use a ferramenta
`approve` do gofi (`ref`: o id da mudança, ou o contexto da spec). Rodados por
um agente, `gofi change approve` e `gofi spec approve` recusam. Nunca aprove
por iniciativa própria nem para destravar o seu trabalho. "Aprovado" é só esse
selo; o veredicto do QA na memória é `auditado` ou `reprovado`.

## Entrada: planeje antes de começar

Tarefa em texto livre passa pela ferramenta `intake` (ou `gofi intake
"<pedido>"`): contexto, fases na ordem, o que ler e o que falta decidir. Siga
as fases na ordem; pergunta em aberto vai à pessoa (contexto novo, nunca por
ela); pare para revisão depois de um PRD ou de uma mudança. Skill invocada
(`/gofi-spec …`) passa direto.

## Onde os agents leem o conteúdo

| Recurso | Caminho |
|---------|---------|
| Configuração | `.gofi.yaml` |
| Skills / papéis do loop | `.claude/skills/<name>/SKILL.md` / `.claude/gofi/roles/<name>/SKILL.md` |
| **Conhecimento portável — carregue por aqui** | `.claude/knowledge/INDEX.md`: núcleo ⬤ + só os módulos que a tarefa pede |
| Aprendizado do time (vence expertise e sdk) | `.claude/knowledge/shared/`, `.claude/knowledge/{papel}/` — pelo mesmo `INDEX.md` |
| API, boilerplates e knowledge do SDK | `.claude/sdk/<lang>/` — pelo mesmo `INDEX.md`; símbolo por `gofi find --in sdk` |
| Especialidades | `.claude/expertise/<pack>/` — o `PACK.md` diz quando; de `harness-protocols`, só o protocolo que a tarefa pede |
| Institucional | `.claude/institutional/{project.name}/` — `INDEX.md` e só os chunks relevantes |
| Templates | `.claude/templates/`; design do stack em `.claude/sdk/<lang>/templates/` |
| Memória global | `.claude/memory/project.md` |
| Estado de um contexto | `.claude/memory/contexts/{contexto}.md` — leia o `## Estado atual` |
| Spec viva | `specs/{contexto}/spec.md` — um requisito: `gofi show specs/{contexto}/spec.md#REQ-NN` |
| Mudanças | `specs/{contexto}/changes/{id}/` (a de vários contextos, `specs/changes/{id}/`; a primeira de um contexto sem spec, a **gênese**, em `specs/{contexto}/` com o id do contexto); arquivadas em `changes/archive/`. `gofi change list` mostra a pasta de cada uma |
| PRDs | `prd/{contexto}/prd-{contexto}.md` |
| Léxico das facetas | `.claude/lexicon/*.md` |
| Grafo de código | `.gofi/index/code/` — consulte por `gofi find --in code` e `gofi show`, nunca o JSON |

## Convenção de leitura dos agents

A pré-execução exata vive em cada skill; o denominador comum:

1. `.gofi.yaml`; `.claude/memory/project.md`; do contexto, só o
   `## Estado atual` de `.claude/memory/contexts/{contexto}.md`.
2. `.claude/knowledge/INDEX.md` — o núcleo ⬤ e só os módulos que a tarefa
   pede (do time e do SDK também). **Nunca** por glob.
3. Documento: `gofi find "<pergunta>"` (doc, §seção, linhas) e leia a faixa; a
   mudança em que você trabalha, inteira.
4. Documento novo: template de `.claude/templates/`, `formato:`, facetas do
   léxico, `keywords`, zero proveniência
   (`.claude/expertise/harness-protocols/rag-retrieval.md`).
5. Todo pacote de um contexto nasce com `//gofi:context {contexto}`.

## Persistência de estado

Ao concluir uma fase:
- `.claude/memory/contexts/{contexto}.md` — frontmatter (`status`,
  `atualizado`, `keywords`) e reescrita do `## Estado atual`. O
  `## Histórico de versões` ganha uma linha só quando a versão sobe.
- `.claude/memory/project.md` só quando nasce um serviço ou binário novo.
- **Nunca** a spec viva nem uma mudança aprovada: ambas são seladas.

Protocolo: `.claude/expertise/harness-protocols/memory.md`.

## Aprendizado contínuo

Quando a pessoa corrigir, ensinar ou validar algo não-óbvio, a lição vale para
todos os papéis. Procedimento: `.claude/expertise/harness-protocols/learning.md`.

## Projeto

<!-- gofi:project:begin -->
<!-- Instruções deste projeto: convenções, comandos, avisos que valem só aqui.
     O gofi preserva este bloco em toda atualização; o resto do arquivo vem do
     gofi e é atualizado por `gofi update agents`. -->
<!-- gofi:project:end -->
