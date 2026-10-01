# Regras de escrita do PRD

## Regra de Ouro (prioritária)

> **PRD é problema, negócio e intenção. Spec é solução técnica. Não invada o
> território da spec.**

- **PRD contém apenas dados de negócio/intenção**: problema, atores, regras,
  fluxos, critérios de aceite, métricas, escopo.
- **Spec (gerada pelo `/gofi-spec`) contém o "como" técnico**: pacotes, paths,
  libs, SQL, migrations, código, performance budgets, mapeamento campo-a-campo.
- **Mesmo quando o usuário trouxer informação técnica** (snippets, migrations,
  nomes de tabela, código pronto, escolha de lib, paths, índices, constraints),
  **NÃO copie isso para o PRD**. Extraia apenas a **visão de modelagem em nível
  de negócio**:
  - Entidades, agregados, value objects, eventos de domínio (nome lógico +
    finalidade)
  - Relacionamentos conceituais (1↔N, soft join, materialização assíncrona) —
    sem FK/constraint/índice
  - Invariantes e regras que protegem o agregado
  - Linguagem ubíqua (alinhada ao glossário institucional)
- **Tudo que for "como construir"** (tipo SQL, nome de constraint, biblioteca,
  decomposição de latência, estratégia de cache) **vai para a spec**.
- Em dúvida, pergunte: *"isso muda se trocarmos a stack? se mudar, é técnico —
  fora do PRD"*.

Esta regra **prevalece sobre qualquer outra orientação de estilo** neste
arquivo. Se o usuário insistir em incluir detalhe técnico no PRD, ofereça
registrá-lo como nota para o `/gofi-spec` consumir depois.

> **Discovery não-software** (processo, operação, marketing): a Regra de Ouro
> se aplica analogamente — o PRD descreve **o quê e por quê**; o **como
> executar** (ferramenta, canal, automação específica) é detalhamento posterior,
> não entra no corpo do PRD.

## Estrutura do PRD (Saída Esperada)

O PRD final **deve seguir exatamente** o layout de
`.claude/templates/prd-template.md`.

### Regras de geração

- **Copie o template** `.claude/templates/prd-template.md` como base.
- Salve em `{pathPrd}/{contexto}/prd-{contexto}.md`.
- **Mantenha todas as 19 seções** — preserve títulos, numeração e tabelas.
- Seções **(Opcional)** podem ser omitidas se não se aplicarem; documente a razão
  ou remova a seção.
- Nunca invente seções fora do template — extra vai em §17 (Considerações
  Técnicas) ou §19 (Anexos).

### Escrita RAG do PRD

**Leia `.claude/templates/prd-template.md`** — layout obrigatório do PRD (já no formato RAG: frontmatter + `keywords`, **sem** `**Autor/Data:**`/Rastreabilidade/Histórico). Ao gerar o PRD, siga a seção *Escrita* de `.claude/expertise/harness-protocols/rag-retrieval.md`: frontmatter + `keywords` (8–14 termos de busca), **zero proveniência/rastro de agent/nome de pessoa**, e **regenere** `prd/INDEX.md` (`gofi index docs`) ao criar/renomear. Para descobrir PRDs existentes, consulte `prd/INDEX.md` — não varra a pasta.

### Estilo — direto, sem invadir spec

Aplicação prática da **Regra de Ouro (§Regra de Ouro, acima)**. PRD é **fonte para `/gofi-spec`**,
não a spec. O **valor do PRD é clareza de negócio + regras + critérios**.

**Filtro de extração — quando o input vier técnico:**

| Entrada técnica do usuário | O que vai pro PRD (negócio) | O que NÃO vai (fica pra spec) |
|----------------------------|-----------------------------|--------------------------------|
| Migration `CREATE TABLE foo (...)` | "`foo` — entidade que guarda X por cliente" | tipos SQL, índices, `uq_*`, `DEFAULT` |
| Snippet `func Calculate(...) ...` | "o motor avalia a regra e produz o resultado" | nome de função, lib, assinatura |
| `services/domain/x/service/y.go` | "serviço de domínio Y dentro do contexto X" | path do arquivo, nome do pacote |
| FK `REFERENCES bar(id)` | "A referencia B via junction concreta" | `FOREIGN KEY`, `ON DELETE`, constraint |
| Estratégia de cache | (nada — é decisão técnica) | TTL, chave, invalidação |

**Regra simples:** se trocar a stack muda a frase? Então é técnico — não entra no PRD.

