# Protocolo de Aprendizado Contínuo — cross-agent

Quando o usuário corrigir, ensinar ou validar algo não-óbvio, **todos os
agents devem aprender** — não apenas o que recebeu a correção. Esta é a
regra fundamental que mantém o sistema coerente ao longo do tempo.

---

## Duas zonas — onde o aprendizado pode ser gravado

A pasta de agentes tem dono declarado por área:

| Zona do **gofi** — o `gofi update` reescreve | Zona do **projeto** — o gofi nunca reescreve |
|---|---|
| `skills/`, `expertise/`, `templates/`, `sdk/<lang>/`, a parte do gofi no `AGENTS.md` | `knowledge/`, `institutional/`, `memory/`, `lexicon/`, o bloco do projeto no `AGENTS.md`, `specs/`, `prd/` |

**O aprendizado do projeto vai sempre para a zona do projeto.** Arquivo da zona
do gofi editado no projeto é mantido pelo update, mas com aviso, e diverge do
upstream para sempre. Por isso não se corrige a skill, o pack ou o knowledge do
SDK dentro do projeto: registra-se a correção em `knowledge/`, que vence na
busca (`specs > prd > memory > knowledge > expertise > sdk`).

Quando a correção diverge de uma regra do gofi, declare o que ela corrige no
frontmatter — o caminho a partir da pasta de agentes e, se for uma seção só, o
título exato dela depois de `#`:

```yaml
---
overrides: [expertise/event-driven/executor-pattern.md#Retry transient vs permanent]
keywords: [retry, idempotência]
---
```

Com isso, `gofi find` mostra a correção logo acima da regra, mesmo quando a
pergunta só alcança a regra, e `gofi show` da regra aponta a correção. Se um
update renomear ou remover a seção, `gofi index check` acusa erro: reaponte o
`overrides:` para onde a regra está agora.

## Regra absoluta — knowledge é domínio-neutro

> `knowledge/`, `expertise/` e `sdk/<lang>/` descrevem **padrão técnico** —
> como usar o SDK, como estruturar código, como elicitar, como auditar.
> **Nunca** carregam estado de domínio do projeto.

**Não pode aparecer em knowledge:**

- Nomes de entidades do produto (qualquer entidade de negócio que não seja
  `tenant`/`user` enquanto padrão SaaS universal).
- Roles, perfis ou hierarquias concretas do produto — usar placeholders
  `RoleA`/`RoleB` quando precisar exemplificar.
- Termos do domínio em qualquer idioma.
- Module paths concretos (`github.com/<org>/<projeto>/...`) — usar
  `<module>/...` como placeholder.
- Referências a versões ou ADRs de specs específicas.
- Endpoints, rotas, telas ou microcopy concretos do produto.
- Cloud, região, OCID/ARN/account id, nome de compartment ou de serviço
  reais (no papel de ops).
- Decisões que valem para um único contexto/serviço deste projeto.

**Pode (e deve) aparecer:** padrões do SDK e como compô-los; estrutura de
pastas, naming, regras absolutas, anti-padrões; princípios cross-agent;
placeholders e exemplos genéricos (`{contexto}`, `<module>`, `RoleA`,
`entity`, `<cloud>`, `<env>`, `{service}`).

**Onde vai conteúdo de domínio:**

| Tipo | Lugar |
|------|-------|
| Entidades, regras de negócio, RNs, ciclo de vida | `specs/{contexto}/sdd-{contexto}.md` |
| Negócio da empresa além da spec | `institutional/{projeto}/` |
| Fato global do projeto (serviços/binários, convenções) | `memory/project.md` |
| Estado por-contexto (status, versão, fase, decisões da fase, gotchas) | `memory/contexts/{contexto}.md` — índice via `/gofi-status` |
| PRD do contexto | `prd/{contexto}/prd-{contexto}.md` |
| Provedor, topologia, sizing, regiões, secrets (ops) | spec de infra + `memory/` |

**Teste antes de escrever em knowledge:**

> "Este texto serviria, sem alteração, a um projeto totalmente diferente
> que use o mesmo SDK (ou framework, ou ferramenta de IaC)?"

Se não serviria — mova o trecho para `specs/`, `institutional/` ou `memory/`.
Se serviria com troca de placeholder — generalize antes de salvar.

---

## Onde gravar, por papel

Todo destino abaixo fica em `knowledge/` (zona do projeto). A subpasta é o
escopo: `shared/` vale para todos os papéis; `{papel}/` só para um.

| Papel | Situação | Destino |
|---|---|---|
| todos | Decisão de arquitetura validada, qualquer linguagem | `knowledge/shared/<topico>.md` |
| todos | Correção de uso de API do SDK, padrão de código da linguagem | `knowledge/shared/<topico>.md` com `overrides:` apontando o arquivo de `sdk/<lang>/knowledge/` que corrige |
| todos | Precisou ler o código real em `.gofi/gofi-sdk-<lang>/` para decidir | `knowledge/shared/<topico>.md` — o próximo agent não precisa do mesmo desvio |
| spec | Correção nas perguntas de elicitação ou no formato da spec | `knowledge/spec/<topico>.md` |
| eng | Correção na geração de código ou num boilerplate | `knowledge/eng/<topico>.md` |
| qa | Item de checklist incorreto, nova dimensão de auditoria, lição para o eng não reincidir | `knowledge/qa/<topico>.md` (lição que o eng precisa ler: `knowledge/shared/`) |
| ui | Princípio de UX, token de design, regra da superfície, padrão de componente | `knowledge/ui/<topico>.md` (token: um arquivo só, fonte única) |
| ops | Princípio de IaC/delivery, regra da ferramenta de IaC, módulo ou pipeline | `knowledge/ops/<topico>.md` |

A referência em `sdk/<lang>/api/` é gerada do código do SDK: erro nela se
corrige no SDK, não no projeto.

## Sequência obrigatória

1. **Identifique o escopo.** A lição é cross-AI? cross-language? específica da
   linguagem, da superfície ou da ferramenta de IaC? de um papel só?
2. **Grave no destino mais específico** da tabela acima, com `keywords` e, se
   corrigir regra do gofi, `overrides:`.
3. **Se afeta vários papéis**, grave em `knowledge/shared/`, não em cópias.
4. **Ofereça promover ao upstream** quando a correção serviria a qualquer
   projeto com o mesmo SDK: sem isso, cada projeto conserta a mesma coisa
   sozinho e o gofi não melhora. A promoção é uma mudança no repositório do
   gofi, feita por quem o mantém — nunca pelo agente, dentro do projeto.
5. **Confirme ao usuário** a lista exata de arquivos gravados.

## Regra do exemplo

Se o usuário pedir algo específico ainda não documentado:

1. **Pergunte por um exemplo concreto** antes de implementar.
2. Com o exemplo validado, registre o padrão em `knowledge/`.
3. **Só então escreva o código** — assim os agents futuros já conhecem o padrão.

Esse ciclo "pergunta → documenta → implementa" é o que evita que o mesmo
erro reapareça.
