# Protocolo de Memória — convenção universal

Cross-agent. Lido por todos os agents (gofi-pd, gofi-spec, gofi-eng, gofi-qa) ao
concluir uma fase. Define o que cada agent escreve em
`.claude/memory/contexts/{contexto}.md` para que o próximo agent receba o
handoff correto.

## Onde mora o estado do projeto (modelo sem conflito de git)

> **Regra de ouro:** todo estado **por-contexto** (status, versão, fase, histórico)
> mora **exclusivamente** em `memory/contexts/{contexto}.md` — um arquivo por
> contexto, então dois devs em contextos diferentes **nunca** escrevem no mesmo
> arquivo. `memory/project.md` guarda **só fato global de baixo churn** (visão,
> serviços, convenções) e **não** recebe mais tabelas/changelog por-contexto.
> O índice global (panorama de todos os contextos) é **derivado sob demanda**
> via a skill `/gofi-status` — nenhum agent escreve esse índice à mão.

Isso elimina o ponto de escrita compartilhado que gerava conflitos quando
devs trabalhavam em contextos diferentes no mesmo PR.

## Layout do arquivo `memory/contexts/{contexto}.md`

O arquivo **abre com um frontmatter YAML** (camada legível por máquina, lida
pelo `/gofi-status` para montar o índice) seguido do conteúdo de handoff em
markdown. **Toda fase atualiza o frontmatter** (`status`, `versao_*`,
`atualizado`) além de acrescentar sua entrada no histórico.

```markdown
---
contexto: {contexto}
servicos: [{servico}, ...]        # binários/pacotes que hospedam o contexto
status: prd | spec | em_implementacao | implementado | aprovado | reprovado
versao: "1.0"                     # baseline consolidada do CONTEXTO (nasce em 1.0)
versao_prd: "{X.Y}"               # ou n/a
versao_spec: "{X.Y}"              # ou n/a (contexto sem spec / eng-reverse)
prd: prd/{contexto}/prd-{contexto}.md      # ou n/a
spec: specs/{contexto}/sdd-{contexto}.md   # ou n/a
diretorio: services/domain/{contexto}/
keywords: [{8-14 termos kebab-case de busca — sinal de descoberta RAG}]
atualizado: {YYYY-MM-DD}
---

# {Contexto}

## Serviço
Nome: {nome-do-serviço}
Module path: {<module>}
Banco: {PostgreSQL | ...}
Porta: {8080 | n/a}
Prefixo de rotas: {/api/v1 | n/a}

## Decisões de Arquitetura
Cache: {Redis TTL=Xs em {operação} | não}
Mensageria: {publica {evento} em {tópico} via {broker} | não}
Padrões: {CQRS | Saga | Strategy | Factory | idempotência | nenhum}
Variáveis de ambiente adicionais: {lista ou "padrão"}
Integrações externas: {lista ou "nenhuma"}

## Histórico de versões  (uma linha por versão do contexto, a partir da v1)
| Versão | Data | Mudança |
|--------|------|---------|
| v1.0 | {data} | Baseline consolidada — funcionalidade em produção. |
```

> **Não** registre passo-a-passo de fase (`## gofi-eng: {data} — …`). A cada
> mudança, **reescreva o Estado atual** e adicione **uma linha** ao Histórico de
> versões (ver "Modelo de memória — baseline consolidada" abaixo). O detalhe de
> proveniência mora no git (specs/PRDs, ADRs, commits), não na memória.

## Modelo de memória — baseline consolidada + versão-forward (RAG)

> **Doutrina (a partir da v1).** A memória de um contexto **não** é um jornal
> cronológico de como se chegou ao estado atual. É a **verdade consolidada de
> hoje** (o as-built em produção) + um **changelog de versões** enxuto. Nada de
> `## gofi-eng: {data} — fix X`, nada de "melhoria Y", nada de "supersede a
> decisão da manhã". O **porquê** histórico vive no git (specs/PRDs versionados,
> ADRs, commits) — não na memória de trabalho dos agents, que paga token a cada
> leitura.

**Anatomia da cabeça `contexts/{contexto}.md`:**

```markdown
---
contexto: {contexto}
servicos: [...]
status: {prd|spec|em_implementacao|implementado|aprovado|reprovado}
versao: "1.0"                     # baseline consolidada do CONTEXTO (nasce em 1.0)
versao_prd: "{X.Y}" | n/a         # ponteiro p/ o doc PRD (versão própria dele)
versao_spec: "{X.Y}" | n/a        # ponteiro p/ o doc SDD (versão própria dele)
prd: {path} | n/a
spec: {path} | n/a
diretorio: {path}
keywords: [{8-14 termos kebab-case de busca — sinal de descoberta RAG}]
atualizado: {YYYY-MM-DD}
---
# Contexto: {contexto}

## Estado atual
{A VERDADE DE HOJE, consolidada e evergreen — arquitetura vigente, modelo de
dados, invariantes, comportamento, acoplamentos cross-context com ponteiros
[[nome]], e só as pendências GENUINAMENTE abertas. SEM datas de proveniência
("nos fixes de jun/18"), SEM narrativa de evolução, SEM "1ª geração/superado".
É **reescrito** quando o contexto muda — nunca apendado. Denso, ~50–120 linhas.}

## Histórico de versões
| Versão | Data | Mudança |
|--------|------|---------|
| v1.0 | {data} | Baseline consolidada — funcionalidade em produção. |
```

