# Regras de escrita do PRD

## Regra de Ouro (prioritária)

> **PRD é problema, negócio e intenção. A mudança é o que o sistema deve fazer
> e como. Não invada o território dela.**

- **PRD contém apenas dados de negócio/intenção**: problema, atores, jornadas
  (com as regras de cada passo), métricas, escopo.
- **A mudança (escrita pelo `/gofi-spec`) contém o resto**: os requisitos com
  cenários — o "o quê" verificável — e o design — o "como" técnico: pacotes,
  paths, libs, SQL, migrations, performance budgets, mapeamento campo-a-campo.
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
`.claude/templates/prd-template.md`: sete seções, no formato 3 do corpus
(decisão 0003).

### Regras de geração

- **Copie o template** `.claude/templates/prd-template.md` como base.
- Salve em `{pathPrd}/{contexto}/prd-{contexto}.md`.
- **Mantenha as sete seções**, com títulos e numeração; nenhuma é opcional —
  seção sem conteúdo diz por quê em uma linha.
- Nunca invente seções fora do template: o que não cabe nelas é nota para o
  `/gofi-spec` ou não é do PRD.
- **Requisitos, cenários, IDs e dados não entram.** Nascem no delta da
  primeira mudança, que o `/gofi-spec` escreve citando este PRD.

### Escrita RAG do PRD

**Leia `.claude/templates/prd-template.md`** — layout obrigatório do PRD (já no formato RAG: frontmatter + `keywords`, **sem** `**Autor/Data:**`/Rastreabilidade/Histórico). Ao gerar o PRD, siga a seção *Escrita* de `.claude/expertise/harness-protocols/rag-retrieval.md`: frontmatter + `keywords` (8–14 termos de busca), **zero proveniência/rastro de agent/nome de pessoa**, e **regenere** `prd/INDEX.md` (`gofi index docs`) ao criar/renomear. Para descobrir PRDs existentes, consulte `prd/INDEX.md` — não varra a pasta.

### Estilo — direto, sem invadir a spec

Aplicação prática da **Regra de Ouro (§Regra de Ouro, acima)**. O PRD é
**fonte para o `/gofi-spec`**, não a spec: o seu valor é **clareza do problema,
de quem sofre com ele, do que é sucesso e de onde a entrega termina**.

**Filtro de extração — quando o input vier técnico:**

| Entrada técnica do usuário | O que vai pro PRD (negócio) | O que NÃO vai (fica pra mudança) |
|----------------------------|-----------------------------|--------------------------------|
| Migration `CREATE TABLE foo (...)` | "guardamos X por cliente" | tipos SQL, índices, `uq_*`, `DEFAULT` |
| Snippet `func Calculate(...) ...` | "o motor avalia a regra e produz o resultado" | nome de função, lib, assinatura |
| `services/domain/x/service/y.go` | (nada — é onde o código mora) | path do arquivo, nome do pacote |
| FK `REFERENCES bar(id)` | "todo A pertence a um B" | `FOREIGN KEY`, `ON DELETE`, constraint |
| Estratégia de cache | (nada — é decisão técnica) | TTL, chave, invalidação |

**Regra simples:** se trocar a stack muda a frase? Então é técnico — não entra no PRD.

**Regra de negócio descoberta** entra na jornada a que pertence (§5), como
restrição em linguagem de negócio — "pedido já enviado não se cancela" —,
**sem ID**. O `/gofi-spec` a transforma em requisito NÃO DEVE com cenário; o ID
nasce lá.

**Em §2 (Objetivo e métricas):** cada métrica com valor de hoje, meta e fonte.
"Melhorar a experiência" não é métrica; "tempo para concluir um pedido, de 6
para 2 minutos, medido no funil" é. É daqui que saem os requisitos não
funcionais e os critérios de sucesso da mudança.

**Em §6 (Restrições, premissas e riscos):** risco de **produto/negócio** (drift
entre componentes, edge cases de uso, mudança de contrato com fonte externa).
Risco puramente técnico → design da mudança.

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
- O PRD é discovery: depois que a primeira mudança é arquivada, a verdade do
  contexto é a spec viva. PRD não é reescrito para acompanhar o sistema — um
  discovery novo é um PRD novo.
- `atualizado` muda **sempre**; `status` diz a fase. São eles que respondem
  "está fresco?" e "onde isto está?" — não a versão.

### Seções do template

| # | Seção | Responde |
|---|-------|----------|
| 1 | Problema | o que dói, para quem, com que frequência, a que custo — com evidência |
| 2 | Objetivo e métricas de sucesso | o resultado e como medir (hoje, meta, fonte) |
| 3 | Personas | quem usa, o que precisa, o que a impede hoje |
| 4 | Escopo (Dentro / Fora) | onde a entrega começa e termina |
| 5 | Jornadas | os passos de negócio que mudam, com as regras de cada um |
| 6 | Restrições, premissas e riscos | o que limita, o que se assume, o que pode dar errado |
| 7 | Dúvidas em aberto | o que ainda depende de alguém (`[PRECISA ESCLARECER: …]`) |

### Convenções

- **Jornadas:** `J1`, `J2`, … — cada uma vira um ou mais requisitos `REQ-NN`
  no delta, que citam a jornada de origem.
- **Nenhum outro identificador** no PRD: requisito e cenário têm ID só no delta.
- Dúvida sempre como `[PRECISA ESCLARECER: pergunta — quem decide]`; a
  primeira mudança não é aprovável enquanto ela existir.

### Mapeamento contexto institucional → template

| Fonte institucional (`institutional-context.md`) | Seção do template |
|--------------------------|-------------------|
| `domain.md` (domínio/subdomínios) | §1 Problema |
| `actors.md` | §3 Personas |
| `business-rules.md` | §5 Jornadas (como restrição de cada passo) |
| `metrics.md` | §2 Objetivo e métricas de sucesso |
| `integrations.md` | §5 Jornadas (passos que cruzam outro sistema) / §6 Restrições |
| `integrations.md` (restrições/premissas) | §6 Restrições, premissas e riscos |
| `roadmap.md` (item correspondente) | §1 Problema + §4 Escopo |
| `glossary.md` | termos usados como estão, em todas as seções |
