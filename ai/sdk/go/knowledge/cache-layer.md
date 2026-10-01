---
name: cache-layer
description: Cache fica no repository, nunca no service — WithCache por query, Get/Set para DTO composto, invalidação por Del do nome
type: feedback
sdk: v0.8.2
keywords: [cache, redis, WithCache, NewCache, invalidation, Del, repository, CACHE_URI]
---

# Cache — só no repository

Cache é detalhe de acesso a dados: vive **só no repository**. O service não
conhece Redis, chave, TTL nem invalidação — chama métodos do repo. API:
`.claude/sdk/go/api/sqln-cache.md` (`cache.Cache`) e
`gofi-component-cache.md`.

## Infra

- Redis compartilhado aberto pelo componente `cache.New()`
  (`gofi/component/cache`) a partir de `CACHE_*` (`CACHE_URI`,
  `CACHE_PASSWORD`, `CACHE_USE_TLS`); chaves prefixadas por `APP_NAME`.
- Falha de cache nunca falha a query: leitura com erro vira miss, escrita é
  best-effort. Misses simultâneos da mesma chave viram **uma** ida ao banco
  (singleflight).

## (a) Uma query — `.WithCache` inline

```go
const {ctx}ListCacheTTL = 5 * time.Minute

func {ctx}ListCache(tenantID string) *sqln.Cache[model.{Entidade}] {
    return sqln.NewCache[model.{Entidade}]("{ctx}:list:"+tenantID, {ctx}ListCacheTTL)
}

func (r *{ctx}Repository) FindByFilter(ctx context.Context, f model.{Ctx}Filter) (*sqln.Page[model.{Entidade}], error) {
    // ... criteria + page ...
    return sqln.FindFromCriteria[model.{Entidade}](ctx, q).
        WithCache({ctx}ListCache(f.TenantID)).
        WithPage(page).
        PagedList()
}
```

- Vale para `.List()`, `.UniqueResult()` e `.PagedList()` (a `Page` inteira
  é cacheada). O tipo do `NewCache` é o **tipo da linha**, não `Page[T]`.
  `.All()` ignora cache.
- **O `nome` é o escopo de invalidação, não a chave.** O SDK grava cada
  resultado sob `nome` + hash de SQL, argumentos, página e contagem — filtros
  e páginas diferentes nunca colidem. Não monte hash de filtro na mão.
- Escolha o `nome` pelo que muda junto: `"{ctx}:list:"+tenantID` invalida só
  o tenant afetado.

## (b) DTO composto de várias queries — `Get`/`Set` manual

```go
c := sqln.NewCache[model.{Ctx}Summary]("{ctx}:summary:"+id, ttl)
var out model.{Ctx}Summary
if hit, _ := c.Get(ctx, &out); hit {
    return &out, nil
}
// ... N queries + composição em memória ...
_ = c.Set(ctx, out) // best-effort
return &out, nil
```

`UniqueResult(ctx)`/`List(ctx)` do `Cache` também leem a entrada base.

## Invalidação — `Del` no nome

```go
// interface do repository
Invalidate{Ctx}ListCache(ctx context.Context, tenantID string) error

func (r *{ctx}Repository) Invalidate{Ctx}ListCache(ctx context.Context, tenantID string) error {
    return {ctx}ListCache(tenantID).Del(ctx) // entrada base + todas as entradas por query
}
```

O service chama o método depois de mutações que mudam a listagem. Sem
invalidação explícita, a consistência é pelo TTL (curto para filtros
arbitrários).

## Anti-padrões

- `sqln.InstanceRedis().Get/Set/Keys` + `json.Marshal` à mão.
- `KEYS pattern*` para invalidar — use `Cache.Del` do nome.
- Chave com hash de filtro/página montada à mão para `WithCache`.
- Cache, TTL ou Redis no service.
