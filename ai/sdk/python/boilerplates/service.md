# Boilerplate — Service (FastAPI)

```python
from app.{contexto}.repository import {Contexto}Repository
from app.{contexto}.schemas import {Entidade}Out


class {Entidade}NotFoundError(Exception):
    pass


class {Contexto}Service:
    def __init__(self, repository: {Contexto}Repository) -> None:
        self._repository = repository

    # gofi:implements REQ-NN
    def get(self, id: int) -> {Entidade}Out:
        found = self._repository.find_by_id(id)
        if found is None:
            raise {Entidade}NotFoundError(f"{entidade} {id} não encontrado")
        return {Entidade}Out.model_validate(found, from_attributes=True)
```
