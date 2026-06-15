# Institutional Index — {{NOME_DO_PRODUTO}} (RAG manifest)

> **Manifesto de retrieval.** Este é o **único** arquivo institucional que a
> skill carrega **sempre**. Os demais são **chunks temáticos** carregados
> **sob demanda**, só quando o tópico do discovery casa com eles. Objetivo:
> performance e baixo consumo de tokens — não leia o que não é relevante.

## Protocolo de retrieval (como a skill usa esta pasta)

1. Carregue **só este INDEX** na pré-execução.
2. Identifique o assunto do discovery atual.
3. Pelas colunas **Tópicos** e **Carregar quando**, selecione **apenas os
   chunks relevantes** e leia esses arquivos.
4. Não carregue chunks fora do tema. Em dúvida entre 1–2 chunks próximos,
   carregue só o de maior match; expanda só se faltar contexto.

## Regra de escrita (institucional é a memória, não a skill)

- Conhecimento de negócio **específico deste produto/empresa** vive **aqui** —
  **nunca** dentro da skill `/gofi-pd` (que é genérica e portável).
- Ao aprender algo novo e durável (termo, ator, regra, integração, item de
  roadmap), grave no **chunk correto** e **registre a linha** na tabela abaixo.
- Um fato = um lugar. Não duplique entre chunks; cruze com link relativo.

## Chunks

> [GUIA] Ajuste a coluna "Tópicos (keywords)" com os termos reais do seu negócio
> — são eles que a skill usa para casar o assunto do discovery com o chunk.

| Chunk | Descrição | Tópicos (keywords) | Carregar quando |
|-------|-----------|--------------------|-----------------|
| [domain.md](domain.md) | Domínio principal, subdomínios, não-escopo | {{KEYWORDS_DOMINIO}} | Enquadrar o problema; checar se o tema está no escopo do produto |
| [glossary.md](glossary.md) | Glossário de negócio + vocabulário público vs interno | {{KEYWORDS_GLOSSARIO}} | Alinhar termos; evitar pedir definição já existente |
| [actors.md](actors.md) | Atores, personas, agents internos, multi-tenancy | {{KEYWORDS_ATORES}} | Identificar usuários/atores; tenancy/isolamento |
| [business-rules.md](business-rules.md) | Regras conhecidas: guard rails, sync, ingestão, identidade | {{KEYWORDS_REGRAS}} | Discovery que toca regra de negócio, sincronização, ingestão, identidade de dado |
| [integrations.md](integrations.md) | Sistemas externos, serviços internos, stack, restrições/premissas | {{KEYWORDS_INTEGRACOES}} | Integração externa, dependências de sistema, restrições/premissas |
| [metrics.md](metrics.md) | Métricas e KPIs a elicitar | {{KEYWORDS_METRICAS}} | Discovery que toca dashboard, relatório ou critério mensurável |
| [roadmap.md](roadmap.md) | Itens previstos (a virar plataforma, gated por QA) | {{KEYWORDS_ROADMAP}} | Tema do discovery casa com item de roadmap |

## Para portar a outro produto/empresa

Crie `.claude/institutional/{outro-produto}/` com um `INDEX.md` próprio + seus
chunks. As skills não mudam — só a pasta institucional resolvida por
`project.name` do `.gofi.yaml`.
