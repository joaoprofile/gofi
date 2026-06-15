# Pattern — Os 4 estados (mobile)

Mesma regra do [web](../../../web/gofi-ui/patterns/states.md): **toda tela
com dados** entrega loading, empty, error, success.

| Estado | Mobile |
|--------|--------|
| **loading** | [Skeleton](../components/skeleton.md) no shape (ou `ActivityIndicator` p/ ação); nunca tela vazia |
| **empty** | [EmptyState](../components/empty-state.md) como `ListEmptyComponent`; distinga primeiro-uso × sem-resultado × tudo-em-dia |
| **error** | banner inline + **retry** (pull-to-refresh também); preserva contexto |
| **success** | dados (FlatList/conteúdo) |

## Mobile específico
- **Pull-to-refresh** (`RefreshControl`) em listas: recarrega sem perder posição.
- **Paginação infinita**: `onEndReached` + footer de loading; anuncie "carregando mais".
- Erro de rede comum no mobile → mensagem clara + retry; offline → banner de modo offline.

## Acessibilidade
- loading: `accessibilityState={{ busy:true }}`. error: anuncie a mensagem.
- empty: título header; CTA real. Retry alcançável por toque ≥ 44pt.

## Do / Don't
- ✅ Skeleton no shape + pull-to-refresh. ❌ Spinner de tela cheia onde cabe skeleton.
- ❌ Reaproveitar vazio para erro.
