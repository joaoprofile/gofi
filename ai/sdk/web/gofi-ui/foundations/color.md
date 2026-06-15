# Cor — uso e acessibilidade

Valores em [design-tokens.md](../../../../knowledge/ui/design-tokens.md). Este
arquivo cobre **quando** usar cada papel.

## O duplo papel da cor de marca (regra central)

`--brand` e `--action` **não são intercambiáveis** — resolvem problemas de
contraste opostos. As cores são definidas pelo **projeto** (`.gofi.yaml`, bloco
`ui.brand`) e aplicadas via `<ThemeProvider>`; os hex abaixo são apenas o padrão
neutro de partida.

| Papel | Token | Onde | Texto sobre ela |
|------|-------|-------|--------------|
| **Brand** | `--brand` (ex.: `#AAD7FF`) | grandes superfícies (hero, blocos, ilustração) | `--tx-on-brand` (ex.: navy `#0B2942`) |
| **Action** | `--action` (ex.: `#1B72D8`) | botão primário, link, foco, item ativo, controle filled | `#FFFFFF` |

> **Nunca** texto branco sobre uma superfície de marca clara (reprova no AA) — use
> `--tx-on-brand`/`text-on-brand`. **Nunca** a superfície de marca clara como fundo
> de um botão pequeno com texto branco. A marca é uma superfície; a action é uma
> affordance. (Se o projeto escolher uma marca **escura**, a lógica inverte: o
> on-brand vira o tom claro.)

## Contraste mínimo (WCAG 2.2 AA)

| Conteúdo | Mínimo |
|---------|---------|
| Texto normal (< 18.66px regular) | 4.5:1 |
| Texto grande (≥ 24px, ou ≥ 18.66px bold) | 3:1 |
| Componente de UI / foco / ícone informativo | 3:1 |

Verifique com uma ferramenta, **não a olho**. Pares de referência já validados:
`--tx-on-brand` sobre `--brand` (9.84:1); `#FFFFFF` sobre `--action` (4.72:1).

## Status — semântico, não decorativo

| Família | base | `-bg` | Uso |
|--------|------|-------|-------|
| success | `--success` | `--success-bg` | confirmação, estado positivo |
| warning | `--warning` | `--warning-bg` | atenção reversível |
| danger  | `--danger`  | `--danger-bg`  | erro, ação destrutiva |
| info    | `--info`    | `--info-bg`    | dica neutra, em progresso |

A cor **nunca** é o único canal de significado (a11y): combine-a com ícone + texto
(ex: erro = ícone + `--danger` + microcopy, não apenas uma borda vermelha).

## Tints sobre a marca

Destaques dentro de uma superfície de marca usam `primary-100/300` ou branco
translúcido — nunca uma cor de status que conflitaria com a cor de marca.
