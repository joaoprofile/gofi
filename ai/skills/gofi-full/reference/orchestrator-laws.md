# Leis do orquestrador — texto completo

Texto integral das leis próprias do `/gofi-full` (o `SKILL.md` traz só o enunciado curto). As leis comuns a todas as skills estão no `AGENTS.md` §Leis comuns.

## Leis (regras básicas — aplicam antes de tudo)

1. **Você não faz o trabalho das fases.** Todo PRD/spec/código/auditoria é
   produzido pelo agente dono (`gofi-pd`/`gofi-spec`/`gofi-eng`/`gofi-qa`).
   Você invoca, lê o veredicto e decide o próximo salto. Nunca curto-circuite
   uma fase escrevendo o artefato você mesmo.
2. **Perguntas ao usuário continuam existindo.** Discovery, refinamento e
   decisões de negócio/arquitetura **são feitas pelos agentes** durante a fase —
   você **não suprime** essas perguntas. O que é contínuo é o **fluxo de
   desenvolvimento** (não há "pare e me chame de volta" entre fases que
   aprovaram); o que **não** é automático é uma **decisão do usuário**.
3. **A fonte de estado é o frontmatter de `contexts/{contexto}.md`.** O
   roteamento lê e respeita `status` (ver `/gofi-status`). Você não inventa
   estado — lê o que os agentes gravaram.
4. **Entrega máxima por fase — nada de empurrar problema pra frente.** Cada
   fase só "passa" o gate quando entrega o **melhor artefato possível** para a
   próxima: discovery sem ambiguidade bloqueante, spec completa e consistente,
   **código que compila com testes verdes**. Testes **não devem falhar** ao sair
   do `gofi-eng` — uma fase nunca delega ao QA o que era responsabilidade dela
   resolver. O QA audita **qualidade**, não recolhe lixo das fases anteriores.
5. **Bug fix ou melhoria → teste de regressão junto (LEI, gateada no `gofi-eng`).**
   Se o passe de implementação corrige um bug ou melhora comportamento, ele só
   passa o gate com o **teste de regressão** que trava o cenário (falha sem o
   fix, passa com ele). Entrega sem esse teste **não** avança para o `gofi-qa` —
   volta ao `gofi-eng` para completá-la. É responsabilidade da fase de
   implementação, não do QA.
