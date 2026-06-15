---
description: Itens de roadmap previstos (a virar plataforma, gated por aprovação de QA)
topics: [roadmap, {{ITEM_ROADMAP_1}}, {{ITEM_ROADMAP_2}}, {{ITEM_ROADMAP_3}}]
---

# Roadmap (a virar plataforma) — {{NOME_DO_PRODUTO}}

> Itens previstos para o produto. Quando o usuário pedir PRD/discovery sobre um
> destes temas, este é o conhecimento prévio — use para calibrar perguntas,
> antecipar dependências e evitar redundância.
>
> **Transição roadmap → plataforma:** um item permanece nesta lista até ser
> **aprovado pelo `/gofi-qa`**. Só então mova-o daqui para o lugar correto nos
> demais arquivos institucionais (subdomínios em [domain.md](domain.md),
> integrações em [integrations.md](integrations.md), regras em
> [business-rules.md](business-rules.md), atores em [actors.md](actors.md) etc.)
> e remova daqui. PRD criado, spec criada e eng implementado **não disparam** a
> transição — só a aprovação de QA dispara.

> [GUIA] Um bloco `##` por item de roadmap. Sugestão de campos por item abaixo —
> use os que fizerem sentido. O campo "A elicitar no PRD" é o mais importante:
> são as perguntas que o discovery deve fazer quando o tema vier à tona.

## {{ITEM_ROADMAP_1}}

- **Forma:** {{COMO_SERA_CONSTRUIDO / abordagem}}.
- **Problema que resolve:** {{DOR_DO_USUARIO}}.
- **Escopo inicial (MVP):** {{O_QUE_ENTRA}}.
- **Escopo futuro (não-MVP):** {{O_QUE_FICA_PARA_DEPOIS}}.
- **Novo ator (se houver):** {{ATOR}} — {{O_QUE_FAZ}}.
- **Dependências:** {{DO_QUE_DEPENDE}}.
- **A elicitar no PRD:** {{PERGUNTAS_EM_ABERTO}}.

## {{ITEM_ROADMAP_2}}

- **Forma:** {{...}}.
- **A elicitar no PRD:** {{...}}.

## {{ITEM_ROADMAP_3}}

- **Status:** {{ex.: PRD criado + spec vX + eng implementado (AAAA-MM-DD); aguardando /gofi-qa}}.
- **Decisões já consolidadas** (não re-elicitar): {{...}}.
- **A elicitar nos PRDs derivados:** {{...}}.
