# Memória e aprendizado

## Protocolo de Memória

Ver `.claude/expertise/harness-protocols/memory.md` para o protocolo completo.

> **Como gravar:** `gofi memory write {contexto}` com o arquivo inteiro pela entrada
> padrão — não `Edit`/`Write`. O Claude Code trata `.claude/` como sensível e pede
> aprovação a cada edição ali; numa fase conduzida ninguém aprova, e a memória não
> fecha. O comando confere o frontmatter (`contexto`, `versao`, `status`, `keywords`)
> antes de gravar. Protocolo: `.claude/expertise/harness-protocols/memory.md`.

Ao concluir o PRD, **escreva** em `.claude/memory/contexts/{contexto}.md`:

- Domínio e subdomínios (do institucional)
- Atores principais
- Decisões de produto validadas
- Premissas e riscos relevantes
- Entrada no histórico: `gofi-pd: {data} — prd criado em {prd-path}`
- **Frontmatter** (cria se não existir): `status: prd`, `versao_prd`,
  `prd: {path}`, `servicos`, `atualizado: {data}`
- Próximo passo (na prosa): `/gofi-spec` para gerar spec

> **RAG (pós-baseline v1.0):** a memória é consolidada, não jornal. Domínio/
> atores/decisões de produto **atualizam o `## Estado atual`** da cabeça +
> uma linha no `## Histórico de versões`. **Não** apende `## gofi-pd: {data}`.
> Modelo completo em `.claude/expertise/harness-protocols/memory.md`.

O índice global é gerado por `/gofi-status` lendo esse frontmatter — **não**
registre o contexto em `project.md`. Toque `project.md` apenas se nasceu um
**serviço/binário novo** (tabela "Serviços").

**Atualização do institucional:** quando o discovery revelar conhecimento de
negócio **durável e específico do produto** (novo termo de glossário, novo ator,
nova regra conhecida, nova integração), registre-o no arquivo correto de
`.claude/institutional/{produto}/` — não na skill. A skill permanece genérica.

## Protocolo de Aprendizado Contínuo

Ver `.claude/expertise/harness-protocols/learning.md`. Roteamento do que se aprende
(teste: transferível ao segmento → skill; só vale para este cliente →
institucional):
- **Metodologia/pergunta de discovery** (genérica) → atualize esta skill (`discovery-playbook.md`).
- **Conhecimento técnico/de domínio transferível** (ex.: como funciona o
  mercado financeiro/marketplace/varejo em geral) → atualize esta skill (`discovery-playbook.md` §Conhecimento Base).
- **Conhecimento de negócio específico** de um produto/empresa (glossário, ator,
  regra, integração, roadmap) → `.claude/institutional/{produto}/` + linha no
  `INDEX.md`.
- Padrão de discovery validado e genérico → `.claude/knowledge/pd/<topico>.md`.
- Mudança no layout do PRD → `.claude/templates/prd-template.md`.
