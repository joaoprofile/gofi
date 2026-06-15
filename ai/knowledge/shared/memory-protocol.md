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
versao_prd: "{X.Y}"               # ou n/a
versao_spec: "{X.Y}"              # ou n/a (contexto sem spec / eng-reverse)
prd: prd/{contexto}/prd-{contexto}.md      # ou n/a
spec: specs/{contexto}/sdd-{contexto}.md   # ou n/a
diretorio: services/domain/{contexto}/
atualizado: {YYYY-MM-DD}
---

# {Contexto}

## Serviço
Nome: {nome-do-serviço}
Module path: {db1.com.br/...}
Banco: {PostgreSQL | ...}
Porta: {8080 | n/a}
Prefixo de rotas: {/api/v1 | n/a}

## Decisões de Arquitetura
Cache: {Redis TTL=Xs em {operação} | não}
Mensageria: {publica {evento} em {tópico} via {broker} | não}
Padrões: {CQRS | Saga | Strategy | Factory | idempotência | nenhum}
Variáveis de ambiente adicionais: {lista ou "padrão"}
Integrações externas: {lista ou "nenhuma"}

## Histórico de agentes  (uma linha por evento, cronológico)
- gofi-pd:   {data} — prd criado em {prd-path}
- gofi-spec: {data} — spec v{X.Y} criada em {spec-path}
- gofi-eng:  {data} — implementação concluída ({resumo})
- gofi-qa:   {data} — auditoria concluída ({score})
```

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

**Antes de agir**, ler `memory/project.md` (global) + o `memory/contexts/{contexto}.md`
do contexto-alvo. Para o panorama geral, rodar `/gofi-status`.

A memória é o canal de handoff entre agents. Falha em atualizar quebra a continuidade.
