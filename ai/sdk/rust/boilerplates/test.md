# Boilerplate — Teste (service_tests.rs)

```rust
use std::sync::Arc;

use super::model::{Entidade};
use super::repository::{Contexto}Repository;
use super::service::{{Contexto}Error, {Contexto}Service};

struct Fake(Option<{Entidade}>);

#[async_trait::async_trait]
impl {Contexto}Repository for Fake {
    async fn find_by_id(&self, _id: i64) -> Option<{Entidade}> {
        self.0.clone()
    }
}

// gofi:tests REQ-NN.2
#[tokio::test]
async fn recusa_quem_nao_existe() {
    let service = {Contexto}Service::new(Arc::new(Fake(None)));
    assert!(matches!(service.get(9).await, Err({Contexto}Error::NotFound(9))));
}
```
