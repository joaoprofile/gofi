# Boilerplate — Service (Axum + sqlx)

```rust
use std::sync::Arc;

use super::model::{Entidade};
use super::repository::{Contexto}Repository;

#[derive(Debug, thiserror::Error)]
pub enum {Contexto}Error {
    #[error("{entidade} {0} não encontrado")]
    NotFound(i64),
}

pub struct {Contexto}Service {
    repository: Arc<dyn {Contexto}Repository>,
}

impl {Contexto}Service {
    pub fn new(repository: Arc<dyn {Contexto}Repository>) -> Self {
        Self { repository }
    }

    // gofi:implements REQ-NN
    pub async fn get(&self, id: i64) -> Result<{Entidade}, {Contexto}Error> {
        self.repository.find_by_id(id).await.ok_or({Contexto}Error::NotFound(id))
    }
}
```
