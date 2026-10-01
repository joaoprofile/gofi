# Princípios de UX e regras universais

## Princípios de UX inegociáveis

Os cinco princípios em [expertise/ui-design/ux-principles.md](ux-principles.md)
são **operacionais**, não decorativos. Toda PR sua precisa demonstrar:

1. **Empatia > simpatia** — toda tela começa por "quem usa, em que
   contexto, com qual dor". Documentado no comentário inicial da page ou
   feature quando não-óbvio.
2. **Jornada do usuário** — mapeie o fluxo completo (entrada → ação → saída
   ou erro) **antes** do primeiro componente. Cada pain point precisa ter
   um caminho alternativo.
3. **Erros vs deslizes** — trate ambos. Erro consciente → validação clara
   com microcopy útil. Deslize inconsciente → confirmação para destrutivo,
   undo para reversível.
4. **Mobile-first** — comece o código e a discussão pelo mobile. Desktop é
   refinamento, não ponto de partida.
5. **Acessibilidade + branding juntos** — contraste mínimo 4.5:1 (WCAG
   2.2 AA), no máximo 2 typefaces, microcopy em PT-BR ("entrar", "sair",
   "salvar", nunca "logar"/"deslogar").

## Regras universais (cross-framework)

Aplicam-se em qualquer framework UI suportado:

- **Toda tela tem 4 estados** — loading, empty, error, success. Nunca
  entregar tela com só o caminho feliz.
- **Todo input tem label + erro + hint** quando aplicável. Placeholder
  **não substitui label**.
- **Foco visível** — nunca remova outline sem alternativa visível.
- **Contraste 4.5:1 mínimo** (WCAG 2.2 AA) — verifique com ferramenta,
  não no olho.
- **Mobile-first** — escreva os estilos base (mobile) primeiro, na **forma do DS**
  (utilitários responsivos se o DS for utilitário; media queries com os breakpoints do
  DS se for SCSS/CSS Modules; `StyleSheet` no RN). Expanda para telas maiores — desktop
  é refinamento.
- **Microcopy em PT-BR** — "entrar" não "logar", "sair" não "deslogar",
  "salvar" não "submit". Tom direto, sem jargão.
- **Mutação destrutiva exige confirmação ou undo** (nunca ambos ausentes).
- **`prefers-reduced-motion`** — animações > 200ms respeitam o preference.
- **Nunca prop drilling > 2 níveis** — extraia hook/serviço/contexto.
- **Sem CSS inline para layout** — só `style={}` para valores
  computados em runtime (ex: posição de tooltip).
- **Sem fetch direto em componente** — passa pela **camada de data configurada**
  (`state` do `.gofi.yaml` + o que o DS/knowledge documenta: ex. TanStack Query,
  hooks de axios, etc.). Não presuma a lib de dados.

Os tokens e o design system da superfície são a fonte da verdade visual: o
**manifesto do DS configurado** — o `.md` na raiz de `.claude/sdk/<surface>/<ds>/`
(descoberto via `.claude/sdk/<surface>/INDEX.md`), com `<ds>` vindo do `.gofi.yaml`.
Regras framework-specific (ex: nunca `any`, nunca `useEffect` para derivar estado),
quando existirem, em `.claude/sdk/<surface>/knowledge/absolute-rules.md`.

> **Mobile (condicional).** Só quando o `.gofi.yaml` **configurar uma superfície
> mobile** (`framework: react-native`/`expo`, ou sub-bloco `ui.mobile` com seu `ds`):
> leia **também** o DS mobile em `.claude/sdk/mobile/<ds>/` — do mesmo modo
> config-driven (manifesto na raiz, foundations/components/patterns), aplicando a
> **forma mobile** (ex.: `makeTheme`/`useTheme`, `StyleSheet`). Sem superfície mobile
> configurada, ignore — não crie nem procure DS mobile por conta própria.
