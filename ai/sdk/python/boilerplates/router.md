# Boilerplate — Router (FastAPI)

```python
from fastapi import APIRouter, Depends, HTTPException

from app.{contexto}.dependencies import get_service
from app.{contexto}.schemas import {Entidade}Out
from app.{contexto}.service import {Contexto}Service, {Entidade}NotFoundError

router = APIRouter(prefix="/{rota}", tags=["{contexto}"])


@router.get("/{id}", response_model={Entidade}Out)
def get(id: int, service: {Contexto}Service = Depends(get_service)) -> {Entidade}Out:
    try:
        return service.get(id)
    except {Entidade}NotFoundError as err:
        raise HTTPException(status_code=404, detail=str(err)) from err
```
