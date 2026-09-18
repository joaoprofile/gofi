# CLAUDE.md — projeto gofi

Arquivo lido por Claude Code antes de qualquer interação. Documenta como
Claude lê o conteúdo do projeto e quais skills estão
disponíveis. Regras de código, padrões de SDK e armadilhas conhecidas vivem
em arquivos cross-AI sob `.claude/sdk/<lang>/knowledge/` e
`.claude/knowledge/shared/` — Claude consome via os agents.

> **Modelo de duas camadas** (v2.5+): `.claude/` é a camada de decisão da IA
> — markdown destilado (agents, knowledge, sdk-docs, boilerplates).
> `.gofi/gofi-sdk-<lang>/` é a camada de execução do toolchain (código real
> do SDK, importável pelo módulo do projeto via `go.work`); os agents
> normalmente NÃO leem daí. Se um agent precisou abrir código real para
> decidir, é gap de curadoria — atualizar `.claude/sdk/<lang>/`.

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
> `.claude/institutional/`. Padrão técnico genérico vive em `.claude/knowledge/`
> e `.claude/sdk/<lang>/`, sempre **domínio-neutro** (placeholders `{contexto}`,
> `<module>`, `RoleA`, `entity`).
>
> **Teste de pertencimento:** *serviria, sem mudar uma palavra, a outro projeto
> com o mesmo SDK? → skill/knowledge. Só vale aqui? → spec/memória/institucional.*

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
| `/gofi-migrate` | Corpus Migration — traz specs, PRDs e memória de um formato antigo para o atual (facetas, índice em dois níveis, grafo); orquestra `gofi docs migrate` |
| `/gofi-status` | Índice de Contextos — monta sob demanda o panorama (Implementados/Spec/PRD) lendo o frontmatter dos `contexts/*.md` |
| `/gofi-full` | Full-Cycle Orchestrator — encadeia `gofi-pd → gofi-spec → gofi-eng → gofi-qa` em fluxo contínuo, volta à fase anterior quando reprova e segue até o QA aprovar sem ressalvas |

Pipeline típico: `/gofi-pd → /gofi-spec → /gofi-eng → /gofi-qa` (ou `/gofi-full` para o ciclo inteiro orquestrado).
Camada de apresentação: `/gofi-ui` após a spec (e o contrato do `gofi-eng`).
Plataforma/infra: `/gofi-spec` (infra) → `/gofi-ops`.
Doc de contrato: abrir o handler no IDE → `/gofi-doc`.

## Onde os agents leem o conteúdo

