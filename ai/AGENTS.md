# AGENTS.md — projeto gofi

Instruções do projeto para qualquer agente de código — Claude Code, Codex,
Copilot, Cursor, Windsurf e os demais que leem `AGENTS.md` —, carregadas antes
da primeira mensagem. Documenta como o agente lê o conteúdo do projeto e quais
skills estão disponíveis. Regras de código, padrões de SDK e armadilhas
conhecidas vivem em arquivos cross-AI sob `.claude/sdk/<lang>/knowledge/` e
`.claude/expertise/<pack>/` — o agente consome via as skills. O aprendizado do
time fica em `.claude/knowledge/` e vence os dois.

> **Um arquivo só.** Não crie `CLAUDE.md`, `.claude/CLAUDE.md` nem
> `CLAUDE.local.md` no projeto: o Claude Code deixa de ler este arquivo quando
> encontra qualquer um deles. Instrução pessoal vai em `~/.claude/CLAUDE.md`,
> que não interfere.

> **Modelo de duas camadas** (v2.5+): `.claude/` é a camada de decisão da IA
> — markdown destilado (agents, knowledge, referência de API, boilerplates).
> `.gofi/gofi-sdk-<lang>/` é a camada de execução do toolchain (código real
> do SDK, importável pelo módulo do projeto via `go.work`); os agents
> normalmente NÃO leem daí. Se um agent precisou abrir código real para
> decidir, é gap de curadoria — registrar em `.claude/knowledge/` e oferecer a
> correção ao upstream (protocolo de aprendizado).

## Doutrina das skills (o que entra e o que NÃO entra)

Eixo ortogonal ao modelo de duas camadas acima. Esta é a regra que governa o
conteúdo de cada skill — enunciada **uma vez aqui**; as "Leis" no topo de cada
`.claude/skills/<name>/SKILL.md` apenas replicam para auto-contenção.

> **Skill carrega só especialidade transferível.** Cada skill é um especialista
> **genérico e portável**: leva metodologia e técnica que serviriam, sem mudar
> uma palavra, a outro projeto com o mesmo SDK. **Nada** de produto/empresa/
> instituição entra na skill (nomes de entidade, roles, module paths, endpoints,
> valores de negócio). Trocar de projeto **não** muda a skill.
>
> **Conhecimento específico mora FORA da skill** — nas linhas marcadas
> *Específico* no mapa abaixo: `.gofi.yaml`, `specs/`, `.claude/memory/`,
> `.claude/institutional/`. Padrão técnico genérico vive em `.claude/expertise/`
> e `.claude/sdk/<lang>/` (e o aprendizado do time, em `.claude/knowledge/`), sempre **domínio-neutro** (placeholders `{contexto}`,
> `<module>`, `RoleA`, `entity`).
>
> **Teste de pertencimento:** *serviria, sem mudar uma palavra, a outro projeto
> com o mesmo SDK? → skill/knowledge. Só vale aqui? → spec/memória/institucional.*

## Leis comuns a todas as skills

Valem para toda skill, antes de qualquer passo; cada `SKILL.md` só acrescenta
as leis que são dela. Numeradas para serem citadas (`Lei comum 3`).

