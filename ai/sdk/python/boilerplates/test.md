# Boilerplate — Teste (pytest)

```python
import pytest

from app.{contexto}.service import {Contexto}Service, {Entidade}NotFoundError


class FakeRepository:
    def __init__(self, found: object | None) -> None:
        self.found = found

    def find_by_id(self, id: int) -> object | None:
        return self.found


# gofi:tests REQ-NN.1
def test_req_nn_1_encontra() -> None:
    service = {Contexto}Service(FakeRepository({"id": 1}))  # type: ignore[arg-type]
    assert service.get(1).id == 1


# gofi:tests REQ-NN.2
def test_req_nn_2_recusa_quem_nao_existe() -> None:
    with pytest.raises({Entidade}NotFoundError):
        {Contexto}Service(FakeRepository(None)).get(9)  # type: ignore[arg-type]
```
