# Pattern — App Shell

O esqueleto persistente de uma aplicação web autenticada: uma **sidebar** para navegação +
uma **top bar** + uma área de conteúdo (referência visual fiel — o dashboard do
"Portal do Aluno").

## Anatomia
```
┌───────────┬─────────────────────────────────────────────┐
│           │  top bar: [busca] ............ [🔔] [avatar]  │
│  sidebar  ├─────────────────────────────────────────────┤
│  (nav)    │                                             │
│  · item   │   conteúdo da página                        │
│  · item●  │   (PageHeader + corpo)                      │
│  · item   │                                             │
│  config   │                                             │
└───────────┴─────────────────────────────────────────────┘
```

## Sidebar
- Itens com ícone + label; o item **ativo** em uma pill `--action` (texto
  branco), `aria-current="page"`. Agrupar por seção quando necessário; o item de
  "configurações" ancorado na base.
- Recolhível (somente ícones) em telas médias; vira um **drawer** off-canvas no
  mobile (gatilho hamburger na top bar).
- `<nav aria-label="Main">`, uma lista de links reais.

## Top bar
- Busca global (input com ícone), notificações (badge numérico acessível), avatar do
  usuário com um [Menu](../components/menu-popover.md) (perfil, sair).
- Fixa (`--z-sticky`); esmaece/condensa ao rolar (opcional).

## Layout
```css
.shell { display: grid; grid-template-columns: 1fr; min-height: 100dvh; }
@media (min-width: 1024px) {
  .shell { grid-template-columns: 264px 1fr; } /* sidebar fixa no desktop */
}
```
Conteúdo em um `Container` (max-width, padding por breakpoint).

## Responsivo (mobile-first)
| Largura | Navegação |
|-------|------------|
| base (mobile) | sidebar vira drawer; considere um bottom nav para 3–5 destinos |
| `md` (tablet) | sidebar recolhida (ícones) |
| `lg` (desktop) | sidebar expandida e fixa |

## Acessibilidade
- Landmarks: `<header>` (top bar), `<nav>` (sidebar), `<main>` (conteúdo).
- "Pular para o conteúdo" (skip link) como primeiro foco.
- Item ativo com `aria-current`; drawer mobile com focus trap ao abrir.

## Do / Don't
- ✅ Estado ativo claro (pill preenchida), não apenas a cor.
- ✅ Persistir recolher/expandir por usuário.
- ❌ Esconder a navegação primária atrás de um menu no desktop. ❌ `100vh` (use `100dvh`).
