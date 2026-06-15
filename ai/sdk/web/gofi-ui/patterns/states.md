# Pattern — Os 4 estados (obrigatório)

**Toda tela com dados** entrega os quatro. Entregar só o "happy path" é um
bug, não simplificação. (Princípios 1–3 de
[ux-principles.md](../../../../knowledge/ui/ux-principles.md).)

## Os estados
| Estado | Regra | Componente |
|-------|------|-----------|
| **loading** | nunca uma tela em branco; skeleton no formato do conteúdo (spinner só para uma ação pontual) | [Skeleton/Spinner](../components/skeleton-spinner.md) |
| **empty** | ilustração + microcopy + CTA; distinguir primeiro uso × busca-sem-resultado × tudo-resolvido | [EmptyState](../components/empty-state.md) |
| **error** | mensagem útil (o que + o que fazer) + **tentar de novo** quando fizer sentido; não mostrar como empty | [Banner](../components/toast-banner.md) |
| **success** | o estado normal com dados | — |

## Decisão
```
loading? ──────────────────► loading (skeleton)
erro técnico? ─────────────► error (banner + tentar de novo)
sem dados?
  ├─ nunca existiu ────────► empty: primeiro-uso (CTA de criar)
  ├─ filtro/busca ─────────► empty: sem-resultado (limpar filtro)
  └─ tudo concluído ───────► empty: tudo-em-dia (tom positivo)
caso contrário ────────────► success (dados)
```

## Erros: negócio × técnico
- **Negócio** (ex.: validação, permissão): uma mensagem clara inline/Banner, sem
  tentar de novo às cegas — explique a regra.
- **Técnico** (rede/500): Banner + **tentar de novo**, preservando o contexto do usuário.

## Acessibilidade
- loading: `aria-busy="true"`, skeleton `aria-hidden`.
- error: `role="alert"` na mensagem; foco/anúncio apropriados.
- empty: o título é um heading; o CTA é um controle real.

## Do / Don't
- ✅ Cada estado tem sua própria microcopy pensada.
- ❌ Reusar o empty para erros. ❌ Um spinner em tela cheia onde cabe um skeleton.
- ❌ "Algo deu errado" sem uma ação ou causa quando poderia ser específico.
