# Pattern — Navegação mobile

React Navigation. Combina **stack** (profundidade) e **tabs** (destinos de topo).

## Estruturas
| Estrutura | Uso |
|-----------|-----|
| **Bottom Tabs** | 3–5 destinos de topo ([Tab Bar](../components/tab-bar.md)) |
| **Native Stack** | empilhar telas (push/pop), swipe-back nativo |
| **Modal stack** | apresentação modal (sobe de baixo) |
| **Drawer** | menu lateral (apps com muitas seções) — use com parcimônia |

Padrão comum: Tabs na raiz, cada aba com seu Stack.

## Regras
- **Voltar** sempre claro: [Header](../components/header.md) com ‹ + gesto swipe-back.
- Estado de cada aba persiste; voltar preserva scroll/posição da lista.
- Deep links / estado navegável quando fizer sentido.
- Transições seguem a plataforma; respeitam reduce-motion.
- Telas pesadas: lazy + skeleton.

## Acessibilidade
- Header voltar rotulado ("Voltar"); tabs com `accessibilityRole="tab"` + selected.
- Foco/anúncio ao trocar de tela (leitor anuncia o novo título).
- Respeita safe area em topo e base ([safe-area.md](safe-area.md)).

## Do / Don't
- ✅ Tabs para destinos; Stack para profundidade.
- ❌ Esconder navegação primária em drawer quando bottom tabs resolve.
- ❌ Perder trabalho não salvo ao voltar sem avisar (princípio 3).
