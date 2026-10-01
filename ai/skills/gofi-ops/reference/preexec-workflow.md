# Pré-execução e workflow detalhados

## Pré-execução obrigatória

Antes de qualquer recurso:

1. Ler `.gofi.yaml` (raiz) — extrair `project.name`, `project.language` e o
   bloco **`ops:`**:
   ```yaml
   ops:
     cloud: <oci|aws|gcp|azure|...>        # provedor alvo
     iac: <terraform|opentofu|pulumi>      # ferramenta de IaC
     target: <k8s|oke|eks|gke|swarm|container-instances|paas>  # runtime de deploy
     cicd: <github-actions|azure-devops|gitlab-ci|oci-devops>  # plataforma de pipeline
     registry: <ocir|ecr|gar|acr|...>      # registry de imagem
     path: ops                             # pasta guarda-chuva na raiz
   ```
   Se o bloco `ops:` **não existir**, **pare e peça ao usuário** para
   configurá-lo — não infira cloud/ferramenta/runtime.
2. O `AGENTS.md` da raiz (já carregado) — mapa de paths físicos do projeto.
3. Ler `.claude/memory/project.md` — **inventário de serviços/binários**
   (o que precisa ser empacotado e deployado) + convenções. Índice de
   contextos via `/gofi-status`.
4. Ler a **spec de infra/plataforma**. Ache por busca, não por glob de caminho:
   `gofi find "<tópico de infra>"` ou `gofi show ctx:infra` — o nome do
   contexto de infra varia por projeto, e adivinhar o caminho falha em silêncio
   — **fonte da verdade da topologia**: recursos a provisionar, sizing,
   rede/sub-redes, ambientes (dev/staging/prod), política de secrets,
   domínios/DNS, estratégia de migração do que já existe. Se a spec **não
   existir**, **pare**: topologia é decisão de spec, não se infere. Ofereça
   rodar `/gofi-spec` para a infra (ou elicitar e escrever a spec primeiro).
5. Ler **knowledge cross-agent**: `.claude/knowledge/INDEX.md` (núcleo ⬤ + só os módulos que a tarefa pede) (inclui
   `.claude/expertise/diagramming/conventions.md` — diagramas de arquitetura/topologia em
   ADR/README devem ser PlantUML).
6. Ler **knowledge per-agent**: `.claude/knowledge/ops/*.md` (user-treinado,
   se existir).
7. Para a stack do bloco `ops:` (`iac` + `cloud` + `target` + `cicd`), ler o
   conteúdo tool/cloud-specific **se existir**:
   - os módulos de `sdk/<iac>/knowledge/` que o `.claude/knowledge/INDEX.md` indicar — regras, estrutura de módulos,
     naming, state, armadilhas da ferramenta de IaC
   - `.claude/sdk/<iac>/boilerplates/*.md` — esqueletos de módulo/root/pipeline
   - Se o conteúdo não existir ainda, é **gap de curadoria**: gere com base
     nas regras universais de `iac-principles.md` e registre o aprendizado (ver `.claude/expertise/harness-protocols/learning.md`)
     para popular `.claude/sdk/<iac>/`.
8. **Mapear o estado atual de build/deploy do repo** — Dockerfiles,
   Makefile(s) de build, descritores de deploy, mecanismo vigente (PaaS,
   compose, script). É a **base de referência**: o novo formato **coexiste**
   com o atual e **nunca o substitui/remove sem spec de migração explícita**.
9. **Perguntar onde está a infra/deploy legado** sempre que a tarefa for
   migração ou formalização de algo que já roda manualmente (igual
   `gofi-eng` pede o código legado). Leia o setup atual antes de gerar.
10. Confirmar ambiguidades com o dev **antes** de provisionar.

> Se a spec for ambígua, contradizer um princípio inviolável (`.claude/expertise/platform-delivery/iac-principles.md`), ou não
> declarar ambientes/secrets/rede, **pare e pergunte**. Nunca infira
> topologia, credencial, região ou conta.

> **Execução sempre step by step.** Trabalhe em passos pequenos e
> verificáveis (`validate`/`plan`/`diff` por etapa), confirmando cada um com
> o dev antes de seguir — especialmente em migração. Não despeje a infra
> inteira de uma vez.

## Workflow

```
1. Ler spec de infra → inventariar recursos, ambientes, rede, secrets, domínios
2. Mapear o estado atual (build/deploy vigente) → o que coexiste, o que migra,
   o que sai (só com corte declarado na spec)
3. Desenhar a topologia (diagrama PlantUML no README) → validar com o dev
4. IaC bottom-up, em módulos:
   a. global/ — backend de state, identity, registry, dns topo (bootstrap)
   b. modules/ — uma capability por vez (network → cluster → db → cache →
      messaging → observability → dns → secrets)
   c. envs/dev — compor módulos + tfvars; `validate` → `plan` → revisar
5. Empacotamento: Dockerfile por serviço + script de build no CI (artefato
   por SHA, sem binário no git)
6. CD: manifests/helm base + overlays por ambiente; imagem referenciada por SHA
7. CI/CD: pipeline fino que chama ci/ e deploy/ (lint→test→scan→build→
   plan→apply→deploy→migrate); gates por ambiente
8. Secrets: provisionar secret store + wiring; nada versionado
9. Aplicar incremental: dev primeiro, `plan` aprovado por etapa; staging/prod
   só após validação
10. Atualizar memória e spec (ver `output-memory.md`)
```

A ordem é guia, não rígida — ajuste se a spec exigir.
