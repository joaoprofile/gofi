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
`.claude/skills/*.md` apenas replicam para auto-contenção.

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
| `/gofi-status` | Índice de Contextos — monta sob demanda o panorama (Implementados/Spec/PRD) lendo o frontmatter dos `contexts/*.md` |

Pipeline típico: `/gofi-pd → /gofi-spec → /gofi-eng → /gofi-qa`.
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
| Skills (slash) | `.claude/skills/<name>.md` | Portável |
| Documentação do SDK por linguagem | `.claude/sdk/<lang>/sdk-docs/*.md` | Portável |
| Boilerplates por camada | `.claude/sdk/<lang>/boilerplates/*.md` | Portável |
| Knowledge específico da linguagem | `.claude/sdk/<lang>/knowledge/*.md` | Portável |
| Knowledge cross-agent (universal) | `.claude/knowledge/shared/*.md` | Portável |
| Knowledge per-agent (`gofi train`) | `.claude/knowledge/{agent}/*.md` (criado sob demanda; hoje `eng/`, `ui/`) | Portável |
| Conhecimento institucional (negócio específico do produto/empresa) | `.claude/institutional/{project.name}/` — RAG: `INDEX.md` (sempre) + chunks sob demanda | Específico |
| Templates SDD/PRD | `.claude/specs-template/`, `.claude/prd-template/` | Portável |
| Memória global (visão, serviços, convenções) | `.claude/memory/project.md` | Específico |
| Estado por-contexto (frontmatter + histórico) | `.claude/memory/contexts/{contexto}.md` | Específico |
| Índice de contextos (gerado sob demanda) | `/gofi-status` (lê o frontmatter dos `contexts/*.md`) | Específico |
| Specs do projeto | `specs/{contexto}/sdd-{contexto}.md` | Específico |
| PRDs do projeto | `prd/{contexto}/prd-{contexto}.md` | Específico |

## Convenção de leitura dos agents

Esqueleto comum. A pré-execução **exata** vive no topo de cada skill (passos
e arquivos variam por agent); aqui fica o denominador comum:

1. Ler `.gofi.yaml` para descobrir linguagem-alvo (`project.language`), nome do projeto e demais configurações.
2. Ler `.claude/CLAUDE.md` (este arquivo) — mapa de paths físicos + doutrina das skills.
3. Ler `.claude/memory/project.md` para visão global (serviços + convenções). Para o índice de contextos existentes, rodar `/gofi-status`.
4. Ler `.claude/memory/contexts/{contexto}.md` se já houver — frontmatter (estado) + handoff de fases anteriores.
5. Ler **`.claude/knowledge/shared/*.md`** — princípios universais cross-agent (DDD, protocolo de memória, protocolo de aprendizado).
6. Ler **`.claude/knowledge/{agent}/*.md`** — knowledge user-treinado para esse agent (criado sob demanda por `gofi train`).
7. **Contexto institucional (RAG)** — quando precisar de negócio além da spec, ler `.claude/institutional/{project.name}/INDEX.md` e **só os chunks relevantes**; nunca a pasta inteira.
8. Para a linguagem-alvo, ler conteúdo language-specific:
   - `.claude/sdk/<lang>/knowledge/*.md` — regras, naming, estrutura, layers, armadilhas
   - `.claude/sdk/<lang>/sdk-docs/*.md` — API do SDK (apenas módulos relevantes)
   - `.claude/sdk/<lang>/boilerplates/*.md` — esqueletos antes de implementar (gofi-eng/qa)

## Persistência de estado

Ao concluir uma fase, cada agent **deve atualizar**:
- `.claude/memory/contexts/{contexto}.md` — **frontmatter** (`status`, `versao_*`, `atualizado`) + entry no histórico (`gofi-{nome}: {data} — ...`). É o **único** lugar de estado por-contexto.
- `.claude/memory/project.md` **só** quando nasce um serviço/binário novo (tabela "Serviços").
- A spec em `specs/{contexto}/sdd-{contexto}.md` quando o agent é gofi-eng ou gofi-qa

> Estado por-contexto **nunca** vai para `project.md` (evita conflito de git entre devs em contextos diferentes). O índice global é gerado por `/gofi-status`. Protocolo completo em `knowledge/shared/memory-protocol.md`.

## Aprendizado contínuo

Quando o usuário corrigir, ensinar ou validar algo não-óbvio, **todos os
agents aprendem juntos** — não apenas o que recebeu a correção. Procedimento
em `.claude/knowledge/shared/learning-protocol.md`.