> **A memória do contexto tem regra própria de versão.** Ela não é contrato com
> consumidor: é o rastreador de estado do contexto, e o `versao` dela acompanha a
> evolução do contexto (fase concluída, spec criada, código implementado). Os
> **documentos** — PRD e spec — seguem `document-versioning.md`, onde a versão
> conta estados de produção e não edições. `versao_prd`/`versao_spec` no
> frontmatter são **ponteiros** para a versão própria de cada documento.

**Regra de escrita (a partir da v1) — mudança nova no contexto:**
1. **Reescreva o `## Estado atual`** refletindo a nova verdade (substitui o que
   mudou; não vira changelog nem cresce sem limite).
2. **Adicione uma linha** ao `## Histórico de versões` (bump `versao`: 1.0 → 1.1
   p/ melhoria, → 2.0 p/ mudança estrutural). Uma linha por versão, curta.
3. Atualize o **frontmatter** (`versao`, `status`, `versao_*`, `atualizado`).

**Como gravar:** com `gofi memory write {contexto}`, o arquivo inteiro pela
entrada padrão (`gofi memory write project` para o `project.md`). O Claude Code
trata tudo sob `.claude/` como sensível e pede aprovação a cada Edit/Write ali,
seja qual for a regra de permissão — um agente que trabalha sozinho não
conseguiria fechar. O comando é liberado no projeto e confere o frontmatter
(`contexto`, `versao`, `status`, `keywords`) antes de gravar.

**Transbordo para chunks (só quando o changelog crescer — ≳ 400 linhas na cabeça):**
o `## Histórico de versões` antigo migra para `contexts/{contexto}/history.md`
(sub-pasta; **não** varrida pelo `/gofi-status` — glob `*.md` não-recursivo — e
**sem** frontmatter), deixando na cabeça só as versões recentes + um ponteiro.
O `## Estado atual` **nunca** transborda: é sempre a cabeça. A subpasta preserva
o invariante "1 dono por contexto = zero conflito de git".

**Valores válidos de `status`** (refletem a fase mais recente concluída):

| `status` | Significado | Quem grava |
|----------|-------------|------------|
| `prd` | PRD criado, aguardando spec | gofi-pd |
| `spec` | Spec gerada, aguardando implementação | gofi-spec |
| `em_implementacao` | gofi-eng iniciou, ainda não concluiu | gofi-eng |
| `implementado` | Implementação concluída, aguardando QA | gofi-eng |
| `aprovado` | QA aprovou | gofi-qa |
| `reprovado` | QA reprovou (blockers pendentes) | gofi-qa |

## Regras de escrita por agent

| Agent | O que adiciona ao arquivo |
|-------|---------------------------|
| **gofi-pd** | Domínio + subdomínios (do contexto de negócio), atores principais, decisões de produto validadas, premissas e riscos. `Status: prd criado`. |
| **gofi-spec** | Manifesto do serviço (nome, module, banco, porta, prefixo), decisões de infraestrutura (cache, mensageria, padrões). `Status: spec criada`. |
| **gofi-eng** | Lista de arquivos criados; decisões de implementação não-óbvias (ex: usou índice composto, escolheu transação serializável). `Status: implementação concluída`. |
| **gofi-qa** | Score (`N blockers, N majors, N minors, N suggestions`); pendências ou "nenhuma". `Status: aprovado | reprovado`. |

## Memória de projeto — `memory/project.md` (global, baixo churn)

Arquivo **global**. Guarda só o que vale para o projeto inteiro e muda raramente.
**Não** contém mais tabelas por-contexto nem changelog agregado.

| Seção | Mantida por | Quando muda |
|-------|-------------|-------------|
| Visão Geral / Tecnologias | gofi-pd, gofi-spec | raro |
| Serviços | gofi-spec, gofi-eng | só quando nasce um **serviço/binário novo** (deliberado) |
| Convenções Consolidadas | qualquer agent | quando confirma um padrão recorrente do projeto |

**Nenhum agent escreve estado por-contexto aqui.** Status, versão e histórico de
cada contexto vivem em `memory/contexts/{contexto}.md` (ver acima). O panorama
de todos os contextos (o que antes eram as tabelas "Contextos Implementados /
Spec Gerada / PRD Criado") é **gerado sob demanda** pela skill `/gofi-status`,
que lê o frontmatter de todos os `contexts/*.md`.

## Índice global — `/gofi-status`

Para ver o estado de todos os contextos, rode `/gofi-status`. Ele lê o
frontmatter de cada `memory/contexts/*.md` e imprime as tabelas agrupadas por
`status`. **Não há arquivo de índice commitado** — logo, não há alvo de escrita
compartilhado e não há conflito de git entre devs em contextos diferentes.

## Regra absoluta

**Ao concluir uma fase, atualizar o frontmatter de `memory/contexts/{contexto}.md`**
(`status`, `versao_*`, `atualizado`) **e acrescentar a entrada no histórico do
mesmo arquivo.** Nunca registrar estado de contexto em `memory/project.md`.

**Antes de agir**, ler `memory/project.md` (global) + a **cabeça** de
`memory/contexts/{contexto}.md` do contexto-alvo (frontmatter + Estado atual +
Histórico de versões). O Estado atual é a verdade consolidada — normalmente
basta. Só abra `contexts/{contexto}/history.md` (se existir) quando precisar de
uma versão antiga que já transbordou da cabeça. Para o panorama geral, rodar
`/gofi-status`.

A memória é o canal de handoff entre agents. Falha em atualizar quebra a continuidade.
