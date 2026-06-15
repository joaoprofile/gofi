---
description: Atores humanos, atores não-humanos (workers/agents/integrações) e multi-tenancy
topics: [persona, {{PERSONA_1}}, {{PERSONA_2}}, agent, worker, tenancy, isolamento, {{ENTIDADE_TENANT_RAIZ}}, {{ENTIDADE_TENANT_FILHA}}]
---

# Atores e personas — {{NOME_DO_PRODUTO}}

## Humanos
> [GUIA] Quem usa o produto. Inclua personas externas (clientes) e internas
> (operação/CS). Para cada uma: o que faz no produto e qual o objetivo principal.

| Ator | Responsabilidade | Objetivo principal |
|------|------------------|--------------------|
| **{{PERSONA_1}}** | {{O_QUE_FAZ}} | {{OBJETIVO}} |
| **{{PERSONA_2}}** | {{O_QUE_FAZ}} | {{OBJETIVO}} |
| **{{PERSONA_3}}** | {{O_QUE_FAZ}} | {{OBJETIVO}} |
| **Operador Interno (CS / Suporte / Onboarding)** | {{O_QUE_FAZ_PELO_LADO_DA_EMPRESA}} | {{OBJETIVO}} |

## Não-humanos (workers internos, agents, receivers)
> [GUIA] Atores automáticos: workers assíncronos, jobs, agents, receivers de
> webhook, integrações que agem sozinhas. Remova esta seção se o produto não
> tiver processamento assíncrono relevante.

| Ator | Responsabilidade |
|------|------------------|
| **{{WORKER_1}}** | {{O_QUE_FAZ}} |
| **{{WORKER_2}}** | {{O_QUE_FAZ}} |
| **{{RECEIVER_DE_EVENTOS}}** | {{RECEBE_O_QUE_E_PRODUZ_O_QUE}} |

**Padrão arquitetural:** {{DESCREVA_COMO_OS_WORKERS_SAO_ACIONADOS}}
> [GUIA] Ex.: "Todos os workers podem ser acionados por scheduler OU webhook; o
> receiver produz mensagem e os workers consomem assincronamente." Ajuste à sua
> arquitetura (síncrono, fila, cron, event-driven...).

## Hierarquia / multi-tenant
> [GUIA] Como os dados são particionados por cliente. Defina a entidade-raiz do
> tenant, suas filhas, e a regra de isolamento. Remova se for single-tenant.

- Estrutura: `{{ENTIDADE_TENANT_RAIZ}} → N {{ENTIDADE_TENANT_FILHA}}` ({{REGRA_DE_CARDINALIDADE}})
- **Isolamento total por {{ENTIDADE_TENANT_RAIZ}}** — dados de um(a) {{ENTIDADE_TENANT_RAIZ}} NUNCA podem vazar para outro(a).
- **Admin {{NOME_DO_PRODUTO}}** tem acesso cross-tenant ({{ESCOPO_DO_ACESSO_ADMIN}}).
