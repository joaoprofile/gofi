# GOFI Design System — Web (React)

Ponto de entrada para construir **qualquer** interface web. Este DS é
**neutro em relação ao domínio**: descreve o *padrão* (tokens, componentes,
layouts), nunca as telas de um produto. Telas concretas, rotas e o microcopy
oficial vivem em `specs/` e `.claude/memory/` — **nunca** aqui.

> **Stack:** React + TypeScript, estilizado com **Tailwind CSS v4** (tokens como
> utilitários — `bg-action`, `text-ink`). Sem Shadow DOM. Dados via TanStack Query;
> estado local via hooks.

> **Lib de design system:** estes componentes são materializados pela biblioteca
> pública **`gofi-ui`** (React + Tailwind v4 + `class-variance-authority` — fonte em
> `src/`, publicada no npm/GitHub). Ao gerar código: `import 'gofi-ui/styles'` uma
> vez, envolva a app no `<ThemeProvider>` e importe os componentes —
> `import { Button, Card } from 'gofi-ui'`. Estes docs são a especificação
> domínio-neutra que a `gofi-ui` implementa.

---

## Filosofia

1. **Acessibilidade desde o início** — não é uma camada final. Todo componente já
   nasce com label, foco visível, navegação por teclado e contraste AA. Ver
   [foundations/accessibility.md](foundations/accessibility.md).
2. **Cor de marca dominante em grandes superfícies** — `--brand` (ex.: `#AAD7FF`)
   preenche heroes/blocos; o texto sobre ela usa `--tx-on-brand` (o tom que passa
   AA sobre a marca). Affordances que precisam de contraste sobre o branco usam
   `--action` (ex.: `#1B72D8`). As cores são definidas pelo **projeto** (em
   `.gofi.yaml`, bloco `ui.brand`) e aplicadas via `<ThemeProvider>` — os hex aqui
   são apenas o padrão neutro de partida.
3. **Tokens/modos como motor de temas** — light/dark vêm dos mesmos tokens.
   Fonte única: [knowledge/ui/design-tokens.md](../../../knowledge/ui/design-tokens.md).
4. **Geometria arredondada e arejada** — radius generoso, botões pill, espaço para
   respirar (escala 4/8), sombras suaves.

---

## Como o agente usa este DS

```
1. Ler design-tokens.md (fonte única) + tokens-web.md (forma em CSS)
2. Mapear a tela com os patterns/ (app-shell, page-templates, states)
3. Compor com os components/ existentes ANTES de criar qualquer novo
   - precisa de variação? → uma variant do componente, não um clone
4. Garantir os 4 estados (patterns/states.md) e a11y (foundations/accessibility.md)
```

> **Um componente do DS antes de um novo** (princípio 9). Um novo componente só
> nasce quando nenhum existente, nem mesmo com uma variant, resolve o problema.

---

## Índice

### Foundations
- [tokens-web.md](foundations/tokens-web.md) — tokens como CSS custom properties
- [color.md](foundations/color.md) — uso de cor, o duplo papel da cor de marca, status
- [typography.md](foundations/typography.md) — escala, pesos, hierarquia
- [spacing-layout.md](foundations/spacing-layout.md) — escala 4/8, grid, container
- [radius-elevation.md](foundations/radius-elevation.md) — radius e sombra
- [motion.md](foundations/motion.md) — durações, easing, reduced-motion
- [iconography.md](foundations/iconography.md) — estilo, tamanho, a11y de ícones
- [accessibility.md](foundations/accessibility.md) — WCAG 2.2 AA operacional

### Components
- [components/_index.md](components/_index.md) — catálogo completo e taxonomia

### Patterns
- [patterns/app-shell.md](patterns/app-shell.md) — sidebar + barra superior
- [patterns/navigation.md](patterns/navigation.md) — rotas, breadcrumbs, tabs
- [patterns/forms.md](patterns/forms.md) — form-as-page, validação, microcopy
- [patterns/data-display.md](patterns/data-display.md) — tabela, lista, cards
- [patterns/feedback.md](patterns/feedback.md) — toast, banner, modal, confirm
- [patterns/page-templates.md](patterns/page-templates.md) — list, detail, dashboard, hero
- [patterns/states.md](patterns/states.md) — loading, empty, error, success

---

## Regras inegociáveis (resumo operacional)

| Regra | Detalhada em |
|------|-------------|
| 4 estados em toda tela com dados | [patterns/states.md](patterns/states.md) |
| Todo input tem label + erro + hint | [patterns/forms.md](patterns/forms.md) |
| Foco sempre visível (`--focus`) | [foundations/accessibility.md](foundations/accessibility.md) |
| Contraste ≥ 4.5:1 (texto) / 3:1 (UI) | [foundations/color.md](foundations/color.md) |
| Mobile-first (base → `md:`/`lg:`) | [foundations/spacing-layout.md](foundations/spacing-layout.md) |
| Microcopy direto e claro (verbos) | [patterns/forms.md](patterns/forms.md) |
| Destrutivo: confirmação ou desfazer | [patterns/feedback.md](patterns/feedback.md) |
| Token, nunca um literal | [foundations/tokens-web.md](foundations/tokens-web.md) |
