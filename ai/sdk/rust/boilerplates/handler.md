# Boilerplate — Handler (Axum)

```rust
use std::sync::Arc;

use axum::{extract::{Path, State}, http::StatusCode, response::{IntoResponse, Response}, Json};

use super::dto::{Entidade}Response;
use super::service::{{Contexto}Error, {Contexto}Service};

impl IntoResponse for {Contexto}Error {
    fn into_response(self) -> Response {
        let status = match self {
            {Contexto}Error::NotFound(_) => StatusCode::NOT_FOUND,
        };
        (status, self.to_string()).into_response()
    }
}

pub async fn get(State(service): State<Arc<{Contexto}Service>>, Path(id): Path<i64>) -> Result<Json<{Entidade}Response>, {Contexto}Error> {
    Ok(Json(service.get(id).await?.into()))
}
```
