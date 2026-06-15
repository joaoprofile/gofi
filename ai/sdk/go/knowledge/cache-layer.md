---
name: cache-layer
description: Cache deve ficar na camada de repository, nunca no service — padrão gofi
type: feedback
---

Cache é responsabilidade exclusiva da camada de **repository**. O service não conhece Redis, chaves de cache, TTL ou invalidação.

**Why:** Separação de concerns — cache é detalhe de infraestrutura de acesso a dados, não regra de negócio. O service opera sobre o repositório como abstração pura.

**How to apply:**

- `sqln.NewCache[T]` + `.WithCache(cache)` ficam em `FindByFilter` do repository
- `InvalidateListCache(ctx, tenantID)` é método da interface do repository, chamado pelo service após mutations
- `sqln.InstanceRedis()` só é acessado dentro do repository — nunca no service
- `cacheTTL` é constante de pacote no repository

```go
// repository — correto
func (r *repo) FindByFilter(ctx context.Context, f model.Filter) (model.Response, error) {
    // ... build criteria ...
    cacheKey := fmt.Sprintf("entity:%s:list:%s:%d:%d", f.TenantID, f.Name, f.Page, f.Limit)
    cache := sqln.NewCache[model.Entity](cacheKey, cacheTTL)
    return sqln.FindFromCriteria[model.Entity](ctx, q).
        WithCache(cache).WithPage(page).PagedList()
}

func (r *repo) InvalidateListCache(ctx context.Context, tenantID string) {
    pattern := fmt.Sprintf("*entity:%s:list:*", tenantID)
    keys, _ := sqln.InstanceRedis().Keys(ctx, pattern).Result()
    if len(keys) > 0 {
        sqln.InstanceRedis().Del(ctx, keys...)
    }
}

// service — correto: sem menção a cache
func (s *svc) Create(ctx context.Context, req model.CreateRequest) errs.AppError {
    // ... validação e save ...
    s.repo.InvalidateListCache(ctx, req.TenantID)
    return errs.AppError{}
}
```
