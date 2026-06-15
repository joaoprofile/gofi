---
description: Sistemas externos integrados, serviços internos, stack e restrições/premissas
topics: [{{FONTE_EXTERNA_1}}, {{FONTE_EXTERNA_2}}, {{TECNOLOGIA_1}}, {{TECNOLOGIA_2}}, stack, modelo comercial, geografia, retenção, restrições]
---

# Integrações, stack e restrições — {{NOME_DO_PRODUTO}}

## Sistemas externos integrados
> [GUIA] Fontes/destinos externos com que o produto troca dados (marketplaces,
> gateways, ERPs, APIs de terceiros...). Agrupe por tipo se ajudar.

- {{FONTE_EXTERNA_1}}
- {{FONTE_EXTERNA_2}}
- {{FONTE_EXTERNA_3}}

## Hubs / agregadores integrados
> [GUIA] Intermediários que NÃO são a fonte final mas roteiam dados. Deixe claro
> que não devem ser confundidos com a fonte. Remova se não houver.

- **{{HUB}}** ({{O_QUE_E — e o que NÃO é}})

## Serviços internos do monorepo
> [GUIA] Os serviços/módulos que compõem o sistema. Um por linha, com uma frase.

- `{{SERVICO_1}}` — {{O_QUE_FAZ}}
- `{{SERVICO_2}}` — {{O_QUE_FAZ}}
- `{{SERVICO_3}}` — {{O_QUE_FAZ}}

## Infraestrutura
> [GUIA] Componentes de infra com que os PRDs precisam contar.

- **Mensageria:** {{MENSAGERIA}}
- **Banco de dados:** {{BANCO}}
- **Cache:** {{CACHE}}

## Stack técnica (validar imutabilidade caso a caso)
> [GUIA] Linguagem, frameworks, padrões arquiteturais. O que é fixo e o que pode
> mudar por PRD.

- Backend em **{{LINGUAGEM}}** + {{FRAMEWORK}}
- {{BANCO}}, {{CACHE}}, {{MENSAGERIA}}
- Padrão arquitetural: {{PADRAO}}

## A elicitar no refinamento — sistemas não-core
> [GUIA] Categorias de ferramenta que provavelmente existem mas ainda não foram
> mapeadas. Mantenha as relevantes; o discovery pergunta qual ferramenta concreta.

- CRM de vendas ({{exemplos}}…)
- Onboarding ({{exemplos}}…)
- Product analytics ({{exemplos}}…)
- BI / dashboards ({{exemplos}}…)
- Billing / cobrança ({{exemplos}}…)
- Notificação (e-mail transacional, push, in-app)
- Observabilidade ({{exemplos}}…)

## A elicitar no refinamento — restrições e premissas
> [GUIA] Premissas assumidas como verdade que, se falsas, quebram requisitos.
> O discovery deve confirmá-las antes de virar critério de aceite.

- **Modelo comercial** ({{SaaS mensal? success fee? híbrido?}})
- **Geografia** ({{país único ou multi-região?}})
- **Limite de scale** ({{atende cliente de qualquer volume?}})
- **Política de retenção** de dados ({{quanto tempo, particionado como?}})
- **Restrições contratuais** específicas com cada fonte externa
- **Premissas operacionais** assumidas como verdade (ex.: {{"campo X sempre
  preenchido", "evento nunca chega fora de ordem", "moeda sempre Y"}})
