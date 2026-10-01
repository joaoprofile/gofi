# Resolução do DS e bloco de UI no `.gofi.yaml`

Implementa no(s) framework(s) declarado(s) no `.gofi.yaml`. **O DS é sempre o que
estiver configurado no `.gofi.yaml` — a skill nunca fixa um nome de DS.** Você lê o
nome, resolve a pasta e segue os docs dela.

## Resolução do DS (a partir do `.gofi.yaml`, nunca chumbado na skill)

O bloco de UI declara a chave **`ds`** = **nome da pasta do DS**. O placeholder
**`<ds>`** designa esse valor; a **superfície** vem do `framework`:

| `framework` | Superfície (`<surface>`) | Pasta do DS |
|-------------|--------------------------|-------------|
| `react` (`angular`/`vue`) | **web** | `.claude/sdk/web/<ds>/` |
| `react-native` / `expo` | **mobile** | `.claude/sdk/mobile/<ds>/` |

- `<ds>` = valor de `frontend.ds`/`ui.ds` (superfície única) ou `ui.<surface>.ds`
  (multi-superfície). **A skill não presume nenhum nome.** Se `ds` não estiver no
  `.gofi.yaml`, **pare e peça** ao usuário para configurá-lo.
- **A forma (estilo/estado/teste/import) vem do config + dos docs do DS — a skill
  não assume Tailwind, npm, SCSS Modules nem nenhuma stack.** Pode ser Tailwind +
  utilitários, SCSS Modules + tokens, `makeTheme`/`useTheme` (mobile), etc. Leia
  `styling`/`state`/`testing` e o **manifesto do DS** antes de codificar.
- O DS pode ser **lib npm** (`import { Button } from '<ds>'`) **ou componentes no
  próprio repo** (importe do path do app, ex. `~/components/...`) — **quem manda é o
  manifesto/docs do DS**. Em ambos os casos você **compõe** com o que o DS expõe e
  **não o recria**; os docs são a especificação, não código a copiar.

> **Um projeto pode ter uma superfície ou as duas.** Se houver `ui.web`/`ui.mobile`,
> processe cada uma como superfície-alvo independente e leia **o DS de cada uma**
> (`.claude/sdk/<surface>/<ds>/`, com o `<ds>` de cada sub-bloco). **Nunca** aplique o
> DS/forma de uma superfície na outra.

## Bloco de UI no `.gofi.yaml`

O bloco pode se chamar **`frontend:`** (comum em full-stack) ou **`ui:`** — leia o que
existir. **Os valores abaixo são exemplos: leia os reais do projeto, não presuma.**

**Uma superfície:**

```yaml
frontend:                 # ou `ui:`
  framework: react        # react|angular|vue → web · react-native|expo → mobile
  path: frontend          # raiz do app de UI
  ds: <ds>                # NOME DA PASTA DO DS (.claude/sdk/<surface>/<ds>/) — OBRIGATÓRIO
  styling: <styling>      # ex.: tailwind | scss-modules | stylesheet — NÃO presuma
  state: <state>          # ex.: tanstack-query | axios-hooks | redux
  testing: <testing>      # ex.: jest | vitest
  brand: <brand>          # UMA string: preset (blue|violet|green) ou a cor-semente do projeto ("#AAD7FF") — omitir = neutro
```

**Duas superfícies** (sub-blocos `web:`/`mobile:`, cada um com o seu `ds`/`framework`/`path`):

```yaml
ui:
  web:    { framework: react,        path: apps/web,    ds: <ds-web> }
  mobile: { framework: react-native, path: apps/mobile, ds: <ds-mobile> }
```

**Além dessas duas** — um back office, um console de admin — o projeto usa
`surfaces:`, com os mesmos campos sob o nome que ele deu:

```yaml
surfaces:
  backoffice: { framework: react, path: frontend/backoffice, ds: <ds> }
```

Regra de leitura: se existir `ui.web`/`ui.mobile`, processe **cada** sub-bloco como
superfície-alvo independente (com o seu `ds`); senão é superfície única (derive pelo
`framework`). Cada entrada de `surfaces:` é mais uma superfície-alvo independente. **Em todos os casos, `<ds>` vem do config — a skill nunca fixa o nome,
nem a stack.** Frameworks suportados: **React + TS** (web) e **React Native** (mobile);
a **forma** (utilitários, SCSS Modules, `makeTheme`/`useTheme`, etc.) e os componentes
são os que o **DS configurado** documenta.

## Escopo e postura

Você **não escreve código fora do escopo da spec** e **não inventa regras**
que não estejam documentadas. Quando faltar contexto, pergunte antes de
codificar.

UX **não é decoração** — é o produto. Toda tela passa pelos cinco
princípios em [expertise/ui-design/ux-principles.md](../expertise/ui-design/ux-principles.md)
antes de ser dada como pronta.
