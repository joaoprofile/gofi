---
scope: cross-framework
applies_to: [gofi-ui]
surface: [web, mobile]
package: gofi-ui
---

# GOFI Design Tokens — estrutura e padrões

Este arquivo define a **estrutura** de tokens (os papéis semânticos) e as **escalas**
que web e mobile consomem — nunca redefinindo a estrutura por conta própria.
Distribuídos pelas libs (`gofi-ui` no web como variáveis CSS, `gofi-ui-native` no
mobile como objeto TS).

> **As cores são definidas pelo PROJETO — não há paleta fixa.** Os valores de cor
> neste arquivo (marca, ação, apoio, escalas) são apenas o **padrão neutro / exemplo
> de partida** — **não** um limite. Cada projeto escolhe **suas próprias cores** e o
> agente as aplica configurando o tema da lib (web: `<ThemeProvider>`/vars de tema;
> mobile: `makeTheme`/`<ThemeProvider>`). **As libs aceitam cores arbitrárias.**
> O que **não** muda é a **estrutura semântica** (os papéis: marca, ação, on-brand,
> superfícies, status), as **escalas** (espaçamento, raio, tipografia, motion) e as
> **regras de acessibilidade** (contraste). A escolha de cores do projeto vive **no
> projeto** (`.gofi.yaml`), nunca neste conhecimento neutro.

A filosofia: acessibilidade desde o início, uma cor de marca dominante em superfícies
grandes, e tokens/modos como o motor de theming.

---

## Duas camadas de tokens

1. **Primitivos** — a paleta crua (escalas numéricas). Nunca usados diretamente na UI.
2. **Semânticos** — o papel (`--sf-card`, `--tx-ink`, `--action`).
   É isso que componentes e páginas consomem. Mudam de valor por tema; o
   primitivo não.

```
primitive  →  semantic   →  uso na UI (utilitário web)
#1B72D8    →  --action   →  bg-action (botão, foco, link)
gray-900   →  --tx-ink   →  text-ink  (texto padrão, claro)
```

---

## Primitivos — paleta

> Os hex abaixo são o **exemplo/padrão neutro** — substitua pelos da marca do
> projeto. O que importa é a **estrutura de escala** (50→900) e os **papéis**, não
> os valores específicos.

### Primária (marca) — exemplo derivado de `#AAD7FF`

| Token | Hex | Papel típico |
|-------|-----|--------------|
| `primary-50`  | `#F0F7FF` | tint de fundo, hover sutil |
| `primary-100` | `#DCEDFF` | superfície de destaque clara |
| `primary-200` | `#AAD7FF` | **cor de marca** — superfícies grandes (hero, blocos) |
| `primary-300` | `#7FC0FF` | bordas/destaques sobre a marca |
| `primary-400` | `#54A8FF` | — |
| `primary-500` | `#2E90FA` | info / accent intermediário |
| `primary-600` | `#1B72D8` | **action** — botão primário, foco, link sobre branco |
| `primary-700` | `#1259AE` | hover/pressed do action |
| `primary-800` | `#0E4585` | — |
| `primary-900` | `#0B2942` | **on-primary** — texto sobre a superfície de marca |

> **Regra de a11y inegociável (o duplo papel da cor de marca).** Vale para qualquer
> cor que o projeto escolher: se a superfície de marca for **clara** (ex.: `primary-200`
> `#AAD7FF`), texto **branco** sobre ela **falha** no WCAG AA — o texto on-brand é o
> shade escuro (`primary-900`). Para affordances que precisam de contraste sobre
> **branco** (botão preenchido, link, foco), use o shade que passa AA sobre branco
> (`primary-600`+) — nunca o tom claro da superfície. (Se o projeto escolher uma marca
> **escura**, a lógica inverte: on-brand vira o tom claro.)

### Secundária (accent de apoio) — base derivada de `#6172F3`

Uma cor de **apoio**: destaques secundários, gráficos, estados selecionados
alternativos, ilustração. **Padrão** (índigo) — substituída pela secundária do
projeto.

