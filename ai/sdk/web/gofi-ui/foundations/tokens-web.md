# Tokens — forma web (Tailwind CSS v4)

Os valores vêm de [design-tokens.md](../../../../knowledge/ui/design-tokens.md)
(fonte única). Este arquivo é a **forma** que eles assumem no web: a lib `gofi-ui`
compila tudo em `theme.css` e o consumo é por **utilitários Tailwind v4** — você
escreve `className`, **não** `style` inline.

> Um componente **nunca** redeclara um valor nem usa um literal: usa a **classe**
> (`bg-action`, `text-ink`, `rounded-pill`). A marca vem das cores do projeto, que o
> `<ThemeProvider>` injeta como vars (`--brand`/`--action`…); mudar o tema = trocar
> `data-theme`. Nada disso mora no componente.

## Setup (uma vez no app)

```ts
import 'gofi-ui/styles';            // CSS pré-compilado — não precisa configurar Tailwind
import { ThemeProvider } from 'gofi-ui';
```

Para **estender** os tokens no Tailwind v4 do próprio app (utilitários extras),
importe a camada de tema em vez do CSS pronto:

```css
@import 'gofi-ui/theme.css';
```

## As três camadas do `theme.css`

```css
@import 'tailwindcss';

/* 1) @theme — primitivos/escala viram utilitários (rounded-*, shadow-*, text-*, ease-*) */
@theme {
  --font-sans: 'Inter', system-ui, sans-serif;
  --text-display: 2.25rem; --text-h1: 1.75rem; --text-h2: 1.375rem; --text-h3: 1.125rem;
  --text-body: 1rem; --text-body-sm: .875rem; --text-caption: .75rem;
  --color-primary-50: #f0f7ff; /* … 100..900 */ --color-primary-900: #0b2942;
  --color-accent-500: #6172f3; /* apoio (web chama "accent") */
  --color-green-500: #16b364;  /* marca green */
  --radius-sm: 8px; --radius-md: 12px; --radius-lg: 16px; --radius-xl: 24px; --radius-pill: 999px;
  --shadow-sm: 0 1px 2px rgba(16,24,40,.06);
  --shadow-md: 0 4px 12px rgba(16,24,40,.08);
  --shadow-lg: 0 12px 32px rgba(16,24,40,.12);
  --ease-standard: cubic-bezier(.2,0,0,1);
}

/* 2) vars semânticas — papel por tema/marca (NÃO usadas direto: alimentam a camada 3) */
:root {
  --sf-page: var(--color-gray-50);  --sf-card: #fff;       --sf-hover: var(--color-gray-100);
  --sf-sunken: var(--color-gray-100); --sf-border: var(--color-gray-200);
  --tx-ink: var(--color-gray-900);  --tx-ink-2: var(--color-gray-500);
  --tx-on-brand: var(--color-primary-900); --tx-on-secondary: #fff;
  --brand: var(--color-primary-200); --action: var(--color-primary-600);
  --action-hover: var(--color-primary-700);
  --accent: var(--color-accent-500); --accent-hover: var(--color-accent-600);
  --focus: var(--color-primary-600);
  --success:#12b76a; --success-bg:#ecfdf3; --warning:#f79009; --warning-bg:#fffaeb;
  --danger:#f04438; --danger-bg:#fef3f2; --info:#2e90fa; --info-bg:#eff8ff;
  --z-base:0; --z-sticky:100; --z-dropdown:200; --z-overlay:300; --z-modal:400; --z-toast:500;
}
[data-theme="dark"] {
  --sf-page: var(--color-gray-900); --sf-card:#161b26; --sf-hover: var(--color-gray-800);
  --sf-sunken:#0c111d; --sf-border: var(--color-gray-700);
  --tx-ink: var(--color-gray-50);  --tx-ink-2: var(--color-gray-400);
  --action: var(--color-primary-500); --action-hover: var(--color-primary-600);
  --focus: var(--color-primary-400);
}
/* A marca vem das cores do projeto: o <ThemeProvider> injeta --brand/--action/etc.
   na raiz a partir do bloco `ui.brand` do .gofi.yaml. A lib aceita cores arbitrárias;
   sem `brand` no projeto, valem os defaults neutros de :root acima. */

/* 3) @theme inline — expõe as vars semânticas como utilitários de cor (bg-*, text-*, border-*) */
@theme inline {
  --color-page: var(--sf-page);   --color-card: var(--sf-card);   --color-hover: var(--sf-hover);
  --color-sunken: var(--sf-sunken); --color-border: var(--sf-border);
  --color-ink: var(--tx-ink);     --color-ink-secondary: var(--tx-ink-2);
  --color-on-brand: var(--tx-on-brand); --color-on-secondary: var(--tx-on-secondary);
  --color-brand: var(--brand);    --color-action: var(--action); --color-action-hover: var(--action-hover);
  --color-accent: var(--accent);  --color-accent-hover: var(--accent-hover); --color-focus: var(--focus);
  --color-success: var(--success); /* … success-bg, warning, danger, info … */
}
```

