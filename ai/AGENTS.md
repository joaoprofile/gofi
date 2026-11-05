# AGENTS.md — projeto gofi

Instruções do projeto para qualquer agente de código (Claude Code, Codex,
Copilot, Cursor…), carregadas antes da primeira mensagem: como ler o projeto,
quais skills existem e o que só a pessoa decide. O detalhe de cada regra vive
nos protocolos de `.claude/expertise/harness-protocols/`, lidos quando a
tarefa pede.

> **Um arquivo só.** Não crie `CLAUDE.md`, `.claude/CLAUDE.md` nem
> `CLAUDE.local.md` no projeto. Instrução pessoal vai em `~/.claude/CLAUDE.md`.

## Doutrina das skills (o que entra e o que NÃO entra)

Uma skill carrega só especialidade **transferível**: método e técnica que
serviriam, sem mudar uma palavra, a outro projeto com o mesmo SDK. Nada de
produto, empresa ou instituição (entidades, roles, module paths, endpoints,
valores de negócio). O específico mora em `.gofi.yaml`, `specs/`,
`.claude/memory/` e `.claude/institutional/`; o aprendizado do time, em
`.claude/knowledge/`. **Teste:** serviria a outro projeto? → skill/knowledge.
Só vale aqui? → spec/memória/institucional.

## Leis comuns a todas as skills

Valem para toda skill; cada `SKILL.md` só acrescenta as dela (`Lei comum 3`).

1. **Portável; o específico mora fora.** Fato do projeto vai para spec,
   memória ou institucional — nunca para a skill.
2. **Toda busca passa pelo grafo.** Achar símbolo ou documento é `gofi find`
   (`--in code` para código); quem chama, `gofi show`; como A chega em B,
   `gofi path`; o que depende de algo, `gofi impact`. Texto exato — string,
   chave de config, SQL, mensagem de erro — é `gofi find --text`/`--regex`, que
   devolve cada linha sob o símbolo que a contém. `Grep`, `Glob` e `grep`/`rg`
   não varrem o projeto: no loop eles não existem, e na sessão o guard
   (`ai.guard: enforce`, o padrão) os recusa. Consulta vazia: reformule com o
   termo técnico. O arquivo que o grafo, a mudança ou um achado apontou, leia
   direto. `--deep` só para afirmar ausência de uso. Protocolo:
   `.claude/expertise/harness-protocols/graph-retrieval.md`.
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

| Decisão | Quando | Como a pessoa decide | Trava o loop? |
|---|---|---|---|
| Revisar | antes de aprovar | lê os achados de `gofi review <mudança>` e corrige a mudança, ou diz que o achado não procede | não — informa |
| Aprovar | contrato pronto | ferramenta `approve` (o host confirma), ou `gofi change approve` / `gofi spec approve` no terminal dela | sim — sem selo não roda; editar depois desfaz o selo |
| Responder o portão | uma execução parou numa decisão que a mudança não cobre | `gofi answer` (ou os cartões do painel) | sim — a execução espera |
| Decidir a emenda | a execução terminou em `impasse` e a triagem escreveu `.gofi/proposals/` | edita a mudança e aprova de novo, ou escreve outra mudança | sim — a mudança só volta a rodar aprovada |
| Aceitar a lição | uma execução aceita com retrabalho propôs uma lição | `gofi learn accept` / `gofi learn reject` | não |

Aprovar: quando a pessoa disser na conversa que aprova, use a ferramenta
`approve` do gofi (`ref`: o id da mudança, ou o contexto da spec). Rodados por
um agente, `gofi change approve` e `gofi spec approve` recusam. Nunca aprove
por iniciativa própria nem para destravar o seu trabalho. "Aprovado" é só esse
selo; o veredicto do QA na memória é `auditado` ou `reprovado`.

## Entrada: planeje antes de começar

Tarefa em texto livre passa primeiro pela ferramenta `intake` (ou
`gofi intake "<pedido>"`): ela diz o contexto, as fases na ordem, o que ler e
o que falta decidir.

1. **Pergunta em aberto:** resolva pela conversa ou pergunte à pessoa com as
   opções dadas. Nunca escolha um contexto novo pela pessoa.
2. **Siga as fases na ordem**, cada uma com o `SKILL.md` do papel.
3. **Pare para revisão** depois de um PRD ou de uma mudança.
4. **Passe direto** quando a pessoa invocar uma skill (`/gofi-spec …`).

## Onde os agents leem o conteúdo

| Recurso | Caminho |
|---------|---------|
| Configuração | `.gofi.yaml` |
| Skills / papéis do loop | `.claude/skills/<name>/SKILL.md` / `.claude/gofi/roles/<name>/SKILL.md` |
| **Conhecimento portável — carregue por aqui** | `.claude/knowledge/INDEX.md`: núcleo ⬤ + só os módulos que a tarefa pede |
| Aprendizado do time (vence expertise e sdk) | `.claude/knowledge/shared/`, `.claude/knowledge/{papel}/` — pelo mesmo `INDEX.md` |
| API, boilerplates e knowledge do SDK | `.claude/sdk/<lang>/` — pelo mesmo `INDEX.md`; símbolo por `gofi find --in sdk` |
| Especialidades | `.claude/expertise/<pack>/` (`harness-protocols` vale para toda tarefa) |
| Institucional | `.claude/institutional/{project.name}/` — `INDEX.md` e só os chunks relevantes |
| Templates | `.claude/templates/`; design do stack em `.claude/sdk/<lang>/templates/` |
| Memória global | `.claude/memory/project.md` |
| Estado de um contexto | `.claude/memory/contexts/{contexto}.md` — leia o `## Estado atual` |
| Spec viva | `specs/{contexto}/spec.md` — um requisito: `gofi show specs/{contexto}/spec.md#REQ-NN` |
| Mudanças | `specs/{contexto}/changes/{id}/` (a de vários contextos, `specs/changes/{id}/`); arquivadas no `archive/` ao lado |
| PRDs | `prd/{contexto}/prd-{contexto}.md` |
| Léxico das facetas | `.claude/lexicon/*.md` |
| Grafo de código | `.gofi/index/code/` — consulte por `gofi find --in code` e `gofi show`, nunca o JSON |

## Convenção de leitura dos agents

A pré-execução exata vive em cada skill; o denominador comum:

1. `.gofi.yaml` — linguagem, nome do projeto, superfícies.
2. `.claude/memory/project.md` e, do contexto, o `## Estado atual` de
   `.claude/memory/contexts/{contexto}.md`.
3. `.claude/knowledge/INDEX.md` — o núcleo ⬤ e só os módulos que a tarefa
   pede, inclusive os do time e do SDK. **Nunca** por glob.
4. Documento se acha por `gofi find "<pergunta>"` (doc, §seção e faixa de
   linhas) e se lê pela faixa; a mudança em que você trabalha, leia inteira.
5. Ao criar documento: o template de `.claude/templates/`, `formato:`, as
   facetas do léxico e `keywords`; zero proveniência. Protocolo:
   `.claude/expertise/harness-protocols/rag-retrieval.md`.
6. Todo pacote de um contexto nasce com `//gofi:context {contexto}` — o elo
   entre o código e `specs/{contexto}/`.

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