| Token | Hex | Papel |
|-------|-----|------|
| `secondary-100` | `#E0E3FF` | tint de fundo |
| `secondary-200` | `#C3C9FF` | superfície de destaque |
| `secondary-500` | `#6172F3` | **accent** (base, para tints/destaques) |
| `secondary-600` | `#444CE7` | accent preenchido (texto branco legível) / pressed |
| `secondary-700` | `#3538CD` | hover do accent preenchido |
| `secondary-900` | `#2D31A6` | on-secondary (texto sobre tint claro) |

> Primária e secundária têm **papéis diferentes**: a primária carrega action e
> superfície de marca; a secundária **complementa** (não compete). Evite usar
> ambas na mesma força na mesma tela.

### Neutros (cinza)

| Token | Hex |  | Token | Hex |
|-------|-----|--|-------|-----|
| `gray-50`  | `#F9FAFB` |  | `gray-500` | `#667085` |
| `gray-100` | `#F2F4F7` |  | `gray-600` | `#475467` |
| `gray-200` | `#EAECF0` |  | `gray-700` | `#344054` |
| `gray-300` | `#D0D5DD` |  | `gray-800` | `#1D2939` |
| `gray-400` | `#98A2B3` |  | `gray-900` | `#0C111D` |

### Semântica de status (cada uma com base, `-bg` claro, `on-`)

| Família | base | `-bg` (tint) | `on-` (texto sobre a base) |
|--------|------|--------------|------------------------|
| success | `#12B76A` | `#ECFDF3` | `#FFFFFF` |
| warning | `#F79009` | `#FFFAEB` | `#0C111D` |
| danger  | `#F04438` | `#FEF3F2` | `#FFFFFF` |
| info    | `#2E90FA` | `#EFF8FF` | `#FFFFFF` |

> `warning` é claro → `on-warning` é escuro (`gray-900`), mesma lógica do azul.

---

## Semânticos — claro e escuro

Mesmo **papel** nas duas superfícies; o **nome do token muda de forma**: no web são
vars CSS expostas como utilitários Tailwind v4 (`bg-action`, `text-ink`); no mobile
são chaves camelCase de um objeto TS (`makeTheme`). Ver `tokens-web.md` /
`tokens-mobile.md`.

| Papel | Web (`var` → utilitário) | Mobile (chave TS) | Claro | Escuro |
|-------|--------------------------|-------------------|-------|--------|
| fundo da página | `--sf-page` → `bg-page` | `surfacePage` | `gray-50` | `#0C111D` |
| card / modal / input | `--sf-card` → `bg-card` | `surfaceCard` | `#FFFFFF` | `#161B26` |
| hover de linha/item | `--sf-hover` → `bg-hover` | `surfaceHover` | `gray-100` | `#1D2939` |
| área rebaixada | `--sf-sunken` → `bg-sunken` | `surfaceSunken` | `gray-100` | `#0C111D` |
| borda / divisor | `--sf-border` → `border-border` | `surfaceBorder` | `gray-200` | `#344054` |
| texto padrão | `--tx-ink` → `text-ink` | `textColor` | `gray-900` | `#F9FAFB` |
| texto de apoio | `--tx-ink-2` → `text-ink-secondary` | `textSecondary` | `gray-500` | `#98A2B3` |
| texto sobre a marca | `--tx-on-brand` → `text-on-brand` | `textOnBrand` | `primary-900` | `primary-900` |
| superfície de marca | `--brand` → `bg-brand` | `colorBrand` | `primary-200` | `primary-200` |
| ação (botão/foco/link) | `--action` → `bg-action` | `colorAction` | `primary-600` | `primary-500` |
| hover/pressed da ação | `--action-hover` | `colorActionHover` | `primary-700` | `primary-600` |
| **cor de apoio** ⚠️ | `--accent` → `bg-accent` | `colorSecondary` | `secondary-600` | `secondary-600` |
| texto sobre o apoio | `--tx-on-secondary` → `text-on-secondary` | `textOnSecondary` | `#FFFFFF` | `#FFFFFF` |
| anel de foco | `--focus` | `focusRing` | `primary-600` | `primary-400` |
| tint de ativo/selecionado | — (use `bg-action/10`) | `colorActionSubtle` | ação @ ~12% | ação @ ~16% |