**Em §17 (Considerações Técnicas):** nível de intenção (dependências cross-context,
integrações, padrão arquitetural em uma frase). Sem paths, nomes de pacote,
snippets, libs específicas, performance budgets quebrados. Feche com "detalhes de
pacote, injeção, lib e mapeamento campo-a-campo vivem na spec".

**Em §15 (Modelo de Dados):** nome lógico + finalidade (visão DDD). Agregados,
entidades, value objects, relação conceitual (1↔N, soft join, materialização
assíncrona). Sem tipo SQL preciso, constraints, índices, FK. Exceção: quando o
nome/valor é decisão de produto (ex.: enum de status e seus valores).

**Em §14 (RNF):** SLA em termos de produto ("avaliação ≤ 50ms p95"), sem
decomposição em budget por etapa.

**Em §11 (Entradas e Saídas):** campos por nome de negócio + origem lógica.
Schema/tipo concreto → spec.

**Em §18 (Riscos):** risco de **produto/negócio** (drift entre componentes, edge
cases de uso, mudança de contrato com fonte externa). Risco puramente técnico →
spec.

### Versionamento — a versão conta estados de produção

Política completa em `.claude/expertise/harness-protocols/document-versioning.md` — **leia
antes de tocar o frontmatter**. O essencial:

- Todo PRD **nasce em `1.0` e permanece em `1.0`** enquanto a solução descrita
  não estiver em produção. Refinar, reescrever, inverter escopo, corrigir
  premissa: nada disso bumpa, e nada disso entra no histórico.
- A versão sobe **uma vez por estado de produção**: a solução já está em
  produção **e** o PRD passa a descrever um comportamento diferente **que irá**
  para produção.
- **Teste:** *esta edição vai fazer alguém mudar código que já está rodando em
  produção?* Não → não bumpa.
- Reformulação profunda vira **PRD novo** (sufixo `-v2`), que nasce em 1.0. O
  antigo não bumpa por ganhar nota de superseded.
- **Nunca bumpe ao editar no meio de um fluxo:** a versão sobe uma vez, quando o
  fluxo fecha com a auditoria aprovada, e quem sobe é o `gofi-qa` (lei comum 4).
- `atualizado` muda **sempre**; `status` diz a fase. São eles que respondem
  "está fresco?" e "onde isto está?" — não a versão.

### Seções do template

| # | Seção | Status |
|---|-------|--------|
| 1 | Informações Gerais | obrigatória |
| 2 | Contexto de Negócio | obrigatória |
| 3 | Objetivo do Requisito | obrigatória |
| 4 | Definições | opcional |
| 5 | Escopo (Dentro / Fora) | obrigatória |
| 6 | Personas / Usuários Impactados | obrigatória |
| 7 | Descrição do Processo de Negócio | obrigatória |
| 8 | Regras de Negócio (RN-XX) | obrigatória |
| 9 | Fluxo de Negócio (Principal / Alternativos / Exceção) | obrigatória |
| 10 | BPMN — Fluxo de Negócio | opcional |
| 11 | Entradas e Saídas | obrigatória |
| 12 | Critérios de Aceite (Dado/Quando/Então) | obrigatória |
| 13 | Requisitos Funcionais (RF-XX) | obrigatória |
| 14 | Requisitos Não Funcionais (RNF-XX) | obrigatória |
| 15 | Modelo de Dados | opcional |
| 16 | Regras de Validação | opcional |
| 17 | Considerações Técnicas | opcional |
| 18 | Riscos e Premissas | obrigatória |
| 19 | Anexos | opcional |

### Convenções de código

- **Regras de Negócio:** `RN-01`, `RN-02`, …
- **Requisitos Funcionais:** `RF-01`, … com referência à `RN-XX` que originou.
- **Requisitos Não Funcionais:** `RNF-01`, … categorizados (Performance,
  Segurança, Confiabilidade, etc.).
- **Critérios de Aceite:** `CA-01`, … no formato **Dado / Quando / Então**.
- **Fluxos alternativos:** `FA-01`, … **Fluxos de exceção:** `FE-01`, …

### Mapeamento contexto institucional → template

| Fonte institucional (`institutional-context.md`) | Seção do template |
|--------------------------|-------------------|
| `domain.md` (domínio/subdomínios) | §2 Contexto de Negócio |
| `glossary.md` | §4 Definições |
| `actors.md` | §6 Personas |
| `business-rules.md` | §8 Regras de Negócio |
| `integrations.md` | §11 Entradas e Saídas / §17 Considerações Técnicas |
| `metrics.md` | §12 Critérios de Aceite |
| `integrations.md` (restrições/premissas) | §18 Riscos e Premissas |
| `roadmap.md` (item correspondente) | §2 Contexto de Negócio + §17 Considerações Técnicas |