Coluna **Natureza** = lado da [doutrina](#doutrina-das-skills-o-que-entra-e-o-que-não-entra):
*Portável* (genérico, viaja entre projetos do mesmo SDK) vs *Específico* (só
vale neste projeto — nunca entra em skill).

| Recurso | Caminho no projeto | Natureza |
|---------|--------------------|----------|
| Configuração do projeto | `.gofi.yaml` (raiz) | Específico |
| Skills (slash) | `.claude/skills/<name>/SKILL.md` — pasta por skill, com `name` e `description` no frontmatter; é o único layout que o Claude Code descobre | Portável |
| Documentação do SDK por linguagem | `.claude/sdk/<lang>/sdk-docs/*.md` | Portável |
| Boilerplates por camada | `.claude/sdk/<lang>/boilerplates/*.md` | Portável |
| Knowledge específico da linguagem | `.claude/sdk/<lang>/knowledge/` — **carregue pelo `knowledge/INDEX.md`**, nunca por glob | Portável |
| **Manifesto do conhecimento portável (carregue por aqui)** | `.claude/knowledge/INDEX.md` — núcleo ⬤ + sob demanda. Derivado: `gofi docs build` | Portável (derivado) |
| Knowledge cross-agent (universal) | `.claude/knowledge/shared/` — idem: pelo `INDEX.md` | Portável |
| Knowledge per-agent (`gofi train`) | `.claude/knowledge/{agent}/*.md` (criado sob demanda; hoje `eng/`, `ui/`) | Portável |
| Conhecimento institucional (negócio específico do produto/empresa) | `.claude/institutional/{project.name}/` — RAG: `INDEX.md` (sempre) + chunks sob demanda. **Espelho pull-only** do repo `sources.institutional` quando configurado (atualizado por `gofi institutional update`, que **substitui a pasta por completo**); **sem repo**, mantido à mão no git do projeto | Específico |
| Templates SDD/PRD | `.claude/templates/` (`sdd-template.md`, `prd-template.md`) | Portável |
| Memória global (visão, serviços, convenções) | `.claude/memory/project.md` | Específico |
| Estado por-contexto (cabeça: frontmatter + Estado atual consolidado + Histórico de versões) | `.claude/memory/contexts/{contexto}.md` | Específico |
| Versões antigas transbordadas (só quando o changelog cresce) | `.claude/memory/contexts/{contexto}/history.md` | Específico |
| Índice de contextos (gerado sob demanda) | `/gofi-status` (lê o frontmatter dos `contexts/*.md`) | Específico |
| Specs do projeto | `specs/{contexto}/sdd-{contexto}.md` | Específico |
| PRDs do projeto | `prd/{contexto}/prd-{contexto}.md` | Específico |
| Índice de retrieval de specs/PRDs (2 níveis) | `specs/INDEX.md` (roteador de contextos) + `specs/{contexto}/INDEX.md` (shard); idem `prd/`. Derivado: `gofi docs build` | Específico (derivado) |
| **Busca e navegação no corpus** | `gofi find` — pergunta e devolve doc/§/linhas; `--entity`, `--context`, `--links`, `--orphans`, `--hubs` | Específico (derivado) |
| Grafo de documentos | `.gofi/docs/graph.json` — `declara`/`cita`/`wikilink`/`toca`/`pertence`/`implementa`. Derivado: `gofi docs build` | Específico (derivado) |
| Léxico controlado das facetas | `.claude/lexicon/*.md` — `entidades` (do schema), `operacoes`, `marketplaces`, `sinonimos` (ponte entre a língua da pergunta e a do identificador) | Específico |
| Protocolo de retrieval (ler os corpora gastando poucos tokens) | `.claude/knowledge/shared/rag-retrieval-protocol.md` | Portável |
| Grafo de código (mapa derivado: quem chama quem, a que contexto pertence) | `.gofi/graph/` — **um escopo por árvore**: `gofi_graph_index.json` na raiz lista todos (backend em `.`, cada superfície de UI em `{nome}/`, SDK em `sdk/`), e cada escopo tem o seu `gofi_graph_report.md`. Consulta por `gofi graph explain`. Gerado por `gofi graph build`, **nunca** editado à mão | Específico (derivado) |
| Protocolo de consulta ao grafo | `.claude/knowledge/shared/graph-retrieval-protocol.md` | Portável |

## Convenção de leitura dos agents

Esqueleto comum. A pré-execução **exata** vive no topo de cada skill (passos
e arquivos variam por agent); aqui fica o denominador comum:

1. Ler `.gofi.yaml` para descobrir linguagem-alvo (`project.language`), nome do projeto e demais configurações.
2. Ler `.claude/CLAUDE.md` (este arquivo) — mapa de paths físicos + doutrina das skills.
3. Ler `.claude/memory/project.md` para visão global (serviços + convenções). Para o índice de contextos existentes, rodar `/gofi-status`.
4. Ler `.claude/memory/contexts/{contexto}.md` se já houver — frontmatter (estado) + handoff de fases anteriores.
5. Ler **`.claude/knowledge/INDEX.md`** — manifesto das camadas portáveis. Carregar o **núcleo ⬤** (curto, universal) e **só os módulos que a tarefa pede**. **Nunca** carregar `knowledge/shared/*.md` nem `sdk/<lang>/knowledge/*.md` por glob: é o maior custo fixo do harness, maior que a spec do contexto.
6. Ler **`.claude/knowledge/{agent}/*.md`** — knowledge user-treinado para esse agent (criado sob demanda por `gofi train`).
7. **Contexto institucional (RAG)** — quando precisar de negócio além da spec, ler `.claude/institutional/{project.name}/INDEX.md` e **só os chunks relevantes**; nunca a pasta inteira.
8. Para a linguagem-alvo, carregar **pelo mesmo `.claude/knowledge/INDEX.md`** (ele
   já cobre `sdk/<lang>/`), nunca por glob:
   - `sdk/<lang>/knowledge/` — regras, naming, estrutura, layers, armadilhas
   - `sdk/<lang>/sdk-docs/` — API do SDK (só os módulos relevantes)
   - `sdk/<lang>/boilerplates/` — esqueletos, antes de implementar (gofi-eng/qa)

> **Procurar documento é `gofi find`, não carregar índice.** O corpus tem camada
> de consulta, simétrica ao `gofi graph explain` do código: pergunte, não carregue.
> `gofi find "<pergunta>"` devolve doc, §seção e **faixa de linhas** — leia com
> `Read(offset, limit)`. Vazio quase sempre é vocabulário, não documento faltando:
> tente o termo técnico e **registre o par que faltou** em
> `.claude/lexicon/sinonimos.md`. Os `INDEX.md` servem para **navegar** um
> contexto, não para **achar** um assunto; `grep -r` é fallback declarado.
>
> `gofi find --entity <tabela>` responde *quem documenta* e *quem implementa* de
> uma vez — a tabela é o único símbolo que existe nos dois mundos.
> `gofi find --links <doc>` é a análise de impacto barata **antes** de editar.
>
> **Specs/PRDs/contextos são RAG — gaste poucos tokens.** Regras de **leitura e criação** em `.claude/knowledge/shared/rag-retrieval-protocol.md`.
> - **Ler:** nunca leia um doc inteiro por reflexo. `gofi find "<pergunta>"` devolve doc, §seção e faixa de linhas — leia com `Read(offset, limit)`.
> - **Criar/editar:** base nos templates `.claude/templates/` — `formato:` + as facetas (`entidades` do schema, `operacoes`/`marketplaces` do `.claude/lexicon/`) + `keywords` com o que sobrou (teto 12). **Zero proveniência** (sem `**Autor/Versão/Data:**`, sem `## Rastreabilidade`, sem nome de agent/pessoa, sem journal), Histórico de **1 linha**. Ao fechar: `gofi docs build` e `gofi docs validate` — termo de faceta fora do léxico reprova, e é isso que impede o vocabulário de voltar a ser nuvem de tags.

> **O código também tem índice — o grafo. Procurar código é `gofi graph`, não `grep -r`.**
> Suba a escada de 3 degraus: `gofi_graph_index.json` (que escopos existem, em
> que pasta, **em que modo**) → o `gofi_graph_report.md` **do escopo** →
> `gofi graph explain <símbolo>`. Não sabe o nome exato? `gofi graph explain
> <termo> <termo>` busca dentro do grafo. `grep -r` é **fallback declarado**:
> linguagem sem extractor, ou alvo que não é símbolo (string, chave de config,
> SQL). **Nunca** leia `gofi_graph.json`. Isto vale para **código**; navegar
> spec/PRD por `grep -n '^## '` é o protocolo RAG de documento e continua valendo.
>
> **Valide o modo antes de concluir ausência.** Os hooks de git reconstroem
> **sempre em `fast`**; `init` e `update`, no modo do `.gofi.yaml`
> (`graph: deep:`), que **por padrão também é `fast`** — e em `fast` chamada
> ambígua não vira aresta, então ausência de aresta não é prova de ausência de
> uso. `deep` é sempre pedido explícito: precisa afirmar "isto não quebra nada",
> "ninguém mais chama", "quem implementa esta interface"? Rode
> `gofi graph build --deep` ou declare a limitação. Os gatilhos estão em
> *Quando rodar `--deep`* do protocolo do grafo.
>
> Quem implementa roda **`gofi graph build --update` ao fechar**, para que a
> fase seguinte leia um mapa atualizado. Todo pacote de um contexto nasce com
> **`//gofi:context {contexto}`** — é o elo entre o símbolo e `specs/{contexto}/`.
> Protocolo em `.claude/knowledge/shared/graph-retrieval-protocol.md`.

## Persistência de estado

Ao concluir uma fase, cada agent **deve atualizar**:
- `.claude/memory/contexts/{contexto}.md` — **frontmatter** (`versao`, `status`, `versao_*`, `atualizado`, `keywords`) + reescrita do `## Estado atual` + 1 linha no `## Histórico de versões` (baseline consolidada — **sem** journal datado `gofi-{nome}: {data}`). É o **único** lugar de estado por-contexto.
- `.claude/memory/project.md` **só** quando nasce um serviço/binário novo (tabela "Serviços").
- A spec em `specs/{contexto}/sdd-{contexto}.md` quando o agent é gofi-eng ou gofi-qa

> Estado por-contexto **nunca** vai para `project.md` (evita conflito de git entre devs em contextos diferentes). O índice global é gerado por `/gofi-status`. Protocolo completo em `knowledge/shared/memory-protocol.md`.

## Aprendizado contínuo

Quando o usuário corrigir, ensinar ou validar algo não-óbvio, **todos os
agents aprendem juntos** — não apenas o que recebeu a correção. Procedimento
em `.claude/knowledge/shared/learning-protocol.md`.
