---
tipo: prd
formato: 3                          # versão do formato do corpus — não invente outro valor
contexto: {contexto}
versao: "1.0"
status: proposto                    # ou discovery | aprovado (depois da revisão da pessoa)
spec: specs/{contexto}/spec.md      # a spec viva, quando existir — ou n/a
operacoes: [{processos do léxico — .claude/lexicon/operacoes.md}]
marketplaces: [{canais citados, se houver}]
keywords: [{até 12 termos que ainda não couberam nas facetas acima}]
atualizado: {YYYY-MM-DD}
---
# PRD — {contexto} — {Nome do Requisito}

> **O PRD responde por quê e para quem** — discovery de um contexto ou produto
> novo. Requisitos, regras, cenários e dados **não** entram aqui: nascem no
> delta da primeira mudança (`gofi change new`), que cita este PRD. Se uma
> frase deste documento diz "o sistema deve…", ela é de um requisito.
>
> As facetas (`operacoes`, `marketplaces`) são vocabulário controlado do
> `.claude/lexicon/`; termo novo entra no léxico antes. `keywords` é o resto,
> com teto de 12.

## 1. Problema

> O que dói hoje, para quem, com que frequência e a que custo. Evidência
> (números, relatos, incidentes) vale mais que opinião.

## 2. Objetivo e métricas de sucesso

> O resultado esperado em uma frase e como saber que foi atingido.

| Métrica | Hoje | Meta | Como medir |
|---|---|---|---|
| {métrica} | {valor} | {valor} | {fonte} |

## 3. Personas

| Persona | Papel | O que precisa | O que a impede hoje |
|---|---|---|---|
| {Persona} | {papel} | {necessidade} | {obstáculo} |

## 4. Escopo

### Dentro
- {o que esta entrega cobre}

### Fora
- {o que fica de fora, e por quê}

## 5. Jornadas

> As jornadas que a entrega muda, do ponto de vista de quem usa — passos de
> negócio, não telas nem endpoints. Cada jornada vira um ou mais requisitos
> com cenários no delta. Fluxo com ramificação real pode levar um diagrama PlantUML
> (`.claude/expertise/diagramming/conventions.md`); não é obrigatório.

### J1 — {nome da jornada}
1. {passo}
2. {passo}

## 6. Restrições, premissas e riscos

| Tipo | Descrição | Impacto | Mitigação |
|---|---|---|---|
| Restrição | {prazo, regulação, legado} | | |
| Premissa | {o que se assume verdadeiro} | | |
| Risco | {o que pode dar errado} | | |

## 7. Dúvidas em aberto

> Cada dúvida que ainda depende de alguém. A primeira mudança só é aprovável
> sem nenhum marcador: responda aqui antes de escrevê-la.

- [PRECISA ESCLARECER: {pergunta} — {quem decide}]
