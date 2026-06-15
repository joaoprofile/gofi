# Theming & dark mode (cross-framework)

Quando o app tem mais de um tema (claro/escuro), **cor nunca é literal no
componente/página** — é sempre um *token* que muda de valor por tema. Cor
hardcoded é o defeito nº 1 de dark mode: o componente fica "preso" no claro e
some no escuro (ou vice-versa).

## Regra

- **Toda cor de superfície, borda e texto vem de uma variável de tema**
  (CSS custom property, token do design system, ou algoritmo do UI kit).
  Ex.: `background: var(--sf-card)`, `color: var(--tx-ink)`,
  `border-color: var(--sf-border)`.
- O tema define os tokens **uma vez** (`:root` claro + bloco
  `[data-theme="dark"]`/algoritmo escuro). Os tokens são **context-aware** —
  o mesmo `var(--tx-ink)` resolve para escuro no claro e claro no escuro.
  Por isso trocar literal → token **conserta os dois modos de uma vez**.
- **Exceções legítimas (podem ficar literais):**
  - Texto branco/claro sobre fundo de acento sólido (botão colorido,
    gradiente) — lê bem nos dois temas.
  - Painel de marca intencionalmente sempre-escuro (hero de login).
  - Tints de status translúcidos (`rgba(accent, 0.1)`) — leves nos dois.
  - Valores dentro de um bloco `[data-theme="dark"]` (são o override do tema).

## Mapeamento típico (literal → token)

| Literal hardcoded | Token |
|---|---|
| `#fff` / `white` (fundo) | `--sf-card` |
| cinza claro de hover (`#f8fafc`, `#f3f4f6`) | `--sf-hover` |
| borda clara (`#e5e7eb`, `#d1d5db`) | `--sf-border` |
| texto escuro (`#111827`, `#1f2937`) | `--tx-ink` |
| texto secundário (`#6b7280`, `#9ca3af`) | `--tx-ink-2` |
| fundo de acento claro (`#eff6ff`) | tint de status (`--info-color-light`) |

## Armadilha do UI kit (AntD/MUI/etc.)

UI kits com *algoritmo* de tema (ex.: AntD `darkAlgorithm`) só adaptam os
componentes que estão **dentro do provider de tema**. Inputs/Selects "brancos
no escuro" quase sempre são: (a) componente renderizado **fora** do
`ConfigProvider`/ThemeProvider, ou (b) CSS custom sobrescrevendo o fundo do
input com literal. Verifique a árvore de providers e os overrides de CSS antes
de culpar o kit.

## Autofill do navegador (gotcha clássico)

Chrome/Safari pintam inputs preenchidos por **autofill** com fundo
branco/amarelo e texto escuro — ignorando o tema e o UI kit. Resultado:
"input branco no dark mode" mesmo com tudo certo. Fix global (uma vez,
no CSS raiz), mascarando o fundo com `box-shadow` inset e forçando a cor
do texto pelo token:

```css
input:-webkit-autofill,
input:-webkit-autofill:hover,
input:-webkit-autofill:focus {
  -webkit-text-fill-color: var(--tx-ink);
  -webkit-box-shadow: 0 0 0 1000px var(--sf-card) inset;
  caret-color: var(--tx-ink);
  transition: background-color 600000s 0s, color 600000s 0s;
}
```

## Estilo inline em componente também conta

`style={{ background: '#fff', color: '#374151' }}` em JSX quebra igual a
CSS hardcoded. Inline aceita token: `style={{ background: 'var(--sf-card)' }}`.
Componentes "placeholder/coming-soon", cards de estado vazio e badges são
onde mais aparece.

## Auditoria rápida

`grep` por literais de cor nos estilos e classifique cada um pela tabela
acima. Não confie no olho — teste a tela nos **dois** temas antes de marcar
como pronta. Telas de formulário e dashboards (muita superfície + input) são
onde o defeito mais aparece.
