# Protocolo de Aprendizado Contínuo — cross-agent

Quando o usuário corrigir, ensinar ou validar algo não-óbvio, **todos os
agents devem aprender** — não apenas o que recebeu a correção. Esta é a
regra fundamental que mantém o sistema coerente ao longo do tempo.

---

## Regra absoluta — knowledge é domínio-neutro

> Arquivos sob `.claude/knowledge/` (`shared/` e per-agent
> `pd|spec|eng|qa|ui/`) e sob `.claude/sdk/<lang>/` descrevem **padrão
> técnico** — como usar o SDK, como estruturar código, como elicitar,
> como auditar. **Nunca** carregam estado de domínio do projeto que
> está consumindo este toolchain.

**Não pode aparecer em knowledge:**

- Nomes de entidades do produto (ex.: `pool`, `order`, `bettor`,
  `balance_movement`, `invoice`, qualquer entidade de negócio que não seja
  `tenant`/`user` enquanto padrão SaaS universal).
- Roles, perfis ou hierarquias concretas do produto (ex.: `ADMIN`,
  `GERENTE`, `ATENDENTE`) — usar placeholders `RoleA`/`RoleB` quando
  precisar exemplificar.
- Termos do domínio em qualquer idioma (ex.: "bolões", "apostadores",
  "movimentação contábil").
- Module paths concretos (`github.com/<org>/<projeto>/...`) — usar
  `<module>/...` como placeholder.
- Referências a versões ou ADRs de specs específicas (ex.: "ADR-06 da
  spec tenant v1.5", "RN-14 do user v1.3").
- Endpoints concretos do produto (ex.: matriz fixa de quem acessa
  `/pools`, `/orders`, `/bettors`).
- Decisões que valem para um único contexto/serviço deste projeto.

**Pode (e deve) aparecer em knowledge:**

- Padrões do SDK e como compô-los (`sqln`, `iam`, `environment`, etc.).
- Estrutura de pastas, naming, regras absolutas, anti-padrões da linguagem.
- Princípios cross-agent (DDD, memória, este protocolo, UUIDs em
  identidades, clean code).
- Placeholders e exemplos genéricos (`{contexto}`, `<module>`, `RoleA`,
  `audit_log`, `entity`).

**Onde vai conteúdo de domínio:**

| Tipo | Lugar |
|------|-------|
| Entidades, regras de negócio, RNs, ciclo de vida | `specs/{contexto}/sdd-{contexto}.md` |
| Fato global do projeto (serviços/binários, convenções) | `.claude/memory/project.md` |
| Estado por-contexto (status, versão, fase, histórico, decisões da fase, gotchas) | `.claude/memory/contexts/{contexto}.md` (frontmatter + histórico) — índice via `/gofi-status` |
| PRD do contexto | `prd/{contexto}/prd-{contexto}.md` |

**Teste antes de escrever em knowledge:**

> "Este texto serviria, sem alteração, a um projeto totalmente diferente
> que use o mesmo SDK?"

Se não serviria — mova o trecho para `specs/` ou `.claude/memory/`. Se
serviria com troca de placeholder — generalize antes de salvar.

---

## Tabela de propagação

| Situação | Arquivos a atualizar |
|----------|----------------------|
| Correção de uso de API do SDK | `sdk/<lang>/knowledge/<modulo>.md` ou `sdk/<lang>/sdk-docs/<modulo>.md` + boilerplate relevante |
| Novo padrão de código ensinado | `sdk/<lang>/knowledge/<topico>.md` + boilerplate relevante |
| Correção nas perguntas de elicitação (gofi-spec) | `agents/gofi-spec/agent.md` + `specs-template/sdd-template.md` se necessário |
| Correção na geração de código | `agents/gofi-eng/agent.md` + boilerplate relevante |
| Correção no checklist de QA | `agents/gofi-qa/agent.md` + `sdk/<lang>/knowledge/qa-checklist.md` |
| Novo padrão de projeto solicitado | Pergunte exemplo → `sdk/<lang>/boilerplates/` + `sdk/<lang>/knowledge/` + agent skill |
| Decisão de arquitetura validada (cross-lang) | `knowledge/shared/<topico>.md` |
| Decisão de arquitetura validada (lang-specific) | `sdk/<lang>/knowledge/<topico>.md` |
| Agent precisou ler código real em `.gofi/gofi-sdk-<lang>/` para decidir | `sdk/<lang>/knowledge/<topico>.md` (registre o aprendizado para o próximo agent não precisar do mesmo desvio) |

## Sequência obrigatória

1. **Identifique o escopo.** A lição é cross-AI? cross-language? lang-specific? Específica de um agent?
2. **Atualize o arquivo mais específico primeiro** (knowledge concreto antes da skill genérica).
3. **Atualize a skill do agent responsável** (`agents/<name>/agent.md`) se a regra precisa ser citada explicitamente.
4. **Se afeta múltiplos agents**, propague para todos os arquivos relevantes.
5. **Confirme ao usuário** a lista exata de arquivos atualizados.

## Regra do exemplo

Se o usuário pedir algo específico ainda não documentado:

1. **Pergunte por um exemplo concreto** antes de implementar.
2. Com o exemplo validado, atualize `sdk/<lang>/boilerplates/` e/ou `sdk/<lang>/knowledge/` ou `knowledge/shared/`.
3. **Só então escreva o código** — assim os agents futuros já conhecem o padrão.

Esse ciclo "pergunta → documenta → implementa" é o que evita que o mesmo
erro reapareça em projetos futuros.
