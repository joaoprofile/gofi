# Contexto institucional

## Divisor skill × institucional

1. **A skill acumula expertise genérica; o institucional acumula o negócio
   específico.** O divisor é a **transferibilidade**:
   - **Pode entrar na skill:** metodologia de discovery, boas práticas e
     **conhecimento técnico / de domínio transferível** — ex.: como funciona o
     mercado financeiro, dinâmica de marketplaces, padrões de SaaS, modelagem de
     dados, GTM. É o que torna o consultor melhor em **qualquer** cliente do
     segmento. Aprendeu expertise genérica nova? Pode registrar na skill (`discovery-playbook.md`).
   - **NÃO entra na skill:** qualquer coisa específica de **um** produto/empresa/
     instituição — glossário, atores, regras de negócio próprias, integrações,
     nomes próprios, roadmap. Isso vai **sempre** para o institucional.
   - **Teste rápido:** *esse conhecimento vale para outro cliente do mesmo
     segmento? → skill. Só vale para este produto/empresa? → institucional.*
2. **O institucional é a memória do contexto específico.** Tudo que é específico
   do produto/empresa vive em `.claude/institutional/{project.name}/`. Aprendeu
   algo durável e específico (termo, ator, regra, integração, item de roadmap)?
   Grave no **chunk correto** e **registre a linha no `INDEX.md`**. Um fato = um
   lugar.

## Contexto institucional (onde mora o conhecimento específico)

O conhecimento de negócio **específico** (domínio, subdomínios, glossário,
atores/personas, regras conhecidas, integrações, métricas, restrições, roadmap)
**não vive nesta skill** — vive em `.claude/institutional/{produto}/`, organizado
como um **RAG**: um `INDEX.md` (manifesto sempre carregado) + chunks temáticos
carregados **sob demanda** por relevância. Trate-o como sua base de especialista.

**Retrieval (sempre):** carregue o `INDEX.md`, case o tema do discovery com a
coluna "Carregar quando", e leia **só os chunks relevantes**:

| Chunk institucional | Uso no discovery |
|---------------------|------------------|
| `domain.md` | Domínio, subdomínios, não-escopo → enquadramento do problema |
| `glossary.md` | Linguagem ubíqua → não pedir definição do que já está no glossário |
| `actors.md` | Atores/personas/tenancy → não re-elicitar persona já mapeada |
| `business-rules.md` | Regras conhecidas → validar só se o PRD as afeta |
| `integrations.md` | Sistemas/stack/restrições → dependências e premissas |
| `metrics.md` | Métricas a elicitar → alimenta critérios de aceite |
| `roadmap.md` | Itens previstos → antecipar dependências, evitar redundância |

**Escrita (ao aprender):** fato de negócio novo e durável vai no chunk correto +
linha registrada no `INDEX.md` (§Divisor skill × institucional, regra 2). Nunca na skill.

**Calibração pelo institucional (antes de perguntar):**
- Termo já no glossário → não peça definição.
- Ator já mapeado → não peça persona de novo.
- Regra já conhecida → valide apenas se o PRD a afeta.
- Item de roadmap relacionado → use como conhecimento prévio, não re-descubra.

> Itens de roadmap permanecem em `roadmap.md` até serem **aprovados pelo
> `/gofi-qa`**; só então migram para o arquivo institucional definitivo. PRD/
> spec/eng não disparam a transição — só a aprovação de QA.

## Carga via RAG na pré-execução

7. **Carregue o contexto institucional via RAG** (não leia a pasta inteira):
   - Leia **só** `.claude/institutional/{project.name}/INDEX.md` — o manifesto de
     retrieval (chunks + descrição + tópicos + "carregar quando").
   - Identifique o assunto do discovery e carregue **apenas os chunks relevantes**
     (`domain.md`, `glossary.md`, `actors.md`, `business-rules.md`,
     `integrations.md`, `metrics.md`, `roadmap.md`) conforme o INDEX. É daqui que
     você vira especialista no negócio concreto — lendo só o que importa.
   - Se a pasta **não existir**, opere em **modo discovery puro** (só a
     metodologia genérica deste arquivo) e **ofereça bootstrapar** a pasta
     institucional ao final (criar `INDEX.md` + chunks), registrando o que
     descobriu em `.claude/institutional/{project.name}/`.

## Calibração pelo contexto institucional

Antes de perguntar, verifique os arquivos de `.claude/institutional/{produto}/`
(§Contexto institucional): glossário, atores, regras e roadmap já cobrem boa parte — não re-elicite o
que já está documentado; valide apenas o que o PRD afeta.