> ⚠️ **Divergência de nome real entre as libs (mesmo papel):** a cor de apoio é
> **`accent`** no web (`gofi-ui`) e **`secondary`** no mobile (`gofi-ui-native`). É
> a variante **preenchida** (`secondary-600`/`#444CE7`) porque precisa de texto
> branco legível (6.12:1 AA); `secondary-500` (`#6172F3`) é só para tints.
>
> No escuro o **action** clareia um passo (`primary-600 → primary-500`) para manter
> contraste; a **marca** (a cor de marca do projeto) costuma ser estável nos dois
> modos — é a identidade.
> O `colorActionSubtle` (mobile) é a ação em alpha baixo, só para estado ativo.

---

## Tipografia

No máximo **2 famílias tipográficas** (princípio 5). Sugestão (neutra): uma sans
para UI/texto e, opcionalmente, uma para números/dados. Escala modular (≈1.25),
com `clamp()` no web para fluidez e Dynamic Type no mobile.

| Token | Tamanho | Line-height | Peso | Uso |
|-------|------|-------------|--------|-------|
| `--text-display` | 36 | 44 | 700 | título de marca / hero |
| `--text-h1` | 28 | 36 | 700 | título de página |
| `--text-h2` | 22 | 30 | 600 | seção |
| `--text-h3` | 18 | 26 | 600 | sub-seção, título de card |
| `--text-body` | 16 | 24 | 400 | corpo padrão |
| `--text-body-sm` | 14 | 20 | 400 | apoio, labels |
| `--text-caption` | 12 | 16 | 500 | legendas, badges |

Pesos: `400` regular · `500` medium · `600` semibold · `700` bold. Tamanho mínimo
legível de corpo: **16** (web), suportando resize de até 200% (a11y).

---

## Espaçamento — escala 4/8

Nada de `margin: 13px` solto. Toda medida é múltiplo da escala.

| Token | px |  | Token | px |
|-------|----|--|-------|----|
| `space-1` | 4  |  | `space-6`  | 24 |
| `space-2` | 8  |  | `space-8`  | 32 |
| `space-3` | 12 |  | `space-10` | 40 |
| `space-4` | 16 |  | `space-12` | 48 |
| `space-5` | 20 |  | `space-16` | 64 |

---

## Raio e elevação

Geometria **generosa, arredondada**.

| Raio | px | Uso |
|--------|----|-------|
| `--radius-sm` | 8  | inputs, badges, tags |
| `--radius-md` | 12 | botões, cards pequenos |
| `--radius-lg` | 16 | cards, modais |
| `--radius-xl` | 24 | card hero, superfícies de marca |
| `--radius-pill` | 999 | botões pill, chips |

Elevação via sombra suave (web) / `elevation`+`shadow*` (mobile RN):

| Token | Web (box-shadow) |
|-------|------------------|
| `--shadow-sm` | `0 1px 2px rgba(16,24,40,.06)` |
| `--shadow-md` | `0 4px 12px rgba(16,24,40,.08)` |
| `--shadow-lg` | `0 12px 32px rgba(16,24,40,.12)` |

---

## Motion

| Token | Valor | Uso |
|-------|-------|-------|
| `motion-fast` | 100ms | feedback de tap/hover |
| `motion-base` | 200ms | transições padrão |
| `motion-slow` | 300ms | entrada de overlay/sheet |
| `ease-standard` | `cubic-bezier(.2,0,0,1)` | easing padrão in/out |

> **`prefers-reduced-motion`** (web) / `AccessibilityInfo.isReduceMotionEnabled`
> (mobile): animação > 200ms vira transição instantânea ou um fade curto. Sempre.

---

## Z-index (web)

| Token | Valor | Camada |
|-------|-------|-------|
| `--z-base` | 0 | conteúdo |
| `--z-sticky` | 100 | header/sidebar fixo |
| `--z-dropdown` | 200 | menus, popovers |
| `--z-overlay` | 300 | backdrop de modal/drawer |
| `--z-modal` | 400 | modal/drawer/sheet |
| `--z-toast` | 500 | toasts (acima de tudo) |

---

## Breakpoints (web — mobile-first)

| Token | min-width | Alvo |
|-------|-----------|--------|
| (base) | 0 | mobile (escreva o CSS aqui primeiro) |
| `--bp-md` | 768 | tablet |
| `--bp-lg` | 1024 | desktop |
| `--bp-xl` | 1280 | desktop largo |