## Mapa token → utilitário (o que você escreve)

| Papel | var semântica | Utilitário Tailwind |
|------|---------------|---------------------|
| fundo da página | `--sf-page` | `bg-page` |
| card / input / modal | `--sf-card` | `bg-card` |
| hover de linha | `--sf-hover` | `bg-hover` |
| borda / divisor | `--sf-border` | `border-border` |
| texto padrão | `--tx-ink` | `text-ink` |
| texto de apoio | `--tx-ink-2` | `text-ink-secondary` |
| superfície de marca | `--brand` | `bg-brand` |
| texto sobre a marca | `--tx-on-brand` | `text-on-brand` |
| ação (botão/link/foco) | `--action` | `bg-action` / `text-action` |
| cor de apoio (**web = accent**) | `--accent` | `bg-accent` |
| raio pill / cards | `--radius-pill` / `--radius-lg` | `rounded-pill` / `rounded-lg` |
| sombra | `--shadow-md` | `shadow-md` |
| tipografia | `--text-h1` … | `text-h1` … |
| espaçamento | — (escala padrão do Tailwind) | `p-4`, `gap-6`, `px-5` |
| duração | — (sem var no web) | `duration-200` |
| easing | `--ease-standard` | `ease-standard` |

> **Atenção (assimetria com o mobile):** a cor de apoio é **`accent`** no web e
> **`secondary`** no mobile — mesmo papel, nome de token diferente. O web também
> **não** define `--space-*` nem durações: usa a escala/utilitários do Tailwind
> (o número da escala coincide: `--space-4` ⇒ `p-4` = 16px).

## Uso em um componente

```tsx
// ✅ utilitário — segue tema e marca automaticamente
<button className="bg-action text-on-brand rounded-pill px-5 py-3 shadow-sm
                   transition-colors duration-200 ease-standard hover:bg-action-hover" />

// ❌ literal/inline — quebra dark mode, marca e o motor de tokens
<button style={{ background: '#1B72D8', borderRadius: 999 }} />
```

## Tema e marca

Definidos **uma vez** na raiz pelo `<ThemeProvider>`, que alterna `data-theme`
(`light`/`dark`) no `<html>` e injeta as **cores do projeto** (do bloco `ui.brand`
do `.gofi.yaml`) como vars `--brand`/`--action`/`--tx-on-brand`/`--accent`. A lib
aceita cores arbitrárias; sem `brand` no projeto, valem os defaults neutros.
Nenhum componente gerencia tema/marca localmente — ver [color.md](color.md) e o
modelo de marca em
[design-tokens.md](../../../../knowledge/ui/design-tokens.md). Armadilhas (autofill,
literais) em [theming-dark-mode.md](../../../../knowledge/ui/theming-dark-mode.md).
</content>
