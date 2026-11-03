# Boilerplate — Teste (Jest)

```ts
import { {Contexto}Service, {Entidade}NotFoundError } from './{contexto}.service';

describe('{Contexto}Service', () => {
  const repository = { findById: jest.fn() };
  const service = new {Contexto}Service(repository as never);

  beforeEach(() => jest.resetAllMocks());

  // gofi:tests REQ-NN.1
  it('REQ-NN.1 — <título do cenário>', async () => {
    repository.findById.mockResolvedValue({ id: 1 });
    await expect(service.get(1)).resolves.toEqual({ id: 1 });
  });

  // gofi:tests REQ-NN.2
  it('REQ-NN.2 — <título do cenário de recusa>', async () => {
    repository.findById.mockResolvedValue(null);
    await expect(service.get(9)).rejects.toBeInstanceOf({Entidade}NotFoundError);
  });
});
```