No mobile (RN) não há media queries: o layout responde a `Dimensions`/
`useWindowDimensions` e `flex`; densidade de toque é o padrão.

---

## Marca: definida pelo projeto

A marca **não** é um valor fixo deste arquivo nem um catálogo fechado. Cada projeto
escolhe **as próprias cores** e o agente as aplica configurando o tema da lib. Os
valores abaixo são só um **exemplo** de como uma marca preenche os papéis:

| Papel | Exemplo (substitua pelo do projeto) |
|---|---|
| `--brand` (superfície dominante) | `#AAD7FF` |
| `--tx-on-brand` (texto sobre a marca) | `#0B2942` |
| `--action` (affordance: botão/link/foco) | `#1B72D8` |
| apoio (`accent`/`secondary`) | `#444CE7` |

> A superfície de marca tende a ser estável entre claro/escuro (é a identidade); a
> `--action` costuma clarear um passo no escuro para manter contraste. Mas tudo isso
> é **derivado das cores do projeto** — não há paleta pré-fixada.

### Como o projeto escolhe (e o agente aplica)

O projeto declara suas cores em `.gofi.yaml`; o agente as injeta no tema da lib —
web via `<ThemeProvider>` (vars `--brand`/`--action`…); mobile via
`makeTheme(brand, mode)`/`<ThemeProvider>`. **As libs aceitam cores arbitrárias.**

```yaml
# .gofi.yaml (no projeto, não no harness)
ui:
  framework: react
  brand:
    surface: "#AAD7FF"   # cor de marca do projeto (superfície dominante)
    onBrand: "#0B2942"   # texto sobre a marca (validar AA sobre `surface`)
    action:  "#1B72D8"   # affordance — validar ≥ 4.5:1 sobre branco
    accent:  "#444CE7"   # cor de apoio (opcional)
```

Sem o bloco `brand` → usa o padrão neutro deste arquivo. **As cores do projeto vivem
no projeto** — nunca neste conhecimento neutro.

### Escolher cores com segurança (a receita de contraste é o que importa)

Quaisquer que sejam as cores do projeto, o agente as valida ao aplicar:

| Papel | Regra |
|------|------|
| `--brand` | a cor de superfície dominante escolhida pelo projeto (clara **ou** escura) |
| `--tx-on-brand` | o tom (claro ou escuro) que atingir **≥ 4.5:1** sobre `--brand` |
| `--action` | um tom com **≥ 4.5:1 sobre branco** (affordance) |
| `--focus` | = `--action` |
| apoio (`accent`/`secondary`) | o hue complementar, no tom onde o texto sobre ele passa AA |

> **Invariante:** valide `--tx-on-brand` sobre `--brand` e `--action` sobre branco —
> ambos ≥ 4.5:1. Não atingiu? Ajuste o tom **dentro da cor do projeto**. Aplique as
> mesmas cores **nas duas superfícies** (web + mobile) para manter paridade, e
> registre a decisão em `.claude/memory/project.md`.

## Como cada surface consome

| Surface | Forma do token | Pacote | Doc |
|---------|------------|---------|-----|
| Web (React) | **Tailwind v4** — `@theme` + vars semânticas (`[data-theme]`/`[data-brand]`) expostas como utilitários (`bg-action`) | `gofi-ui` | `sdk/web/gofi-ui/foundations/tokens-web.md` |
| Mobile (RN) | objeto TS via `makeTheme(brand, mode)` + `<ThemeProvider>`/`useTheme()` | `gofi-ui-native` | `sdk/mobile/gofi-ui-native/foundations/tokens-mobile.md` |

> **Onde as formas divergem** (mesmo papel, realização diferente): cor de apoio
> = `accent` (web) vs `secondary` (mobile); espaçamento/duração são tokens **só no
> mobile** (web usa a escala/utilitários do Tailwind); as **sombras** do mobile são
> mais sutis que as do web; `--text-display` é 36 no web e 34 no mobile. Cada
> `tokens-*.md` traz a forma exata da sua superfície.

> Ao corrigir/evoluir um token, atualize **aqui primeiro** e propague conforme o
> [learning-protocol](../shared/learning-protocol.md). Nunca duplique valores nas
> surfaces — elas só **traduzem o formato**, não redefinem o valor.
</content>
</invoke>