1. **Portável; o específico mora fora.** A skill carrega só método e técnica
   transferível (ver [doutrina](#doutrina-das-skills-o-que-entra-e-o-que-não-entra)).
   Fato do projeto vai para spec, memória ou institucional — **nunca** para a
   skill. Institucional é RAG: `INDEX.md` e só os chunks relevantes.
2. **O índice é o primeiro movimento (LEI absoluta).** Achar é `gofi find`
   (`--in code` para código); entender quem chama é `gofi show`; como A chega
   em B é `gofi path`. O gate não é *"isto é símbolo?"* — essa se responde de
   cabeça e é por ela que a varredura volta —, é *"eu já consultei o índice?"*:
   uma consulta antes da primeira busca literal, sempre. Alvo que não é nó
   (`const`/`var`, diretiva em comentário, string) se acha pelo `gofi show` do
   símbolo concreto que o referencia (o DTO, o service, o tipo). Consulta
   vazia **não** autoriza varrer: reformule (dois termos, o vizinho concreto);
   ausência pede `gofi index code --deep` (texto não resolve dispatch por
   interface); código que você acabou de escrever pede `gofi index code` antes. Só então a busca
   literal `gofi find --text`/`--regex`, **declarada** ("caí na busca literal
   porque X"). `grep`/`Glob` puro só sem gofi disponível. Protocolo:
   `.claude/expertise/harness-protocols/graph-retrieval.md`.
3. **Fechar é reconstruir o índice (LEI absoluta).** Toda entrega termina na
   ordem: memória do contexto (gravada com `gofi memory write <contexto>`, o
   arquivo inteiro pela entrada padrão — `Edit`/`Write` em `.claude/` pedem
   aprovação que numa fase conduzida ninguém dá) → `gofi index code` (se mexeu em código) →
   `gofi index docs` (se mexeu em spec/PRD/memória) → `gofi index check`
   (faceta fora do léxico reprova — barata de corrigir agora, cara três fases
   depois). O índice de documentos se refaz sozinho na consulta, mas os
   `INDEX.md` versionados só no `gofi index docs`; o hook de pre-commit só age
   no commit, e a próxima fase roda antes dele. Índice velho é pior que
   nenhum: aponta com confiança para o lugar errado.
4. **Versão de documento conta estado de produção, não edição (LEI).** PRD e
   spec nascem e ficam em `1.0` até a solução estar em produção; refinar,
   reescrever, trocar decisão, acrescentar ADR ou corrigir erro não bumpa, nem
   entra no Histórico. **Teste:** *esta edição vai fazer alguém mudar código
   que já roda em produção?* Não → não bumpa. `atualizado` muda sempre;
   `status` diz a fase; reformulação profunda vira documento novo (sufixo
   `-v2`), em `1.0`. **E sobe uma vez, no fim do fluxo:** mesmo quando a
   mudança passa no teste, ninguém bumpa no meio — nem quem edita a spec, nem
   quem implementa. A versão (e a linha do Histórico) sobe quando o fluxo
   inteiro fecha: documento e código alterados e a auditoria aprovada. Várias
   edições do mesmo fluxo são uma versão só. Política: `.claude/expertise/harness-protocols/document-versioning.md`.
5. **Bug fix ou melhoria de comportamento → teste de regressão na mesma
   entrega (LEI absoluta, para quem escreve código).** O teste reproduz o
   cenário, falha sem o fix e passa com ele, e nomeia o defeito para ele
   nunca voltar. Sem ele a entrega não fecha — não se delega ao QA.
6. **Só se implementa o que está documentado (LEI).** Código, tela ou
   infraestrutura nascem da spec — ou, antes dela, do PRD. O que não está lá
   não se implementa por conta própria, por mais óbvio que pareça: vira
   pergunta à pessoa (ou, numa fase conduzida, pergunta gravada e fase
   encerrada), e a resposta entra na spec antes do código.
7. **Documento e memória só recebem o combinado (LEI).** PRD, spec e memória
   de contexto registram o que foi decidido e confirmado — nada de rascunho de
   raciocínio, suposição não confirmada, alternativa descartada, TODO, "a
   definir", trecho de conversa, marca de fase (✅, data, nome de agente) ou
   proveniência. Dúvida não entra no documento: vira pergunta. O que a pessoa
   não confirmou fica fora.

## Skills disponíveis

| Comando | Função |
|---------|--------|
| `/gofi-pd` | Product Discovery — gera PRD a partir de problema bruto |
| `/gofi-spec` | Specification Architect — gera spec SDD a partir do PRD |
| `/gofi-eng` | Context Engineer — implementa contexto a partir da spec |
| `/gofi-ui` | Context UI/UX — implementa a camada de apresentação a partir da spec |
| `/gofi-ops` | Platform & Delivery — DevOps especialista (Terraform, OCI, Go build, CI/CD Azure DevOps/GitHub Actions); provisiona IaC + pipelines a partir da spec de infra |
| `/gofi-qa` | Quality Auditor — audita implementação contra spec e padrões |
| `/gofi-doc` | Documentation Generator (Frontend & QA) — gera doc de contrato a partir de handlers Go |
| `/gofi-migrate` | Corpus Migration — traz specs, PRDs e memória de um formato antigo para o atual (facetas, índice em dois níveis, grafo); orquestra `gofi index migrate` |
| `/gofi-status` | Índice de Contextos — monta sob demanda o panorama (Implementados/Spec/PRD) lendo o frontmatter dos `contexts/*.md` |
| `/gofi-full` | Full-Cycle Orchestrator — encadeia `gofi-pd → gofi-spec → gofi-eng → gofi-qa` em fluxo contínuo, volta à fase anterior quando reprova e segue até o QA aprovar sem ressalvas |

Pipeline típico: `/gofi-pd → /gofi-spec → /gofi-eng → /gofi-qa` (ou `/gofi-full` para o ciclo inteiro orquestrado).
Camada de apresentação: `/gofi-ui` após a spec (e o contrato do `gofi-eng`).
Plataforma/infra: `/gofi-spec` (infra) → `/gofi-ops`.
Doc de contrato: abrir o handler no IDE → `/gofi-doc`.

## Entrada: planeje antes de começar

Uma tarefa em texto livre — "altere o fluxo de pricing…", "crie um PRD de…" —
passa primeiro pela entrada do gofi: a ferramenta MCP `intake` (ou
`gofi intake "<pedido>"` no terminal). Ela diz, a partir do índice e dos
contratos das skills, o que o pedido quer, o contexto e o que ele já tem, as
fases na ordem — cada papel no seu nível, com o motivo —, as especialidades a
ler, o que falta decidir e o pedido montado da primeira fase.

1. **Pergunta em aberto:** resolva pela conversa se ela já disser; se não,
   pergunte à pessoa com as opções que a entrada deu, e chame de novo com
   `answers`. Nunca escolha um contexto novo pela pessoa.
2. **Siga as fases na ordem**, cada uma com o `SKILL.md` do papel
   (`.claude/skills/<papel>/SKILL.md`), começando pelo pedido montado.
3. **Pare para revisão** depois de um PRD ou de uma spec: o que vem depois
   constrói em cima deles.
4. **Passe direto** quando a pessoa invocar uma skill (`/gofi-eng …`): o pedido
   já disse o papel.

## Onde os agents leem o conteúdo

Coluna **Natureza** = lado da [doutrina](#doutrina-das-skills-o-que-entra-e-o-que-não-entra):
*Portável* (genérico, viaja entre projetos do mesmo SDK) vs *Específico* (só
vale neste projeto — nunca entra em skill).

| Recurso | Caminho no projeto | Natureza |
|---------|--------------------|----------|
| Configuração do projeto | `.gofi.yaml` (raiz) | Específico |
| Skills (slash) | `.claude/skills/<name>/SKILL.md` — pasta por skill, com `name` e `description` no frontmatter; é o único layout que o Claude Code descobre | Portável |
| Referência de API do SDK por linguagem | `.claude/sdk/<lang>/api/` — **gerada** do código do SDK na versão fixada (`gofi update sdk`); um arquivo por pacote, lista em `api/INDEX.md`. Não edite | Portável (derivado) |
| Boilerplates por camada | `.claude/sdk/<lang>/boilerplates/*.md` | Portável |
| Knowledge específico da linguagem | `.claude/sdk/<lang>/knowledge/` — **carregue pelo `knowledge/INDEX.md`**, nunca por glob | Portável |
| **Manifesto do conhecimento portável (carregue por aqui)** | `.claude/knowledge/INDEX.md` — núcleo ⬤ + sob demanda. Derivado: `gofi index docs` | Portável (derivado) |
| Especialidades (packs) | `.claude/expertise/<pack>/` — `PACK.md` (contrato: quando se aplica, a quais papéis serve) + seções; `harness-protocols` vale para toda tarefa. Idem: pelo `INDEX.md` | Portável |
| Aprendizado do time | `.claude/knowledge/shared/` (todos os papéis) e `.claude/knowledge/{agent}/` (um papel) — gravado pelo protocolo de aprendizado; vence `expertise/` e `sdk/` e declara `overrides:` quando corrige regra do gofi. O `gofi update` nunca reescreve | Específico |
| Conhecimento institucional (negócio específico do produto/empresa) | `.claude/institutional/{project.name}/` — RAG: `INDEX.md` (sempre) + chunks sob demanda. **Espelho pull-only** do repo `sources.institutional` quando configurado (atualizado por `gofi update institutional`, que **substitui a pasta por completo**); **sem repo**, mantido à mão no git do projeto | Específico |
| Templates SDD/PRD | `.claude/templates/` (`sdd-template.md`, `prd-template.md`) | Portável |
| Memória global (visão, serviços, convenções) | `.claude/memory/project.md` | Específico |
| Estado por-contexto (cabeça: frontmatter + Estado atual consolidado + Histórico de versões) | `.claude/memory/contexts/{contexto}.md` | Específico |
| Versões antigas transbordadas (só quando o changelog cresce) | `.claude/memory/contexts/{contexto}/history.md` | Específico |
| Índice de contextos (gerado sob demanda) | `/gofi-status` (lê o frontmatter dos `contexts/*.md`) | Específico |
| Specs do projeto | `specs/{contexto}/sdd-{contexto}.md` | Específico |
| PRDs do projeto | `prd/{contexto}/prd-{contexto}.md` | Específico |
| Índice de retrieval de specs/PRDs (2 níveis) | `specs/INDEX.md` (roteador de contextos) + `specs/{contexto}/INDEX.md` (shard); idem `prd/`. Derivado: `gofi index docs` | Específico (derivado) |
| **Busca e navegação no corpus** | `gofi find "<pergunta>"` — busca documentos e código de uma vez e devolve doc/§/linhas (`--in` estreita as áreas); `gofi show` descreve um doc, §seção, `ctx:`, `table:` ou símbolo; `gofi index check` aponta os órfãos | Específico (derivado) |
| Grafo de documentos | `.gofi/index/docs/graph.json` — `declara`/`cita`/`wikilink`/`toca`/`pertence`/`implementa`. Derivado, fora do git: `gofi find`/`gofi show` o reconstroem sozinhos quando falta ou quando algum documento muda | Específico (derivado) |
| Léxico controlado das facetas | `.claude/lexicon/*.md` — `entidades` (do schema), `operacoes`, `marketplaces`, `sinonimos` (ponte entre a língua da pergunta e a do identificador) | Específico |
| Protocolo de retrieval (ler os corpora gastando poucos tokens) | `.claude/expertise/harness-protocols/rag-retrieval.md` | Portável |
| Grafo de código (mapa derivado: quem chama quem, a que contexto pertence) | `.gofi/index/code/` — **um escopo por árvore**: `gofi_graph_index.json` na raiz lista todos (backend em `.`, cada superfície de UI em `{nome}/`, SDK em `sdk/`), e cada escopo tem o seu `gofi_graph_report.md`. Consulta por `gofi find --in code` (achar) e `gofi show` (entender quem chama). Gerado por `gofi index code`, **nunca** editado à mão | Específico (derivado) |
| Protocolo de consulta ao grafo | `.claude/expertise/harness-protocols/graph-retrieval.md` | Portável |

## Convenção de leitura dos agents

Esqueleto comum. A pré-execução **exata** vive no topo de cada skill (passos
e arquivos variam por agent); aqui fica o denominador comum:

1. Ler `.gofi.yaml` para descobrir linguagem-alvo (`project.language`), nome do projeto e demais configurações.
2. `AGENTS.md` (este arquivo, já carregado) — mapa de paths físicos + doutrina das skills.
3. Ler `.claude/memory/project.md` para visão global (serviços + convenções). Para o índice de contextos existentes, rodar `/gofi-status`.
4. Ler `.claude/memory/contexts/{contexto}.md` se já houver — frontmatter (estado) + handoff de fases anteriores.
5. Ler **`.claude/knowledge/INDEX.md`** — manifesto das camadas portáveis. Carregar o **núcleo ⬤** (curto, universal) e **só os módulos que a tarefa pede**. **Nunca** carregar `expertise/**`, `knowledge/**` nem `sdk/<lang>/knowledge/*.md` por glob: é o maior custo fixo do harness, maior que a spec do contexto.
6. Ler **`.claude/knowledge/shared/*.md`** e **`.claude/knowledge/{agent}/*.md`** — aprendizado do time (vence expertise e sdk quando divergem).
7. **Contexto institucional (RAG)** — quando precisar de negócio além da spec, ler `.claude/institutional/{project.name}/INDEX.md` e **só os chunks relevantes**; nunca a pasta inteira.
8. Para a linguagem-alvo, carregar **pelo mesmo `.claude/knowledge/INDEX.md`** (ele
   já cobre `sdk/<lang>/`), nunca por glob:
   - `sdk/<lang>/knowledge/` — regras, naming, estrutura, layers, armadilhas
   - `sdk/<lang>/api/` — API do SDK, gerada do código (só os pacotes relevantes; símbolo por `gofi find --in sdk`)
   - `sdk/<lang>/boilerplates/` — esqueletos, antes de implementar (gofi-eng/qa)

> **Procurar documento é `gofi find`, não carregar índice.** O corpus tem camada
> de consulta, e é o mesmo comando que busca o código: pergunte, não carregue.
> `gofi find "<pergunta>"` devolve doc, §seção e **faixa de linhas** — leia com
> `Read(offset, limit)`. Vazio quase sempre é vocabulário, não documento faltando:
> tente o termo técnico e **registre o par que faltou** em
> `.claude/lexicon/sinonimos.md`. Os `INDEX.md` servem para **navegar** um
> contexto, não para **achar** um assunto; texto exato é
> `gofi find --text "<literal>"` (ou `--regex`, `-i`), fallback declarado.
>
> `gofi show table:<tabela>` responde *quem documenta* e *quem implementa* de
> uma vez — a tabela é o único símbolo que existe nos dois mundos.
> `gofi show <doc>` é a análise de impacto barata **antes** de editar.
>
> **Specs/PRDs/contextos são RAG — gaste poucos tokens.** Regras de **leitura e criação** em `.claude/expertise/harness-protocols/rag-retrieval.md`.
> - **Ler:** nunca leia um doc inteiro por reflexo. `gofi find "<pergunta>"` devolve doc, §seção e faixa de linhas — leia com `Read(offset, limit)`.
> - **Criar/editar:** base nos templates `.claude/templates/` — `formato:` + as facetas (`entidades` do schema, `operacoes`/`marketplaces` do `.claude/lexicon/`) + `keywords` com o que sobrou (teto 12). **Zero proveniência** (sem `**Autor/Versão/Data:**`, sem `## Rastreabilidade`, sem nome de agent/pessoa, sem journal), Histórico de **1 linha**. Ao fechar: `gofi index docs` e `gofi index check` — termo de faceta fora do léxico reprova, e é isso que impede o vocabulário de voltar a ser nuvem de tags.

> **O código também tem índice — o grafo. Procurar código é `gofi find --in code` (achar) e `gofi show` (entender quem chama), não `grep -r`.**
> Suba a escada de 3 degraus: `gofi_graph_index.json` (que escopos existem, em
> que pasta, **em que modo**) → o `gofi_graph_report.md` **do escopo** →
> `gofi show <símbolo>`. Não sabe o nome exato? `gofi find --in code
> "<termo> <termo>"` busca dentro do grafo; `gofi path A B` mostra como A
> alcança B. `gofi find --text "<literal>"`/`--regex "<re>"` é **fallback
> declarado**: linguagem sem extractor, ou alvo que não é símbolo (string, chave
> de config, SQL, tag, import, mensagem de erro) — cada linha vem sob o símbolo
> que a contém, mas texto não resolve dispatch por interface. `grep` puro só se o
> `gofi` não alcançar (árvore fora do projeto). **Nunca** leia `gofi_graph.json`.
> Isto vale para **código**; navegar spec/PRD por §seção (`gofi show <doc>`
> lista as seções com a faixa de linhas) é o protocolo RAG de documento e
> continua valendo.
>
> **Valide o modo antes de concluir ausência.** O build (`init`,
> `gofi index code`) segue o modo do `.gofi.yaml` (`graph: deep:`), que **por
> padrão é `fast`** — e em `fast` chamada ambígua não vira aresta, então
> ausência de aresta não é prova de ausência de uso. `deep` é sempre pedido explícito: precisa afirmar "isto não quebra nada",
> "ninguém mais chama", "quem implementa esta interface"? Rode
> `gofi index code --deep` ou declare a limitação. Os gatilhos estão em
> *Quando rodar `--deep`* do protocolo do grafo.
>
> O hook de pre-commit põe o grafo em dia **no commit** — mas as fases e as
> consultas acontecem antes dele. Quem implementa roda
> **`gofi index code` ao fechar** (incremental: só as árvores que mudaram), e
> antes de consultar código que acabou de mudar, para que a fase seguinte leia
> um mapa atualizado; `gofi index status` diz se o grafo ficou para trás. O
> índice de documentos não pede nada: a própria consulta o atualiza. Todo
> pacote de um contexto nasce com **`//gofi:context {contexto}`** — é o elo
> entre o símbolo e `specs/{contexto}/`.
> Protocolo em `.claude/expertise/harness-protocols/graph-retrieval.md`.

## Persistência de estado

Ao concluir uma fase, cada agent **deve atualizar**:
- `.claude/memory/contexts/{contexto}.md` — **frontmatter** (`versao`, `status`, `versao_*`, `atualizado`, `keywords`) + reescrita do `## Estado atual` + 1 linha no `## Histórico de versões` (baseline consolidada — **sem** journal datado `gofi-{nome}: {data}`). É o **único** lugar de estado por-contexto.
- `.claude/memory/project.md` **só** quando nasce um serviço/binário novo (tabela "Serviços").
- A spec em `specs/{contexto}/sdd-{contexto}.md` quando o agent é gofi-eng ou gofi-qa

> Estado por-contexto **nunca** vai para `project.md` (evita conflito de git entre devs em contextos diferentes). O índice global é gerado por `/gofi-status`. Protocolo completo em `expertise/harness-protocols/memory.md`.

## Aprendizado contínuo

Quando o usuário corrigir, ensinar ou validar algo não-óbvio, **todos os
agents aprendem juntos** — não apenas o que recebeu a correção. Procedimento
em `.claude/expertise/harness-protocols/learning.md`.

## Projeto

<!-- gofi:project:begin -->
<!-- Instruções deste projeto: convenções, comandos, avisos que valem só aqui.
     O gofi preserva este bloco em toda atualização; o resto do arquivo vem do
     gofi e é atualizado por `gofi update agents`. -->
<!-- gofi:project:end -->
